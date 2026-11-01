package repo_test

import (
	"context"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// R24: an apply's System activity row is the batch's one summary. Once no job of the batch is
// queued, it carries the renamed and failed counts, taken from each video's latest run so a
// retried failure counts once; the refused-by-reason detail written at apply time stays.
func TestFinishRenameBatch(t *testing.T) {
	r, _ := newRepoDB(t)
	ctx := context.Background()
	now := time.Now()
	run := func(entity int64, updated, errs int) {
		t.Helper()
		st := model.JobStatusOK
		if errs > 0 {
			st = model.JobStatusErr
		}
		if err := r.RecordJobRun(ctx, model.JobRun{Kind: model.JobKindRename, Trigger: model.TriggerManual,
			Status: st, StartedAt: now, FinishedAt: now, EntityType: model.EnrichEntityVideo,
			EntityID: entity, BatchID: "b", Updated: updated, Errors: errs}); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.RecordJobRun(ctx, model.JobRun{Kind: model.JobKindRename, Trigger: model.TriggerManual,
		Status: model.JobStatusOK, StartedAt: now, FinishedAt: now, BatchID: "b",
		Seen: 4, Skipped: 2, Detail: "clash 1 · locked 1"}); err != nil {
		t.Fatal(err)
	}
	id, err := r.UpsertVideo(ctx, legacyVideo("/m/a.mkv"), nil)
	if err != nil {
		t.Fatal(err)
	}
	run(1, 1, 0)
	run(2, 0, 1)
	run(2, 1, 0) // retried: renamed
	run(3, 0, 1)
	run(4, 0, 0) // already named
	summary := func() model.JobRun {
		t.Helper()
		runs, err := r.ListJobRunsByBatch(ctx, "b")
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range runs {
			if j.EntityID == 0 {
				return j
			}
		}
		t.Fatal("no summary row")
		return model.JobRun{}
	}

	r.EnqueueRenames(ctx, []repo.RenameEnqueue{{VideoID: id, Payload: `{"name":"b.mkv"}`}}, "b")
	if err := r.FinishRenameBatch(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	if s := summary(); s.Updated != 0 || s.Errors != 0 {
		t.Fatalf("summarised while a job was queued: %+v", s)
	}
	j, _ := r.ClaimNextWriteback(ctx)
	r.FinishWriteback(ctx, j.ID, true, "")
	if err := r.FinishRenameBatch(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	s := summary()
	if s.Updated != 2 || s.Errors != 1 || s.Status != model.JobStatusErr {
		t.Errorf("summary = updated %d, errors %d, status %s; want 2, 1, error", s.Updated, s.Errors, s.Status)
	}
	if s.Seen != 4 || s.Skipped != 2 || s.Detail != "clash 1 · locked 1" {
		t.Errorf("apply-time counts changed: %+v", s)
	}
}

// R18: a rename row rides the tag-write lane, never the maintenance lane, and never runs while
// the same video has a running job of another kind; boot recovery requeues it; Dismiss works.
// R17: a purge cascades the journal and the name state.
func TestRenameQueueLaneAndCascade(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, legacyVideo("/m/a.mkv"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.EnqueueMaintenance(ctx, id, `{"op":"hflip"}`); err != nil {
		t.Fatal(err)
	}
	if n, err := r.EnqueueRenames(ctx, []repo.RenameEnqueue{{VideoID: id, Payload: `{"name":"b.mkv"}`}}, "b"); err != nil || n != 1 {
		t.Fatalf("enqueue = %d, %v", n, err)
	}
	m, err := r.ClaimNextMaintenance(ctx)
	if err != nil || m == nil || m.Kind != repo.WritebackKindMaintenance {
		t.Fatalf("maintenance claim = %+v, %v", m, err)
	}
	if j, _ := r.ClaimNextWriteback(ctx); j != nil {
		t.Fatalf("a rename was claimed while a re-encode ran: %+v", j)
	}
	r.FinishWriteback(ctx, m.ID, true, "")
	j, err := r.ClaimNextWriteback(ctx)
	if err != nil || j == nil || j.Kind != repo.WritebackKindRename || j.BatchID != "b" {
		t.Fatalf("rename claim = %+v, %v", j, err)
	}
	if n, _ := r.RecoverRunningWritebacks(ctx); n != 1 {
		t.Errorf("recovered %d, want 1", n)
	}
	if again, _ := r.ClaimNextWriteback(ctx); again == nil || again.ID != j.ID {
		t.Fatalf("the recovered rename wasn't claimable: %+v", again)
	}
	r.FinishWriteback(ctx, j.ID, false, "boom")
	if n, _ := r.DismissFailedRename(ctx, id); n != 1 {
		t.Errorf("dismissed %d", n)
	}

	if _, err := r.InsertPendingRename(ctx, repo.FileRename{VideoID: id, OldName: "a.mkv", NewName: "b.mkv", Trigger: repo.RenameTriggerManual}); err != nil {
		t.Fatal(err)
	}
	r.SetNameLock(ctx, id, true, repo.LockOwner, time.Now())
	mustExec(t, database, `DELETE FROM videos WHERE id = ?`, id)
	for _, tbl := range []string{"file_renames", "video_file_names"} {
		if n := scalar[int](t, database, `SELECT count(*) FROM `+tbl); n != 0 {
			t.Errorf("%s kept %d rows after a purge", tbl, n)
		}
	}
}
