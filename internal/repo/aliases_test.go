package repo_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// newRepoDB is like newRepo but also returns the underlying *sql.DB so a test can
// exercise FK cascade (which has no repo method — there is no hard person delete).
func newRepoDB(t *testing.T) (*repo.Repo, *sql.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return repo.New(database), database
}

// personIDByName upserts nothing; it finds an existing person's id by exact name.
func personIDByName(t *testing.T, r *repo.Repo, name string) int64 {
	t.Helper()
	people, err := r.ListPeople(context.Background(), false)
	if err != nil {
		t.Fatalf("list people: %v", err)
	}
	for _, p := range people {
		if p.Name == name {
			return p.ID
		}
	}
	t.Fatalf("person %q not found", name)
	return 0
}

func TestPersonAliasesCRUD(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", []string{"Robert Smith"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	linkPeople(t, r, id, "Robert Smith")
	pid := personIDByName(t, r, "Robert Smith")

	if _, err := r.AddPersonAlias(ctx, pid, "Rob"); err != nil {
		t.Fatalf("add alias: %v", err)
	}
	// Case-insensitive idempotency: "rob" must not create a second row.
	a, err := r.AddPersonAlias(ctx, pid, "rob")
	if err != nil {
		t.Fatalf("re-add alias: %v", err)
	}
	aliases, err := r.AliasesForPerson(ctx, pid)
	if err != nil {
		t.Fatalf("list aliases: %v", err)
	}
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias after idempotent add, got %d: %+v", len(aliases), aliases)
	}
	if aliases[0].Alias != "Rob" { // original casing preserved
		t.Errorf("alias casing = %q, want %q", aliases[0].Alias, "Rob")
	}
	if a.ID != aliases[0].ID {
		t.Errorf("idempotent add returned id %d, want existing %d", a.ID, aliases[0].ID)
	}

	// F43 (ADR-061 RD1): an alias key belongs to exactly one entity. The same alias
	// on a different person is no longer silently allowed — it collides, and
	// PersonConflict surfaces the current owner so the handler can 409 (merge or keep
	// separate) instead of forking identity.
	id2, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "T2", []string{"Bobby"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	linkPeople(t, r, id2, "Bobby")
	pid2 := personIDByName(t, r, "Bobby")
	conflict, err := r.PersonConflict(ctx, pid2, "Rob")
	if err != nil {
		t.Fatalf("person conflict: %v", err)
	}
	if conflict == nil || conflict.ID != pid {
		t.Fatalf("expected 'Rob' to collide with person %d, got %+v", pid, conflict)
	}

	// Empty alias is rejected by the repo guard.
	if _, err := r.AddPersonAlias(ctx, pid, ""); err == nil {
		t.Error("expected error for empty alias")
	}
}

func TestPersonAliasesDeleteScopeAndCascade(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()
	idA, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", []string{"Alice"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	linkPeople(t, r, idA, "Alice")
	idB, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "T2", []string{"Bob"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	linkPeople(t, r, idB, "Bob")
	alice := personIDByName(t, r, "Alice")
	bob := personIDByName(t, r, "Bob")

	aliceAlias, err := r.AddPersonAlias(ctx, alice, "Al")
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	// Deleting Alice's alias id scoped to Bob must not delete it.
	if err := r.DeletePersonAlias(ctx, bob, aliceAlias.ID); err != repo.ErrNotFound {
		t.Errorf("cross-person delete = %v, want ErrNotFound", err)
	}
	if got, _ := r.AliasesForPerson(ctx, alice); len(got) != 1 {
		t.Fatalf("alias wrongly removed by cross-person delete: %+v", got)
	}
	// Unknown id → ErrNotFound.
	if err := r.DeletePersonAlias(ctx, alice, 99999); err != repo.ErrNotFound {
		t.Errorf("unknown delete = %v, want ErrNotFound", err)
	}
	// Correct scope → removed.
	if err := r.DeletePersonAlias(ctx, alice, aliceAlias.ID); err != nil {
		t.Fatalf("scoped delete: %v", err)
	}
	if got, _ := r.AliasesForPerson(ctx, alice); len(got) != 0 {
		t.Errorf("alias not removed: %+v", got)
	}

	// ON DELETE CASCADE: removing the person removes its aliases.
	if _, err := r.AddPersonAlias(ctx, bob, "Bobby"); err != nil {
		t.Fatalf("add to bob: %v", err)
	}
	if _, err := database.ExecContext(ctx, `DELETE FROM people WHERE id = ?`, bob); err != nil {
		t.Fatalf("delete person: %v", err)
	}
	if got, _ := r.AliasesForPerson(ctx, bob); len(got) != 0 {
		t.Errorf("aliases survived person delete (no cascade): %+v", got)
	}
}

// TestReconcileVideoPeople_ExternalIDDedup is the F32/ADR-055 crux (the person
// analogue of TestReconcileVideoStudios_ExternalIDDedup): two videos whose resolved
// person name is a DIFFERENT spelling of the SAME provider person converge to one
// Person, because resolve-or-create matches external_id before name. A third video
// with a name and no id resolves by name only (the fallback).
func TestReconcileVideoPeople_ExternalIDDedup(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	a, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	b, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)

	// A: "Robert Downey Jr." carrying tmdb:3223 → creates the person + attaches the id.
	if err := r.ReconcileVideoPeople(ctx, a, []repo.PersonRoleName{{Name: "Robert Downey Jr.", Role: "actor"}},
		map[string]string{"Robert Downey Jr.": "tmdb:3223"}); err != nil {
		t.Fatalf("reconcile a: %v", err)
	}
	// B: a DIFFERENT spelling, SAME id → converges to A's person, not a second entity.
	if err := r.ReconcileVideoPeople(ctx, b, []repo.PersonRoleName{{Name: "Robert Downey Jr", Role: "actor"}},
		map[string]string{"Robert Downey Jr": "tmdb:3223"}); err != nil {
		t.Fatalf("reconcile b: %v", err)
	}
	people, err := r.ListPeople(ctx, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(people) != 1 || people[0].VideoCount != 2 {
		t.Fatalf("people = %+v, want ONE person (id-deduped) count=2", people)
	}
	// The canonical name is the first spelling to create it (deterministic, mirrors
	// studio's RD6).
	if people[0].Name != "Robert Downey Jr." {
		t.Fatalf("converged name = %q, want %q (first-seen)", people[0].Name, "Robert Downey Jr.")
	}

	// C: a distinct person, no id → resolves/creates independently by name.
	c, _ := r.UpsertVideo(ctx, sampleVideo("/m/c.mkv", "C", nil, nil), nil)
	if err := r.ReconcileVideoPeople(ctx, c, []repo.PersonRoleName{{Name: "Chris Evans", Role: "actor"}}, nil); err != nil {
		t.Fatalf("reconcile c: %v", err)
	}
	people, err = r.ListPeople(ctx, false)
	if err != nil {
		t.Fatalf("list after c: %v", err)
	}
	if len(people) != 2 {
		t.Fatalf("people = %+v, want 2 (one id-deduped, one name-only)", people)
	}
}

// TestReconcileVideoPeople_ExternalIDCascade proves person_external_ids' ON DELETE
// CASCADE (migration 0038): hard-deleting the person row removes its external-id row
// too, so a later reconcile for the same provider id is not silently orphaned onto a
// nonexistent person — it creates a fresh one. Hard delete bypasses the orphan grace
// period (there is no repo method for it; mirrors TestPersonAliasesDeleteScopeAndCascade's
// direct-DB cascade check above).
func TestReconcileVideoPeople_ExternalIDCascade(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()

	a, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	if err := r.ReconcileVideoPeople(ctx, a, []repo.PersonRoleName{{Name: "Denis Villeneuve", Role: "director"}},
		map[string]string{"Denis Villeneuve": "tmdb:137"}); err != nil {
		t.Fatalf("reconcile a: %v", err)
	}
	first := personIDByName(t, r, "Denis Villeneuve")
	if _, err := database.ExecContext(ctx, `DELETE FROM people WHERE id = ?`, first); err != nil {
		t.Fatalf("delete person: %v", err)
	}

	b, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)
	if err := r.ReconcileVideoPeople(ctx, b, []repo.PersonRoleName{{Name: "Denis Villeneuve", Role: "director"}},
		map[string]string{"Denis Villeneuve": "tmdb:137"}); err != nil {
		t.Fatalf("reconcile b after delete: %v", err)
	}
	second := personIDByName(t, r, "Denis Villeneuve")
	if second == first {
		t.Fatalf("re-resolve returned the deleted person id %d (external id row leaked)", first)
	}
}

func TestSearchMatchesAlias(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	idBowie, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "Life on Mars", []string{"David Bowie"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	linkPeople(t, r, idBowie, "David Bowie")
	bowie := personIDByName(t, r, "David Bowie")
	if _, err := r.AddPersonAlias(ctx, bowie, "Ziggy"); err != nil {
		t.Fatalf("add Ziggy: %v", err)
	}
	if _, err := r.AddPersonAlias(ctx, bowie, "Bowie"); err != nil { // also a substring of the name
		t.Fatalf("add Bowie: %v", err)
	}

	// Alias-only match.
	res, err := r.Search(ctx, "zig", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if got := countPeople(res.People, bowie); got != 1 {
		t.Errorf("q=zig: Bowie appears %d times, want 1", got)
	}

	// Name + alias both match → person appears exactly once (dedup).
	res, err = r.Search(ctx, "bowie", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if got := countPeople(res.People, bowie); got != 1 {
		t.Errorf("q=bowie: Bowie appears %d times, want 1 (dedup)", got)
	}

	// Diacritic folding on aliases.
	idSinger, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "Halo", []string{"Singer"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	linkPeople(t, r, idSinger, "Singer")
	singer := personIDByName(t, r, "Singer")
	if _, err := r.AddPersonAlias(ctx, singer, "Beyoncé"); err != nil {
		t.Fatalf("add accented alias: %v", err)
	}
	res, err = r.Search(ctx, "beyonce", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if got := countPeople(res.People, singer); got != 1 {
		t.Errorf("q=beyonce: accented alias matched %d times, want 1", got)
	}
}

func TestAliasesSurviveRescan(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	v := sampleVideo("/m/a.mkv", "T", []string{"Alice"}, nil)
	id, err := r.UpsertVideo(ctx, v, nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	linkPeople(t, r, id, "Alice")
	alice := personIDByName(t, r, "Alice")
	if _, err := r.AddPersonAlias(ctx, alice, "Ziggy"); err != nil {
		t.Fatalf("add: %v", err)
	}

	// A re-scan re-upserts the video and RelinkVideoPeople re-derives video_people
	// via resolveOrCreatePerson (alias-routed, same choke point as getOrCreateByName).
	if _, err := r.UpsertVideo(ctx, v, nil); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	linkPeople(t, r, id, "Alice")

	if got, _ := r.AliasesForPerson(ctx, alice); len(got) != 1 || got[0].Alias != "Ziggy" {
		t.Errorf("alias did not survive re-scan: %+v", got)
	}
	res, err := r.Search(ctx, "zig", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if countPeople(res.People, alice) != 1 {
		t.Error("alias not search-matchable after re-scan")
	}
}

func TestScanResolvesAliasToCanonical(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	idA, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Jennifer Lawrence"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	linkPeople(t, r, idA, "Jennifer Lawrence")
	jen := personIDByName(t, r, "Jennifer Lawrence")
	if _, err := r.AddPersonAlias(ctx, jen, "J Law"); err != nil {
		t.Fatalf("add alias: %v", err)
	}

	// A NEW file tagged with the alias must link to the canonical person, not create
	// a "J Law" person.
	idB, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"J Law"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert alias-tagged: %v", err)
	}
	linkPeople(t, r, idB, "J Law")
	people, _ := r.ListPeople(ctx, false)
	if len(people) != 1 {
		t.Fatalf("expected 1 person (alias routed), got %d: %+v", len(people), people)
	}
	p, _ := r.GetPerson(ctx, jen)
	if p.VideoCount != 2 {
		t.Errorf("canonical video count = %d, want 2 (both files routed)", p.VideoCount)
	}

	// And it survives a re-scan of the alias-tagged file.
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"J Law"}, nil), nil); err != nil {
		t.Fatalf("re-scan: %v", err)
	}
	linkPeople(t, r, idB, "J Law")
	if people, _ := r.ListPeople(ctx, false); len(people) != 1 {
		t.Errorf("re-scan re-split the person: %+v", people)
	}
}

func TestMergePersons(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	idA, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Jennifer Lawrence"}, nil), nil)
	linkPeople(t, r, idA, "Jennifer Lawrence")
	idB, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"J Law"}, nil), nil)
	linkPeople(t, r, idB, "J Law")
	// A film credited under both spellings — the union must de-dupe to one.
	idC, _ := r.UpsertVideo(ctx, sampleVideo("/m/c.mkv", "C", []string{"Jennifer Lawrence", "J Law"}, nil), nil)
	linkPeople(t, r, idC, "Jennifer Lawrence", "J Law")
	jen := personIDByName(t, r, "Jennifer Lawrence")
	jlaw := personIDByName(t, r, "J Law")

	if err := r.MergePersons(ctx, jen, jlaw); err != nil {
		t.Fatalf("merge: %v", err)
	}
	// In production, a post-merge relink pass (RelinkVideoPeople) re-derives
	// every affected video's person links from its resolved fields — both
	// "J Law" and "Jennifer Lawrence" now resolve (alias-routed) to the same
	// survivor, so B and C's re-derived link sets collapse to one row apiece.
	// Simulate that pass here, same as the re-scan derivation below.
	linkPeople(t, r, idB, "J Law")
	linkPeople(t, r, idC, "Jennifer Lawrence", "J Law")

	// The duplicate is gone.
	if _, err := r.GetPerson(ctx, jlaw); err != repo.ErrNotFound {
		t.Errorf("merged person still present: %v", err)
	}
	// Canonical owns the de-duped union (A, B, C = 3, not 4).
	p, err := r.GetPerson(ctx, jen)
	if err != nil {
		t.Fatalf("get canonical: %v", err)
	}
	if p.VideoCount != 3 {
		t.Errorf("canonical video count = %d, want 3 (union de-duped)", p.VideoCount)
	}
	// The merged name became a searchable alias.
	if len(p.Aliases) != 1 || p.Aliases[0].Alias != "J Law" {
		t.Errorf("aliases after merge = %+v, want [J Law]", p.Aliases)
	}
	if res, _ := r.Search(ctx, "j law", 10); countPeople(res.People, jen) != 1 {
		t.Error("merged alias not search-matchable")
	}
	// And re-scanning the alias-tagged file keeps it merged.
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"J Law"}, nil), nil); err != nil {
		t.Fatalf("re-scan: %v", err)
	}
	linkPeople(t, r, idB, "J Law")
	if people, _ := r.ListPeople(ctx, false); len(people) != 1 {
		t.Errorf("re-scan re-split merged person: %+v", people)
	}
}

// The merge's own association move must dedupe on video_people's full
// (video_id, person_id, role) key (F40, ADR-072) — not just the post-merge relink
// pass a real scan/curation change would trigger afterward. Without carrying the
// loser's role across, the loser's 'actor' link would land as a second row with
// role='' instead of colliding with the survivor's own 'actor' link on the same video.
func TestMergePersons_DedupesSameRoleLinkAtMergeTime(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, _ := r.UpsertVideo(ctx, sampleVideo("/m/dup.mkv", "Dup", nil, nil), nil)
	linkPeople(t, r, id, "Jennifer Lawrence", "J Law")
	jen := personIDByName(t, r, "Jennifer Lawrence")
	jlaw := personIDByName(t, r, "J Law")

	if err := r.MergePersons(ctx, jen, jlaw); err != nil {
		t.Fatalf("merge: %v", err)
	}
	p, err := r.GetPerson(ctx, jen)
	if err != nil {
		t.Fatalf("get canonical: %v", err)
	}
	if p.VideoCount != 1 {
		t.Errorf("canonical video count = %d, want 1 (merge must dedupe same-role link on its own)", p.VideoCount)
	}
}

// TestMergePersons_RepointsExternalID proves person_external_ids' idMove
// (identity_ops.go, F32/ADR-055): merging two people repoints the LOSER's provider id
// onto the survivor instead of losing it to person_external_ids' ON DELETE CASCADE
// when the loser row is deleted. A later reconcile carrying that same id must resolve
// to the survivor, not create a third person.
func TestMergePersons_RepointsExternalID(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	a, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	if err := r.ReconcileVideoPeople(ctx, a, []repo.PersonRoleName{{Name: "Jennifer Lawrence", Role: "actor"}},
		map[string]string{"Jennifer Lawrence": "tmdb:1"}); err != nil {
		t.Fatalf("reconcile a: %v", err)
	}
	b, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)
	if err := r.ReconcileVideoPeople(ctx, b, []repo.PersonRoleName{{Name: "J Law", Role: "actor"}},
		map[string]string{"J Law": "tmdb:2"}); err != nil {
		t.Fatalf("reconcile b: %v", err)
	}
	jen := personIDByName(t, r, "Jennifer Lawrence")
	jlaw := personIDByName(t, r, "J Law")

	if err := r.MergePersons(ctx, jen, jlaw); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// The loser's id (tmdb:2) must now resolve to the survivor: without the idMove
	// fix, it would have cascade-deleted with the loser row, and this reconcile would
	// create a THIRD person instead of converging onto jen.
	c, _ := r.UpsertVideo(ctx, sampleVideo("/m/c.mkv", "C", nil, nil), nil)
	if err := r.ReconcileVideoPeople(ctx, c, []repo.PersonRoleName{{Name: "Someone New", Role: "actor"}},
		map[string]string{"Someone New": "tmdb:2"}); err != nil {
		t.Fatalf("reconcile c: %v", err)
	}
	people, err := r.ListPeople(ctx, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(people) != 1 || people[0].ID != jen || people[0].VideoCount != 3 {
		t.Fatalf("people = %+v, want ONE person (id %d) count=3 (repointed id resolved c onto the survivor)", people, jen)
	}
}

func TestMergePersonsValidation(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	idA, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Alice"}, nil), nil)
	linkPeople(t, r, idA, "Alice")
	alice := personIDByName(t, r, "Alice")
	if err := r.MergePersons(ctx, alice, alice); err == nil {
		t.Error("merge into self should error")
	}
	if err := r.MergePersons(ctx, alice, 99999); err != repo.ErrNotFound {
		t.Errorf("merge unknown = %v, want ErrNotFound", err)
	}
}

func TestPersonConflict(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	idA, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Chris Evans"}, nil), nil)
	linkPeople(t, r, idA, "Chris Evans")
	idB, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Some DJ"}, nil), nil)
	linkPeople(t, r, idB, "Some DJ")
	chris := personIDByName(t, r, "Chris Evans")
	dj := personIDByName(t, r, "Some DJ")

	// Adding "Chris Evans" as an alias of the DJ collides with the actor.
	c, err := r.PersonConflict(ctx, dj, "Chris Evans")
	if err != nil {
		t.Fatalf("conflict: %v", err)
	}
	if c == nil || c.ID != chris {
		t.Errorf("conflict = %+v, want Chris Evans (#%d)", c, chris)
	}
	// A free name has no conflict.
	if c, _ := r.PersonConflict(ctx, dj, "Totally New Name"); c != nil {
		t.Errorf("unexpected conflict for free name: %+v", c)
	}
	// The person's own name is not a conflict with itself.
	if c, _ := r.PersonConflict(ctx, chris, "Chris Evans"); c != nil {
		t.Errorf("self-name reported as conflict: %+v", c)
	}
}

func TestSearchReturnsPersonMedia(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	// The video TITLE deliberately shares no terms with the person/alias, so a hit
	// can only come from person association (the merge promise), not title FTS.
	idA, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "Untitled Clip", []string{"Zeta Person"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	linkPeople(t, r, idA, "Zeta Person")
	zeta := personIDByName(t, r, "Zeta Person")

	res, err := r.Search(ctx, "zeta", 10)
	if err != nil {
		t.Fatalf("search by name: %v", err)
	}
	if !hasVideoTitle(res.Videos, "Untitled Clip") {
		t.Errorf("search by name: videos = %v, want the person's media", videoTitles(res.Videos))
	}

	if _, err := r.AddPersonAlias(ctx, zeta, "Zed"); err != nil {
		t.Fatalf("add alias: %v", err)
	}
	res, err = r.Search(ctx, "zed", 10)
	if err != nil {
		t.Fatalf("search by alias: %v", err)
	}
	if !hasVideoTitle(res.Videos, "Untitled Clip") {
		t.Errorf("search by alias: videos = %v, want the person's media", videoTitles(res.Videos))
	}
}

func hasVideoTitle(vids []model.Video, title string) bool {
	for _, v := range vids {
		if v.Title == title {
			return true
		}
	}
	return false
}

func videoTitles(vids []model.Video) []string {
	out := make([]string, len(vids))
	for i, v := range vids {
		out[i] = v.Title
	}
	return out
}

func countPeople(people []model.Person, id int64) int {
	n := 0
	for _, p := range people {
		if p.ID == id {
			n++
		}
	}
	return n
}
