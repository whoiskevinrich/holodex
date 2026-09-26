package repo_test

import (
	"context"
	"database/sql"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// The write-time contested-provider-id guard on Repo.AttachExternalID — spec F71 P0-3,
// ADR-107 D4. Before it, the enrich path's INSERT OR IGNORE discarded an identity claim
// with no error, no warning and no review row, while the field values were stored against
// the colliding id anyway. Every collision the live library carries was produced that way.

// spineOwner returns the entity entity_external_ids assigns an id to, or 0 when unowned.
func spineOwner(t *testing.T, db *sql.DB, entityType, externalID string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(`SELECT entity_id FROM entity_external_ids
		WHERE entity_type = ? AND external_id = ?`, entityType, externalID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0
	}
	if err != nil {
		t.Fatalf("spine owner of %s %q: %v", entityType, externalID, err)
	}
	return id
}

// pairDetail returns the detail recorded for a review pair — for shared-external-id rows,
// the asserting provider's namespace, which the row's chip cites (P0-6).
func pairDetail(t *testing.T, db *sql.DB, entityType string, lo, hi int64) string {
	t.Helper()
	var d string
	err := db.QueryRow(`SELECT detail FROM identity_review_queue
		WHERE entity_type = ? AND id_lo = ? AND id_hi = ?`, entityType, lo, hi).Scan(&d)
	if err == sql.ErrNoRows {
		return ""
	}
	if err != nil {
		t.Fatalf("detail of %s (%d,%d): %v", entityType, lo, hi, err)
	}
	return d
}

func mustAttach(t *testing.T, r *repo.Repo, entityType string, entityID int64, externalID string) {
	t.Helper()
	// The enrich must still succeed: the provider's field values are already stored, and
	// only the identity claim is contested. A guard that returned an error here would turn
	// a stored pass into a failed one — ADR-088's queue-don't-fail posture.
	if err := r.AttachExternalID(context.Background(), entityType, entityID, externalID); err != nil {
		t.Fatalf("attach %s id %q to %d: %v", entityType, externalID, entityID, err)
	}
}

// TestAttachExternalIDQueuesContestedID is the headline case: the owner picks the wrong
// person in the enrich UI, so a provider id another person already owns is claimed twice.
// The pair must reach the review queue, the spine must keep its existing owner, and the
// call must not fail.
//
// It also covers the anti-flood case the ADR calls out as the one an obvious
// implementation gets wrong: re-attaching an id THIS entity already owns queues nothing.
// INSERT OR IGNORE returns nil for inserted, already-owned-by-me and owned-by-another
// alike, so a guard keyed on RowsAffected() == 0 would queue on every re-enrich and
// refresh sweep.
func TestAttachExternalIDQueuesContestedID(t *testing.T) {
	r, db := newRepoDB(t)

	mustExec(t, db, `INSERT INTO people (id, name) VALUES (1,'Ada Lovelace'),(2,'Grace Hopper')`)

	// 1. A free id: recorded, nothing queued.
	mustAttach(t, r, model.EnrichEntityPerson, 1, "prov1:p7")
	if got := spineOwner(t, db, model.EnrichEntityPerson, "prov1:p7"); got != 1 {
		t.Fatalf("spine owner after first attach = %d, want person 1", got)
	}
	if n := rowCount(t, db, "identity_review_queue"); n != 0 {
		t.Fatalf("queue rows after a free attach = %d, want 0", n)
	}

	// 2. THE ANTI-FLOOD CASE. Re-enriching person 1 against the id person 1 already owns
	//    is the common path on every refresh sweep. It must queue nothing.
	mustAttach(t, r, model.EnrichEntityPerson, 1, "prov1:p7")
	if n := rowCount(t, db, "identity_review_queue"); n != 0 {
		t.Fatalf("queue rows after a benign re-attach = %d, want 0 — the guard is keyed on the OWNER, not on rows-affected", n)
	}

	// 3. The contest: person 2 claims person 1's id.
	mustAttach(t, r, model.EnrichEntityPerson, 2, "prov1:p7")
	pairs := readReviewQueue(t, db)
	if want := (reviewPair{model.EnrichEntityPerson, 1, 2, "shared-external-id"}); !hasPair(pairs, want) {
		t.Fatalf("queue = %+v, want %+v", pairs, want)
	}
	if len(pairs) != 1 {
		t.Errorf("queue rows = %d, want exactly 1", len(pairs))
	}
	// The spine keeps its owner and person 2 gains nothing: the owner's merge decides
	// which entity the provider record names, never this write (ADR-107 D3).
	if got := spineOwner(t, db, model.EnrichEntityPerson, "prov1:p7"); got != 1 {
		t.Errorf("spine owner after the contest = %d, want person 1 unchanged", got)
	}
	if n := rowCount(t, db, "entity_external_ids"); n != 1 {
		t.Errorf("spine rows = %d, want 1 — a contested claim must not fork the id", n)
	}
	// detail names the asserting provider, which the row's chip cites (P0-6) and the pair
	// cannot be read off the two entities.
	if got := pairDetail(t, db, model.EnrichEntityPerson, 1, 2); got != "prov1" {
		t.Errorf("detail = %q, want the asserting provider %q", got, "prov1")
	}

	// 4. Idempotent: the same contest again leaves one row.
	mustAttach(t, r, model.EnrichEntityPerson, 2, "prov1:p7")
	if n := rowCount(t, db, "identity_review_queue"); n != 1 {
		t.Errorf("queue rows after re-contesting = %d, want 1", n)
	}
}

// TestAttachExternalIDGuardHonorsKeepSeparateAndUpgrades covers the two ways an existing
// row changes the outcome: a pair the owner has already dismissed is never re-proposed
// (ADR-061's durable no), and a pair the NAME detector queued under a weaker variation is
// upgraded rather than left wearing a label the queue renders as "weak" (spec RD8).
func TestAttachExternalIDGuardHonorsKeepSeparateAndUpgrades(t *testing.T) {
	r, db := newRepoDB(t)

	mustExec(t, db, `INSERT INTO people (id, name) VALUES
		(1,'Ada Lovelace'),(2,'Grace Hopper'),(3,'Alan Turing'),(4,'Alonzo Church')`)
	mustExec(t, db, `INSERT INTO entity_keep_separate (entity_type, id_lo, id_hi) VALUES ('person',1,2)`)
	mustExec(t, db, `INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation)
		VALUES ('person',3,4,'punctuation')`)

	mustAttach(t, r, model.EnrichEntityPerson, 1, "prov1:p1")
	mustAttach(t, r, model.EnrichEntityPerson, 2, "prov1:p1") // contested, but dismissed
	for _, p := range readReviewQueue(t, db) {
		if p.idLo == 1 && p.idHi == 2 {
			t.Errorf("kept-separate pair was queued as %q — a dismissal is durable", p.variation)
		}
	}

	mustAttach(t, r, model.EnrichEntityPerson, 3, "prov1:p3")
	mustAttach(t, r, model.EnrichEntityPerson, 4, "prov1:p3") // contested, already queued weakly
	pairs := readReviewQueue(t, db)
	if want := (reviewPair{model.EnrichEntityPerson, 3, 4, "shared-external-id"}); !hasPair(pairs, want) {
		t.Fatalf("queue = %+v, want the punctuation pair upgraded to %+v", pairs, want)
	}
	if len(pairs) != 1 {
		t.Errorf("queue rows = %d, want 1 — the upgrade replaces the variation, it does not add a row", len(pairs))
	}
}

// TestProviderAliasRowSurfacesOnceUpgraded is ADR-108 D2's escape hatch. A provider-alias
// pair is not listed in the Duplicates queue, but when the provider's ID lands on both
// entities the same row is upgraded to 'shared-external-id' in place (spec RD8), and that
// strong evidence must reach the owner despite the row having started hidden.
func TestProviderAliasRowSurfacesOnceUpgraded(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()

	mustExec(t, db, `INSERT INTO people (id, name) VALUES (1,'Ada Lovelace'),(2,'Grace Hopper')`)
	mustExec(t, db, `INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation, detail)
		VALUES ('person',1,2,'provider-alias','Countess')`)

	if pairs, err := r.ListReviewPairs(ctx); err != nil || len(pairs) != 0 {
		t.Fatalf("before the upgrade: pairs = %+v, err = %v, want none listed", pairs, err)
	}

	mustAttach(t, r, model.EnrichEntityPerson, 1, "prov1:p1")
	mustAttach(t, r, model.EnrichEntityPerson, 2, "prov1:p1")

	pairs, err := r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatalf("list review pairs: %v", err)
	}
	if len(pairs) != 1 || pairs[0].Variation != "shared-external-id" {
		t.Fatalf("after the upgrade: pairs = %+v, want the one pair as shared-external-id", pairs)
	}
}

// TestAttachExternalIDGuardKindScope proves ADR-107 D5's scope with tests rather than
// comments. Studio and film run the identical path and are in scope; TAG is excluded
// because it is not enrichable, so a contested tag id keeps the old silent-ignore
// behaviour. Video never reaches this method at all — enrich.identityEntityType stops it a
// layer up, and two files of one movie sharing a provider id is correct, not a duplicate.
func TestAttachExternalIDGuardKindScope(t *testing.T) {
	r, db := newRepoDB(t)

	mustExec(t, db, `INSERT INTO studios (id, name) VALUES (1,'Spine Pictures'),(2,'Memo Pictures')`)
	mustExec(t, db, `INSERT INTO films (id, name, year) VALUES (1,'Film Alpha',2001),(2,'Film Beta',2002)`)
	mustExec(t, db, `INSERT INTO tags (id, name) VALUES (1,'noir'),(2,'neo-noir')`)

	mustAttach(t, r, model.EnrichEntityStudio, 1, "prov1:s1")
	mustAttach(t, r, model.EnrichEntityStudio, 2, "prov1:s1")
	mustAttach(t, r, model.EnrichEntityFilm, 1, "prov1:f1")
	mustAttach(t, r, model.EnrichEntityFilm, 2, "prov1:f1")
	mustAttach(t, r, model.EntityTag, 1, "prov1:t1")
	mustAttach(t, r, model.EntityTag, 2, "prov1:t1")

	pairs := readReviewQueue(t, db)
	for _, want := range []reviewPair{
		{model.EnrichEntityStudio, 1, 2, "shared-external-id"},
		{model.EnrichEntityFilm, 1, 2, "shared-external-id"},
	} {
		if !hasPair(pairs, want) {
			t.Errorf("queue = %+v, want %+v", pairs, want)
		}
	}
	for _, p := range pairs {
		if p.entityType == model.EntityTag {
			t.Errorf("tag pair %+v was queued — tag is excluded by construction (ADR-107 D5)", p)
		}
	}
	if len(pairs) != 2 {
		t.Errorf("queue rows = %d, want exactly 2 (studio + film)", len(pairs))
	}
	// The tag id still behaves as it always did: first claimant keeps it, silently.
	if got := spineOwner(t, db, model.EntityTag, "prov1:t1"); got != 1 {
		t.Errorf("tag spine owner = %d, want tag 1 (silent-ignore preserved)", got)
	}
}

// TestScanPathAttachStillSilent asserts — rather than assumes — that the scan path is
// unchanged: a credit carrying an id another person owns resolves to THAT person and
// writes no review row. The name in the credit deliberately disagrees with the resolved
// person, which is exactly the case id-first precedence exists for.
//
// Note what this does and does not prove. resolveOrCreateByName's step 1 ("external-id
// first") returns the owner before ever reaching the private attachExternalID, so a contested id
// structurally cannot arrive at that writer — which is a stronger reason to keep the guard
// off it than ADR-107 D4's "would fire on every relink", and it means no test can
// distinguish a guard placed there by its queue output. What remains testable, and tested
// here, is that the relink path still resolves and still queues nothing.
func TestScanPathAttachStillSilent(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()

	mustExec(t, db, `INSERT INTO people (id, name) VALUES (1,'Ada Lovelace')`)
	mustAttach(t, r, model.EnrichEntityPerson, 1, "prov1:p7")

	got, err := r.ResolveOrCreatePeopleByExternalID(ctx, []repo.PersonCredit{
		{Name: "Grace Hopper", ExternalID: "prov1:p7"},
	})
	if err != nil {
		t.Fatalf("resolve credit: %v", err)
	}
	if got["prov1:p7"] != 1 {
		t.Fatalf("credit resolved to person %d, want 1 — id-first precedence (ADR-061 D5)", got["prov1:p7"])
	}
	if n := rowCount(t, db, "identity_review_queue"); n != 0 {
		t.Errorf("queue rows after a scan-path resolve = %d, want 0 — the guard must not reach the private writer", n)
	}
	if n := rowCount(t, db, "people"); n != 1 {
		t.Errorf("people = %d, want 1 — the credit resolved, it did not create", n)
	}
}

// TestSweepSharedExternalIDs is the every-boot reconciliation — spec F71 P0-1/P0-4/P0-5,
// ADR-107 D1/D5/D6. Migration 0052 drains the historical backlog once; this keeps it
// drained as enrichment keeps adding memos.
//
// The fixture seeds entity_enrichment with raw SQL rather than through UpsertEnrichment on
// purpose: the stale-narrow-re-enrich case needs two DIFFERENT fetched_at values for one
// (entity, provider), and two UpsertEnrichment calls in the same test would be free to land
// on the same timestamp, turning a rule this asserts into a coin flip.
func TestSweepSharedExternalIDs(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()

	mustExec(t, db, `INSERT INTO people (id, name) VALUES
		(1,'Ada Lovelace'),(2,'Grace Hopper'),(3,'Alan Turing'),(4,'Alonzo Church'),
		(5,'Barbara Liskov'),(6,'Leslie Lamport'),(7,'Edsger Dijkstra'),(8,'Tony Hoare'),
		(9,'Ken Thompson'),(10,'Dennis Ritchie'),(11,'Robin Milner'),(12,'Niklaus Wirth')`)
	mustExec(t, db, `INSERT INTO studios (id, name) VALUES
		(100,'Spine Pictures'),(101,'Memo Pictures'),(102,'Other Memo Pictures')`)
	mustExec(t, db, `INSERT INTO films (id, name, year) VALUES (200,'Film Alpha',2001),(201,'Film Beta',2002)`)
	mustExec(t, db, `INSERT INTO tags (id, name) VALUES (300,'noir'),(301,'neo-noir')`)
	mustExec(t, db, `INSERT INTO videos (id, file_path, indexed_at, file_mtime) VALUES
		(1,'/m/a.mkv','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z'),
		(2,'/m/b.mkv','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)

	// The spine as it stands: two ids recorded.
	mustExec(t, db, `INSERT INTO entity_external_ids (entity_type, entity_id, external_id) VALUES
		('person',   3, 'prov1:p3'),
		('person',   6, 'prov1:p6'),
		('studio', 100, 'prov1:s1')`)

	mustExec(t, db, `INSERT INTO entity_enrichment
		(entity_type, entity_id, provider, field_key, value, external_id, fetched_at) VALUES
		-- memo-to-memo, NO spine owner on either side: invisible to a spine-anchored join.
		('person',  1, 'prov1', 'bio',    'x', 'prov1:p7',  '2026-03-01T00:00:00Z'),
		('person',  2, 'prov1', 'bio',    'x', 'prov1:p7',  '2026-03-01T00:00:00Z'),
		-- memo disagrees with the spine, which gives prov1:p3 to person 3.
		('person',  4, 'prov1', 'bio',    'x', 'prov1:p3',  '2026-03-01T00:00:00Z'),
		-- stale narrow re-enrich: the OLD memo names person 6's id, the NEWEST a free one.
		('person',  5, 'prov1', 'bio',    'x', 'prov1:p6',  '2026-01-01T00:00:00Z'),
		('person',  5, 'prov1', 'height', 'x', 'prov1:p5',  '2026-02-01T00:00:00Z'),
		-- the filename-extract population memoizes '' by design (rule 1).
		('person',  7, 'filename', 'bio', 'x', '',          '2026-03-01T00:00:00Z'),
		-- rule 1 INSIDE a group: person 11's NEWEST memo for this provider carries no id at
		-- all, an older one names prov1:p11, and person 12 claims the same id. The newest
		-- NON-EMPTY memo is the winner, so the pair is still found. Without the non-empty
		-- test in the winner subquery the empty memo wins, the shape test discards it, and
		-- the pair goes MISSING -- a false negative no other case in this fixture produces.
		('person', 11, 'prov1', 'bio',    'x', '',          '2026-04-01T00:00:00Z'),
		('person', 11, 'prov1', 'height', 'x', 'prov1:p11', '2026-03-01T00:00:00Z'),
		('person', 12, 'prov1', 'bio',    'x', 'prov1:p11', '2026-03-01T00:00:00Z'),
		-- a shape identityShaped rejects.
		('person',  8, 'prov1', 'bio',    'x', 'nocolon',   '2026-03-01T00:00:00Z'),
		-- contested, but the owner already dismissed the pair (rule 4).
		('person',  9, 'prov1', 'bio',    'x', 'prov1:p9',  '2026-03-01T00:00:00Z'),
		('person', 10, 'prov1', 'bio',    'x', 'prov1:p9',  '2026-03-01T00:00:00Z'),
		-- THREE claimants on one studio id: spine owner 100 plus two memo holders.
		('studio', 101, 'prov1', 'bio',   'x', 'prov1:s1',  '2026-03-01T00:00:00Z'),
		('studio', 102, 'prov1', 'bio',   'x', 'prov1:s1',  '2026-03-01T00:00:00Z'),
		-- film is in scope (ADR-107 D5).
		('film',   200, 'prov1', 'bio',   'x', 'prov1:f1',  '2026-03-01T00:00:00Z'),
		('film',   201, 'prov1', 'bio',   'x', 'prov1:f1',  '2026-03-01T00:00:00Z'),
		-- VIDEO IS EXCLUDED BY CONSTRUCTION (P0-5): two files of one movie sharing a
		-- provider id is correct, not a duplicate. On the live library this alone would
		-- have produced 23 wrong findings — more than there are right ones.
		('video',    1, 'prov1', 'bio',   'x', 'prov1:v1',  '2026-03-01T00:00:00Z'),
		('video',    2, 'prov1', 'bio',   'x', 'prov1:v1',  '2026-03-01T00:00:00Z'),
		-- TAG IS EXCLUDED (P0-5): it is not enrichable, so a contested tag id is not the
		-- owner-mis-pick this feature reports.
		('tag',    300, 'prov1', 'bio',   'x', 'prov1:t1',  '2026-03-01T00:00:00Z'),
		('tag',    301, 'prov1', 'bio',   'x', 'prov1:t1',  '2026-03-01T00:00:00Z')`)

	mustExec(t, db, `INSERT INTO entity_keep_separate (entity_type, id_lo, id_hi) VALUES ('person',9,10)`)
	// Already queued by the NAME detector as the weaker variation — RD8 upgrades it.
	mustExec(t, db, `INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation)
		VALUES ('person',1,2,'punctuation')`)

	written, err := r.SweepSharedExternalIDs(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if written != 7 {
		t.Errorf("sweep wrote %d rows, want 7 (6 new pairs + 1 upgraded)", written)
	}

	pairs := readReviewQueue(t, db)
	for _, want := range []reviewPair{
		{model.EnrichEntityPerson, 1, 2, "shared-external-id"},     // memo-to-memo, upgraded
		{model.EnrichEntityPerson, 3, 4, "shared-external-id"},     // memo vs spine
		{model.EnrichEntityStudio, 100, 101, "shared-external-id"}, // the clique: all three
		{model.EnrichEntityStudio, 100, 102, "shared-external-id"}, // pairs, including the
		{model.EnrichEntityStudio, 101, 102, "shared-external-id"}, // memo-to-memo one
		{model.EnrichEntityFilm, 200, 201, "shared-external-id"},
		{model.EnrichEntityPerson, 11, 12, "shared-external-id"}, // newest memo has no id
	} {
		if !hasPair(pairs, want) {
			t.Errorf("pair %+v not queued", want)
		}
	}
	if len(pairs) != 7 {
		t.Errorf("queue rows = %d, want exactly 7: %+v", len(pairs), pairs)
	}

	// The exclusions, as assertions rather than comments.
	for _, none := range []struct {
		what   string
		et     string
		lo, hi int64
	}{
		{"two files of one movie sharing a provider id", model.EnrichEntityVideo, 1, 2},
		{"two tags sharing a provider id", model.EntityTag, 300, 301},
		{"the stale narrow re-enrich", model.EnrichEntityPerson, 5, 6},
		{"a pair the owner keeps separate", model.EnrichEntityPerson, 9, 10},
	} {
		for _, p := range pairs {
			if p.entityType == none.et && p.idLo == none.lo && p.idHi == none.hi {
				t.Errorf("%s was queued as %q", none.what, p.variation)
			}
		}
	}

	// Every row names its asserting provider, including the one whose weaker variation was
	// upgraded over an empty detail.
	for _, want := range []struct {
		et     string
		lo, hi int64
	}{
		{model.EnrichEntityPerson, 1, 2},
		{model.EnrichEntityPerson, 3, 4},
		{model.EnrichEntityStudio, 100, 101},
		{model.EnrichEntityFilm, 200, 201},
	} {
		if got := pairDetail(t, db, want.et, want.lo, want.hi); got != "prov1" {
			t.Errorf("%s (%d,%d) detail = %q, want %q", want.et, want.lo, want.hi, got, "prov1")
		}
	}

	// The sweep QUEUES; it never folds. Repairing the spine was migration 0052's job, and
	// assigning a contested id to one claimant would be an adjudication (ADR-107 D3).
	if n := rowCount(t, db, "entity_external_ids"); n != 3 {
		t.Errorf("spine rows = %d, want the 3 seeded — the sweep must not write identity", n)
	}

	// Idempotent: an unchanged library writes nothing on the next boot, so the activity
	// row reads 0 rather than re-reporting every pair forever.
	written, err = r.SweepSharedExternalIDs(ctx)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if written != 0 {
		t.Errorf("second sweep wrote %d rows, want 0", written)
	}
	if n := len(readReviewQueue(t, db)); n != 7 {
		t.Errorf("queue rows after the second sweep = %d, want 7", n)
	}
}

// TestSharedExternalIDSortsAboveEveryOtherVariation is spec P0-8. Before F71 every
// non-fuzzy variation shared ListReviewPairs' -1 sort slot with no tiebreak, so the
// strongest signal the queue can carry and the weakest resolvable conflict were ordered by
// nothing but insertion order. The name that sorts alphabetically LAST carries the shared-id
// pair here, so the assertion cannot pass by accident of the name tiebreak.
func TestSharedExternalIDSortsAboveEveryOtherVariation(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()

	mustExec(t, db, `INSERT INTO people (id, name) VALUES
		(1,'Aaron Alias'),(2,'Aaron Aliass'),
		(3,'Mary Jane'),(4,'MaryJane'),
		(5,'Zoe Zulu'),(6,'Zane Zulu')`)
	mustExec(t, db, `INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation) VALUES
		('person',1,2,'provider-alias'),
		('person',3,4,'internal-whitespace'),
		('person',5,6,'shared-external-id')`)

	pairs, err := r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatalf("list review pairs: %v", err)
	}
	var got []string
	for _, p := range pairs {
		if p.EntityType == model.EnrichEntityPerson {
			got = append(got, p.Variation)
		}
	}
	// The provider-alias row is seeded but not listed (ADR-108): it must neither appear nor
	// displace the order of the rows that are.
	want := []string{"shared-external-id", "internal-whitespace"}
	if len(got) != len(want) {
		t.Fatalf("person pairs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sort order = %v, want %v", got, want)
		}
	}

	// The -2 slot against another NON-fuzzy variation, which person no longer lists. Film
	// still has one: same-title shares the -1 slot, and it carries the alphabetically
	// first names so the name tiebreak alone would put it on top.
	mustExec(t, db, `INSERT INTO films (id, name, year) VALUES
		(1,'Alpha One',2001),(2,'Alpha Two',2002),(3,'Zulu One',2003),(4,'Zulu Two',2004)`)
	mustExec(t, db, `INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation) VALUES
		('film',1,2,'same-title'),
		('film',3,4,'shared-external-id')`)
	pairs, err = r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatalf("list review pairs: %v", err)
	}
	var films []string
	for _, p := range pairs {
		if p.EntityType == model.EnrichEntityFilm {
			films = append(films, p.Variation)
		}
	}
	if len(films) != 2 || films[0] != "shared-external-id" || films[1] != "same-title" {
		t.Fatalf("film sort order = %v, want [shared-external-id same-title]", films)
	}
}

// TestListReviewPairsDetailScopedToSharedID pins the minimal-disclosure scoping the
// security gate asked for (2026-09-24). `detail` is shared by variations that mean
// different things: for 'shared-external-id' it is a provider namespace the row's chip
// cites, but for 'provider-alias' it is a SKIPPED PERSON NAME whose only correct reader is
// SkippedAliasesForEntity — which returns it to the denied side of the pair only, because
// on the holding side the same name asserts the opposite of the truth. So the queue payload
// projects the column for the one variation that asked for it and '' for the rest. Widening
// that CASE means deciding what the new consumer is allowed to conclude from the value.
func TestListReviewPairsDetailScopedToSharedID(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()

	mustExec(t, db, `INSERT INTO people (id, name) VALUES
		(1,'Ada Lovelace'),(2,'Grace Hopper'),(3,'Alan Turing'),(4,'Alonzo Church')`)
	mustExec(t, db, `INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation, detail) VALUES
		('person',1,2,'shared-external-id','prov1'),
		('person',3,4,'provider-alias','Some Skipped Name')`)

	pairs, err := r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatalf("list review pairs: %v", err)
	}
	seen := map[string]string{}
	for _, p := range pairs {
		seen[p.Variation] = p.Detail
	}
	if got := seen["shared-external-id"]; got != "prov1" {
		t.Errorf("shared-external-id detail = %q, want the asserting provider", got)
	}
	// Since ADR-108 the provider-alias row is not listed at all, so this holds twice over.
	// No other variation writes `detail` today, which is why the CASE is kept and pinned
	// here anyway: removing the filter must not start leaking a skipped person name.
	if got := seen["provider-alias"]; got != "" {
		t.Errorf("provider-alias detail = %q, want it withheld from this payload", got)
	}
}
