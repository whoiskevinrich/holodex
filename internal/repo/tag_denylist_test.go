package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/repo"
)

// TestTagDenylistCRUD covers list/add/remove and idempotent re-deny (ADR-075 D2).
func TestTagDenylistCRUD(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if terms, err := r.ListDeniedTags(ctx); err != nil || len(terms) != 0 {
		t.Fatalf("initial list = %v, %v, want empty", terms, err)
	}

	if err := r.DenyTag(ctx, "Gnome"); err != nil {
		t.Fatalf("deny: %v", err)
	}
	// Re-denying a case/whitespace variant is a no-op, not a duplicate row.
	if err := r.DenyTag(ctx, " gnome "); err != nil {
		t.Fatalf("re-deny: %v", err)
	}

	terms, err := r.ListDeniedTags(ctx)
	if err != nil || len(terms) != 1 {
		t.Fatalf("list after deny = %v, %v, want exactly one term", terms, err)
	}
	if terms[0].Term != "Gnome" {
		t.Errorf("stored term = %q, want original casing %q", terms[0].Term, "Gnome")
	}

	if err := r.RemoveDeniedTag(ctx, "GNOME"); err != nil {
		t.Fatalf("remove (case-insensitive): %v", err)
	}
	if terms, err := r.ListDeniedTags(ctx); err != nil || len(terms) != 0 {
		t.Fatalf("list after remove = %v, %v, want empty", terms, err)
	}
	if err := r.RemoveDeniedTag(ctx, "gnome"); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("remove already-removed = %v, want ErrNotFound", err)
	}
}

// TestIsTagDenied covers the read-only, non-tx check genre writeback (F50 S6,
// ADR-075 RD9) uses to filter the raw resolved genres union: same fold as the
// tx-scoped isTagDenied resolveOrCreateByName gates new tags with (case-fold
// match, not substring), just callable outside a write transaction.
func TestIsTagDenied(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if err := r.DenyTag(ctx, "TV Movie"); err != nil {
		t.Fatalf("deny: %v", err)
	}
	if denied, err := r.IsTagDenied(ctx, "tv movie"); err != nil || !denied {
		t.Errorf("case-fold match = %v, %v, want true", denied, err)
	}
	if denied, err := r.IsTagDenied(ctx, "Movie"); err != nil || denied {
		t.Errorf("substring non-match = %v, %v, want false", denied, err)
	}
}

// TestDeniedTagBlocksScanSilently proves the single-choke-point enforcement
// (ADR-075 D2): a denied term is skipped, not errored, when the scanner
// (replaceAssociations, via UpsertVideo) encounters it, and it never becomes a
// tags row. A sibling, non-denied tag on the same video still attaches
// normally, and denying "gnome" must not block "garden gnome" (exact-string,
// not substring).
func TestDeniedTagBlocksScanSilently(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if err := r.DenyTag(ctx, "gnome"); err != nil {
		t.Fatalf("deny: %v", err)
	}

	id, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", nil, []string{"gnome", "garden gnome", "documentary"}), nil)
	if err != nil {
		t.Fatalf("upsert with denied tag present: %v", err)
	}

	got, _, err := r.GetVideo(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	names := make(map[string]bool, len(got.Tags))
	for _, tg := range got.Tags {
		names[tg.Name] = true
	}
	if names["gnome"] {
		t.Errorf("denied term became a tag: %+v", got.Tags)
	}
	if !names["garden gnome"] {
		t.Errorf("deny-list matched as substring, not exact string: %+v", got.Tags)
	}
	if !names["documentary"] {
		t.Errorf("sibling non-denied tag missing: %+v", got.Tags)
	}
	if len(got.Tags) != 2 {
		t.Errorf("want exactly 2 surviving tags, got %d: %+v", len(got.Tags), got.Tags)
	}
}
