package repo_test

import (
	"context"
	"path/filepath"
	"testing"
)

func TestWritebackSnapshots_RoundTrip(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "v.mp4"), "Original Title", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	if err := r.InsertWritebackSnapshots(ctx, id, "batch-1", map[string]string{
		"title":  "Original Title",
		"studio": "",
	}); err != nil {
		t.Fatalf("insert snapshots: %v", err)
	}

	snaps, err := r.SnapshotsForBatch(ctx, "batch-1")
	if err != nil {
		t.Fatalf("snapshots for batch: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 snapshot rows, got %d: %+v", len(snaps), snaps)
	}
	byField := map[string]string{}
	for _, s := range snaps {
		if s.VideoID != id {
			t.Errorf("want video id %d, got %d", id, s.VideoID)
		}
		if s.WrittenAt.IsZero() {
			t.Errorf("want written_at set, got zero")
		}
		byField[s.FieldKey] = s.PriorValue
	}
	if byField["title"] != "Original Title" || byField["studio"] != "" {
		t.Errorf("unexpected values: %+v", byField)
	}
}

func TestWritebackSnapshots_UnknownBatchIsEmpty(t *testing.T) {
	r := newRepo(t)
	snaps, err := r.SnapshotsForBatch(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatalf("snapshots for batch: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("want no rows for unknown batch, got %+v", snaps)
	}
}

func TestWritebackSnapshots_EmptyMapIsNoop(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "v.mp4"), "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.InsertWritebackSnapshots(ctx, id, "batch-empty", nil); err != nil {
		t.Fatalf("insert empty snapshots: %v", err)
	}
	snaps, err := r.SnapshotsForBatch(ctx, "batch-empty")
	if err != nil {
		t.Fatalf("snapshots for batch: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("want no rows, got %+v", snaps)
	}
}

// TestSnapshotExistsForVideo_ScopedPerVideo is the F48.8 shared-batch-id
// correctness guard: a batch spanning several videos (merge propagation) must
// let snapshotBeforeWrite's own-job idempotency check see only the video it's
// currently processing, not any sibling video's already-taken snapshot in the
// same batch — otherwise the second/third video's job would wrongly believe
// it already snapshotted and skip taking its own.
func TestSnapshotExistsForVideo_ScopedPerVideo(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "a.mp4"), "A", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video a: %v", err)
	}
	b, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "b.mp4"), "B", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video b: %v", err)
	}
	if err := r.InsertWritebackSnapshots(ctx, a, "merge-batch", map[string]string{"actors": "Old Name"}); err != nil {
		t.Fatalf("insert snapshot for a: %v", err)
	}

	// Video A already has a snapshot under the shared batch; video B does not yet.
	aExists, err := r.SnapshotExistsForVideo(ctx, "merge-batch", a)
	if err != nil || !aExists {
		t.Fatalf("video a exists = %v, err=%v, want true", aExists, err)
	}
	bExists, err := r.SnapshotExistsForVideo(ctx, "merge-batch", b)
	if err != nil || bExists {
		t.Fatalf("video b exists = %v, err=%v, want false (must not see video a's)", bExists, err)
	}

	// Whereas the batch-wide read (Revert's input) sees both once B is added.
	if err := r.InsertWritebackSnapshots(ctx, b, "merge-batch", map[string]string{"actors": "Other Name"}); err != nil {
		t.Fatalf("insert snapshot for b: %v", err)
	}
	all, err := r.SnapshotsForBatch(ctx, "merge-batch")
	if err != nil || len(all) != 2 {
		t.Fatalf("batch-wide snapshots = %+v, err=%v, want 2 (both videos)", all, err)
	}
}

// TestWritebackSnapshots_VideoDeleteCascades confirms the AFTER DELETE trigger
// (migration 0026) prunes a deleted video's snapshots, mirroring
// metadata_extraction_review's cascade (0025).
func TestWritebackSnapshots_VideoDeleteCascades(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "v.mp4"), "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.InsertWritebackSnapshots(ctx, id, "batch-cascade", map[string]string{"title": "T"}); err != nil {
		t.Fatalf("insert snapshots: %v", err)
	}
	if err := r.HardDelete(ctx, id); err != nil {
		t.Fatalf("hard delete video: %v", err)
	}
	snaps, err := r.SnapshotsForBatch(ctx, "batch-cascade")
	if err != nil {
		t.Fatalf("snapshots for batch: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("want snapshots cascade-deleted with the video, got %+v", snaps)
	}
}

// TestHardDelete_WritebackHistoryAndQueueCascade guards against a regression
// found in production (2026-08-21): file_writebacks (0011) and writeback_queue
// (0014) were created without ON DELETE CASCADE, so HardDelete failed with a
// FOREIGN KEY constraint error for any video with writeback history — silently
// wedging the purge job into an infinite hourly retry loop. Migration 0042 adds
// the missing CASCADE; this confirms HardDelete now succeeds and cleans up both
// tables, matching video_people/video_tags/video_metadata's existing behavior.
func TestHardDelete_WritebackHistoryAndQueueCascade(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "v.mp4"), "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.InsertWriteback(ctx, id, "title", "Title", "T", "filename"); err != nil {
		t.Fatalf("insert file_writebacks row: %v", err)
	}
	if _, err := r.EnqueueWriteback(ctx, id, `{"title":"T"}`, "batch-cascade"); err != nil {
		t.Fatalf("enqueue writeback_queue row: %v", err)
	}

	if err := r.HardDelete(ctx, id); err != nil {
		t.Fatalf("hard delete video with writeback history: %v", err)
	}
}
