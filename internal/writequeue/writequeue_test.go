package writequeue_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newRepo(t *testing.T) *repo.Repo {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return repo.New(database)
}

// seedVideo inserts a video with the given container so TagForField resolves.
func seedVideo(t *testing.T, r *repo.Repo, container string) int64 {
	t.Helper()
	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: filepath.Join(t.TempDir(), "v.mp4"), FileSize: 1, Title: "t",
		Container: container, FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	return id
}

// waitFor polls until cond or the deadline.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}

func TestQueue_WritesAndAudits(t *testing.T) {
	r := newRepo(t)
	id := seedVideo(t, r, "MP4")

	var gotPath atomic.Value
	var gotFields atomic.Int32
	write := func(_ context.Context, path string, fields []writeback.FieldWrite) error {
		gotPath.Store(path)
		gotFields.Store(int32(len(fields)))
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := writequeue.New(r, write, testLogger(), 1, "")
	q.Start(ctx)

	if _, err := q.Enqueue(ctx, id, []writequeue.JobField{
		{Field: "title", Values: []string{"My Film"}, Source: "manual:title"},
		{Field: "genres", Values: []string{"Drama", "Sci-Fi"}, Source: "tmdb:genres"},
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Worker drains: the file write runs, the queue empties, a job_run is recorded.
	waitFor(t, func() bool {
		n, _ := r.PendingWritebackCount(ctx)
		return n == 0 && gotFields.Load() == 2
	})

	runs, err := r.ListJobRuns(ctx, 30)
	if err != nil {
		t.Fatalf("list job runs: %v", err)
	}
	var wb *model.JobRun
	for i := range runs {
		if runs[i].Kind == model.JobKindWriteback {
			wb = &runs[i]
		}
	}
	if wb == nil || wb.Status != model.JobStatusOK || wb.Updated != 2 {
		t.Fatalf("want a successful writeback job_run with 2 fields, got %+v", wb)
	}
}

// TestQueue_EnqueueMany_OneCallEnqueuesEveryJob is F48.8's bulk enqueue path:
// several videos' jobs, submitted in one EnqueueMany call, all end up
// queued and all drain — the single-transaction insert (EnqueueWritebackBatch)
// doesn't drop or reorder anything. Jobs with no fields are skipped.
func TestQueue_EnqueueMany_OneCallEnqueuesEveryJob(t *testing.T) {
	r := newRepo(t)
	a := seedVideo(t, r, "MP4")
	b := seedVideo(t, r, "MP4")

	var written atomic.Int32
	write := func(context.Context, string, []writeback.FieldWrite) error {
		written.Add(1)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := writequeue.New(r, write, testLogger(), 1, "")
	q.Start(ctx)

	ids, err := q.EnqueueMany(ctx, []writequeue.BatchJob{
		{VideoID: a, Fields: []writequeue.JobField{{Field: "title", Values: []string{"A"}}}},
		{VideoID: b, Fields: []writequeue.JobField{{Field: "title", Values: []string{"B"}}}},
		{VideoID: seedVideo(t, r, "MP4"), Fields: nil}, // no fields — must be skipped, not enqueued
	}, "shared-batch")
	if err != nil {
		t.Fatalf("enqueue many: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("enqueued job ids = %v, want 2 (the empty-fields job skipped)", ids)
	}

	waitFor(t, func() bool {
		n, _ := r.PendingWritebackCount(ctx)
		return n == 0 && written.Load() == 2
	})
}

// TestQueue_EnqueueMany_EmptyIsNoop confirms an all-empty-fields batch
// enqueues nothing rather than erroring or creating empty rows.
func TestQueue_EnqueueMany_EmptyIsNoop(t *testing.T) {
	r := newRepo(t)
	write := func(context.Context, string, []writeback.FieldWrite) error { return nil }
	q := writequeue.New(r, write, testLogger(), 1, "")

	ids, err := q.EnqueueMany(context.Background(), nil, "batch")
	if err != nil || len(ids) != 0 {
		t.Fatalf("enqueue many (nil) = %v, err=%v, want none", ids, err)
	}
	ids, err = q.EnqueueMany(context.Background(), []writequeue.BatchJob{{VideoID: 1, Fields: nil}}, "batch")
	if err != nil || len(ids) != 0 {
		t.Fatalf("enqueue many (empty fields) = %v, err=%v, want none", ids, err)
	}
}

func TestQueue_FailureMarksFailedAndKeepsRow(t *testing.T) {
	r := newRepo(t)
	id := seedVideo(t, r, "MP4")

	write := func(_ context.Context, _ string, _ []writeback.FieldWrite) error {
		return errors.New("boom")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := writequeue.New(r, write, testLogger(), 1, "")
	q.Start(ctx)

	if _, err := q.Enqueue(ctx, id, []writequeue.JobField{{Field: "title", Values: []string{"X"}}}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// A failed write keeps the row (status=failed) so it isn't silently lost, and the
	// original file is untouched (the stub never wrote). The pending count is 0 because
	// 'failed' is terminal (not pending/running).
	waitFor(t, func() bool {
		n, _ := r.PendingWritebackCount(ctx)
		return n == 0
	})
	runs, _ := r.ListJobRuns(ctx, 30)
	if len(runs) == 0 || runs[0].Kind != model.JobKindWriteback || runs[0].Status != model.JobStatusErr {
		t.Fatalf("want a failed writeback job_run, got %+v", runs)
	}
}

func TestQueue_RecoverRunningRequeues(t *testing.T) {
	r := newRepo(t)
	id := seedVideo(t, r, "MP4")
	ctx := context.Background()

	// Simulate a crash: enqueue then claim (→ running) but never finish.
	if _, err := r.EnqueueWriteback(ctx, id, `[{"field":"title","values":["X"]}]`, ""); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if _, err := r.ClaimNextWriteback(ctx); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// A fresh queue's boot recovery should requeue the running row and a worker drains it.
	var wrote sync.WaitGroup
	wrote.Add(1)
	var once sync.Once
	write := func(_ context.Context, _ string, _ []writeback.FieldWrite) error {
		once.Do(wrote.Done)
		return nil
	}
	qctx, cancel := context.WithCancel(ctx)
	defer cancel()
	writequeue.New(r, write, testLogger(), 1, "").Start(qctx)

	done := make(chan struct{})
	go func() { wrote.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("recovered job was not re-run")
	}
}
