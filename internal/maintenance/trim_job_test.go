package maintenance

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// trimRig is a workerRig whose cut is stubbed: the real cut has its own proof
// (trim_integration_test.go). These tests cover the job's lifecycle around it.
func trimRig(t *testing.T, thumbState string) *workerRig {
	t.Helper()
	g := newWorkerRig(t, thumbState)
	g.w.runTrim = func(_ context.Context, _, _, dst string, _ *TrimParams, _ string) ([]SkippedCheck, error) {
		write(t, dst, "trimmed")
		return nil, nil
	}
	return g
}

func (g *workerRig) enqueueTrim(t *testing.T, params TrimParams) *repo.WritebackJob {
	t.Helper()
	ctx := context.Background()
	raw, _ := json.Marshal(Payload{Op: string(OpTrim), Trim: &params})
	if _, err := g.r.EnqueueMaintenance(ctx, g.videoID, string(raw)); err != nil {
		t.Fatal(err)
	}
	job, err := g.r.ClaimNextMaintenance(ctx)
	if err != nil || job == nil {
		t.Fatalf("claim: %+v %v", job, err)
	}
	return job
}

func TestTrimReplaceHoldsTheOriginal(t *testing.T) {
	g := trimRig(t, model.ThumbnailGenerated)
	var thumbs []int64
	g.w.SetTrimHooks(TrimHooks{EnqueueThumb: func(id int64) { thumbs = append(thumbs, id) }})
	g.w.runTrim = func(_ context.Context, _, _, dst string, _ *TrimParams, edition string) ([]SkippedCheck, error) {
		if edition != "" {
			t.Errorf("a replace writes no edition: %q", edition)
		}
		write(t, dst, "trimmed")
		return []SkippedCheck{{End: "last", Reason: SkipStill}}, nil
	}
	ctx := context.Background()
	g.w.process(ctx, g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimReplace}))

	if read(g.path) != "trimmed" || read(HeldPath(g.path)) != "original" {
		t.Errorf("disk: path=%q held=%q", read(g.path), read(HeldPath(g.path)))
	}
	if s := g.queueStatus(t); s.Pending || s.Failed {
		t.Errorf("queue: %+v", s)
	}
	if g.posts != 1 {
		t.Errorf("read-backs = %d", g.posts)
	}
	if len(thumbs) != 1 || thumbs[0] != g.videoID {
		t.Errorf("a generated thumbnail must be queued again: %v", thumbs)
	}
	held, err := g.r.GetHeldOriginal(ctx, g.videoID)
	if err != nil || held == nil || held.Op != string(OpTrim) {
		t.Errorf("held = %+v, %v", held, err)
	}
	res, err := g.r.GetTrimResult(ctx, g.videoID)
	if err != nil || res == nil || res.Mode != TrimReplace || !strings.Contains(res.Skipped, SkipStill) {
		t.Errorf("result = %+v, %v", res, err)
	}
}

// A hold's end ends the replace's notice, so its lines can't attach to a later operation's hold.
func TestTrimReplaceOutcomeEndsWithItsHold(t *testing.T) {
	for _, op := range []string{OpDiscard, OpRestore} {
		t.Run(op, func(t *testing.T) {
			g := trimRig(t, model.ThumbnailGenerated)
			ctx := context.Background()
			g.w.process(ctx, g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimReplace}))
			if res, _ := g.r.GetTrimResult(ctx, g.videoID); res == nil {
				t.Fatal("no outcome after the replace")
			}
			g.runOne(t, ctx, op)
			if res, _ := g.r.GetTrimResult(ctx, g.videoID); res != nil {
				t.Errorf("outcome outlived the hold: %+v", res)
			}
		})
	}
}

// A6: an owner's poster stays through a replace, whatever frame it came from.
func TestTrimReplaceKeepsAnOwnersThumbnail(t *testing.T) {
	g := trimRig(t, model.ThumbnailUploaded)
	var thumbs []int64
	g.w.SetTrimHooks(TrimHooks{EnqueueThumb: func(id int64) { thumbs = append(thumbs, id) }})
	g.w.process(context.Background(), g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimReplace}))
	if len(thumbs) != 0 {
		t.Errorf("queued a thumbnail over the owner's: %v", thumbs)
	}
}

func TestTrimEditionCreatesAnItemAndLeavesTheOriginal(t *testing.T) {
	g := trimRig(t, model.ThumbnailGenerated)
	var copied [2]int64
	g.w.SetTrimHooks(TrimHooks{CopyOwnerSlots: func(_ context.Context, src, dst int64) error {
		copied = [2]int64{src, dst}
		return nil
	}})
	var wrote string
	g.w.runTrim = func(_ context.Context, _, _, dst string, _ *TrimParams, edition string) ([]SkippedCheck, error) {
		wrote = edition
		write(t, dst, "trimmed")
		return nil, nil
	}
	ctx := context.Background()
	g.w.process(ctx, g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimEdition, Edition: "Trim/med"}))

	if s := g.queueStatus(t); s.Pending || s.Failed {
		t.Fatalf("queue: %+v", s)
	}
	target := filepath.Join(filepath.Dir(g.path), "a {edition-Trimmed}.mkv")
	if read(target) != "trimmed" {
		t.Errorf("edition file = %q", read(target))
	}
	if wrote != "Trimmed" {
		t.Errorf("the file was told edition %q, not the sanitized name", wrote)
	}
	if read(g.path) != "original" {
		t.Errorf("original changed: %q", read(g.path))
	}
	if _, err := os.Stat(HeldPath(g.path)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an edition holds nothing: %v", err)
	}
	if _, err := os.Stat(NewPath(target)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("working file left behind: %v", err)
	}
	res, err := g.r.GetTrimResult(ctx, g.videoID)
	if err != nil || res == nil || res.EditionName != "Trimmed" || res.EditionVideoID == nil {
		t.Fatalf("result = %+v, %v", res, err)
	}
	if copied != [2]int64{g.videoID, *res.EditionVideoID} {
		t.Errorf("poster slots copied %v", copied)
	}
	from, err := g.r.GetTrimmedFrom(ctx, *res.EditionVideoID)
	if err != nil || from == nil || from.VideoID != g.videoID {
		t.Errorf("trimmed from = %+v, %v", from, err)
	}
}

// U7, F86: a file or library row holding the name is refused, and the original isn't touched.
func TestTrimEditionRefusesATakenName(t *testing.T) {
	g := trimRig(t, model.ThumbnailGenerated)
	target := filepath.Join(filepath.Dir(g.path), "a {edition-Trimmed}.mkv")
	write(t, target, "already here")
	g.w.process(context.Background(), g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimEdition, Edition: "Trimmed"}))
	s := g.queueStatus(t)
	if !s.Failed || !strings.Contains(s.Error, ReasonTargetExists) {
		t.Fatalf("queue: %+v", s)
	}
	if read(target) != "already here" || read(g.path) != "original" {
		t.Errorf("disk changed: target=%q path=%q", read(target), read(g.path))
	}
}

// U7: a library row holding the name counts as taken with no file there: a trashed video's state must
// not merge into the new edition.
func TestTrimEditionRefusesANameARowHolds(t *testing.T) {
	g := trimRig(t, model.ThumbnailGenerated)
	ctx := context.Background()
	target := filepath.Join(filepath.Dir(g.path), "a {edition-Trimmed}.mkv")
	if _, err := g.r.UpsertVideo(ctx, &model.Video{FilePath: target, Title: "gone", FileMtime: time.Now().UTC().Truncate(time.Second)}, nil); err != nil {
		t.Fatal(err)
	}
	g.w.process(ctx, g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimEdition, Edition: "Trimmed"}))
	if s := g.queueStatus(t); !s.Failed || !strings.Contains(s.Error, ReasonTargetExists) {
		t.Fatalf("queue: %+v", s)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("wrote the edition anyway: %v", err)
	}
}

// U7: a symlink at the target counts as taken even when it points at nothing, or a write could
// follow it out of the library.
func TestTrimEditionRefusesADanglingSymlink(t *testing.T) {
	g := trimRig(t, model.ThumbnailGenerated)
	target := filepath.Join(filepath.Dir(g.path), "a {edition-Trimmed}.mkv")
	if err := os.Symlink(filepath.Join(t.TempDir(), "nowhere"), target); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	cut := false
	g.w.runTrim = func(context.Context, string, string, string, *TrimParams, string) ([]SkippedCheck, error) {
		cut = true
		return nil, nil
	}
	g.w.process(context.Background(), g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimEdition, Edition: "Trimmed"}))
	if s := g.queueStatus(t); !s.Failed || !strings.Contains(s.Error, ReasonTargetExists) {
		t.Fatalf("queue: %+v", s)
	}
	if cut {
		t.Error("cut a file for a taken name")
	}
}

// A5: a file that takes the name after the check isn't overwritten at landing.
func TestTrimEditionLandingNeverOverwrites(t *testing.T) {
	g := trimRig(t, model.ThumbnailGenerated)
	target := filepath.Join(filepath.Dir(g.path), "a {edition-Trimmed}.mkv")
	g.w.runTrim = func(_ context.Context, _, _, dst string, _ *TrimParams, _ string) ([]SkippedCheck, error) {
		write(t, dst, "trimmed")
		write(t, target, "arrived during the cut")
		return nil, nil
	}
	g.w.process(context.Background(), g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimEdition, Edition: "Trimmed"}))
	if s := g.queueStatus(t); !s.Failed {
		t.Fatalf("queue: %+v", s)
	}
	if read(target) != "arrived during the cut" {
		t.Errorf("target overwritten: %q", read(target))
	}
	// The failed landing must not leave a journal a Retry would believe.
	ms, err := g.r.GetVideoMaintenanceStatus(context.Background(), g.videoID)
	if err != nil {
		t.Fatal(err)
	}
	var p Payload
	if err := json.Unmarshal([]byte(ms.Payload), &p); err != nil {
		t.Fatal(err)
	}
	if p.Trim.Stage != "" || p.Trim.Target != "" {
		t.Errorf("journal after a failed landing: %+v", p.Trim)
	}
	if _, err := os.Stat(NewPath(target)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("working file left behind: %v", err)
	}
}

// A5: the job reads the file's current path when it runs, not the one it was queued with.
func TestTrimUsesThePathAtRunTime(t *testing.T) {
	g := trimRig(t, model.ThumbnailGenerated)
	var sawSrc string
	g.w.runTrim = func(_ context.Context, _, src, dst string, _ *TrimParams, _ string) ([]SkippedCheck, error) {
		sawSrc = src
		write(t, dst, "trimmed")
		return nil, nil
	}
	job := g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimReplace})
	moved := filepath.Join(filepath.Dir(g.path), "renamed.mkv")
	if err := os.Rename(g.path, moved); err != nil {
		t.Fatal(err)
	}
	if err := g.r.RepointVideo(context.Background(), g.videoID, &model.Video{FilePath: moved, Title: "A", FileMtime: time.Now().UTC().Truncate(time.Second)}, nil); err != nil {
		t.Fatal(err)
	}
	g.w.process(context.Background(), job)
	if sawSrc != moved {
		t.Errorf("cut %q, want the renamed file", sawSrc)
	}
}

// U10: a crash after the file landed finishes the clone rather than cutting again.
func TestTrimRecoveryFinishesALandedEdition(t *testing.T) {
	g := trimRig(t, model.ThumbnailGenerated)
	cuts := 0
	g.w.runTrim = func(_ context.Context, _, _, dst string, _ *TrimParams, _ string) ([]SkippedCheck, error) {
		cuts++
		write(t, dst, "trimmed")
		return nil, nil
	}
	target := filepath.Join(filepath.Dir(g.path), "a {edition-T}.mkv")
	write(t, target, "landed before the crash")
	ctx := context.Background()
	g.w.process(ctx, g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimEdition, Edition: "T", Stage: StageLanded, Target: target}))
	if cuts != 0 {
		t.Errorf("cut again %d times", cuts)
	}
	if s := g.queueStatus(t); s.Pending || s.Failed {
		t.Fatalf("queue: %+v", s)
	}
	if read(target) != "landed before the crash" {
		t.Errorf("file changed: %q", read(target))
	}
	res, _ := g.r.GetTrimResult(ctx, g.videoID)
	if res == nil || res.EditionVideoID == nil {
		t.Fatalf("no clone: %+v", res)
	}
}

func TestTrimRecoveryRestartsWhenTheFileIsGone(t *testing.T) {
	g := trimRig(t, model.ThumbnailGenerated)
	cuts := 0
	g.w.runTrim = func(_ context.Context, _, _, dst string, _ *TrimParams, _ string) ([]SkippedCheck, error) {
		cuts++
		write(t, dst, "trimmed")
		return nil, nil
	}
	target := filepath.Join(filepath.Dir(g.path), "a {edition-T}.mkv")
	g.w.process(context.Background(), g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimEdition, Edition: "T", Stage: StageLanded, Target: target}))
	if cuts != 1 {
		t.Errorf("cuts = %d, want a fresh start", cuts)
	}
	if read(target) != "trimmed" {
		t.Errorf("target = %q", read(target))
	}
}

// U11: captures go when a job succeeds, stay for Retry when it fails, and a sweep clears strays.
func TestTrimCaptureLifecycle(t *testing.T) {
	g := trimRig(t, model.ThumbnailGenerated)
	root := t.TempDir()
	g.w.SetTrimHooks(TrimHooks{CaptureRoot: root})
	mk := func(name string) string {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	gone := func(dir string) bool {
		_, err := os.Stat(dir)
		return errors.Is(err, os.ErrNotExist)
	}
	ctx := context.Background()

	ok := mk("okjob")
	g.w.process(ctx, g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimReplace, Captures: "okjob"}))
	if !gone(ok) {
		t.Error("a finished job's captures stayed")
	}

	failed := mk("failedjob")
	g.w.runTrim = func(context.Context, string, string, string, *TrimParams, string) ([]SkippedCheck, error) {
		return nil, fail(RuleCutCheck, "first:2")
	}
	g.w.process(ctx, g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimReplace, Captures: "failedjob"}))
	if gone(failed) {
		t.Error("a failed job's captures must wait for Retry")
	}

	stray := mk("stray")
	if err := g.w.SweepCaptures(ctx); err != nil {
		t.Fatal(err)
	}
	if !gone(stray) {
		t.Error("a stray folder survived the sweep")
	}
	if gone(failed) {
		t.Error("the sweep took a failed job's captures")
	}
}

func TestTrimFailureNamesTheCheck(t *testing.T) {
	g := trimRig(t, model.ThumbnailGenerated)
	g.w.runTrim = func(context.Context, string, string, string, *TrimParams, string) ([]SkippedCheck, error) {
		return nil, fail(RuleCutCheck, "last:2")
	}
	g.w.process(context.Background(), g.enqueueTrim(t, TrimParams{InTime: 1, OutTime: 2, Mode: TrimReplace}))
	s := g.queueStatus(t)
	if !s.Failed || !strings.Contains(s.Error, "cut_check") || !strings.Contains(s.Error, "last:2") {
		t.Errorf("queue: %+v", s)
	}
	if read(g.path) != "original" {
		t.Errorf("original changed: %q", read(g.path))
	}
}

// U5: one correction across both checks, and never a third run.
func TestCheckedCut(t *testing.T) {
	screenFail := fail(RuleScreenCheck, "first:1")
	cases := []struct {
		name      string
		screens   []bool   // does each screen check move the frames?
		cuts      [][2]int // each cut's offset
		wantErr   string
		wantCuts  int
		wantMoved [][2]int
	}{
		{name: "clean", screens: []bool{false}, cuts: [][2]int{{}}, wantCuts: 1},
		{name: "cut off, then right", screens: []bool{false, false}, cuts: [][2]int{{1, 0}, {}}, wantCuts: 2, wantMoved: [][2]int{{1, 0}}},
		{name: "cut off twice", screens: []bool{false, false}, cuts: [][2]int{{1, 0}, {0, 2}}, wantCuts: 2, wantMoved: [][2]int{{1, 0}}, wantErr: "last:2"},
		{name: "screen moved, then right", screens: []bool{true, false}, cuts: [][2]int{{}}, wantCuts: 1},
		{name: "screen moved twice costs no encode", screens: []bool{true, true}, wantErr: "first:1"},
		{name: "screen moved, then the cut off", screens: []bool{true, false}, cuts: [][2]int{{1, 0}}, wantCuts: 1, wantErr: "first:1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var screens, cuts int
			var moved [][2]int
			err := checkedCut(
				func() (bool, error, error) {
					screens++
					return c.screens[screens-1], screenFail, nil
				},
				func() ([2]int, error) {
					cuts++
					return c.cuts[cuts-1], nil
				},
				func(off [2]int) { moved = append(moved, off) })
			if c.wantErr == "" && err != nil {
				t.Fatalf("err = %v", err)
			}
			if c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)) {
				t.Fatalf("err = %v, want %s", err, c.wantErr)
			}
			if cuts != c.wantCuts {
				t.Errorf("%d encodes, want %d", cuts, c.wantCuts)
			}
			if len(moved) != len(c.wantMoved) {
				t.Errorf("moved %v, want %v", moved, c.wantMoved)
			}
		})
	}
}

func TestCutFailureNamesTheWorstEnd(t *testing.T) {
	err := cutFailure([2]int{1, -3})
	re, ok := err.(*RuleError)
	if !ok || re.Rule != RuleCutCheck || re.Detail != "last:3" {
		t.Errorf("got %v", err)
	}
	if re := cutFailure([2]int{2, 0}).(*RuleError); re.Detail != "first:2" {
		t.Errorf("got %v", re)
	}
}
