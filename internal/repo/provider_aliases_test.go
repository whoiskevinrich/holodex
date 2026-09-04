package repo_test

import (
	"context"
	"database/sql"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

const tmdb = "tmdb"

func aliasStrings(t *testing.T, r *repo.Repo, entityType string, id int64) []string {
	t.Helper()
	got, err := r.AliasesForEntity(context.Background(), entityType, id)
	if err != nil {
		t.Fatalf("aliases for %s %d: %v", entityType, id, err)
	}
	out := make([]string, len(got))
	for i, a := range got {
		out[i] = a.Alias
	}
	return out
}

func hasAlias(t *testing.T, r *repo.Repo, entityType string, id int64, want string) bool {
	t.Helper()
	for _, a := range aliasStrings(t, r, entityType, id) {
		if a == want {
			return true
		}
	}
	return false
}

// TestApplyProviderAliases_LiveOnArrival is the payoff assertion, and it is deliberately a
// pair: a provider-supplied name must BOTH find the person in search AND route a file
// tagged with it on the next scan (spec F58 P0-3, ADR-088 D3). Either half passing alone
// means the collapse shipped the original bug in a new table -- Holodex displaying a name
// it cannot act on. Both halves reuse the routes TestSearchMatchesAlias and
// TestScanResolvesAliasToCanonical already prove for an owner-typed alias, so what this
// test adds is that a provider-authored row is the same kind of row.
func TestApplyProviderAliases_LiveOnArrival(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	idA, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Hayao Miyazaki"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	linkPeople(t, r, idA, "Hayao Miyazaki")
	hayao := personIDByName(t, r, "Hayao Miyazaki")

	skipped, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, hayao, tmdb,
		[]string{"宮崎駿", "Miyazaki Hayao"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("nothing should have been skipped, got %+v", skipped)
	}

	// (a) Searchable — the FTS triggers fire on insert, so no new search work exists.
	res, err := r.Search(ctx, "Miyazaki Hayao", 10, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if countPeople(res.People, hayao) != 1 {
		t.Error("provider alias does not find the person in search")
	}

	// (b) Scan-routing — a new file credited with the provider's spelling attaches to the
	// existing person instead of creating a second one.
	idB, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Miyazaki Hayao"}, nil), nil)
	if err != nil {
		t.Fatalf("upsert alias-tagged: %v", err)
	}
	linkPeople(t, r, idB, "Miyazaki Hayao")
	people, _ := r.ListPeople(ctx, false)
	if len(people) != 1 {
		t.Fatalf("expected 1 person (provider alias routed the scan), got %d: %+v", len(people), people)
	}
	p, _ := r.GetPerson(ctx, hayao)
	if p.VideoCount != 2 {
		t.Errorf("canonical video count = %d, want 2 (both files routed)", p.VideoCount)
	}
}

// TestApplyProviderAliases_Filters table-drives spec F58 RD6 and the surrounding input
// guards. The false-positive half matters more than the true-positive half: an over-eager
// filter silently costs the entity reach, which is worse than importing one redundant
// name, so most rows here assert something is KEPT.
func TestApplyProviderAliases_Filters(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id := seedPerson(t, r, "Hayao Miyazaki")

	long := make([]rune, model.MaxNameLen+1)
	for i := range long {
		long[i] = 'x'
	}

	for _, tc := range []struct {
		name      string
		candidate string
		want      bool // want it stored
	}{
		{"exact own name", "Hayao Miyazaki", false},
		{"own name, different case", "hayao miyazaki", false},
		{"own name, hyphenated", "Hayao-Miyazaki", false},
		{"own name, extra spacing", "Hayao  Miyazaki", false},
		{"own name, trailing punctuation", "Hayao Miyazaki.", false},
		{"empty", "   ", false},
		{"punctuation only", "...", false},
		{"over the name cap", string(long), false},
		{"initialed form", "H. Miyazaki", true},
		{"reordered form", "Miyazaki, Hayao", true},
		{"non-latin script", "宮崎駿", true},
		{"unrelated name", "Ghibli Founder", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, id, tmdb,
				[]string{tc.candidate}); err != nil {
				t.Fatalf("apply: %v", err)
			}
			if got := hasAlias(t, r, model.EnrichEntityPerson, id, tc.candidate); got != tc.want {
				t.Errorf("stored=%v, want %v for %q", got, tc.want, tc.candidate)
			}
		})
	}
}

// TestApplyProviderAliases_AdditiveAndIdempotent covers spec F58 RD5. Re-enriching must
// not duplicate rows, and a name the provider has since DROPPED stays: provider input is
// additive, and only the owner or a merge ever removes an alias. Mirroring the provider's
// current list instead would let a routine re-enrich silently stop routing files.
func TestApplyProviderAliases_AdditiveAndIdempotent(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id := seedPerson(t, r, "Hayao Miyazaki")

	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, id, tmdb,
		[]string{"宮崎駿", "Miyazaki Hayao"}); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	first, err := r.AliasesForEntity(ctx, model.EnrichEntityPerson, id)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Second pass: one name repeated in a different casing, the other dropped entirely,
	// and a new one added.
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, id, tmdb,
		[]string{"  miyazaki hayao  ", "H. Miyazaki"}); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	second, err := r.AliasesForEntity(ctx, model.EnrichEntityPerson, id)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if len(second) != 3 {
		t.Fatalf("want 3 aliases (2 kept + 1 new), got %d: %+v", len(second), second)
	}
	if !hasAlias(t, r, model.EnrichEntityPerson, id, "宮崎駿") {
		t.Error("a name the provider stopped listing was removed; provider input is additive (RD5)")
	}
	// The re-listed name kept its original row rather than being deleted and reinserted.
	var firstID int64
	for _, a := range first {
		if a.Alias == "Miyazaki Hayao" {
			firstID = a.ID
		}
	}
	for _, a := range second {
		if a.Alias == "Miyazaki Hayao" && a.ID != firstID {
			t.Errorf("re-enrich replaced the row: id %d -> %d", firstID, a.ID)
		}
	}
	// A provider listing the same name twice in two spellings inserts it once.
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, id, tmdb,
		[]string{"Ghibli Guy", "ghibli  guy"}); err != nil {
		t.Fatalf("third apply: %v", err)
	}
	if got := aliasStrings(t, r, model.EnrichEntityPerson, id); len(got) != 4 {
		t.Errorf("intra-batch duplicate stored twice: %+v", got)
	}
}

// TestProviderAliasSuppressionIsDurable is the user-trust invariant (ADR-088 D4): deleting
// a provider alias has to survive a re-enrich, or deleting it was pointless. The three
// asymmetries below are deliberate and asserted so a later reader cannot "fix" them.
func TestProviderAliasSuppressionIsDurable(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	hayao := seedPerson(t, r, "Hayao Miyazaki")
	other := seedPerson(t, r, "Someone Else")

	// Two provider names, both then deleted, so each asymmetry below gets its own name
	// and no assertion depends on what a previous one did to a shared one.
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, hayao, tmdb,
		[]string{"Miyazaki Hayao", "Miyazaki-san"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	aliases, _ := r.AliasesForEntity(ctx, model.EnrichEntityPerson, hayao)
	if len(aliases) != 2 {
		t.Fatalf("setup: want 2 aliases, got %d", len(aliases))
	}
	for _, a := range aliases {
		if err := r.DeleteEntityAlias(ctx, model.EnrichEntityPerson, hayao, a.ID); err != nil {
			t.Fatalf("delete %q: %v", a.Alias, err)
		}
	}

	// The whole point: a full second enrich pass must not bring either back.
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, hayao, tmdb,
		[]string{"Miyazaki Hayao", "Miyazaki-san"}); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if got := aliasStrings(t, r, model.EnrichEntityPerson, hayao); len(got) != 0 {
		t.Fatalf("re-enrich resurrected deleted provider aliases: %v", got)
	}

	// Asymmetry 1: the suppression gates the enrich path only, never the owner. Re-typing
	// a suppressed name by hand succeeds, and the standing suppression is what keeps a
	// later re-enrich from being the thing that re-adds it.
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityPerson, hayao, "Miyazaki-san"); err != nil {
		t.Fatalf("owner re-add of a suppressed name was refused: %v", err)
	}
	if !hasAlias(t, r, model.EnrichEntityPerson, hayao, "Miyazaki-san") {
		t.Error("owner re-add of a suppressed name did not stick")
	}

	// Asymmetry 2: suppression is scoped per-entity, so a different person stays free to
	// receive the same name. A global suppression would have made one person's cleanup
	// silently degrade every other person's enrichment.
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, other, tmdb,
		[]string{"Miyazaki Hayao"}); err != nil {
		t.Fatalf("apply to other: %v", err)
	}
	if !hasAlias(t, r, model.EnrichEntityPerson, other, "Miyazaki Hayao") {
		t.Error("a suppression on one person blocked a different person from taking the name")
	}

	// Asymmetry 3: deleting an OWNER-authored alias records nothing -- no path would
	// re-add it, and a suppression there would only get in the owner's way later. The
	// alias deleted here is the one the owner re-typed in asymmetry 1, so its source is
	// now empty even though a provider originally supplied the name.
	owned, _ := r.AliasesForEntity(ctx, model.EnrichEntityPerson, hayao)
	if len(owned) != 1 {
		t.Fatalf("setup: want the one owner-added alias, got %+v", owned)
	}
	before := suppressionCount(t, db)
	if err := r.DeleteEntityAlias(ctx, model.EnrichEntityPerson, hayao, owned[0].ID); err != nil {
		t.Fatalf("delete owner alias: %v", err)
	}
	if after := suppressionCount(t, db); after != before {
		t.Errorf("deleting an owner-authored alias wrote a suppression (%d -> %d)", before, after)
	}
}

func suppressionCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM entity_alias_suppressions`).Scan(&n); err != nil {
		t.Fatalf("count suppressions: %v", err)
	}
	return n
}

func reviewVariations(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT variation FROM identity_review_queue ORDER BY id_lo, id_hi`)
	if err != nil {
		t.Fatalf("read review queue: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, v)
	}
	return out
}

// TestProviderAliasCollisionQueuesReview covers ADR-088 D5. A name another entity already
// holds cannot be inserted (the global UNIQUE from ADR-061 RD1), so it is skipped and the
// pair queued -- never a silent merge, and never a failed enrichment.
func TestProviderAliasCollisionQueuesReview(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	jen := seedPerson(t, r, "Jennifer Lawrence")
	imposter := seedPerson(t, r, "J. Lawrence")
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityPerson, jen, "J Law"); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	// The colliding name is one of several, so this also proves the rest of the batch
	// still lands -- one awkward AKA must not cost the entity everything else.
	skipped, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, imposter, tmdb,
		[]string{"J Law", "Jennifer L."})
	if err != nil {
		t.Fatalf("collision must not fail the apply: %v", err)
	}
	if len(skipped) != 1 || skipped[0].Alias != "J Law" || skipped[0].ConflictID != jen {
		t.Fatalf("skipped = %+v, want one entry for 'J Law' conflicting with %d", skipped, jen)
	}
	if hasAlias(t, r, model.EnrichEntityPerson, imposter, "J Law") {
		t.Error("colliding name was added anyway; it belongs to the other person")
	}
	if !hasAlias(t, r, model.EnrichEntityPerson, imposter, "Jennifer L.") {
		t.Error("a collision on one candidate dropped the rest of the batch")
	}
	if got := reviewVariations(t, db); len(got) != 1 || got[0] != "provider-alias" {
		t.Fatalf("review queue = %v, want one 'provider-alias' pair", got)
	}
	// No merge happened: both people still exist as distinct rows.
	if people, _ := r.ListPeople(ctx, false); len(people) != 2 {
		t.Errorf("collision merged two people; want 2 rows, got %d", len(people))
	}

	// Re-enriching the same collision does not stack duplicate pairs.
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, imposter, tmdb,
		[]string{"J Law"}); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if got := reviewVariations(t, db); len(got) != 1 {
		t.Errorf("re-enrich stacked review pairs: %v", got)
	}
}

// TestProviderAliasCollisionRespectsKeepSeparate extends F43 RD5 ("a kept-separate pair
// never nags") to this new, non-owner-initiated candidate source. Without the guard every
// re-enrich would re-propose a pair the owner has already dismissed -- the exact nagging
// the keep-separate store exists to stop.
func TestProviderAliasCollisionRespectsKeepSeparate(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	jen := seedPerson(t, r, "Jennifer Lawrence")
	imposter := seedPerson(t, r, "J. Lawrence")
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityPerson, jen, "J Law"); err != nil {
		t.Fatalf("seed alias: %v", err)
	}
	if err := r.AddKeepSeparate(ctx, model.EnrichEntityPerson, jen, imposter); err != nil {
		t.Fatalf("keep separate: %v", err)
	}

	skipped, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, imposter, tmdb,
		[]string{"J Law"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// Still skipped -- the name genuinely is not available -- but not re-queued.
	if len(skipped) != 1 {
		t.Fatalf("skipped = %+v, want the collision still reported", skipped)
	}
	if got := reviewVariations(t, db); len(got) != 0 {
		t.Errorf("re-proposed a pair the owner kept separate: %v", got)
	}
}

// TestApplyProviderAliases_Studio proves the entity-generic half (spec F58 RD8). AliasPanel
// is reused verbatim on studio and entity_aliases is polymorphic, so a person-only test
// would leave half the shipped surface unproven.
func TestApplyProviderAliases_Studio(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ghibli := seedStudio(t, r, "Studio Ghibli")

	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityStudio, ghibli, tmdb,
		[]string{"Ghibli", "Studio-Ghibli"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := aliasStrings(t, r, model.EnrichEntityStudio, ghibli)
	if len(got) != 1 || got[0] != "Ghibli" {
		t.Fatalf("studio aliases = %v, want just [Ghibli] (RD6 drops the hyphenated self)", got)
	}
}

// TestApplyProviderAliases_UnknownEntity guards the two argument errors that are real
// programmer mistakes rather than data conditions, so they surface loudly instead of
// silently writing nothing.
func TestApplyProviderAliases_UnknownEntity(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id := seedPerson(t, r, "Someone")

	if _, err := r.ApplyProviderAliases(ctx, "video", id, tmdb, []string{"X"}); err == nil {
		t.Error("want an error for an entity type with no alias spine")
	}
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, id, "", []string{"X"}); err == nil {
		t.Error("want an error for an empty source; provenance is not optional here")
	}
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, 9999, tmdb, []string{"X"}); err == nil {
		t.Error("want an error for a missing entity")
	}
}

// TestPromoteEnrichmentAliases covers the one-time upgrade pass (F58 P0-6). It matters
// because deleting the `aliases` FieldDef does not remove a stored shadow row, it demotes
// it: with the key no longer canonical, F39 auto-registration would render the leftover as
// a display-only "Aliases" field — the second list this feature removes, arriving through
// a different door. The pass moves those values into the spine and deletes the rows.
func TestPromoteEnrichmentAliases(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	hayao := seedPerson(t, r, "Hayao Miyazaki")
	jen := seedPerson(t, r, "Jennifer Lawrence")

	// Shadow rows exactly as a pre-F58 enrich wrote them, including one value that
	// collides with another person's alias and one that is the entity's own name.
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityPerson, jen, "J Law"); err != nil {
		t.Fatalf("seed alias: %v", err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, hayao, tmdb, "tmdb:608",
		map[string][]string{
			"bio":     {"A biography."},
			"aliases": {"宮崎駿", "Hayao Miyazaki", "J Law"},
		}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}

	promoted, err := r.PromoteEnrichmentAliases(ctx)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if promoted != 1 {
		t.Errorf("promoted = %d, want 1 (one entity/provider row)", promoted)
	}

	// The real alias landed; the guards applied to old data exactly as to new — the
	// entity's own name and the name another person holds are both absent.
	got := aliasStrings(t, r, model.EnrichEntityPerson, hayao)
	if len(got) != 1 || got[0] != "宮崎駿" {
		t.Errorf("promoted aliases = %v, want just [宮崎駿]", got)
	}
	if v := reviewVariations(t, db); len(v) != 1 || v[0] != "provider-alias" {
		t.Errorf("the colliding value should have queued for review, got %v", v)
	}

	// The shadow rows are gone, so nothing auto-registers; unrelated keys are untouched.
	var aliasRows, bioRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM entity_enrichment WHERE field_key='aliases'`).Scan(&aliasRows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM entity_enrichment WHERE field_key='bio'`).Scan(&bioRows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if aliasRows != 0 {
		t.Errorf("shadow alias rows survived: %d", aliasRows)
	}
	if bioRows != 1 {
		t.Errorf("the pass deleted an unrelated enrichment key (bio rows = %d)", bioRows)
	}

	// Idempotent: a second run finds nothing and changes nothing.
	again, err := r.PromoteEnrichmentAliases(ctx)
	if err != nil {
		t.Fatalf("second promote: %v", err)
	}
	if again != 0 {
		t.Errorf("second run promoted %d, want 0", again)
	}
	if got := aliasStrings(t, r, model.EnrichEntityPerson, hayao); len(got) != 1 {
		t.Errorf("second run changed the spine: %v", got)
	}
}

// TestSkippedAliasesForEntity covers the read behind the panel's collision review line
// (F58 P0-8). The skipped name exists nowhere but identity_review_queue.detail once the
// enrich pass ends — F58 stopped storing provider aliases in the shadow layer — so
// migration 0045's column is what makes the line able to say *which* name was dropped.
func TestSkippedAliasesForEntity(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	jen := seedPerson(t, r, "Jennifer Lawrence")
	imposter := seedPerson(t, r, "J. Lawrence")
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityPerson, jen, "J Law"); err != nil {
		t.Fatalf("seed alias: %v", err)
	}
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, imposter, tmdb,
		[]string{"J Law"}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Returned to the DENIED side only. The queue row is a pair and reads from both ends,
	// but the panel's sentence ("<name> already belongs to another person") is only true
	// for the entity that was refused the name. On the entity that *owns* "J Law" the same
	// line would assert the opposite of the truth, so it must not appear there.
	got, err := r.SkippedAliasesForEntity(ctx, model.EnrichEntityPerson, imposter)
	if err != nil {
		t.Fatalf("skipped for the denied entity: %v", err)
	}
	if len(got) != 1 || got[0].Alias != "J Law" {
		t.Fatalf("denied side = %+v, want the 'J Law' collision", got)
	}
	if got[0].ConflictID != jen {
		t.Errorf("conflict id = %d, want the holder (%d)", got[0].ConflictID, jen)
	}
	if got, _ := r.SkippedAliasesForEntity(ctx, model.EnrichEntityPerson, jen); len(got) != 0 {
		t.Errorf("the holder's own page reported its own name as skipped: %+v", got)
	}

	// The holder can own the name as its CANONICAL name rather than as an alias — the
	// other route entityConflict walks when it refuses the insert, and one the derivation
	// has to cover too.
	byName := seedPerson(t, r, "Katniss")
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, imposter, tmdb,
		[]string{"Katniss"}); err != nil {
		t.Fatalf("apply canonical-name collision: %v", err)
	}
	if got, _ := r.SkippedAliasesForEntity(ctx, model.EnrichEntityPerson, imposter); len(got) != 2 {
		t.Errorf("denied side should now report both collisions, got %+v", got)
	}
	if got, _ := r.SkippedAliasesForEntity(ctx, model.EnrichEntityPerson, byName); len(got) != 0 {
		t.Errorf("the canonical-name holder reported its own name as skipped: %+v", got)
	}

	// A stale pair goes quiet on both pages: once the holder drops the alias the name is
	// free, nothing is being refused, and there is nothing left to report.
	held, _ := r.AliasesForEntity(ctx, model.EnrichEntityPerson, jen)
	for _, a := range held {
		if a.Alias == "J Law" {
			if err := r.DeleteEntityAlias(ctx, model.EnrichEntityPerson, jen, a.ID); err != nil {
				t.Fatalf("delete: %v", err)
			}
		}
	}
	if got, _ := r.SkippedAliasesForEntity(ctx, model.EnrichEntityPerson, imposter); len(got) != 1 || got[0].Alias != "Katniss" {
		t.Errorf("denied side after the holder freed 'J Law' = %+v, want only the Katniss pair", got)
	}
	if got, _ := r.SkippedAliasesForEntity(ctx, model.EnrichEntityPerson, jen); len(got) != 0 {
		t.Errorf("the former holder still reports a collision: %+v", got)
	}

	// An unrelated person has nothing to report.
	bystander := seedPerson(t, r, "Someone Else")
	if got, _ := r.SkippedAliasesForEntity(ctx, model.EnrichEntityPerson, bystander); len(got) != 0 {
		t.Errorf("uninvolved person = %+v, want none", got)
	}

	// A near-miss pair carries no detail and must not leak into this read — only
	// 'provider-alias' rows describe a skipped name.
	if _, err := db.Exec(`INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation)
		VALUES ('person', ?, ?, 'punctuation')`, bystander, jen); err != nil {
		t.Fatalf("seed near-miss: %v", err)
	}
	if got, _ := r.SkippedAliasesForEntity(ctx, model.EnrichEntityPerson, bystander); len(got) != 0 {
		t.Errorf("a near-miss pair leaked into skipped aliases: %+v", got)
	}

	// Resolving the pair clears the line with no extra bookkeeping — the queue row IS
	// the outstanding question, which is why this read derives from it.
	if err := r.AddKeepSeparate(ctx, model.EnrichEntityPerson, jen, imposter); err != nil {
		t.Fatalf("keep separate: %v", err)
	}
	if err := r.DismissReviewPair(ctx, model.EnrichEntityPerson, jen, imposter); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	// Only that pair clears; the unrelated Katniss collision is untouched, which is what
	// makes this a resolution rather than a blanket mute.
	after, _ := r.SkippedAliasesForEntity(ctx, model.EnrichEntityPerson, imposter)
	for _, a := range after {
		if a.Alias == "J Law" {
			t.Errorf("resolving the pair left its review line standing: %+v", after)
		}
	}
	if len(after) != 1 {
		t.Errorf("resolving one pair disturbed the other: %+v, want just Katniss", after)
	}
}

// TestAliasSourceRoundTrips pins the provenance column on the read path (F58 P0-8): the
// chip badge is the only consumer, but a Scan that silently dropped it would make every
// provider alias look owner-typed and the badge would just never render.
func TestAliasSourceRoundTrips(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id := seedPerson(t, r, "Hayao Miyazaki")

	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, id, tmdb,
		[]string{"宮崎駿"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityPerson, id, "Miyazaki-san"); err != nil {
		t.Fatalf("add owner alias: %v", err)
	}

	bySource := map[string]string{}
	for _, a := range mustAliases(t, r, model.EnrichEntityPerson, id) {
		bySource[a.Alias] = a.Source
	}
	if bySource["宮崎駿"] != tmdb {
		t.Errorf("provider alias source = %q, want %q", bySource["宮崎駿"], tmdb)
	}
	if bySource["Miyazaki-san"] != "" {
		t.Errorf("owner alias source = %q, want empty", bySource["Miyazaki-san"])
	}

	// The batch read (used by completeness and list surfaces) must agree with the
	// per-entity one — they are separate SELECTs and could drift.
	batch, err := r.AliasesForEntities(ctx, model.EnrichEntityPerson, []int64{id})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	for _, a := range batch[id] {
		if a.Source != bySource[a.Alias] {
			t.Errorf("batch read disagrees for %q: %q vs %q", a.Alias, a.Source, bySource[a.Alias])
		}
	}
}

func mustAliases(t *testing.T, r *repo.Repo, entityType string, id int64) []model.EntityAlias {
	t.Helper()
	got, err := r.AliasesForEntity(context.Background(), entityType, id)
	if err != nil {
		t.Fatalf("aliases: %v", err)
	}
	return got
}
