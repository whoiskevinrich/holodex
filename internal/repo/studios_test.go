package repo_test

import (
	"context"
	"testing"

	"holodex/internal/repo"
)

// studioNames returns the sorted studio names currently in the list (active-video
// counts > 0), for compact assertions.
func studioNames(t *testing.T, r *repo.Repo) []string {
	t.Helper()
	studios, err := r.ListStudios(context.Background(), false)
	if err != nil {
		t.Fatalf("list studios: %v", err)
	}
	names := make([]string, len(studios))
	for i, s := range studios {
		names[i] = s.Name
	}
	return names
}

func TestReconcileVideoStudios_CreateReplacePrune(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Create: link to "Acme".
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme"}); err != nil {
		t.Fatalf("reconcile create: %v", err)
	}
	if got := studioNames(t, r); len(got) != 1 || got[0] != "Acme" {
		t.Fatalf("after create: %v, want [Acme]", got)
	}

	// Idempotent: same input, same result, no duplicate rows.
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme"}); err != nil {
		t.Fatalf("reconcile idempotent: %v", err)
	}
	if got := studioNames(t, r); len(got) != 1 || got[0] != "Acme" {
		t.Fatalf("after idempotent: %v, want [Acme]", got)
	}

	// Replace: adopt "Acme Films" — "Acme" is now unlinked and must be pruned.
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme Films"}); err != nil {
		t.Fatalf("reconcile replace: %v", err)
	}
	if got := studioNames(t, r); len(got) != 1 || got[0] != "Acme Films" {
		t.Fatalf("after replace: %v, want [Acme Films] (Acme pruned)", got)
	}

	// Empty (blank-pin / soft-delete): all links dropped and the studio pruned.
	if err := r.ReconcileVideoStudios(ctx, id, nil); err != nil {
		t.Fatalf("reconcile empty: %v", err)
	}
	if got := studioNames(t, r); len(got) != 0 {
		t.Fatalf("after empty: %v, want []", got)
	}
}

func TestReconcileVideoStudios_SharedStudioNotPruned(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	a, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	b, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)

	// Two videos resolve to the same studio → one row, count 2.
	if err := r.ReconcileVideoStudios(ctx, a, []string{"Ghibli"}); err != nil {
		t.Fatalf("reconcile a: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, b, []string{"Ghibli"}); err != nil {
		t.Fatalf("reconcile b: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(studios) != 1 || studios[0].Name != "Ghibli" || studios[0].VideoCount != 2 {
		t.Fatalf("studios = %+v, want one Ghibli count=2", studios)
	}

	// Fixing only A leaves Ghibli alive (B still links it) — no premature prune.
	if err := r.ReconcileVideoStudios(ctx, a, nil); err != nil {
		t.Fatalf("reconcile a empty: %v", err)
	}
	studios, _ = r.ListStudios(ctx, false)
	if len(studios) != 1 || studios[0].VideoCount != 1 {
		t.Fatalf("after fixing A: %+v, want Ghibli count=1", studios)
	}
}

func TestReconcileVideoStudios_MultiValue(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)

	// A multi-mapped studio resolves to several names → one link each.
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Legendary", "Warner Bros.", ""}); err != nil {
		t.Fatalf("reconcile multi: %v", err)
	}
	got := studioNames(t, r)
	if len(got) != 2 { // the empty string is dropped
		t.Fatalf("multi studios = %v, want 2 (empty dropped)", got)
	}
}

func TestGetStudio_NotFound(t *testing.T) {
	r := newRepo(t)
	if _, err := r.GetStudio(context.Background(), 9999); err != repo.ErrNotFound {
		t.Fatalf("GetStudio(missing) err = %v, want ErrNotFound", err)
	}
}

func TestStudioLinkCount_GatesBackfill(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if n, err := r.StudioLinkCount(ctx); err != nil || n != 0 {
		t.Fatalf("initial link count = %d err=%v, want 0", n, err)
	}
	id, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme"}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if n, _ := r.StudioLinkCount(ctx); n != 1 {
		t.Fatalf("after link, count = %d, want 1", n)
	}
}
