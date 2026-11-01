package repo_test

import (
	"context"
	"testing"
	"time"

	"holodex/internal/repo"
)

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
