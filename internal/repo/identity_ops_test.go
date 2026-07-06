package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// studioIDByName finds a seeded studio's id via the list (studios with active videos).
func studioIDByName(t *testing.T, r *repo.Repo, name string) int64 {
	t.Helper()
	studios, err := r.ListStudios(context.Background(), false)
	if err != nil {
		t.Fatalf("list studios: %v", err)
	}
	for _, s := range studios {
		if s.Name == name {
			return s.ID
		}
	}
	t.Fatalf("studio %q not found in %v", name, studioNames(t, r))
	return 0
}

func tagIDByName(t *testing.T, r *repo.Repo, name string) int64 {
	t.Helper()
	id, ok, err := r.TagIDByName(context.Background(), name)
	if err != nil {
		t.Fatalf("tag id: %v", err)
	}
	if !ok {
		t.Fatalf("tag %q not found", name)
	}
	return id
}

func TestStudioAliasCRUD(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	wb := seedStudio(t, r, "Warner Bros.")

	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityStudio, wb, "WB"); err != nil {
		t.Fatalf("add alias: %v", err)
	}
	// Idempotent, case/whitespace-folded: "  wb " is the same alias.
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityStudio, wb, "  wb "); err != nil {
		t.Fatalf("re-add alias: %v", err)
	}
	aliases, err := r.AliasesForEntity(ctx, model.EnrichEntityStudio, wb)
	if err != nil {
		t.Fatalf("list aliases: %v", err)
	}
	if len(aliases) != 1 || aliases[0].Alias != "WB" {
		t.Fatalf("aliases = %+v, want one 'WB'", aliases)
	}
	// Detail read carries the alias.
	if s, _ := r.GetStudio(ctx, wb); len(s.Aliases) != 1 {
		t.Errorf("GetStudio aliases = %+v, want one", s.Aliases)
	}
	// Delete: scoped + 404 on unknown.
	if err := r.DeleteEntityAlias(ctx, model.EnrichEntityStudio, wb, 99999); err != repo.ErrNotFound {
		t.Errorf("delete unknown = %v, want ErrNotFound", err)
	}
	if err := r.DeleteEntityAlias(ctx, model.EnrichEntityStudio, wb, aliases[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if a, _ := r.AliasesForEntity(ctx, model.EnrichEntityStudio, wb); len(a) != 0 {
		t.Errorf("after delete = %+v, want none", a)
	}
}

func TestTagAliasCRUD(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, []string{"sci-fi"}), nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	scifi := tagIDByName(t, r, "sci-fi")

	if _, err := r.AddEntityAlias(ctx, model.EntityTag, scifi, "science fiction"); err != nil {
		t.Fatalf("add alias: %v", err)
	}
	// Tag folding collapses internal whitespace: "ScienceFiction" == "science fiction".
	if _, err := r.AddEntityAlias(ctx, model.EntityTag, scifi, "Science Fiction"); err != nil {
		t.Fatalf("re-add alias: %v", err)
	}
	if a, _ := r.AliasesForEntity(ctx, model.EntityTag, scifi); len(a) != 1 {
		t.Fatalf("tag aliases = %+v, want one (whitespace-folded)", a)
	}
	// The tags list carries aliases (no detail page, RD7).
	tags, err := r.ListTags(ctx, false)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) != 1 || len(tags[0].Aliases) != 1 {
		t.Errorf("ListTags aliases = %+v, want one on the sole tag", tags)
	}
}

func TestStudioMergeSurvivesRederivation(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	va, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	vb, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)
	if err := r.ReconcileVideoStudios(ctx, va, []string{"Warner Bros."}, nil); err != nil {
		t.Fatalf("reconcile a: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, vb, []string{"WB"}, nil); err != nil {
		t.Fatalf("reconcile b: %v", err)
	}
	warner := studioIDByName(t, r, "Warner Bros.")
	wb := studioIDByName(t, r, "WB")

	if err := r.MergeEntities(ctx, model.EnrichEntityStudio, warner, wb); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if _, err := r.GetStudio(ctx, wb); err != repo.ErrNotFound {
		t.Errorf("merged studio still present: %v", err)
	}
	s, err := r.GetStudio(ctx, warner)
	if err != nil {
		t.Fatalf("get survivor: %v", err)
	}
	if s.VideoCount != 2 {
		t.Errorf("survivor video count = %d, want 2 (union)", s.VideoCount)
	}
	if len(s.Aliases) != 1 || s.Aliases[0].Alias != "WB" {
		t.Errorf("aliases after merge = %+v, want [WB]", s.Aliases)
	}

	// The load-bearing guarantee (RD6, QA 2.7): video B's resolved studio is still "WB".
	// Re-deriving its link must route through the alias to the survivor — never recreate WB.
	if err := r.ReconcileVideoStudios(ctx, vb, []string{"WB"}, nil); err != nil {
		t.Fatalf("re-derive: %v", err)
	}
	if got := studioNames(t, r); len(got) != 1 || got[0] != "Warner Bros." {
		t.Fatalf("re-derivation resurrected the merged studio: %v", got)
	}
	if s, _ := r.GetStudio(ctx, warner); s.VideoCount != 2 {
		t.Errorf("after re-derivation count = %d, want 2", s.VideoCount)
	}
}

func TestTagMergeSurvivesRescan(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, []string{"sci-fi"}), nil); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, []string{"science fiction"}), nil); err != nil {
		t.Fatalf("seed b: %v", err)
	}
	scifi := tagIDByName(t, r, "sci-fi")
	sf := tagIDByName(t, r, "science fiction")

	if err := r.MergeEntities(ctx, model.EntityTag, scifi, sf); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if _, err := r.GetTag(ctx, sf); err != repo.ErrNotFound {
		t.Errorf("merged tag still present: %v", err)
	}
	tg, _ := r.GetTag(ctx, scifi)
	if tg.VideoCount != 2 {
		t.Errorf("survivor count = %d, want 2", tg.VideoCount)
	}
	if len(tg.Aliases) != 1 || tg.Aliases[0].Alias != "science fiction" {
		t.Errorf("aliases after merge = %+v, want [science fiction]", tg.Aliases)
	}

	// Re-scan a file still tagged "science fiction" → routes to sci-fi, no re-creation.
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/c.mkv", "C", nil, []string{"science fiction"}), nil); err != nil {
		t.Fatalf("re-scan: %v", err)
	}
	if _, ok, _ := r.TagIDByName(ctx, "science fiction"); ok {
		t.Error("re-scan resurrected the merged tag")
	}
	if tg, _ := r.GetTag(ctx, scifi); tg.VideoCount != 3 {
		t.Errorf("after re-scan count = %d, want 3", tg.VideoCount)
	}
}

func TestRenameStudioKeepsOldNameAsAlias(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	acme := seedStudio(t, r, "Acme")

	cid, err := r.RenameEntity(ctx, model.EnrichEntityStudio, acme, "Acme Studios")
	if err != nil || cid != 0 {
		t.Fatalf("rename = (%d, %v), want (0, nil)", cid, err)
	}
	s, _ := r.GetStudio(ctx, acme)
	if s.Name != "Acme Studios" {
		t.Errorf("name = %q, want 'Acme Studios'", s.Name)
	}
	if len(s.Aliases) != 1 || s.Aliases[0].Alias != "Acme" {
		t.Errorf("aliases = %+v, want [Acme]", s.Aliases)
	}

	// A rename whose nameKey collides with another studio is refused (ErrNameTaken) and
	// carries that studio's id — never a silent merge.
	beta := seedStudio(t, r, "Beta")
	cid, err = r.RenameEntity(ctx, model.EnrichEntityStudio, beta, "acme studios")
	if err != repo.ErrNameTaken || cid != acme {
		t.Fatalf("colliding rename = (%d, %v), want (%d, ErrNameTaken)", cid, err, acme)
	}
	if s, _ := r.GetStudio(ctx, beta); s.Name != "Beta" {
		t.Errorf("refused rename mutated the studio: %q", s.Name)
	}
}

func TestRenameTagInternalWhitespaceConflict(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, []string{"sci fi", "documentary"}), nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	scifi := tagIDByName(t, r, "sci fi")
	doc := tagIDByName(t, r, "documentary")

	// "SciFi" folds (internal whitespace) to the same key as "sci fi" — a conflict.
	cid, err := r.RenameEntity(ctx, model.EntityTag, doc, "SciFi")
	if err != repo.ErrNameTaken || cid != scifi {
		t.Fatalf("colliding tag rename = (%d, %v), want (%d, ErrNameTaken)", cid, err, scifi)
	}
}

func TestEntityConflictExcludesSelf(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	fox := seedStudio(t, r, "Fox")

	// "fox" (any casing) resolves to the existing studio when asked from a different id.
	id, found, err := r.EntityConflict(ctx, model.EnrichEntityStudio, 0, "  FOX ")
	if err != nil || !found || id != fox {
		t.Fatalf("conflict = (%d, %v, %v), want (%d, true, nil)", id, found, err, fox)
	}
	// Excluding the studio itself: the name is free.
	if _, found, _ := r.EntityConflict(ctx, model.EnrichEntityStudio, fox, "fox"); found {
		t.Error("conflict should exclude selfID")
	}
}

func TestKeepSeparateStore(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if err := r.AddKeepSeparate(ctx, model.EntityTag, 9, 5); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Order-independent lookup.
	if ok, _ := r.IsKeptSeparate(ctx, model.EntityTag, 5, 9); !ok {
		t.Error("pair should be kept-separate regardless of order")
	}
	// Idempotent re-add (reversed) is fine.
	if err := r.AddKeepSeparate(ctx, model.EntityTag, 5, 9); err != nil {
		t.Errorf("idempotent re-add: %v", err)
	}
	// Type-scoped: the same ids under another entity type are not kept-separate.
	if ok, _ := r.IsKeptSeparate(ctx, model.EnrichEntityStudio, 5, 9); ok {
		t.Error("keep-separate must be entity-type scoped")
	}
	// A different pair is not marked.
	if ok, _ := r.IsKeptSeparate(ctx, model.EntityTag, 5, 8); ok {
		t.Error("unrelated pair reported kept-separate")
	}
	// A pair with itself is rejected.
	if err := r.AddKeepSeparate(ctx, model.EntityTag, 7, 7); err == nil {
		t.Error("keep-separate with self should error")
	}
}

func TestMergeEntitiesValidation(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	acme := seedStudio(t, r, "Acme")

	if err := r.MergeEntities(ctx, model.EnrichEntityStudio, acme, acme); err == nil {
		t.Error("self-merge should error")
	}
	if err := r.MergeEntities(ctx, model.EnrichEntityStudio, acme, 99999); err != repo.ErrNotFound {
		t.Errorf("merge unknown = %v, want ErrNotFound", err)
	}
	if err := r.MergeEntities(ctx, "bogus", acme, 1); err == nil {
		t.Error("unknown entity type should error")
	}
}
