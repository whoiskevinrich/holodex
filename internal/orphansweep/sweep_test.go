package orphansweep

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/model"
)

type fakeRepo struct {
	deleted, skipped int
	err              error
	graceDays        int
	runs             []model.JobRun
}

func (f *fakeRepo) SweepOrphans(_ context.Context, graceDays int) (int, int, error) {
	f.graceDays = graceDays
	return f.deleted, f.skipped, f.err
}
func (f *fakeRepo) RecordJobRun(_ context.Context, run model.JobRun) error {
	f.runs = append(f.runs, run)
	return nil
}

func TestSweepRecordsRunWithCounts(t *testing.T) {
	fr := &fakeRepo{deleted: 2, skipped: 1}
	New(fr, Config{}, nil).Sweep(context.Background())

	if fr.graceDays != 30 {
		t.Errorf("graceDays = %d, want default 30", fr.graceDays)
	}
	if len(fr.runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(fr.runs))
	}
	run := fr.runs[0]
	if run.Kind != model.JobKindOrphanSweep || run.Status != model.JobStatusOK || run.Removed != 2 || run.Skipped != 1 {
		t.Errorf("run = %+v, want orphan-sweep ok removed=2 skipped=1", run)
	}
}

func TestSweepQuietWhenNothingDue(t *testing.T) {
	fr := &fakeRepo{}
	New(fr, Config{GraceDays: 7}, nil).Sweep(context.Background())
	if fr.graceDays != 7 {
		t.Errorf("graceDays = %d, want configured 7", fr.graceDays)
	}
	if len(fr.runs) != 0 {
		t.Errorf("runs = %d, want 0 (no empty job_runs noise)", len(fr.runs))
	}
}

func TestSweepRecordsError(t *testing.T) {
	fr := &fakeRepo{err: errors.New("boom")}
	New(fr, Config{}, nil).Sweep(context.Background())
	if len(fr.runs) != 1 || fr.runs[0].Status != model.JobStatusErr || fr.runs[0].ErrorMessage != "boom" {
		t.Errorf("runs = %+v, want one error run carrying the message", fr.runs)
	}
}
