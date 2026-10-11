package maintenance

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Payload is a maintenance job's queue payload.
type Payload struct {
	Op string `json:"op"` // OpHFlip, OpRestore or OpDiscard
}

// Held-original operations, queued like a picture operation so they serialize with tag
// writes on the same file.
const (
	OpRestore = "restore"
	OpDiscard = "discard"
)

// PostWriteFunc refreshes the library row from the file after it changed (writequeue's
// post-write hook: cover-art re-extract, then a forced re-extract).
type PostWriteFunc func(ctx context.Context, videoID int64, path string)

// Worker drains the maintenance lane: one job at a time, because a re-encode saturates the CPU.
type Worker struct {
	repo      *repo.Repo
	swap      *Swapper
	tools     Tools
	encoders  map[string]bool
	postWrite PostWriteFunc
	kickWrite func() // wakes the tag-write lane when a job finishes
	log       *slog.Logger
	notify    chan struct{}
	pollEvery time.Duration

	runFlip func(ctx context.Context, family, path string) error // w.flip; a test stubs the re-encode
}

// NewWorker builds the maintenance worker. encoders is DetectEncoders' result.
func NewWorker(r *repo.Repo, s *Swapper, t Tools, encoders map[string]bool, postWrite PostWriteFunc,
	kickWrite func(), log *slog.Logger) *Worker {
	w := &Worker{repo: r, swap: s, tools: t, encoders: encoders, postWrite: postWrite, kickWrite: kickWrite,
		log: log, notify: make(chan struct{}, 1), pollEvery: 5 * time.Second}
	w.runFlip = w.flip
	return w
}

// CheckFlip runs the flip preflight against path with this worker's tools and encoders.
func (w *Worker) CheckFlip(ctx context.Context, path string) (Preflight, error) {
	return CheckFlip(ctx, w.tools, w.encoders, path)
}

// Hold is how long a flipped file's original is held.
func (w *Worker) Hold() time.Duration { return w.swap.hold }

// Kick wakes the worker after an enqueue.
func (w *Worker) Kick() {
	select {
	case w.notify <- struct{}{}:
	default:
	}
}

// Run drains the lane until ctx is cancelled. Boot recovery (RecoverSwaps, then the queue's
// RecoverRunningWritebacks) has already run.
func (w *Worker) Run(ctx context.Context) {
	for {
		job, err := w.repo.ClaimNextMaintenance(ctx)
		if err != nil {
			w.log.Warn("claim maintenance", "err", err)
		}
		if job != nil {
			w.process(ctx, job)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-w.notify:
		case <-time.After(w.pollEvery):
		}
	}
}

func (w *Worker) process(ctx context.Context, job *repo.WritebackJob) {
	started := time.Now()
	var p Payload
	err := json.Unmarshal([]byte(job.Payload), &p)
	if err != nil {
		err = fmt.Errorf("corrupt maintenance payload: %w", err)
	} else {
		err = w.do(ctx, job.VideoID, p.Op)
	}
	if err != nil && ctx.Err() != nil {
		// Shutdown: leave the row running. Boot recovery resets it to pending and the job
		// starts again from the original, which nothing has touched.
		return
	}
	run := model.JobRun{
		Kind: model.JobKindMaintenance, Trigger: model.TriggerManual, Status: model.JobStatusOK,
		StartedAt: started, FinishedAt: time.Now(), DurationMs: time.Since(started).Milliseconds(),
		Detail: p.Op, EntityType: model.EnrichEntityVideo, EntityID: job.VideoID,
	}
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
		run.Status, run.Errors, run.ErrorMessage = model.JobStatusErr, 1, errMsg
		w.log.Warn("maintenance failed", "video", job.VideoID, "op", p.Op, "err", err)
	} else {
		run.Updated = 1
	}
	if err := w.repo.RecordJobRun(ctx, run); err != nil {
		w.log.Warn("record maintenance job run", "id", job.ID, "err", err)
	}
	if err := w.repo.FinishWriteback(ctx, job.ID, err == nil, errMsg); err != nil {
		w.log.Warn("finish maintenance", "id", job.ID, "err", err)
	}
	if w.kickWrite != nil {
		w.kickWrite()
	}
}

// do runs one operation against the video's current file.
func (w *Worker) do(ctx context.Context, videoID int64, op string) error {
	v, _, err := w.repo.GetVideo(ctx, videoID)
	if err != nil {
		return fmt.Errorf("video %d: %w", videoID, err)
	}
	path := v.FilePath
	switch op {
	case string(OpHFlip):
		family, err := familyOf(path)
		if err != nil {
			return err
		}
		if err := w.runFlip(ctx, family, path); err != nil {
			return err
		}
		if err := w.swap.Swap(ctx, videoID, path, op); err != nil {
			os.Remove(NewPath(path))
			return err
		}
	case OpRestore:
		if err := w.swap.Restore(ctx, videoID, path); err != nil {
			return err
		}
	case OpDiscard:
		return w.swap.Discard(ctx, videoID, path) // the file at path is unchanged
	default:
		return fmt.Errorf("unknown maintenance operation %q", op)
	}

	// The picture changed: re-read the file, and regenerate a frame-derived thumbnail. A
	// thumbnail from embedded cover art or an owner upload shows no video frame, so it stays.
	if w.postWrite != nil {
		w.postWrite(ctx, videoID, path)
	}
	if v.ThumbnailState == model.ThumbnailGenerated {
		if err := w.repo.ResetThumbnailState(ctx, videoID); err != nil {
			w.log.Warn("reset thumbnail after maintenance", "video", videoID, "err", err)
		}
	}
	return nil
}

// flip rebuilds path into NewPath(path) and verifies it. On any failure the candidate is gone
// and the original untouched.
func (w *Worker) flip(ctx context.Context, family, path string) error {
	demux := Demuxer(family)
	p, err := RunProbe(ctx, w.tools.FFprobe, demux, path)
	if err != nil {
		return err
	}
	orig, err := CollectSnapshot(ctx, w.tools, family, path, p)
	if err != nil {
		return err
	}
	dst := NewPath(path)
	if err := Rebuild(ctx, w.tools, family, OpHFlip, w.encoders, path, dst, p); err != nil {
		return err
	}
	cp, err := RunProbe(ctx, w.tools.FFprobe, demux, dst)
	if err == nil {
		var cand Snapshot
		if cand, err = CollectSnapshot(ctx, w.tools, family, dst, cp); err == nil {
			err = Verify(orig, cand)
		}
	}
	if err != nil {
		os.Remove(dst)
		return err
	}
	return nil
}

// familyOf maps a library file to its container family by extension (the scanner's own rule).
func familyOf(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mkv":
		return FamilyMKV, nil
	case ".mp4", ".m4v", ".mov":
		return FamilyMP4, nil
	}
	return "", &RefusalError{Reason: ReasonUnsupportedFile, Detail: filepath.Ext(path)}
}

// DetectEncoders lists the video encoders the installed ffmpeg offers (`ffmpeg -encoders`),
// once at boot. An error means no encoder is usable; every flip then refuses at preflight.
func DetectEncoders(ctx context.Context, ffmpeg string) (map[string]bool, error) {
	out, err := run(ctx, ffmpeg, "-hide_banner", "-encoders")
	if err != nil {
		return map[string]bool{}, err
	}
	return parseEncoders(out), nil
}

// parseEncoders reads `ffmpeg -encoders` lines (" V....D libx264  …") for video encoders.
func parseEncoders(out []byte) map[string]bool {
	enc := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) >= 2 && len(f[0]) == 6 && f[0][0] == 'V' && f[1] != "=" { // "=" is the legend
			enc[f[1]] = true
		}
	}
	return enc
}
