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
