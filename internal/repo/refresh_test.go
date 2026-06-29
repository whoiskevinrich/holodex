package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/repo"
)

// RefreshTarget resolves a live row to its path, and distinguishes a missing row
// (ErrNotFound → 404) from a soft-deleted one (ErrDeleted → 409) so a refresh
// never re-reads or reactivates a trashed item (F31, ADR-047; ADR-037 #26 guard).
func TestRefreshTarget(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, sampleVideo("/m/clip.mp4", "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	path, err := r.RefreshTarget(ctx, id)
	if err != nil || path != "/m/clip.mp4" {
		t.Fatalf("live target: path=%q err=%v", path, err)
	}

	if _, err := r.RefreshTarget(ctx, id+1000); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("unknown id: want ErrNotFound, got %v", err)
	}

	if err := r.SoftDelete(ctx, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := r.RefreshTarget(ctx, id); !errors.Is(err, repo.ErrDeleted) {
		t.Fatalf("soft-deleted: want ErrDeleted, got %v", err)
	}
}
