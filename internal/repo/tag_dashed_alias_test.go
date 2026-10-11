package repo_test

import (
	"context"
	"slices"
	"sort"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// tagAliases returns tag id's alias texts, sorted.
func tagAliases(t *testing.T, r *repo.Repo, id int64) []string {
	t.Helper()
	tg, err := r.GetTag(context.Background(), id)
	if err != nil {
		t.Fatalf("get tag %d: %v", id, err)
	}
	out := make([]string, 0, len(tg.Aliases))
	for _, a := range tg.Aliases {
		out = append(out, a.Alias)
	}
	sort.Strings(out)
	return out
}

// F43 P0-12: a scan-created multi-word tag carries its dashed spelling, and a later
// file spelled with dashes routes to it instead of creating a second tag.
func TestDashedTagAlias_ScanCreateAndRoute(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, []string{"Science Fiction"}), nil); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	sf := tagIDByName(t, r, "science fiction")
	if got := tagAliases(t, r, sf); !slices.Equal(got, []string{"science-fiction"}) {
		t.Fatalf("aliases = %v, want [science-fiction]", got)
	}

	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, []string{"Science-Fiction"}), nil); err != nil {
		t.Fatalf("seed b: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("tags = %d (err %v), want 1", n, err)
	}
	if tg, _ := r.GetTag(ctx, sf); tg.VideoCount != 2 {
		t.Errorf("video count = %d, want 2", tg.VideoCount)
	}
}

func TestDashedTagAlias_OnlyMultiWordNames(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	cases := []struct {
		name string
		want []string
	}{
		{"horror", []string{}},
		{"x-men", []string{}},
		{"sci   fi", []string{"sci-fi"}}, // a run of whitespace is one dash
	}
	for _, c := range cases {
		tg, err := r.ResolveOrCreateTag(ctx, c.name)
		if err != nil {
			t.Fatalf("create %q: %v", c.name, err)
		}
		if got := tagAliases(t, r, tg.ID); !slices.Equal(got, c.want) {
			t.Errorf("%q aliases = %v, want %v", c.name, got, c.want)
		}
	}
}

// RD13: a dashed spelling another tag already holds is skipped — never merged — and
// the pair is left to the Duplicates queue.
func TestDashedTagAlias_CollisionSkipsAndQueues(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	dashed, err := r.ResolveOrCreateTag(ctx, "science-fiction")
	if err != nil {
		t.Fatalf("create dashed: %v", err)
	}
	spaced, err := r.ResolveOrCreateTag(ctx, "science fiction")
	if err != nil {
		t.Fatalf("create spaced: %v", err)
	}
	if spaced.ID == dashed.ID {
		t.Fatal("spaced name resolved to the dashed tag")
	}
	if got := tagAliases(t, r, spaced.ID); len(got) != 0 {
		t.Errorf("spaced tag aliases = %v, want none", got)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM identity_review_queue WHERE entity_type = 'tag'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("tag review-queue rows = %d (err %v), want 1", n, err)
	}
}

func TestDashedTagAlias_RenameAddsAndKeepsOld(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	tg, err := r.ResolveOrCreateTag(ctx, "sci fi")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := r.RenameEntity(ctx, model.EntityTag, tg.ID, "Science Fiction"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	want := []string{"sci fi", "sci-fi", "science-fiction"}
	if got := tagAliases(t, r, tg.ID); !slices.Equal(got, want) {
		t.Errorf("aliases = %v, want %v", got, want)
	}
}

// Tags that predate P0-12 get their alias once; a re-run (the boot gate pruned) adds
// nothing, and a collision is skipped there too.
func TestBackfillDashedTagAliases(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	mustExec(t, db, `INSERT INTO tags (name) VALUES ('film noir'), ('horror'), ('a b'), ('a-b')`)
	noir := tagIDByName(t, r, "film noir")
	ab := tagIDByName(t, r, "a b")

	added, err := r.BackfillDashedTagAliases(ctx)
	if err != nil || added != 1 {
		t.Fatalf("backfill added %d (err %v), want 1", added, err)
	}
	if got := tagAliases(t, r, noir); !slices.Equal(got, []string{"film-noir"}) {
		t.Errorf("film noir aliases = %v, want [film-noir]", got)
	}
	if got := tagAliases(t, r, ab); len(got) != 0 {
		t.Errorf("colliding tag aliases = %v, want none", got)
	}
	if added, err := r.BackfillDashedTagAliases(ctx); err != nil || added != 0 {
		t.Errorf("re-run added %d (err %v), want 0", added, err)
	}
}

// Removing the dashed alias is durable: the backfill never brings it back.
func TestDashedTagAlias_RemovalNotReadded(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	tg, err := r.ResolveOrCreateTag(ctx, "science fiction")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	full, _ := r.GetTag(ctx, tg.ID)
	if len(full.Aliases) != 1 {
		t.Fatalf("aliases = %+v, want the dashed one", full.Aliases)
	}
	if err := r.DeleteEntityAlias(ctx, model.EntityTag, tg.ID, full.Aliases[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if added, err := r.BackfillDashedTagAliases(ctx); err != nil || added != 0 {
		t.Fatalf("backfill added %d (err %v), want 0", added, err)
	}
	if got := tagAliases(t, r, tg.ID); len(got) != 0 {
		t.Errorf("aliases after backfill = %v, want none", got)
	}
}

// A rename is the one thing that brings a removed dashed alias back (P0-12): renaming
// away and back to the name produces it again.
func TestDashedTagAlias_RenameReaddsRemoved(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	tg, err := r.ResolveOrCreateTag(ctx, "film noir")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	full, _ := r.GetTag(ctx, tg.ID)
	if err := r.DeleteEntityAlias(ctx, model.EntityTag, tg.ID, full.Aliases[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	for _, name := range []string{"noir", "film noir"} {
		if _, err := r.RenameEntity(ctx, model.EntityTag, tg.ID, name); err != nil {
			t.Fatalf("rename to %q: %v", name, err)
		}
	}
	if got := tagAliases(t, r, tg.ID); !slices.Contains(got, "film-noir") {
		t.Errorf("aliases = %v, want film-noir back", got)
	}
}

// The automatic alias is not authored data: an aged, unattached multi-word tag is swept.
func TestDashedTagAlias_DoesNotShieldOrphan(t *testing.T) {
	r, db := newRepoDB(t)
	tg, err := r.ResolveOrCreateTag(context.Background(), "film noir")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ageOrphan(t, db, "tags", tg.ID)
	if d, s := sweep(t, r); d != 1 || s != 0 {
		t.Fatalf("sweep = (%d deleted, %d skipped), want (1, 0)", d, s)
	}
}
