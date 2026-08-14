package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// peopleCount / tagCount return the number of distinct entities with active videos —
// used to assert that case/whitespace variants converged onto one entity.
func peopleCount(t *testing.T, r *repo.Repo) int {
	t.Helper()
	people, err := r.ListPeople(context.Background(), false)
	if err != nil {
		t.Fatalf("list people: %v", err)
	}
	return len(people)
}

func tagCount(t *testing.T, r *repo.Repo) int {
	t.Helper()
	tags, err := r.ListTags(context.Background(), false)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	return len(tags)
}

// TestNameKeyConvergencePerson proves the "fox"/"Fox" fix (F43 P0-1/RD1): a person
// scanned under one casing and re-scanned under another resolves to the SAME person —
// case/edge-whitespace variants can never fork a second entity.
func TestNameKeyConvergencePerson(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	for _, name := range []string{"fox", "Fox", " fox ", "FOX"} {
		id, err := r.UpsertVideo(ctx, sampleVideo("/m/"+name+".mkv", "T", []string{name}, nil), nil)
		if err != nil {
			t.Fatalf("upsert %q: %v", name, err)
		}
		linkPeople(t, r, id, name)
	}
	if n := peopleCount(t, r); n != 1 {
		t.Fatalf("case/whitespace variants forked identity: got %d people, want 1", n)
	}
}

// TestExternalIDsForEntity proves the HOLODEX-266/ADR-083 badge-projection read: a
// person's attached external id (person_external_ids, ADR-055/F32) round-trips as
// the same namespace-qualified string it was attached with, and an entity with none
// yet reads back empty rather than erroring.
func TestExternalIDsForEntity(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", []string{"Denis Villeneuve"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert video: %v", err)
	}
	if err := r.ReconcileVideoPeople(ctx, vid,
		[]repo.PersonRoleName{{Name: "Denis Villeneuve", Role: "director"}},
		map[string]string{"Denis Villeneuve": "tmdb:137"}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	pid := personIDByName(t, r, "Denis Villeneuve")

	ids, err := r.ExternalIDsForEntity(ctx, model.EnrichEntityPerson, pid)
	if err != nil {
		t.Fatalf("external ids: %v", err)
	}
	if len(ids) != 1 || ids[0] != "tmdb:137" {
		t.Fatalf("external ids = %v, want [tmdb:137]", ids)
	}

	// A person with no attached external id reads back empty, not an error.
	other, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "T2", []string{"No External Id"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert video 2: %v", err)
	}
	linkPeople(t, r, other, "No External Id")
	pid2 := personIDByName(t, r, "No External Id")
	ids2, err := r.ExternalIDsForEntity(ctx, model.EnrichEntityPerson, pid2)
	if err != nil {
		t.Fatalf("external ids 2: %v", err)
	}
	if len(ids2) != 0 {
		t.Fatalf("external ids for unenriched person = %v, want empty", ids2)
	}

	// A tag has no external-id table — nil, not an error.
	if ids3, err := r.ExternalIDsForEntity(ctx, model.EntityTag, 1); err != nil || ids3 != nil {
		t.Fatalf("external ids for tag = (%v, %v), want (nil, nil)", ids3, err)
	}
}

// TestNameKeyConvergenceTag proves the tag fold (RD2): tags additionally fold INTERNAL
// whitespace, so "sci fi", "scifi", and "Sci Fi" are one tag.
func TestNameKeyConvergenceTag(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	for i, name := range []string{"sci fi", "scifi", "Sci Fi", " SCI  FI "} {
		if _, err := r.UpsertVideo(ctx, sampleVideo("/m/t"+string(rune('a'+i))+".mkv", "T", nil, []string{name}), nil); err != nil {
			t.Fatalf("upsert tag %q: %v", name, err)
		}
	}
	if n := tagCount(t, r); n != 1 {
		t.Fatalf("tag whitespace variants forked identity: got %d tags, want 1", n)
	}
}

// TestAliasRoutesOnScan proves alias routing (RD3 step 3): once a name is an alias of
// a person, a later scan crediting that spelling (case-folded) links to the canonical
// person rather than creating a new one — the property that makes a merge survive a
// re-scan.
func TestAliasRoutesOnScan(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	idA, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", []string{"Robert Smith"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	linkPeople(t, r, idA, "Robert Smith")
	pid := personIDByName(t, r, "Robert Smith")
	if _, err := r.AddPersonAlias(ctx, pid, "Bob"); err != nil {
		t.Fatalf("add alias: %v", err)
	}

	// A new file credits "bob" (different casing than the stored alias "Bob").
	idB, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "T2", []string{"bob"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	linkPeople(t, r, idB, "bob")
	if n := peopleCount(t, r); n != 1 {
		t.Fatalf("alias spelling created a second person: got %d, want 1", n)
	}
	// Both files hang off the one canonical person.
	people, err := r.ListPeople(ctx, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if people[0].VideoCount != 2 {
		t.Fatalf("canonical person video count = %d, want 2", people[0].VideoCount)
	}
}

// TestExactEntityMatch proves F48.3c's reuse contract: ExactEntityMatch finds
// an entity via canonical name OR alias (both routes resolveOrCreateByName
// itself uses), and reports ok=false for a name that would create a new
// entity.
func TestExactEntityMatch(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	seedID, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", []string{"Alice Smith"}, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	linkPeople(t, r, seedID, "Alice Smith")

	id, ok, err := r.ExactEntityMatch(ctx, model.EnrichEntityPerson, "alice smith")
	if err != nil || !ok {
		t.Fatalf("expected a case-folded exact match, got ok=%v err=%v", ok, err)
	}
	if id == 0 {
		t.Fatal("expected a non-zero entity id")
	}

	if _, err := r.AddPersonAlias(ctx, id, "Al Smith"); err != nil {
		t.Fatalf("add alias: %v", err)
	}
	aliasID, ok, err := r.ExactEntityMatch(ctx, model.EnrichEntityPerson, "Al Smith")
	if err != nil || !ok || aliasID != id {
		t.Fatalf("expected alias match to the same canonical id, got id=%d ok=%v err=%v", aliasID, ok, err)
	}

	if _, ok, err := r.ExactEntityMatch(ctx, model.EnrichEntityPerson, "Nobody Here"); err != nil || ok {
		t.Fatalf("expected no match for an unknown name, got ok=%v err=%v", ok, err)
	}
}

// TestEntityNames proves the fuzzy-ranking candidate pool (F48.3d) returns
// every known Person/Studio, keyed by id.
func TestEntityNames(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	seedID, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", []string{"Alice Smith", "Bob Jones"}, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	linkPeople(t, r, seedID, "Alice Smith", "Bob Jones")

	names, err := r.EntityNames(ctx, model.EnrichEntityPerson)
	if err != nil {
		t.Fatalf("entity names: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("want 2 people, got %d (%v)", len(names), names)
	}
	var got []string
	for _, n := range names {
		got = append(got, n)
	}
	if !contains(got, "Alice Smith") || !contains(got, "Bob Jones") {
		t.Fatalf("expected both names present, got %v", got)
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
