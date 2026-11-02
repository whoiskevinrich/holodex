package writequeue_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writequeue"
)

// R24 + R18: each rename job records its own kind=rename run, and when the batch's last job
// finishes the apply's summary row carries the renamed and failed counts.
func TestQueue_RenameBatchSummary(t *testing.T) {
	r := newRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, b, c := seedVideo(t, r, "MKV"), seedVideo(t, r, "MKV"), seedVideo(t, r, "MKV")
	now := time.Now()
	if err := r.RecordJobRun(ctx, model.JobRun{Kind: model.JobKindRename, Trigger: model.TriggerManual,
		Status: model.JobStatusOK, StartedAt: now, FinishedAt: now, BatchID: "rb", Seen: 3,
		Skipped: 1, Detail: "locked 1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.EnqueueRenames(ctx, []repo.RenameEnqueue{
		{VideoID: a, Payload: `{"name":"a.mkv"}`}, {VideoID: b, Payload: `{"name":"b.mkv"}`},
		{VideoID: c, Payload: `{"name":"c.mkv"}`}}, "rb"); err != nil {
		t.Fatal(err)
	}
	q := writequeue.New(r, nil, testLogger(), 1, "")
	errFail := errors.New("boom")
	q.SetRename(func(_ context.Context, job *repo.WritebackJob) error {
		if job.VideoID == c {
			return errFail
		}
		return nil
	}, func(error) bool { return false }, func(error) bool { return false })
	q.Start(ctx)

	summary := func() model.JobRun {
		runs, _ := r.ListJobRunsByBatch(ctx, "rb")
		for _, j := range runs {
			if j.EntityID == 0 {
				return j
			}
		}
		return model.JobRun{}
	}
	waitFor(t, func() bool { s := summary(); return s.Updated+s.Errors == 3 })
	s := summary()
	if s.Updated != 2 || s.Errors != 1 || s.Status != model.JobStatusErr || s.Detail != "locked 1" {
		t.Errorf("summary = %+v", s)
	}
	runs, _ := r.ListJobRunsByBatch(ctx, "rb")
	if len(runs) != 4 {
		t.Errorf("runs in the batch = %d, want the summary and one per video", len(runs))
	}
}
