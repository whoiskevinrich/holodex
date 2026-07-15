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
		if _, err := r.UpsertVideo(ctx, sampleVideo("/m/"+name+".mkv", "T", []string{name}, nil), nil); err != nil {
			t.Fatalf("upsert %q: %v", name, err)
		}
	}
	if n := peopleCount(t, r); n != 1 {
		t.Fatalf("case/whitespace variants forked identity: got %d people, want 1", n)
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

	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", []string{"Robert Smith"}, nil), nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	pid := personIDByName(t, r, "Robert Smith")
	if _, err := r.AddPersonAlias(ctx, pid, "Bob"); err != nil {
		t.Fatalf("add alias: %v", err)
	}

	// A new file credits "bob" (different casing than the stored alias "Bob").
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "T2", []string{"bob"}, nil), nil); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
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

	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", []string{"Alice Smith"}, nil), nil); err != nil {
		t.Fatalf("seed: %v", err)
	}

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

	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", []string{"Alice Smith", "Bob Jones"}, nil), nil); err != nil {
		t.Fatalf("seed: %v", err)
	}

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
