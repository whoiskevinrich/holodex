package repo_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"holodex/internal/repo"
)

// F77 queue lanes (docs/architecture/file-maintenance.md, testing-strategy §25.3): each lane
// claims only its own kind, and no lane claims a row whose video already has a running row
// of either kind.

const writePayload = `[{"field":"title","values":["X"]}]`
const flipPayload = `{"op":"hflip"}`

func seedLaneVideo(t *testing.T, r *repo.Repo, name string) int64 {
	t.Helper()
	id, err := r.UpsertVideo(context.Background(), sampleVideo(filepath.Join(t.TempDir(), name), name, nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video %s: %v", name, err)
	}
	return id
}

func TestClaim_EachLaneClaimsOnlyItsKind(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a := seedLaneVideo(t, r, "a.mkv")
	b := seedLaneVideo(t, r, "b.mkv")

	flipID, err := r.EnqueueMaintenance(ctx, a, flipPayload)
	if err != nil {
		t.Fatalf("enqueue maintenance: %v", err)
	}
	writeID, err := r.EnqueueWriteback(ctx, b, writePayload, "")
	if err != nil {
		t.Fatalf("enqueue write: %v", err)
	}

	w, err := r.ClaimNextWriteback(ctx)
	if err != nil || w == nil || w.ID != writeID || w.Kind != repo.WritebackKindWrite {
		t.Fatalf("write lane claimed %+v (err %v), want write job %d", w, err, writeID)
	}
	if again, _ := r.ClaimNextWriteback(ctx); again != nil {
		t.Errorf("write lane claimed maintenance job %+v", again)
	}
	m, err := r.ClaimNextMaintenance(ctx)
	if err != nil || m == nil || m.ID != flipID || m.Kind != repo.WritebackKindMaintenance || m.Payload != flipPayload {
		t.Fatalf("maintenance lane claimed %+v (err %v), want maintenance job %d", m, err, flipID)
	}
}

// A tag write enqueued during a flip of the same file waits; a write for another file
// proceeds; the waiting write becomes claimable once the flip finishes.
func TestClaim_WriteWaitsForRunningMaintenanceOnSameVideo(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a := seedLaneVideo(t, r, "a.mkv")
	b := seedLaneVideo(t, r, "b.mkv")

	if _, err := r.EnqueueMaintenance(ctx, a, flipPayload); err != nil {
		t.Fatalf("enqueue maintenance: %v", err)
	}
	flip, err := r.ClaimNextMaintenance(ctx)
	if err != nil || flip == nil {
		t.Fatalf("claim maintenance: %+v %v", flip, err)
	}
	writeA, _ := r.EnqueueWriteback(ctx, a, writePayload, "")
	writeB, _ := r.EnqueueWriteback(ctx, b, writePayload, "")

	got, err := r.ClaimNextWriteback(ctx)
	if err != nil || got == nil || got.ID != writeB {
		t.Fatalf("claimed %+v (err %v), want video B's write %d (A's must wait)", got, err, writeB)
	}
	if none, _ := r.ClaimNextWriteback(ctx); none != nil {
		t.Fatalf("claimed %+v while A's flip is running", none)
	}

	if err := r.FinishWriteback(ctx, flip.ID, true, ""); err != nil {
		t.Fatalf("finish flip: %v", err)
	}
	got, err = r.ClaimNextWriteback(ctx)
	if err != nil || got == nil || got.ID != writeA {
		t.Fatalf("after the flip finished claimed %+v (err %v), want A's write %d", got, err, writeA)
	}
}

// The reverse: a flip waits for a running tag write on the same file.
func TestClaim_MaintenanceWaitsForRunningWriteOnSameVideo(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a := seedLaneVideo(t, r, "a.mkv")

	if _, err := r.EnqueueWriteback(ctx, a, writePayload, ""); err != nil {
		t.Fatalf("enqueue write: %v", err)
	}
	write, err := r.ClaimNextWriteback(ctx)
	if err != nil || write == nil {
		t.Fatalf("claim write: %+v %v", write, err)
	}
	if _, err := r.EnqueueMaintenance(ctx, a, flipPayload); err != nil {
		t.Fatalf("enqueue maintenance: %v", err)
	}
	if got, _ := r.ClaimNextMaintenance(ctx); got != nil {
		t.Fatalf("claimed flip %+v while A's write is running", got)
	}
	if err := r.FinishWriteback(ctx, write.ID, true, ""); err != nil {
		t.Fatalf("finish write: %v", err)
	}
	if got, err := r.ClaimNextMaintenance(ctx); err != nil || got == nil {
		t.Fatalf("flip not claimable after the write finished: %+v %v", got, err)
	}
}

// Two tag writes for one file never run at once either, whatever WRITEBACK_CONCURRENCY is.
func TestClaim_SecondWriteForSameVideoWaits(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a := seedLaneVideo(t, r, "a.mkv")
	_, _ = r.EnqueueWriteback(ctx, a, writePayload, "")
	_, _ = r.EnqueueWriteback(ctx, a, writePayload, "")
	if first, _ := r.ClaimNextWriteback(ctx); first == nil {
		t.Fatal("first write not claimed")
	}
	if second, _ := r.ClaimNextWriteback(ctx); second != nil {
		t.Errorf("second write for the same video claimed while the first runs: %+v", second)
	}
}

func TestEnqueueMaintenance_RefusesSecondInFlight(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a := seedLaneVideo(t, r, "a.mkv")

	if _, err := r.EnqueueMaintenance(ctx, a, flipPayload); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	if _, err := r.EnqueueMaintenance(ctx, a, flipPayload); !errors.Is(err, repo.ErrMaintenanceInFlight) {
		t.Fatalf("second enqueue while pending = %v, want ErrMaintenanceInFlight", err)
	}
	job, _ := r.ClaimNextMaintenance(ctx)
	if _, err := r.EnqueueMaintenance(ctx, a, flipPayload); !errors.Is(err, repo.ErrMaintenanceInFlight) {
		t.Fatalf("second enqueue while running = %v, want ErrMaintenanceInFlight", err)
	}
	// A failed row is no longer in flight: the owner may queue again, and the new job
	// replaces the failure, so a later Retry can't revive it as a second flip.
	if err := r.FinishWriteback(ctx, job.ID, false, "boom"); err != nil {
		t.Fatalf("fail job: %v", err)
	}
	if _, err := r.EnqueueMaintenance(ctx, a, flipPayload); err != nil {
		t.Fatalf("enqueue after failure: %v", err)
	}
	if n, err := r.RetryFailedWriteback(ctx, a); err != nil || n != 0 {
		t.Fatalf("retry after re-enqueue reset %d rows (err %v); the failure should be gone", n, err)
	}
	if _, err := r.ClaimNextMaintenance(ctx); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if second, _ := r.ClaimNextMaintenance(ctx); second != nil {
		t.Fatalf("a second maintenance job for the same video exists: %+v", second)
	}
	// A pending tag write never blocks a maintenance enqueue.
	b := seedLaneVideo(t, r, "b.mkv")
	_, _ = r.EnqueueWriteback(ctx, b, writePayload, "")
	if _, err := r.EnqueueMaintenance(ctx, b, flipPayload); err != nil {
		t.Fatalf("enqueue beside a pending write: %v", err)
	}
}

func TestRecoverRunningWritebacks_ResetsBothKinds(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a := seedLaneVideo(t, r, "a.mkv")
	b := seedLaneVideo(t, r, "b.mkv")
	_, _ = r.EnqueueMaintenance(ctx, a, flipPayload)
	_, _ = r.EnqueueWriteback(ctx, b, writePayload, "")
	if m, _ := r.ClaimNextMaintenance(ctx); m == nil {
		t.Fatal("claim maintenance")
	}
	if w, _ := r.ClaimNextWriteback(ctx); w == nil {
		t.Fatal("claim write")
	}
	n, err := r.RecoverRunningWritebacks(ctx)
	if err != nil || n != 2 {
		t.Fatalf("recovered %d (err %v), want 2", n, err)
	}
	if m, _ := r.ClaimNextMaintenance(ctx); m == nil {
		t.Error("maintenance job not claimable after recovery")
	}
	if w, _ := r.ClaimNextWriteback(ctx); w == nil {
		t.Error("write job not claimable after recovery")
	}
}

func TestPendingWritebackCount_CountsOnlyWrites(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a := seedLaneVideo(t, r, "a.mkv")
	b := seedLaneVideo(t, r, "b.mkv")
	_, _ = r.EnqueueMaintenance(ctx, a, flipPayload)
	_, _ = r.EnqueueWriteback(ctx, b, writePayload, "")
	n, err := r.PendingWritebackCount(ctx)
	if err != nil || n != 1 {
		t.Fatalf("depth = %d (err %v), want 1: a flip must not push a tag write's queue position", n, err)
	}
}
