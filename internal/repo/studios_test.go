package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
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
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme"}, nil); err != nil {
		t.Fatalf("reconcile create: %v", err)
	}
	if got := studioNames(t, r); len(got) != 1 || got[0] != "Acme" {
		t.Fatalf("after create: %v, want [Acme]", got)
	}

	// Idempotent: same input, same result, no duplicate rows.
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme"}, nil); err != nil {
		t.Fatalf("reconcile idempotent: %v", err)
	}
	if got := studioNames(t, r); len(got) != 1 || got[0] != "Acme" {
		t.Fatalf("after idempotent: %v, want [Acme]", got)
	}

	// Replace: adopt "Acme Films" — "Acme" is now unlinked and must be pruned.
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme Films"}, nil); err != nil {
		t.Fatalf("reconcile replace: %v", err)
	}
	if got := studioNames(t, r); len(got) != 1 || got[0] != "Acme Films" {
		t.Fatalf("after replace: %v, want [Acme Films] (Acme pruned)", got)
	}

	// Empty (blank-pin / soft-delete): all links dropped and the studio pruned.
	if err := r.ReconcileVideoStudios(ctx, id, nil, nil); err != nil {
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
	if err := r.ReconcileVideoStudios(ctx, a, []string{"Ghibli"}, nil); err != nil {
		t.Fatalf("reconcile a: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, b, []string{"Ghibli"}, nil); err != nil {
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
	if err := r.ReconcileVideoStudios(ctx, a, nil, nil); err != nil {
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
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Legendary", "Warner Bros.", ""}, nil); err != nil {
		t.Fatalf("reconcile multi: %v", err)
	}
	got := studioNames(t, r)
	if len(got) != 2 { // the empty string is dropped
		t.Fatalf("multi studios = %v, want 2 (empty dropped)", got)
	}
}

// TestReconcileVideoStudios_ExternalIDDedup is the ADR-054 crux: two videos whose
// resolved studio names are DIFFERENT spellings of the SAME TMDB company (id 174)
// converge to one studio entity, because resolve-or-create matches external_id before
// name. A third video with a name and no id resolves by name only (the fallback).
func TestReconcileVideoStudios_ExternalIDDedup(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	a, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	b, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)

	// A: "Warner Bros." carrying tmdb:174 → creates the studio + attaches the id.
	if err := r.ReconcileVideoStudios(ctx, a, []string{"Warner Bros."},
		map[string]string{"Warner Bros.": "tmdb:174"}); err != nil {
		t.Fatalf("reconcile a: %v", err)
	}
	// B: a DIFFERENT spelling, SAME id → converges to A's studio, not a second entity.
	if err := r.ReconcileVideoStudios(ctx, b, []string{"Warner Bros. Pictures"},
		map[string]string{"Warner Bros. Pictures": "tmdb:174"}); err != nil {
		t.Fatalf("reconcile b: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(studios) != 1 || studios[0].VideoCount != 2 {
		t.Fatalf("studios = %+v, want ONE studio (id-deduped) count=2", studios)
	}
	// The canonical name is the first spelling to create it (deterministic, RD6).
	if studios[0].Name != "Warner Bros." {
		t.Fatalf("converged name = %q, want %q (first-seen)", studios[0].Name, "Warner Bros.")
	}

	// Prune-on-empty must cascade the id row: remove both videos, then a fresh video
	// with the same id re-creates a studio (proving the id was not orphaned/left over).
	if err := r.ReconcileVideoStudios(ctx, a, nil, nil); err != nil {
		t.Fatalf("clear a: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, b, nil, nil); err != nil {
		t.Fatalf("clear b: %v", err)
	}
	if got := studioNames(t, r); len(got) != 0 {
		t.Fatalf("after clearing both: %v, want [] (pruned)", got)
	}
	c, _ := r.UpsertVideo(ctx, sampleVideo("/m/c.mkv", "C", nil, nil), nil)
	if err := r.ReconcileVideoStudios(ctx, c, []string{"WB"},
		map[string]string{"WB": "tmdb:174"}); err != nil {
		t.Fatalf("reconcile c: %v", err)
	}
	if got := studioNames(t, r); len(got) != 1 || got[0] != "WB" {
		t.Fatalf("re-create after prune: %v, want [WB]", got)
	}
}

// TestReconcileVideoStudios_ExternalIDBackfill: a studio first created name-only later
// gains its id when a video supplies one for the same name; a subsequent different
// spelling with that id then converges onto it.
func TestReconcileVideoStudios_ExternalIDBackfill(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	a, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	b, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)
	c, _ := r.UpsertVideo(ctx, sampleVideo("/m/c.mkv", "C", nil, nil), nil)

	// A: name-only "Studio Ghibli" (no id).
	if err := r.ReconcileVideoStudios(ctx, a, []string{"Studio Ghibli"}, nil); err != nil {
		t.Fatalf("reconcile a: %v", err)
	}
	// B: same name, now WITH an id → back-fills the id onto the existing studio.
	if err := r.ReconcileVideoStudios(ctx, b, []string{"Studio Ghibli"},
		map[string]string{"Studio Ghibli": "tmdb:10342"}); err != nil {
		t.Fatalf("reconcile b: %v", err)
	}
	// C: a different spelling with the same id → converges (proves the back-fill took).
	if err := r.ReconcileVideoStudios(ctx, c, []string{"Ghibli"},
		map[string]string{"Ghibli": "tmdb:10342"}); err != nil {
		t.Fatalf("reconcile c: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(studios) != 1 || studios[0].Name != "Studio Ghibli" || studios[0].VideoCount != 3 {
		t.Fatalf("studios = %+v, want one Studio Ghibli count=3 (id back-filled)", studios)
	}
}

// studioByName returns the listed studio with the given name (or a fatal), so logo
// assertions don't depend on list order.
func studioByName(t *testing.T, r *repo.Repo, name string) model.Studio {
	t.Helper()
	studios, err := r.ListStudios(context.Background(), false)
	if err != nil {
		t.Fatalf("list studios: %v", err)
	}
	for _, s := range studios {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("studio %q not in list %+v", name, studios)
	return model.Studio{}
}

// TestListStudios_AttachesLogoVersion covers ADR-056 (superseding HOLODEX-126): the
// list attaches the cached logo row id as LogoVersion (the API turns it into the served
// URL); a studio with no cached logo carries LogoVersion 0, not another studio's logo.
func TestListStudios_AttachesLogoVersion(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	a, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	b, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)
	if err := r.ReconcileVideoStudios(ctx, a, []string{"Acme"}, nil); err != nil {
		t.Fatalf("reconcile a: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, b, []string{"Beta"}, nil); err != nil {
		t.Fatalf("reconcile b: %v", err)
	}

	acmeID := studioByName(t, r, "Acme").ID
	logoID, err := r.ReplaceStudioLogo(ctx, repo.StudioLogoInsert{
		StudioID: acmeID, SourceURL: "https://cdn.example/a.jpg", Provider: "tmdb",
		Width: 100, Height: 40, ByteSize: 999,
	})
	if err != nil {
		t.Fatalf("replace logo: %v", err)
	}

	if got := studioByName(t, r, "Acme").LogoVersion; got != logoID {
		t.Fatalf("Acme LogoVersion = %d, want %d", got, logoID)
	}
	// Beta has no cached logo → version 0.
	if got := studioByName(t, r, "Beta").LogoVersion; got != 0 {
		t.Fatalf("Beta LogoVersion = %d, want 0", got)
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
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme"}, nil); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if n, _ := r.StudioLinkCount(ctx); n != 1 {
		t.Fatalf("after link, count = %d, want 1", n)
	}
}
