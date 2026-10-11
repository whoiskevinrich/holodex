package maintenance

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// workerRig is a real repo with one video whose file is a small text stand-in. The re-encode
// itself is stubbed: Rebuild and Verify have their own real-file proof
// (flip_integration_test.go); these tests cover the job's lifecycle around them.
type workerRig struct {
	r       *repo.Repo
	w       *Worker
	videoID int64
	path    string
	posts   int
	kicks   int
}

func newWorkerRig(t *testing.T, thumbState string) *workerRig {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	ctx := context.Background()

	path := filepath.Join(t.TempDir(), "a.mkv")
	write(t, path, "original")
	id, err := r.UpsertVideo(ctx, &model.Video{FilePath: path, FileSize: 8, Title: "A", Duration: 1,
		Width: 320, Height: 240, FileMtime: time.Now().UTC().Truncate(time.Second)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetThumbnailState(ctx, id, thumbState); err != nil {
		t.Fatal(err)
	}

	rig := &workerRig{r: r, videoID: id, path: path}
	rig.w = NewWorker(r, NewSwapper(r, week), Tools{}, nil,
		func(context.Context, int64, string) { rig.posts++ },
		func() { rig.kicks++ },
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	rig.w.runFlip = func(_ context.Context, _ string, p string) error {
		write(t, NewPath(p), "flipped")
		return nil
	}
	return rig
}

// runOne enqueues op and processes exactly that job.
func (g *workerRig) runOne(t *testing.T, ctx context.Context, op string) {
	t.Helper()
	if _, err := g.r.EnqueueMaintenance(context.Background(), g.videoID, `{"op":"`+op+`"}`); err != nil {
		t.Fatal(err)
	}
	job, err := g.r.ClaimNextMaintenance(context.Background())
	if err != nil || job == nil {
		t.Fatalf("claim: %+v %v", job, err)
	}
	g.w.process(ctx, job)
}

func (g *workerRig) queueStatus(t *testing.T) repo.VideoWritebackStatus {
	t.Helper()
	s, err := g.r.GetVideoMaintenanceStatus(context.Background(), g.videoID)
	if err != nil {
		t.Fatal(err)
	}
	return s.VideoWritebackStatus
}

func (g *workerRig) lastRun(t *testing.T) model.JobRun {
	t.Helper()
	runs, err := g.r.ListJobRuns(context.Background(), 1)
	if err != nil || len(runs) == 0 {
		t.Fatalf("job runs: %v %v", runs, err)
	}
	return runs[0]
}

func TestWorker_FlipSucceeds(t *testing.T) {
	g := newWorkerRig(t, model.ThumbnailGenerated)
	g.runOne(t, context.Background(), string(OpHFlip))

	if read(g.path) != "flipped" || read(HeldPath(g.path)) != "original" {
		t.Errorf("disk: path=%q held=%q", read(g.path), read(HeldPath(g.path)))
	}
	if s := g.queueStatus(t); s.Pending || s.Failed {
		t.Errorf("queue row left behind: %+v", s)
	}
	if h, _ := g.r.GetHeldOriginal(context.Background(), g.videoID); h == nil || h.State != repo.HeldHeld {
		t.Errorf("held row = %+v", h)
	}
	if run := g.lastRun(t); run.Kind != model.JobKindMaintenance || run.Status != model.JobStatusOK || run.Detail != "hflip" {
		t.Errorf("job run = %+v", run)
	}
	v, _, _ := g.r.GetVideo(context.Background(), g.videoID)
	if v.ThumbnailState != model.ThumbnailNone {
		t.Errorf("frame thumbnail not reset: %q", v.ThumbnailState)
	}
	if g.posts != 1 || g.kicks != 1 {
		t.Errorf("post-write %d, write-lane kicks %d; want 1 and 1", g.posts, g.kicks)
	}
}

// A cover-art or uploaded thumbnail shows no video frame, so a flip leaves it alone.
func TestWorker_FlipKeepsANonFrameThumbnail(t *testing.T) {
	for _, state := range []string{model.ThumbnailUploaded, model.ThumbnailEmbedded} {
		g := newWorkerRig(t, state)
		g.runOne(t, context.Background(), string(OpHFlip))
		if v, _, _ := g.r.GetVideo(context.Background(), g.videoID); v.ThumbnailState != state {
			t.Errorf("%s thumbnail became %q", state, v.ThumbnailState)
		}
	}
}

func TestWorker_FailedFlipLeavesTheOriginal(t *testing.T) {
	g := newWorkerRig(t, model.ThumbnailGenerated)
	g.w.runFlip = func(context.Context, string, string) error {
		return &RuleError{Rule: RuleTags, Detail: `tag "PUBLISHER" missing`}
	}
	g.runOne(t, context.Background(), string(OpHFlip))

	if read(g.path) != "original" || read(HeldPath(g.path)) != "<missing>" || read(NewPath(g.path)) != "<missing>" {
		t.Errorf("disk changed: path=%q held=%q new=%q", read(g.path), read(HeldPath(g.path)), read(NewPath(g.path)))
	}
	s := g.queueStatus(t)
	if !s.Failed || s.Error == "" {
		t.Errorf("status = %+v, want failed with the rule", s)
	}
	if run := g.lastRun(t); run.Status != model.JobStatusErr {
		t.Errorf("job run = %+v", run)
	}
	if g.posts != 0 || g.kicks != 1 {
		t.Errorf("post-write %d (want 0), kicks %d (want 1)", g.posts, g.kicks)
	}
}

func TestWorker_RestoreThenDiscard(t *testing.T) {
	ctx := context.Background()
	g := newWorkerRig(t, model.ThumbnailGenerated)
	g.runOne(t, ctx, string(OpHFlip))
	g.runOne(t, ctx, OpRestore)
	if read(g.path) != "original" || read(HeldPath(g.path)) != "<missing>" {
		t.Errorf("after restore: path=%q held=%q", read(g.path), read(HeldPath(g.path)))
	}
	if g.posts != 2 {
		t.Errorf("restore did not re-read the file (post-writes %d)", g.posts)
	}

	g.runOne(t, ctx, string(OpHFlip))
	g.runOne(t, ctx, OpDiscard)
	if read(g.path) != "flipped" || read(HeldPath(g.path)) != "<missing>" {
		t.Errorf("after discard: path=%q held=%q", read(g.path), read(HeldPath(g.path)))
	}
	if g.posts != 3 {
		t.Errorf("discard re-read an unchanged file (post-writes %d, want 3)", g.posts)
	}
}

// Shutdown mid-job: the row stays running, so boot recovery requeues it rather than recording
// a failure the owner never caused.
func TestWorker_ShutdownLeavesTheJobRunning(t *testing.T) {
	g := newWorkerRig(t, model.ThumbnailGenerated)
	ctx, cancel := context.WithCancel(context.Background())
	g.w.runFlip = func(ctx context.Context, _, _ string) error {
		cancel()
		return ctx.Err()
	}
	g.runOne(t, ctx, string(OpHFlip))
	if s := g.queueStatus(t); !s.Pending || s.Failed {
		t.Errorf("status = %+v, want still pending/running", s)
	}
	if n, _ := g.r.RecoverRunningWritebacks(context.Background()); n != 1 {
		t.Errorf("boot recovery reset %d rows, want 1", n)
	}
}

func TestWorker_UnknownOpFails(t *testing.T) {
	g := newWorkerRig(t, model.ThumbnailGenerated)
	g.runOne(t, context.Background(), "rotate")
	if s := g.queueStatus(t); !s.Failed {
		t.Errorf("status = %+v", s)
	}
}

func TestWorker_CorruptPayloadSaysSo(t *testing.T) {
	g := newWorkerRig(t, model.ThumbnailGenerated)
	if _, err := g.r.EnqueueMaintenance(context.Background(), g.videoID, `{"op":`); err != nil {
		t.Fatal(err)
	}
	job, _ := g.r.ClaimNextMaintenance(context.Background())
	g.w.process(context.Background(), job)
	if s := g.queueStatus(t); !s.Failed || !strings.Contains(s.Error, "corrupt") {
		t.Errorf("status = %+v", s)
	}
}

// The expiry pass queues a discard only past the window, through the lane, and an idle pass
// leaves no activity row.
func TestExpireHeld_QueuesDiscardsPastTheWindow(t *testing.T) {
	ctx := context.Background()
	g := newWorkerRig(t, model.ThumbnailGenerated)
	g.runOne(t, ctx, string(OpHFlip))
	before := len(mustRuns(t, g))

	if n := g.w.ExpireHeld(ctx); n != 0 {
		t.Fatalf("queued %d discards inside the window", n)
	}
	if len(mustRuns(t, g)) != before {
		t.Error("an idle pass recorded a job run")
	}

	// Age the hold past its window (journal rows are written in UTC RFC3339).
	if err := g.r.DeleteHeldOriginal(ctx, g.videoID); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-8 * 24 * time.Hour)
	if err := g.r.BeginSwap(ctx, g.videoID, HeldPath(g.path), "hflip", past, past.Add(week)); err != nil {
		t.Fatal(err)
	}
	if err := g.r.SetHeldState(ctx, g.videoID, repo.HeldHeld); err != nil {
		t.Fatal(err)
	}
	if n := g.w.ExpireHeld(ctx); n != 1 {
		t.Fatalf("queued %d discards past the window, want 1", n)
	}
	if run := g.lastRun(t); run.Kind != model.JobKindHeldExpiry || run.Updated != 1 {
		t.Errorf("expiry run = %+v", run)
	}
	if n := g.w.ExpireHeld(ctx); n != 0 {
		t.Errorf("a second pass queued %d more while the first discard waits", n)
	}

	job, _ := g.r.ClaimNextMaintenance(ctx)
	g.w.process(ctx, job)
	if read(HeldPath(g.path)) != "<missing>" || read(g.path) != "flipped" {
		t.Errorf("after expiry discard: path=%q held=%q", read(g.path), read(HeldPath(g.path)))
	}
}

// An unacknowledged failed flip keeps its line on the page: expiry waits for the owner.
func TestExpireHeld_KeepsAnUnacknowledgedFailure(t *testing.T) {
	ctx := context.Background()
	g := newWorkerRig(t, model.ThumbnailGenerated)
	past := time.Now().Add(-8 * 24 * time.Hour)
	write(t, HeldPath(g.path), "original")
	_ = g.r.BeginSwap(ctx, g.videoID, HeldPath(g.path), "hflip", past, past.Add(week))
	_ = g.r.SetHeldState(ctx, g.videoID, repo.HeldHeld)
	g.w.runFlip = func(context.Context, string, string) error { return &RuleError{Rule: RuleTags, Detail: "x"} }
	g.runOne(t, ctx, string(OpHFlip)) // a second flip fails

	if n := g.w.ExpireHeld(ctx); n != 0 {
		t.Fatalf("expiry queued %d discards over an unacknowledged failure", n)
	}
	if s := g.queueStatus(t); !s.Failed {
		t.Error("the failure line vanished")
	}
	if _, err := g.r.DismissFailedMaintenance(ctx, g.videoID); err != nil {
		t.Fatal(err)
	}
	if n := g.w.ExpireHeld(ctx); n != 1 {
		t.Errorf("after dismiss, expiry queued %d, want 1", n)
	}
}

func mustRuns(t *testing.T, g *workerRig) []model.JobRun {
	t.Helper()
	runs, err := g.r.ListJobRuns(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return runs
}

func TestParseEncoders(t *testing.T) {
	out := []byte("Encoders:\n V..... = Video\n A..... = Audio\n ------\n" +
		" V....D libx264              libx264 H.264\n V..... libsvtav1  SVT-AV1\n A....D aac  AAC\n")
	got := parseEncoders(out)
	if !got["libx264"] || !got["libsvtav1"] || got["aac"] || got["="] || len(got) != 2 {
		t.Errorf("encoders = %v", got)
	}
}

func TestFamilyOf(t *testing.T) {
	for p, want := range map[string]string{"a.MKV": FamilyMKV, "a.mp4": FamilyMP4, "a.mov": FamilyMP4} {
		if got, err := familyOf(p); err != nil || got != want {
			t.Errorf("%s: %q %v", p, got, err)
		}
	}
	if _, err := familyOf("a.avi"); err == nil {
		t.Error("avi accepted")
	}
}
