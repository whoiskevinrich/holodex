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
