package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
)

// --- enrichment_dismissals store (F47, ADR-065 D2) -----------------------------------

func TestEnrichmentDismissals_RoundTrip(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Alice"}, nil), nil)
	alice := personIDByName(t, r, "Alice")

	if dismissed, err := r.EnrichmentDismissed(ctx, model.EnrichEntityPerson, alice, "tmdb"); err != nil || dismissed {
		t.Fatalf("before dismiss: dismissed=%v err=%v", dismissed, err)
	}

	// Dismissing is idempotent — re-dismissing refreshes, never duplicates or errors.
	if err := r.DismissEnrichment(ctx, model.EnrichEntityPerson, alice, "tmdb"); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if err := r.DismissEnrichment(ctx, model.EnrichEntityPerson, alice, "tmdb"); err != nil {
		t.Fatalf("re-dismiss: %v", err)
	}
	if dismissed, err := r.EnrichmentDismissed(ctx, model.EnrichEntityPerson, alice, "tmdb"); err != nil || !dismissed {
		t.Fatalf("after dismiss: dismissed=%v err=%v", dismissed, err)
	}
	// A different provider on the same entity is unaffected.
	if dismissed, err := r.EnrichmentDismissed(ctx, model.EnrichEntityPerson, alice, "other"); err != nil || dismissed {
		t.Fatalf("other provider: dismissed=%v err=%v", dismissed, err)
	}

	// Undismiss ("Try again") clears it; clearing an absent dismissal is a no-op success.
	if err := r.UndismissEnrichment(ctx, model.EnrichEntityPerson, alice, "tmdb"); err != nil {
		t.Fatalf("undismiss: %v", err)
	}
	if dismissed, err := r.EnrichmentDismissed(ctx, model.EnrichEntityPerson, alice, "tmdb"); err != nil || dismissed {
		t.Fatalf("after undismiss: dismissed=%v err=%v", dismissed, err)
	}
	if err := r.UndismissEnrichment(ctx, model.EnrichEntityPerson, alice, "tmdb"); err != nil {
		t.Fatalf("undismiss (no-op): %v", err)
	}
}

// A dismissal never outlives its entity — cascades on person merge (the loser is hard
// deleted) exactly as entity_enrichment/entity_aliases do.
func TestEnrichmentDismissals_CascadeOnPersonMerge(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Alice"}, nil), nil)
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Bob"}, nil), nil)
	alice := personIDByName(t, r, "Alice")
	bob := personIDByName(t, r, "Bob")

	if err := r.DismissEnrichment(ctx, model.EnrichEntityPerson, bob, "tmdb"); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if err := r.MergePersons(ctx, alice, bob); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if dismissed, err := r.EnrichmentDismissed(ctx, model.EnrichEntityPerson, bob, "tmdb"); err != nil || dismissed {
		t.Fatalf("merged-away dismissal must be gone: dismissed=%v err=%v", dismissed, err)
	}
}

// A dismissal never outlives its entity — cascades on a video's hard delete.
func TestEnrichmentDismissals_CascadeOnVideoHardDelete(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.DismissEnrichment(ctx, model.EnrichEntityVideo, vid, "tmdb"); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if err := r.HardDelete(ctx, vid); err != nil {
		t.Fatalf("hard delete: %v", err)
	}
	if dismissed, err := r.EnrichmentDismissed(ctx, model.EnrichEntityVideo, vid, "tmdb"); err != nil || dismissed {
		t.Fatalf("deleted video's dismissal must be gone: dismissed=%v err=%v", dismissed, err)
	}
}
