package db_test

import (
	"database/sql"
	"fmt"
	"testing"
)

// variationOf returns the variation recorded for a review pair, or "" when the pair is
// not queued.
func variationOf(t *testing.T, db *sql.DB, entityType string, lo, hi int) string {
	t.Helper()
	var v string
	err := db.QueryRow(`SELECT variation FROM identity_review_queue
		WHERE entity_type = ? AND id_lo = ? AND id_hi = ?`, entityType, lo, hi).Scan(&v)
	if err == sql.ErrNoRows {
		return ""
	}
	if err != nil {
		t.Fatalf("variation of %s (%d,%d): %v", entityType, lo, hi, err)
	}
	return v
}

// detailOf returns the detail recorded for a review pair, or "" when the pair is not queued.
func detailOf(t *testing.T, db *sql.DB, entityType string, lo, hi int) string {
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

// TestMigration0052BackfillsSpineFromMemo exercises spec F71 P0-9 / ADR-107 §3b: the
// one-time fold of entity_enrichment.external_id into entity_external_ids.
//
// The fixture is the probe's seven cases (scripts/detect_shared_external_id.sql) plus the
// three the backfill adds on top of a detector: the three-claimant clique, the memo-only
// contest with no spine owner, and the shapes that must never become a spine row.
//
// The load-bearing assertion is not the row count -- it is that a CONTESTED id is left
// unowned and queued instead of being assigned to one of its claimants.
func TestMigration0052BackfillsSpineFromMemo(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(51); err != nil { // one before the backfill
		t.Fatalf("migrate to 51: %v", err)
	}

	// Distinct names throughout: ADR-061's canonical nameKey unique index forbids two
	// entities of a kind sharing a name, which is why a split identity always wears two.
	mustExec(t, db, `INSERT INTO people (id, name) VALUES
		(1,'Aiden Alpha'),(2,'Bella Bravo'),(3,'Cara Charlie'),(4,'Dana Delta'),
		(5,'Evan Echo'),(6,'Fiona Foxtrot'),(7,'Gina Golf'),(8,'Hank Hotel'),
		(9,'Iris India'),(10,'Jack Juliet'),(11,'Kara Kilo'),(12,'Liam Lima'),
		(13,'Mira Mike'),(14,'Nate November'),(15,'Owen Oscar'),(16,'Pia Papa'),
		(17,'Quinn Quebec'),(18,'Rosa Romeo')`)
	mustExec(t, db, `INSERT INTO studios (id, name) VALUES
		(100,'Spine Pictures'),(101,'Memo Pictures'),(102,'Other Memo Pictures')`)
	mustExec(t, db, `INSERT INTO films (id, name, year) VALUES (200,'Film Alpha',2001)`)
	mustExec(t, db, `INSERT INTO tags (id, name) VALUES (300,'sometag')`)
	mustExec(t, db, `INSERT INTO videos (id, file_path, indexed_at, file_mtime) VALUES
		(1,'/m/a.mkv','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z'),
		(2,'/m/b.mkv','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)

	// The spine as it stands: six ids already recorded.
	mustExec(t, db, `INSERT INTO entity_external_ids (entity_type, entity_id, external_id) VALUES
		('person',   2, 'prov1:p2'),
		('person',   3, 'prov1:p3'),
		('person',   6, 'prov1:p6'),
		('person',  15, 'prov1:pz15'),
		('person',  16, 'prov1:pa16'),
		('studio', 100, 'prov1:s1')`)

	// The memo layer. One row per (entity, provider, field_key); the id and fetched_at
	// are what this migration reads.
	mustExec(t, db, `INSERT INTO entity_enrichment
		(entity_type, entity_id, provider, field_key, value, external_id, fetched_at) VALUES
		-- 1. no spine row, nobody else claims it → folded. The headline case: 511 of 1000
		--    enriched people on the host look like this.
		('person',  1, 'prov1', 'bio',       'x', 'prov1:p1',  '2026-03-01T00:00:00Z'),
		-- 2. memo agrees with the spine → no new row, no finding.
		('person',  2, 'prov1', 'bio',       'x', 'prov1:p2',  '2026-03-01T00:00:00Z'),
		-- 3. memo names an id the spine gives to person 3 → contested pair (3,4).
		('person',  4, 'prov1', 'bio',       'x', 'prov1:p3',  '2026-03-01T00:00:00Z'),
		-- 4. stale narrow re-enrich: the OLD memo points at person 6's id, the NEWEST at a
		--    free one. Rule 2 → fold prov1:p5, and NO (5,6) finding.
		('person',  5, 'prov1', 'bio',       'x', 'prov1:p6',  '2026-01-01T00:00:00Z'),
		('person',  5, 'prov1', 'height',    'x', 'prov1:p5',  '2026-02-01T00:00:00Z'),
		-- 5. two memo holders, NO spine owner → invisible to a spine-anchored join, and the
		--    id must stay unowned (ADR-107 D1).
		('person',  7, 'prov1', 'bio',       'x', 'prov1:p7',  '2026-03-01T00:00:00Z'),
		('person',  8, 'prov1', 'bio',       'x', 'prov1:p7',  '2026-03-01T00:00:00Z'),
		-- 6. contested, but the owner already said keep-separate → still no fold, and no
		--    queue row (ADR-061's durable no, rule 4).
		('person',  9, 'prov1', 'bio',       'x', 'prov1:p9',  '2026-03-01T00:00:00Z'),
		('person', 10, 'prov1', 'bio',       'x', 'prov1:p9',  '2026-03-01T00:00:00Z'),
		-- 7. the filename-extract population: '' by design (rule 1).
		('person', 11, 'filename', 'bio',    'x', '',          '2026-03-01T00:00:00Z'),
		-- 8. shapes identityShaped rejects: no colon, empty namespace, empty id.
		('person', 12, 'prov1', 'bio',       'x', 'nocolon',   '2026-03-01T00:00:00Z'),
		('person', 13, 'prov1', 'bio',       'x', ':p13',      '2026-03-01T00:00:00Z'),
		('person', 14, 'prov1', 'bio',       'x', 'prov1:',    '2026-03-01T00:00:00Z'),
		-- 9. fetched_at TIE: rule 2 breaks toward the memo that AGREES with the spine, so a
		--    tie yields no finding. Ordered by id alone the disagreeing 'prov1:pa16' would
		--    win and queue (15,16).
		('person', 15, 'prov1', 'bio',       'x', 'prov1:pz15','2026-03-01T00:00:00Z'),
		('person', 15, 'prov1', 'height',    'x', 'prov1:pa16','2026-03-01T00:00:00Z'),
		-- 10. a memo whose entity is gone: entity_external_ids has no FK, so an orphan row
		--     is only prevented by the migration's own exists-guard.
		('person', 999, 'prov1', 'bio',      'x', 'prov1:p999','2026-03-01T00:00:00Z'),
		-- 11. THREE claimants on one studio id: the spine owner plus two memo holders. The
		--     host run found exactly this, and it is why D1 pairs the claimant SET.
		('studio', 101, 'prov1', 'bio',      'x', 'prov1:s1',  '2026-03-01T00:00:00Z'),
		('studio', 102, 'prov1', 'bio',      'x', 'prov1:s1',  '2026-03-01T00:00:00Z'),
		-- 12. film is in scope (ADR-107 D5) — 28 of 45 on the host have a memo and no row.
		('film',   200, 'prov1', 'bio',      'x', 'prov1:f1',  '2026-03-01T00:00:00Z'),
		-- 13. video is excluded by construction: it has no spine row at all, and two files
		--     of one movie sharing a provider id is correct, not a duplicate (P0-5).
		('video',    1, 'prov1', 'bio',      'x', 'prov1:v1',  '2026-03-01T00:00:00Z'),
		('video',    2, 'prov1', 'bio',      'x', 'prov1:v1',  '2026-03-01T00:00:00Z'),
		-- 14. tag is excluded: it is not enrichable (P0-5).
		('tag',    300, 'prov1', 'bio',      'x', 'prov1:t1',  '2026-03-01T00:00:00Z'),
		-- 15. ONE pair colliding on TWO providers → exactly one queue row, not two (the queue
		--     PK stores a pair once), and the case SELECT DISTINCT collapses before the upsert.
		('person', 17, 'prov1', 'bio',       'x', 'prov1:p17', '2026-03-01T00:00:00Z'),
		('person', 18, 'prov1', 'bio',       'x', 'prov1:p17', '2026-03-01T00:00:00Z'),
		('person', 17, 'prov2', 'bio',       'x', 'prov2:q17', '2026-03-01T00:00:00Z'),
		('person', 18, 'prov2', 'bio',       'x', 'prov2:q17', '2026-03-01T00:00:00Z')`)

	mustExec(t, db, `INSERT INTO entity_keep_separate (entity_type, id_lo, id_hi) VALUES ('person', 9, 10)`)
	// A pair the NAME-based detector already queued, which is also a shared-id finding --
	// 1 of the host's 15 was exactly this. INSERT OR IGNORE on the queue PK (spec P0-2)
	// leaves the weaker variation standing.
	mustExec(t, db, `INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation)
		VALUES ('person', 7, 8, 'punctuation')`)

	if err := m.Migrate(52); err != nil {
		t.Fatalf("migrate to 52: %v", err)
	}

	// ── The fold: three uncontested memos become spine rows, nothing else does ──────
	for _, want := range []struct {
		entityType, externalID string
		entityID               int
	}{
		{"person", "prov1:p1", 1},    // case 1
		{"person", "prov1:p5", 5},    // case 4 — the NEWEST memo, not the stale one
		{"film", "prov1:f1", 200},    // case 12
		{"person", "prov1:p2", 2},    // case 2 — unchanged, not duplicated
		{"person", "prov1:p3", 3},    // case 3 — the spine keeps its owner
		{"person", "prov1:pz15", 15}, // case 9
		{"studio", "prov1:s1", 100},  // case 11 — the owner keeps it
	} {
		got := count(t, db, fmt.Sprintf(
			`SELECT COUNT(*) FROM entity_external_ids WHERE entity_type='%s' AND external_id='%s' AND entity_id=%d`,
			want.entityType, want.externalID, want.entityID))
		if got != 1 {
			t.Errorf("%s %s → entity %d: %d rows, want 1", want.entityType, want.externalID, want.entityID, got)
		}
	}
	if n := count(t, db, `SELECT COUNT(*) FROM entity_external_ids`); n != 9 {
		t.Errorf("spine rows = %d, want 9 (6 pre-existing + 3 folded)", n)
	}

	// ── Nothing the rules exclude became a spine row ────────────────────────────────
	for _, gone := range []struct{ what, where string }{
		// A contested id stays UNOWNED. Assigning it to one of two claimants would decide
		// which entity the provider record names, and id-first resolve would then route
		// every future credit that way -- an adjudication ADR-107 D3 forbids.
		{"contested memo-only id prov1:p7", `external_id='prov1:p7'`},
		{"contested kept-separate id prov1:p9", `external_id='prov1:p9'`},
		{"stale memo id folded onto the wrong person", `external_id='prov1:p6' AND entity_id=5`},
		{"unshaped id (no colon)", `external_id='nocolon'`},
		{"unshaped id (empty namespace)", `external_id=':p13'`},
		{"unshaped id (empty id)", `external_id='prov1:'`},
		{"orphan memo for a deleted entity", `external_id='prov1:p999'`},
		{"video id (no spine row by construction)", `entity_type='video'`},
		{"tag id (not enrichable)", `entity_type='tag'`},
		{"the empty filename memo", `external_id=''`},
	} {
		if n := count(t, db, `SELECT COUNT(*) FROM entity_external_ids WHERE `+gone.where); n != 0 {
			t.Errorf("%s: %d spine rows, want 0", gone.what, n)
		}
	}

	// ── The queue: every contested pair, and only those ─────────────────────────────
	pairs := map[string]bool{}
	rows, err := db.Query(`SELECT entity_type, id_lo, id_hi FROM identity_review_queue
		WHERE variation = 'shared-external-id' ORDER BY entity_type, id_lo, id_hi`)
	if err != nil {
		t.Fatalf("query queue: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var et string
		var lo, hi int
		if err := rows.Scan(&et, &lo, &hi); err != nil {
			t.Fatalf("scan: %v", err)
		}
		pairs[fmt.Sprintf("%s:%d-%d", et, lo, hi)] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	want := []string{
		"person:3-4",     // memo disagrees with the spine
		"person:7-8",     // memo-to-memo, and an UPGRADE over the name detector's row
		"person:17-18",   // one pair, two providers → one row
		"studio:100-101", // the three-claimant clique: all three pairs, including the
		"studio:100-102", // memo-to-memo one a spine-anchored join would drop
		"studio:101-102",
	}
	for _, w := range want {
		if !pairs[w] {
			t.Errorf("pair %s not queued as shared-external-id", w)
		}
	}
	if len(pairs) != len(want) {
		t.Errorf("shared-external-id pairs = %v, want exactly %v", pairs, want)
	}
	// (7,8) was already queued by the NAME detector as the weaker 'punctuation'. The strong
	// signal upgrades it (owner's decision 2026-09-24) rather than sorting in the fuzzy band
	// behind a label that means "weak" — the failure P0-8 exists to prevent.
	if got := variationOf(t, db, "person", 7, 8); got != "shared-external-id" {
		t.Errorf("pair (7,8) variation = %q, want the upgrade to shared-external-id", got)
	}

	// detail names the ASSERTING PROVIDER, which is what the row's chip cites (P0-6) and the
	// one fact the pair cannot be read off the two entities. A pair colliding on TWO
	// providers gets one of them deterministically — min(), so 'prov1' not 'prov2'.
	for _, want := range []struct {
		et       string
		lo, hi   int
		provider string
	}{
		{"person", 3, 4, "prov1"},
		{"person", 7, 8, "prov1"},   // carried in by the upgrade, over an empty detail
		{"person", 17, 18, "prov1"}, // collides on prov1 AND prov2
		{"studio", 100, 101, "prov1"},
	} {
		if got := detailOf(t, db, want.et, want.lo, want.hi); got != want.provider {
			t.Errorf("%s (%d,%d) detail = %q, want %q", want.et, want.lo, want.hi, got, want.provider)
		}
	}
	// Kept-separate is never re-proposed (ADR-061), and the tie broke toward agreement.
	for _, none := range []struct {
		what   string
		et     string
		lo, hi int
	}{
		{"kept-separate pair", "person", 9, 10},
		{"fetched_at tie broken toward the spine", "person", 15, 16},
		{"the stale narrow re-enrich", "person", 5, 6},
		{"two files of one movie", "video", 1, 2},
	} {
		if n := count(t, db, fmt.Sprintf(
			`SELECT COUNT(*) FROM identity_review_queue WHERE entity_type='%s' AND id_lo=%d AND id_hi=%d`,
			none.et, none.lo, none.hi)); n != 0 {
			t.Errorf("%s: %d queue rows, want 0", none.what, n)
		}
	}

	// ── Down leaves the fold, removes the review rows (0044's precedent) ────────────
	if err := m.Migrate(51); err != nil {
		t.Fatalf("migrate down to 51: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM entity_external_ids`); n != 9 {
		t.Errorf("spine rows after down = %d, want 9 (the fold is documented as one-way)", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM identity_review_queue WHERE variation='shared-external-id'`); n != 0 {
		t.Errorf("shared-external-id rows after down = %d, want 0", n)
	}
	// An upgraded pair loses its row rather than being restored to a variation nothing
	// recorded; SeedIdentityReviewQueue re-derives it from the names on the next boot.
	if got := variationOf(t, db, "person", 7, 8); got != "" {
		t.Errorf("upgraded pair after down = %q, want it gone (the prior variation is unrecorded)", got)
	}

	// ── Re-applying is a no-op on the fold and re-queues the same pairs ─────────────
	if err := m.Migrate(52); err != nil {
		t.Fatalf("re-migrate to 52: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM entity_external_ids`); n != 9 {
		t.Errorf("spine rows after re-apply = %d, want 9 (INSERT OR IGNORE)", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM identity_review_queue WHERE variation='shared-external-id'`); n != 6 {
		t.Errorf("shared-external-id rows after re-apply = %d, want 6", n)
	}
}
