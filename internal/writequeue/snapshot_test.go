package writequeue_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH — skipping writeback snapshot I/O tests")
	}
}

func requireExiftool(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not on PATH — skipping writeback snapshot I/O tests")
	}
}

// newMinimalMKV renders a genuinely valid (if trivial) Matroska file via
// ffmpeg — unlike writeback's synthetic EBML-header fixture, this is a real
// container exiftool and ffmpeg's own remux path can both fully round-trip,
// which the mkvpropedit-less snapshot/revert test below depends on.
func newMinimalMKV(t *testing.T, path string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "anullsrc=r=8000:cl=mono", "-t", "1", "-c:a", "aac", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg fixture: %v — %s", err, out)
	}
}

// batchIDFromDetail extracts the "· batch <id>" suffix detailLine appends
// (F48.9d) so activity history carries the id Revert needs.
func batchIDFromDetail(detail string) string {
	i := strings.LastIndex(detail, "batch ")
	if i < 0 {
		return ""
	}
	return detail[i+len("batch "):]
}

// TestQueue_SnapshotsAndReverts exercises F48.9 end-to-end against real tool
// invocations (no fake WriteFunc): a video with an existing "Original Title"
// tag gets a new title written through the queue, which must snapshot the
// prior value before overwriting it; Revert then restores the original.
func TestQueue_SnapshotsAndReverts(t *testing.T) {
	requireFFmpeg(t)
	requireExiftool(t)

	r := newRepo(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mkv")
	newMinimalMKV(t, path)
	if err := writeback.Write(context.Background(), path, "Title", []string{"Original Title"}); err != nil {
		t.Skipf("fixture not writable: %v", err)
	}

	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: path, FileSize: 1, Title: "Original Title",
		Container: "Matroska", FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := writequeue.New(r, writeback.WriteBatch, testLogger(), 1, "")
	q.Start(ctx)

	if _, err := q.Enqueue(ctx, id, []writequeue.JobField{
		{Field: "title", Values: []string{"New Title"}, Source: "manual:title"},
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitFor(t, func() bool { n, _ := r.PendingWritebackCount(ctx); return n == 0 })

	runs, err := r.ListJobRuns(ctx, 30)
	if err != nil {
		t.Fatalf("list job runs: %v", err)
	}
	var detail string
	for _, run := range runs {
		if run.Kind == model.JobKindWriteback {
			detail = run.Detail
			break
		}
	}
	batchID := batchIDFromDetail(detail)
	if batchID == "" {
		t.Fatalf("no batch id recorded in job_runs detail: %q", detail)
	}

	snaps, err := r.SnapshotsForBatch(ctx, batchID)
	if err != nil {
		t.Fatalf("snapshots for batch: %v", err)
	}
	if len(snaps) != 1 || snaps[0].FieldKey != "title" || snaps[0].PriorValue != "Original Title" {
		t.Fatalf("want one title snapshot of %q, got %+v", "Original Title", snaps)
	}

	got, err := writeback.ReadCurrentValues(ctx, path, []writeback.Mapped{{Field: "title", TagName: "Title"}})
	if err != nil || got["title"] != "New Title" {
		t.Fatalf("want file title %q after write, got %q (err=%v)", "New Title", got["title"], err)
	}

	// Revert restores the pre-write value, itself going through the queue —
	// wait for the revert job to drain, then verify the on-disk value.
	jobIDs, err := q.Revert(ctx, batchID)
	if err != nil {
		t.Fatalf("revert: %v", err)
	}
	if len(jobIDs) != 1 {
		t.Fatalf("want 1 revert job, got %d", len(jobIDs))
	}
	waitFor(t, func() bool { n, _ := r.PendingWritebackCount(ctx); return n == 0 })

	got, err = writeback.ReadCurrentValues(ctx, path, []writeback.Mapped{{Field: "title", TagName: "Title"}})
	if err != nil || got["title"] != "Original Title" {
		t.Fatalf("want file title restored to %q after revert, got %q (err=%v)", "Original Title", got["title"], err)
	}
}

// TestQueue_SnapshotSurvivesCrashRetry simulates a crash between a successful
// write and FinishWriteback: RecoverRunningWritebacks resets the job back to
// 'pending' and a worker reprocesses it from scratch, by which point the file
// already carries the first attempt's write. snapshotBeforeWrite must not
// re-read that already-mutated value as "prior" on the retry — it should find
// and keep the snapshot the first attempt already recorded.
func TestQueue_SnapshotSurvivesCrashRetry(t *testing.T) {
	requireFFmpeg(t)
	requireExiftool(t)

	r := newRepo(t)
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mkv")
	newMinimalMKV(t, path)
	if err := writeback.Write(ctx, path, "Title", []string{"Original Title"}); err != nil {
		t.Skipf("fixture not writable: %v", err)
	}

	id, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: path, FileSize: 1, Title: "Original Title",
		Container: "Matroska", FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	if _, err := r.EnqueueWriteback(ctx, id, `[{"field":"title","values":["New Title"],"source":"manual:title"}]`, ""); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	job, err := r.ClaimNextWriteback(ctx)
	if err != nil || job == nil {
		t.Fatalf("claim: job=%v err=%v", job, err)
	}
	batchID := strconv.FormatInt(job.ID, 10)

	// Simulate attempt 1 completing up through the write: the correct snapshot
	// is recorded, the file is rewritten to the new value — then the process
	// "crashes" (no FinishWriteback call; the row stays 'running').
	if err := r.InsertWritebackSnapshots(ctx, id, batchID, map[string]string{"title": "Original Title"}); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := writeback.Write(ctx, path, "Title", []string{"New Title"}); err != nil {
		t.Skipf("fixture not writable: %v", err)
	}

	// A fresh queue's boot recovery requeues the 'running' row; its worker
	// reprocesses it for real (attempt 2) — the exact retry scenario.
	qctx, cancel := context.WithCancel(ctx)
	defer cancel()
	writequeue.New(r, writeback.WriteBatch, testLogger(), 1, "").Start(qctx)
	waitFor(t, func() bool { n, _ := r.PendingWritebackCount(ctx); return n == 0 })

	snaps, err := r.SnapshotsForBatch(ctx, batchID)
	if err != nil {
		t.Fatalf("snapshots for batch: %v", err)
	}
	if len(snaps) != 1 || snaps[0].PriorValue != "Original Title" {
		t.Fatalf("want the retry to keep attempt 1's snapshot (%q), got %+v", "Original Title", snaps)
	}

	got, err := writeback.ReadCurrentValues(ctx, path, []writeback.Mapped{{Field: "title", TagName: "Title"}})
	if err != nil || got["title"] != "New Title" {
		t.Fatalf("want file title %q after retry, got %q (err=%v)", "New Title", got["title"], err)
	}
}

// TestQueue_EnqueueBatch_SharedBatchIDGroupsMultipleVideos is F48.8's write
// path end to end: several jobs enqueued with the same caller-supplied batch
// id (as merge propagation does — one job per affected video) each take
// their own snapshot under that shared batch despite sharing it, and a
// single Revert restores every one of them.
func TestQueue_EnqueueBatch_SharedBatchIDGroupsMultipleVideos(t *testing.T) {
	requireFFmpeg(t)
	requireExiftool(t)

	r := newRepo(t)
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.mkv")
	pathB := filepath.Join(dir, "b.mkv")
	newMinimalMKV(t, pathA)
	newMinimalMKV(t, pathB)
	if err := writeback.Write(context.Background(), pathA, "Artist", []string{"Old Name"}); err != nil {
		t.Skipf("fixture not writable: %v", err)
	}
	if err := writeback.Write(context.Background(), pathB, "Artist", []string{"Other Name"}); err != nil {
		t.Skipf("fixture not writable: %v", err)
	}

	a, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: pathA, FileSize: 1, Title: "A",
		Container: "Matroska", FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video a: %v", err)
	}
	b, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: pathB, FileSize: 1, Title: "B",
		Container: "Matroska", FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video b: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := writequeue.New(r, writeback.WriteBatch, testLogger(), 1, "")
	q.Start(ctx)

	const batchID = "merge-person-1-2"
	if _, err := q.EnqueueBatch(ctx, a, []writequeue.JobField{
		{Field: "actors", Values: []string{"New Name"}, Source: "merge"},
	}, batchID); err != nil {
		t.Fatalf("enqueue batch (a): %v", err)
	}
	if _, err := q.EnqueueBatch(ctx, b, []writequeue.JobField{
		{Field: "actors", Values: []string{"New Name"}, Source: "merge"},
	}, batchID); err != nil {
		t.Fatalf("enqueue batch (b): %v", err)
	}
	waitFor(t, func() bool { n, _ := r.PendingWritebackCount(ctx); return n == 0 })

	// Both videos' snapshots landed under the one shared batch id — proof
	// that a shared batch id doesn't let video A's already-taken snapshot
	// make video B's job skip taking its own (SnapshotsForBatchVideo's
	// per-video scoping, unit-tested directly in writeback_snapshots_test.go).
	snaps, err := r.SnapshotsForBatch(ctx, batchID)
	if err != nil {
		t.Fatalf("snapshots for batch: %v", err)
	}
	byVideo := map[int64]string{}
	for _, s := range snaps {
		byVideo[s.VideoID] = s.PriorValue
	}
	if len(snaps) != 2 || byVideo[a] != "Old Name" || byVideo[b] != "Other Name" {
		t.Fatalf("want one snapshot per video under the shared batch, got %+v", snaps)
	}

	// A single Revert restores both videos in one call.
	jobIDs, err := q.Revert(ctx, batchID)
	if err != nil {
		t.Fatalf("revert: %v", err)
	}
	if len(jobIDs) != 2 {
		t.Fatalf("revert job ids = %v, want 2 (one per video)", jobIDs)
	}
	waitFor(t, func() bool { n, _ := r.PendingWritebackCount(ctx); return n == 0 })

	gotA, err := writeback.ReadCurrentValues(ctx, pathA, []writeback.Mapped{{Field: "actors", TagName: "Artist"}})
	if err != nil || gotA["actors"] != "Old Name" {
		t.Fatalf("video a after revert = %q (err=%v), want %q", gotA["actors"], err, "Old Name")
	}
	gotB, err := writeback.ReadCurrentValues(ctx, pathB, []writeback.Mapped{{Field: "actors", TagName: "Artist"}})
	if err != nil || gotB["actors"] != "Other Name" {
		t.Fatalf("video b after revert = %q (err=%v), want %q", gotB["actors"], err, "Other Name")
	}
}

// TestQueue_RevertUnknownBatch confirms reverting a batch id with no
// snapshots fails clearly rather than silently no-oping.
func TestQueue_RevertUnknownBatch(t *testing.T) {
	r := newRepo(t)
	write := func(context.Context, string, []writeback.FieldWrite) error { return nil }
	q := writequeue.New(r, write, testLogger(), 1, "")

	if _, err := q.Revert(context.Background(), "no-such-batch"); err == nil {
		t.Fatal("want an error reverting an unknown batch")
	}
}
