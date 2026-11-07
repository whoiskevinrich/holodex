package maintenance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"holodex/internal/model"
	"holodex/internal/rename"
	"holodex/internal/repo"
)

// TrimHooks are the trim's reach outside this package. Each may be nil in a test.
type TrimHooks struct {
	CaptureRoot    string                                              // DATA_PATH/maintenance
	MediaRoot      string                                              // the library bound for an edition's landing
	CopyOwnerSlots func(ctx context.Context, srcID, dstID int64) error // the edition's poster files
	EnqueueThumb   func(id int64)                                      // generate a frame thumbnail now
}

// SetTrimHooks wires what a trim needs beyond a flip.
func (w *Worker) SetTrimHooks(h TrimHooks) { w.trimHooks = h }

// CaptureRoot is the folder holding queued trims' screen captures.
func (w *Worker) CaptureRoot() string { return w.trimHooks.CaptureRoot }

// SkippedCheck records one frame the screen check couldn't run on, and why (Skip*).
type SkippedCheck struct {
	End    string `json:"end"` // "first" or "last"
	Reason string `json:"reason"`
}

// captureFiles are where a trim's two screen captures live; "" when the job carries none.
func (w *Worker) captureFiles(t *TrimParams) (in, out string) {
	if t.Captures == "" || w.trimHooks.CaptureRoot == "" {
		return "", ""
	}
	dir := filepath.Join(w.trimHooks.CaptureRoot, t.Captures)
	return filepath.Join(dir, "in.jpg"), filepath.Join(dir, "out.jpg")
}

// removeCaptures deletes a finished trim's captures. A failed trim keeps them for Retry; dismissing
// it, or a sweep at boot, removes them.
func (w *Worker) removeCaptures(t *TrimParams) {
	if t.Captures != "" && w.trimHooks.CaptureRoot != "" {
		os.RemoveAll(filepath.Join(w.trimHooks.CaptureRoot, t.Captures))
	}
}

// SweepCaptures removes capture folders no queued or failed job names: a crash, or a new trim that
// replaced a failed one. Run at boot, before the worker starts.
func (w *Worker) SweepCaptures(ctx context.Context) error {
	root := w.trimHooks.CaptureRoot
	if root == "" {
		return nil
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	payloads, err := w.repo.MaintenancePayloads(ctx)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, raw := range payloads {
		var p Payload
		if json.Unmarshal([]byte(raw), &p) == nil && p.Trim != nil && p.Trim.Captures != "" {
			keep[p.Trim.Captures] = true
		}
	}
	for _, e := range entries {
		if e.IsDir() && !keep[e.Name()] {
			os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
	return nil
}

// runTrimJob runs one trim end to end against the video's current file. The path and the edition's
// target are read here, not at enqueue: a rename may have landed while the job waited.
func (w *Worker) runTrimJob(ctx context.Context, job *repo.WritebackJob, t *TrimParams) error {
	if t == nil {
		return errors.New("trim job carries no range")
	}
	v, _, err := w.repo.GetVideo(ctx, job.VideoID)
	if err != nil {
		return fmt.Errorf("video %d: %w", job.VideoID, err)
	}
	path := v.FilePath
	family, err := familyOf(path)
	if err != nil {
		return err
	}

	// A crash after an edition's file landed: finish the clone, or start again if the file is gone.
	if t.Stage == StageLanded {
		if ok, err := exists(t.Target); err != nil {
			return err
		} else if ok {
			if err := w.finishEdition(ctx, job.VideoID, t, nil); err != nil {
				return err
			}
			w.removeCaptures(t)
			return nil
		}
		t.Stage, t.Target = "", ""
	}

	var skipped []SkippedCheck
	switch t.Mode {
	case TrimReplace:
		if skipped, err = w.runTrim(ctx, family, path, NewPath(path), t, ""); err != nil {
			return err
		}
		if err := w.swap.Swap(ctx, job.VideoID, path, string(OpTrim)); err != nil {
			os.Remove(NewPath(path))
			return err
		}
		w.afterFileChange(ctx, job.VideoID, path, v.ThumbnailState)
		if err := w.recordTrim(ctx, repo.TrimResult{VideoID: job.VideoID, Mode: TrimReplace}, skipped); err != nil {
			return err
		}
	case TrimEdition:
		target, name, ok := EditionTarget(path, t.Edition)
		if !ok {
			return refuse(ReasonEditionName, "")
		}
		if err := w.targetFree(ctx, target); err != nil {
			return err
		}
		dst := NewPath(target)
		if skipped, err = w.runTrim(ctx, family, path, dst, t, name); err != nil {
			return err
		}
		t.Edition, t.Target, t.Stage = name, target, StageLanded
		payload, _ := json.Marshal(Payload{Op: string(OpTrim), Trim: t})
		if err := w.repo.UpdateMaintenancePayload(ctx, job.ID, string(payload)); err != nil {
			os.Remove(dst)
			return err
		}
		if err := w.landEdition(dst, target); err != nil {
			os.Remove(dst)
			// Nothing landed. Left as landed, a Retry would take whatever holds that name for this
			// trim's file and make a library item of it.
			t.Stage, t.Target = "", ""
			payload, _ := json.Marshal(Payload{Op: string(OpTrim), Trim: t})
			if jerr := w.repo.UpdateMaintenancePayload(ctx, job.ID, string(payload)); jerr != nil {
				w.log.Warn("clear trim journal", "id", job.ID, "err", jerr)
			}
			return err
		}
		if err := w.finishEdition(ctx, job.VideoID, t, skipped); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown trim mode %q", t.Mode)
	}
	w.removeCaptures(t)
	return nil
}

// targetFree refuses a name another file or library row already holds.
func (w *Worker) targetFree(ctx context.Context, target string) error {
	if _, err := os.Lstat(target); err == nil {
		return refuse(ReasonTargetExists, filepath.Base(target))
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	taken, err := w.repo.PathInLibrary(ctx, target)
	if err != nil {
		return err
	}
	if taken {
		return refuse(ReasonTargetExists, filepath.Base(target))
	}
	return nil
}

// landEdition moves the verified file to its name without ever replacing what's there. A file that
// took the name since the check stays, and this trim fails.
func (w *Worker) landEdition(dst, target string) error {
	dir := filepath.Dir(target)
	root := w.trimHooks.MediaRoot
	if root == "" {
		root = dir
	}
	folder, err := rename.OpenFolder(root, dir)
	if err != nil {
		return err
	}
	defer folder.Close()
	// The working name's suffix means this is never a case-only rename, so no half-way step happens.
	noJournal := func(string) error { return nil }
	if err := folder.Move(0, filepath.Base(dst), filepath.Base(target), noJournal); err != nil {
		if errors.Is(err, rename.ErrExists) {
			return refuse(ReasonTargetExists, filepath.Base(target))
		}
		return err
	}
	return nil
}

// finishEdition does what a landed edition still needs. Every step can run again after a crash.
// skipped is nil on a recovery: that run's outcome notice is gone with the process, so it records
// only that the edition exists.
func (w *Worker) finishEdition(ctx context.Context, srcID int64, t *TrimParams, skipped []SkippedCheck) error {
	newID, err := w.repo.CloneVideoAsEdition(ctx, srcID, t.Target, t.Edition)
	if err != nil {
		return err
	}
	if w.trimHooks.CopyOwnerSlots != nil {
		if err := w.trimHooks.CopyOwnerSlots(ctx, srcID, newID); err != nil {
			return err
		}
	}
	run := model.JobRun{Kind: model.JobKindMaintenance, Trigger: model.TriggerManual, Status: model.JobStatusOK,
		StartedAt: time.Now(), FinishedAt: time.Now(), Updated: 1, Detail: OpTrimEdition + " from " + strconv.FormatInt(srcID, 10),
		EntityType: model.EnrichEntityVideo, EntityID: newID}
	if err := w.repo.RecordJobRun(ctx, run); err != nil {
		w.log.Warn("record edition job run", "video", newID, "err", err)
	}
	if err := w.recordTrim(ctx, repo.TrimResult{VideoID: srcID, Mode: TrimEdition, EditionName: t.Edition, EditionVideoID: &newID}, skipped); err != nil {
		return err
	}
	if w.postWrite != nil {
		w.postWrite(ctx, newID, t.Target)
	}
	if w.trimHooks.EnqueueThumb != nil {
		w.trimHooks.EnqueueThumb(newID)
	}
	return nil
}

// recordTrim stores the outcome the page shows until the owner dismisses it.
func (w *Worker) recordTrim(ctx context.Context, res repo.TrimResult, skipped []SkippedCheck) error {
	if skipped == nil {
		skipped = []SkippedCheck{}
	}
	b, err := json.Marshal(skipped)
	if err != nil {
		return err
	}
	res.Skipped = string(b)
	return w.repo.SetTrimResult(ctx, res)
}

// afterFileChange refreshes the row from a replaced file and queues a new frame thumbnail if the
// old one came from a frame. An owner's poster stays, whatever frame it came from.
func (w *Worker) afterFileChange(ctx context.Context, id int64, path, thumbState string) {
	if w.postWrite != nil {
		w.postWrite(ctx, id, path)
	}
	if thumbState == model.ThumbnailGenerated {
		if err := w.repo.ResetThumbnailState(ctx, id); err != nil {
			w.log.Warn("reset thumbnail after trim", "video", id, "err", err)
			return
		}
		if w.trimHooks.EnqueueThumb != nil {
			w.trimHooks.EnqueueThumb(id)
		}
	}
}

// trim cuts path into dst and proves the result: the screen check before any encode, then the gate
// and the cut check on the output. One correction is allowed across both. On failure dst is gone.
func (w *Worker) trim(ctx context.Context, family, path, dst string, t *TrimParams, edition string) ([]SkippedCheck, error) {
	demux := Demuxer(family)
	p, err := RunProbe(ctx, w.tools.FFprobe, demux, path)
	if err != nil {
		return nil, err
	}
	orig, err := CollectSnapshot(ctx, w.tools, family, path, p)
	if err != nil {
		return nil, err
	}
	const window = 10.0
	inW, err := listFrames(ctx, w.tools, family, path, p, t.InTime, window)
	if err != nil {
		return nil, err
	}
	outW, err := listFrames(ctx, w.tools, family, path, p, t.OutTime, window)
	if err != nil {
		return nil, err
	}
	if len(inW) == 0 || len(outW) == 0 {
		return nil, errors.New("no video frames at the chosen times")
	}
	s := trimState{p: p, orig: orig, inW: inW, outW: outW, inAt: inW.nearest(t.InTime), outAt: outW.nearest(t.OutTime)}
	if s.in, s.out, err = w.loadCaptures(ctx, t); err != nil {
		return nil, err
	}

	err = checkedCut(
		func() (bool, error, error) {
			moved, err := w.screenCheck(ctx, family, path, &s)
			return moved, s.screenFailure, err
		},
		func() ([2]int, error) { return w.cutOnce(ctx, family, path, dst, &s, edition) },
		func(off [2]int) {
			os.Remove(dst)
			s.moveCut(off)
		})
	if err != nil {
		return nil, err
	}
	return s.skipped, nil
}

// checkedCut runs the screen check, then the cut and its check, allowing one correction across
// both. The screen check runs first because a disagreement there costs no encode. A second
// disagreement of either kind fails, naming the check and the offset.
func checkedCut(screen func() (moved bool, failure, err error), cut func() ([2]int, error), moveCut func([2]int)) error {
	for corrected := false; ; corrected = true {
		moved, failure, err := screen()
		if err != nil {
			return err
		}
		if moved {
			if corrected {
				return failure
			}
			continue
		}
		off, err := cut()
		if err != nil {
			return err
		}
		if off == [2]int{} {
			return nil
		}
		if corrected {
			return cutFailure(off)
		}
		moveCut(off)
	}
}

// trimState is one trim's frames and how far correction has moved them.
type trimState struct {
	p             Probe
	orig          Snapshot
	inW, outW     frameWindow
	inAt, outAt   int // the frames to keep, as indexes into their windows
	inCut, outCut int // how far the cut itself sits from them, after a cut-check correction
	in, out       grey
	skipped       []SkippedCheck
	screenFailure error
}

func (w *Worker) loadCaptures(ctx context.Context, t *TrimParams) (in, out grey, err error) {
	inFile, outFile := w.captureFiles(t)
	if inFile == "" {
		return nil, nil, nil
	}
	if in, err = decodeCapture(ctx, w.tools, inFile); err != nil {
		return nil, nil, err
	}
	if out, err = decodeCapture(ctx, w.tools, outFile); err != nil {
		return nil, nil, err
	}
	return in, out, nil
}

// screenCheck compares each capture with the file's frames around it. It runs before any encode, so
// a browser disagreeing about which frame a time names costs no encoding at all. corrected reports
// that the chosen frames moved and the check must run again; screenFailure then holds the error a
// second disagreement fails with. The check runs again from scratch, so skips reflect the frames
// finally kept.
func (w *Worker) screenCheck(ctx context.Context, family, path string, s *trimState) (corrected bool, err error) {
	s.skipped = nil
	ends := []struct {
		name    string
		capture grey
		window  frameWindow
		at      *int
	}{{"first", s.in, s.inW, &s.inAt}, {"last", s.out, s.outW, &s.outAt}}
	for i, e := range ends {
		if e.capture == nil {
			continue
		}
		refs, err := loadReferences(ctx, w.tools, family, path, s.p, e.window, *e.at)
		if err != nil {
			return false, err
		}
		if reason := screenSkipReason(refs); reason != "" {
			s.skipped = append(s.skipped, SkippedCheck{End: e.name, Reason: reason})
			continue
		}
		off := screenOffset(e.capture, refs)
		if off == screenNoMatch {
			return false, fail(RuleScreenCheck, "%s:no_match", e.name)
		}
		if off != 0 {
			moved := *e.at + off
			if moved < 0 || moved >= len(e.window) {
				return false, fail(RuleScreenCheck, "%s:%d", e.name, abs(off))
			}
			*e.at = moved
			corrected = true
			s.screenFailure = fail(RuleScreenCheck, "%s:%d", ends[i].name, abs(off))
		}
	}
	return corrected, nil
}

// cutOnce encodes the range, runs the gate, and returns how far the output's first and last frames
// sit from the chosen ones. A gate failure is final: another cut wouldn't change it.
func (w *Worker) cutOnce(ctx context.Context, family, path, dst string, s *trimState, edition string) ([2]int, error) {
	var off [2]int
	r := s.currentRange()
	if s.inW[s.inAt] >= s.outW[s.outAt] {
		return off, errors.New("the first frame kept must come before the last")
	}
	if !r.HasSeek && !r.HasStop {
		return off, errors.New("the range keeps the whole file, so there is nothing to cut")
	}
	fileDur, _ := strconv.ParseFloat(s.p.Format.Duration, 64)
	dur := r.keptDuration(fileDur)
	chapters := ExpectedChapters(s.p.Chapters, r.InTime, dur)
	if err := RebuildTrim(ctx, w.tools, family, w.encoders, path, dst, s.p, r, chapters, edition); err != nil {
		return off, err
	}
	keep := false
	defer func() {
		if !keep {
			os.Remove(dst)
		}
	}()
	cp, err := RunProbe(ctx, w.tools.FFprobe, Demuxer(family), dst)
	if err != nil {
		return off, err
	}
	cand, err := CollectSnapshot(ctx, w.tools, family, dst, cp)
	if err != nil {
		return off, err
	}
	want := TrimExpect{Chapters: chapters, Edition: edition, WantSpan: r.OutTime - r.InTime, CopiedOK: map[int]string{}}
	first, last, err := w.outputFrames(ctx, family, dst, cp)
	if err != nil {
		return off, err
	}
	want.VideoSpan = last.time - first.time
	if err := w.comparePacketStreams(ctx, family, path, dst, s.p, cp, r, fileDur, &want); err != nil {
		return off, err
	}
	if err := VerifyTrim(s.orig, cand, want); err != nil {
		return off, err
	}
	inRefs, err := loadReferences(ctx, w.tools, family, path, s.p, s.inW, s.inAt)
	if err != nil {
		return off, err
	}
	outRefs, err := loadReferences(ctx, w.tools, family, path, s.p, s.outW, s.outAt)
	if err != nil {
		return off, err
	}
	off = [2]int{cutOffset(first.frame, inRefs), cutOffset(last.frame, outRefs)}
	keep = off == [2]int{}
	return off, nil
}

// moveCut corrects the cut after the cut check measured off. The output held the wrong frames, so the
// cut moves the other way. The frames the owner chose don't change.
func (s *trimState) moveCut(off [2]int) {
	s.inCut -= off[0]
	s.outCut -= off[1]
}

func (s *trimState) currentRange() trimRange {
	r := trimRange{InTime: s.inW[s.inAt], OutTime: s.outW[s.outAt]}
	r.Seek, r.HasSeek = s.inW.seekBefore(s.inAt + s.inCut)
	r.Stop, r.HasStop = s.outW.stopAfter(s.outAt + s.outCut)
	return r
}

type outputFrame struct {
	time  float64
	frame grey
}

// outputFrames decodes the output's first and last frames.
func (w *Worker) outputFrames(ctx context.Context, family, path string, p Probe) (first, last outputFrame, err error) {
	dur, _ := strconv.ParseFloat(p.Format.Duration, 64)
	head, err := listFrames(ctx, w.tools, family, path, p, 0, 5)
	if err != nil {
		return first, last, err
	}
	tail, err := listFrames(ctx, w.tools, family, path, p, dur, 10)
	if err != nil {
		return first, last, err
	}
	if len(head) == 0 || len(tail) == 0 {
		return first, last, errors.New("the new file has no video frames")
	}
	frames, err := decodeGrey(ctx, w.tools, family, path, p, 0, false, 1)
	if err != nil {
		return first, last, err
	}
	first = outputFrame{time: head[0], frame: frames[0]}
	seek, hasSeek := tail.seekBefore(len(tail) - 1)
	frames, err = decodeGrey(ctx, w.tools, family, path, p, seek, hasSeek, 1)
	if err != nil {
		return first, last, err
	}
	last = outputFrame{time: tail[len(tail)-1], frame: frames[0]}
	return first, last, nil
}

// comparePacketStreams fills want.CopiedOK for every timeline stream the output kept. Streams are
// matched by position, because dropping an MP4's chapter track can renumber what follows.
func (w *Worker) comparePacketStreams(ctx context.Context, family, path, dst string, op, cp Probe, r trimRange, fileDur float64, want *TrimExpect) error {
	ostreams, cstreams := withoutChapterTrack(op.Streams), withoutChapterTrack(cp.Streams)
	if len(ostreams) != len(cstreams) {
		return nil // VerifyTrim reports the difference
	}
	v, _ := op.MainVideo()
	end := fileDur
	if r.HasStop {
		end = r.Stop
	}
	for i, s := range ostreams {
		if s.Index == v.Index || s.CodecType == "attachment" || s.Disposition["attached_pic"] != 0 {
			continue
		}
		origPkts, err := streamPackets(ctx, w.tools, family, path, s.Index)
		if err != nil {
			return err
		}
		candPkts, err := streamPackets(ctx, w.tools, family, dst, cstreams[i].Index)
		if err != nil {
			return err
		}
		start := 0.0
		if r.HasSeek {
			start = r.Seek
		}
		want.CopiedOK[s.Index] = comparePackets(origPkts, candPkts, start, end)
	}
	return nil
}

// cutFailure names the end furthest off, with the frames it stayed from the owner's choice.
func cutFailure(off [2]int) error {
	if abs(off[0]) >= abs(off[1]) {
		return fail(RuleCutCheck, "first:%d", abs(off[0]))
	}
	return fail(RuleCutCheck, "last:%d", abs(off[1]))
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
