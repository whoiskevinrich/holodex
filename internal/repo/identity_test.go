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
// person's attached external id (entity_external_ids, ADR-096 D2) round-trips as
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

	// A tag with no attached id reads back empty too (tags share the table since
	// ADR-096 D2; see TestEntityExternalIDs_FilmAndTag for the attached case).
	if ids3, err := r.ExternalIDsForEntity(ctx, model.EntityTag, 1); err != nil || len(ids3) != 0 {
		t.Fatalf("external ids for tag = (%v, %v), want empty", ids3, err)
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

// TestResolvePrecedence_ExternalIDBeatsNameKey is the F60 RD3 / F23 invariant, stated
// as precedence rather than convergence (TestReconcileVideoPeople_ExternalIDDedup only
// proves an id-carrying spelling *converges*): when a file's spelling is an EXACT
// nameKey match for entity B but carries entity A's provider id, the link lands on A.
// Step 1 of resolveOrCreateByName (external id) must run before step 2 (nameKey), for
// every kind that has provider identity — otherwise a merge or a provider link is
// undone by the next rescan. One polymorphic table (ADR-096 D2) serves both kinds.
func TestResolvePrecedence_ExternalIDBeatsNameKey(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	a, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	b, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)
	c, _ := r.UpsertVideo(ctx, sampleVideo("/m/c.mkv", "C", nil, nil), nil)

	// Person: A "Denis Villeneuve" owns tmdb:137; B "Dennis Villeneuve" is a distinct,
	// id-less person whose name the third file spells exactly.
	if err := r.ReconcileVideoPeople(ctx, a, []repo.PersonRoleName{{Name: "Denis Villeneuve", Role: "director"}},
		map[string]string{"Denis Villeneuve": "tmdb:137"}); err != nil {
		t.Fatalf("reconcile a: %v", err)
	}
	if err := r.ReconcileVideoPeople(ctx, b, []repo.PersonRoleName{{Name: "Dennis Villeneuve", Role: "director"}}, nil); err != nil {
		t.Fatalf("reconcile b: %v", err)
	}
	if err := r.ReconcileVideoPeople(ctx, c, []repo.PersonRoleName{{Name: "Dennis Villeneuve", Role: "director"}},
		map[string]string{"Dennis Villeneuve": "tmdb:137"}); err != nil {
		t.Fatalf("reconcile c: %v", err)
	}
	if n := peopleCount(t, r); n != 2 {
		t.Fatalf("people = %d, want 2 (no third row from the id-carrying rescan)", n)
	}
	denis, dennis := personIDByName(t, r, "Denis Villeneuve"), personIDByName(t, r, "Dennis Villeneuve")
	people, _ := r.PeopleForVideos(ctx, []int64{c})
	if len(people[c]) != 1 || people[c][0].ID != denis {
		t.Fatalf("video c people = %+v, want the id owner %d (not the nameKey match %d)", people, denis, dennis)
	}

	// Studio: same shape over the same table.
	if err := r.ReconcileVideoStudios(ctx, a, []string{"Warner Bros."}, map[string]string{"Warner Bros.": "tmdb:174"}); err != nil {
		t.Fatalf("studios a: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, b, []string{"Warner Brothers"}, nil); err != nil {
		t.Fatalf("studios b: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, c, []string{"Warner Brothers"}, map[string]string{"Warner Brothers": "tmdb:174"}); err != nil {
		t.Fatalf("studios c: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil {
		t.Fatalf("list studios: %v", err)
	}
	if len(studios) != 2 {
		t.Fatalf("studios = %+v, want 2", studios)
	}
	wb := studioIDByName(t, r, "Warner Bros.")
	got, _ := r.StudiosForVideos(ctx, []int64{c})
	if len(got[c]) != 1 || got[c][0].ID != wb {
		t.Fatalf("video c studios = %+v, want the id owner %d", got, wb)
	}
}

// TestEntityExternalIDs_FilmAndTag proves the two kinds ADR-096 D2 adds to the
// external-id store: AttachExternalID/ExternalIDsForEntity round-trip for film and
// tag, GetFilmByExternalID resolves the provider id, an id is unique per kind but not
// across kinds (PK is (entity_type, external_id)), and the per-kind AFTER DELETE
// trigger removes a film's ids with the film (no FK cascade on a polymorphic table).
func TestEntityExternalIDs_FilmAndTag(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()

	film, err := r.CreateFilm(ctx, "The Matrix", 1999)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	if err := r.AttachExternalID(ctx, model.EnrichEntityFilm, film, "tmdb:603"); err != nil {
		t.Fatalf("attach film id: %v", err)
	}
	// Idempotent: the same pair again is a no-op, not a PK error.
	if err := r.AttachExternalID(ctx, model.EnrichEntityFilm, film, "tmdb:603"); err != nil {
		t.Fatalf("re-attach film id: %v", err)
	}
	got, err := r.GetFilmByExternalID(ctx, "tmdb:603")
	if err != nil || got == nil || got.ID != film {
		t.Fatalf("GetFilmByExternalID = (%+v, %v), want film %d", got, err, film)
	}
	if got, err := r.GetFilmByExternalID(ctx, "tmdb:999"); err != nil || got != nil {
		t.Fatalf("GetFilmByExternalID(unknown) = (%+v, %v), want (nil, nil)", got, err)
	}

	// The same id string under another kind is a different row (unique per kind).
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, []string{"matrix"}), nil); err != nil {
		t.Fatalf("upsert video: %v", err)
	}
	var tagID int64
	if err := database.QueryRow(`SELECT id FROM tags WHERE name = 'matrix'`).Scan(&tagID); err != nil {
		t.Fatalf("tag id: %v", err)
	}
	if err := r.AttachExternalID(ctx, model.EntityTag, tagID, "tmdb:603"); err != nil {
		t.Fatalf("attach tag id: %v", err)
	}
	ids, err := r.ExternalIDsForEntity(ctx, model.EntityTag, tagID)
	if err != nil || len(ids) != 1 || ids[0] != "tmdb:603" {
		t.Fatalf("tag external ids = (%v, %v), want [tmdb:603]", ids, err)
	}

	// Cleanup trigger: deleting the film drops its ids; the tag's row is untouched.
	if _, err := database.ExecContext(ctx, `DELETE FROM films WHERE id = ?`, film); err != nil {
		t.Fatalf("delete film: %v", err)
	}
	if got, err := r.GetFilmByExternalID(ctx, "tmdb:603"); err != nil || got != nil {
		t.Fatalf("film id survived film delete: (%+v, %v)", got, err)
	}
	if ids, _ := r.ExternalIDsForEntity(ctx, model.EntityTag, tagID); len(ids) != 1 {
		t.Fatalf("tag external ids after film delete = %v, want [tmdb:603]", ids)
	}
}
