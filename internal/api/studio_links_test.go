package api_test

import (
	"context"
	"testing"
)

// TestRelinkVideoStudios_UnmappedFieldLeavesExistingLinksUntouched is the studio
// counterpart of TestRelinkVideoPeople_UnmappedFieldLeavesExistingLinksUntouched
// (HOLODEX-256, PR #216): an instance whose metadata-mappings.yaml doesn't map
// `studio` must not have every studio link (and every studio entity) permanently
// deleted the next time RelinkVideoStudios runs (every scan, plus the one-time
// boot backfill). "Not configured" must mean "no opinion," never "resolved to
// nobody." This case is sharper than person's: ReconcileVideoStudios prunes
// immediately with no orphan grace (ADR-053 §2.4), so the pre-fix wipe here was
// not just a display gap but permanent data loss. genresServer (tag_materialize_
// test.go) already wires a repo + mapping config that maps `genres` but never
// `studio` — exactly the shape of a config gap this bug needs.
func TestRelinkVideoStudios_UnmappedFieldLeavesExistingLinksUntouched(t *testing.T) {
	h, r, vid := genresServer(t)
	ctx := context.Background()

	// Seed a pre-existing studio link directly through the repo -- standing in for
	// a link that predates this boot (raw extraction, an earlier config, or a
	// prior successful relink) that the config gap must not touch.
	if err := r.ReconcileVideoStudios(ctx, vid, []string{"Acme"}, nil); err != nil {
		t.Fatalf("seed studio link: %v", err)
	}
	before, err := r.ListStudios(ctx, false)
	if err != nil {
		t.Fatalf("list studios (before): %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("seed: got %d studios, want 1", len(before))
	}

	// RelinkVideoStudios is what runs on every scan and the boot-time backfill.
	// With studio unmapped it must no-op, not wipe-and-prune.
	if err := h.RelinkVideoStudios(ctx, vid); err != nil {
		t.Fatalf("relink video studios: %v", err)
	}

	after, err := r.ListStudios(ctx, false)
	if err != nil {
		t.Fatalf("list studios (after): %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("relink with unmapped studio wiped existing links: got %d studios, want 1", len(after))
	}
}
