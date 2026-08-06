package api_test

import (
	"context"
	"testing"

	"holodex/internal/repo"
)

// TestRelinkVideoPeople_UnmappedFieldLeavesExistingLinksUntouched reproduces
// HOLODEX-256: an instance whose metadata-mappings.yaml doesn't map actors/director
// (predates F40, or the operator simply hasn't added them) must not have every
// person unlinked from every video the next time RelinkVideoPeople runs (every scan,
// plus the one-time boot backfill). "Not configured" must mean "no opinion," never
// "resolved to nobody." genresServer (tag_materialize_test.go) already wires a repo
// + mapping config that maps `genres` but never actors/director — exactly the shape
// of a pre-F40 config this bug needs.
func TestRelinkVideoPeople_UnmappedFieldLeavesExistingLinksUntouched(t *testing.T) {
	h, r, vid := genresServer(t)
	ctx := context.Background()

	// Seed a pre-existing person link directly through the repo -- standing in for a
	// link that predates this boot (raw extraction, an earlier config, or a prior
	// successful relink) that the config gap must not touch.
	if err := r.ReconcileVideoPeople(ctx, vid, []repo.PersonRoleName{{Name: "Jane Doe"}}, nil); err != nil {
		t.Fatalf("seed person link: %v", err)
	}
	before, err := r.ListPeople(ctx, false)
	if err != nil {
		t.Fatalf("list people (before): %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("seed: got %d people, want 1", len(before))
	}

	// RelinkVideoPeople is what runs on every scan and the boot-time backfill. With
	// actors/director unmapped it must no-op, not wipe.
	if err := h.RelinkVideoPeople(ctx, vid); err != nil {
		t.Fatalf("relink video people: %v", err)
	}

	after, err := r.ListPeople(ctx, false)
	if err != nil {
		t.Fatalf("list people (after): %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("relink with unmapped actors/director wiped existing links: got %d people, want 1", len(after))
	}
}
