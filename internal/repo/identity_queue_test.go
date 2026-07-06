package repo_test

import (
	"context"
	"database/sql"
	"testing"
)

// reviewPair mirrors an identity_review_queue row for assertions.
type reviewPair struct {
	entityType string
	idLo, idHi int64
	variation  string
}

// readReviewQueue returns every identity_review_queue row, ordered for stable
// comparison.
func readReviewQueue(t *testing.T, db *sql.DB) []reviewPair {
	t.Helper()
	rows, err := db.Query(`SELECT entity_type, id_lo, id_hi, variation
		FROM identity_review_queue ORDER BY entity_type, id_lo, id_hi`)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	defer rows.Close()
	var out []reviewPair
	for rows.Next() {
		var p reviewPair
		if err := rows.Scan(&p.entityType, &p.idLo, &p.idHi, &p.variation); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, p)
	}
	return out
}

func hasPair(pairs []reviewPair, want reviewPair) bool {
	for _, p := range pairs {
		if p == want {
			return true
		}
	}
	return false
}

// TestIdentityBackfill proves the queue-seed half of the one-time backfill (F43/RD10,
// P0-7): fuzzy name near-misses across all three entity types are recorded as review
// pairs — with the right variation — while singletons are left alone, keep-separate
// pairs are excluded (RD5), and nothing is merged (the seed only writes the queue).
// The fold half (the pure-case hard pairs) is proven separately in the migration test.
func TestIdentityBackfill(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()

	// Fuzzy near-misses the fold can't touch (distinct nameKeys), one per entity type,
	// plus a lone entity that must NOT be queued.
	mustExec(t, db, `INSERT INTO people (id, name) VALUES
		(1,'Mary Jane'), (2,'MaryJane'),   -- internal-whitespace
		(3,'Beyonce')`) // singleton — no pair
	mustExec(t, db, `INSERT INTO studios (id, name) VALUES
		(1,'Warner Bros.'), (2,'Warner Bros')`) // punctuation (a trailing dot)
	mustExec(t, db, `INSERT INTO tags (id, name) VALUES
		(1,'sci-fi'), (2,'scifi'),   -- punctuation (a hyphen)
		(5,'co-op'), (6,'coop')`) // a near-miss the owner keeps separate
	// The owner already asserted tags 5 & 6 are distinct — the seed must skip them.
	mustExec(t, db, `INSERT INTO entity_keep_separate (entity_type, id_lo, id_hi) VALUES ('tag',5,6)`)

	peopleBefore, studiosBefore, tagsBefore := rowCount(t, db, "people"), rowCount(t, db, "studios"), rowCount(t, db, "tags")

	queued, err := r.SeedIdentityReviewQueue(ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if queued != 3 {
		t.Fatalf("queued = %d, want 3 (one pair per type; the kept-separate pair excluded)", queued)
	}

	pairs := readReviewQueue(t, db)
	for _, want := range []reviewPair{
		{"person", 1, 2, "internal-whitespace"},
		{"studio", 1, 2, "punctuation"},
		{"tag", 1, 2, "punctuation"},
	} {
		if !hasPair(pairs, want) {
			t.Fatalf("missing expected queue pair %+v; got %+v", want, pairs)
		}
	}
	if hasPair(pairs, reviewPair{"tag", 5, 6, "punctuation"}) {
		t.Fatalf("kept-separate pair (tag 5,6) was queued")
	}
	if len(pairs) != 3 {
		t.Fatalf("queue has %d rows, want 3: %+v", len(pairs), pairs)
	}

	// The seed writes only the queue — it never merges an entity.
	if p, s, tg := rowCount(t, db, "people"), rowCount(t, db, "studios"), rowCount(t, db, "tags"); p != peopleBefore || s != studiosBefore || tg != tagsBefore {
		t.Fatalf("seed changed entity counts (people %d→%d, studios %d→%d, tags %d→%d): it must never merge",
			peopleBefore, p, studiosBefore, s, tagsBefore, tg)
	}
}

// TestIdentityBackfillIdempotent proves the seed is safe to run every boot: a second
// pass over an unchanged library inserts nothing (INSERT OR IGNORE on the pair PK).
func TestIdentityBackfillIdempotent(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()

	mustExec(t, db, `INSERT INTO tags (id, name) VALUES (1,'sci-fi'), (2,'scifi')`)

	first, err := r.SeedIdentityReviewQueue(ctx)
	if err != nil {
		t.Fatalf("seed 1: %v", err)
	}
	if first != 1 {
		t.Fatalf("first seed queued %d, want 1", first)
	}
	second, err := r.SeedIdentityReviewQueue(ctx)
	if err != nil {
		t.Fatalf("seed 2: %v", err)
	}
	if second != 0 {
		t.Fatalf("second seed queued %d, want 0 (idempotent)", second)
	}
	if n := len(readReviewQueue(t, db)); n != 1 {
		t.Fatalf("queue grew to %d rows across two passes, want 1", n)
	}
}

// TestIdentityBackfillMatchesAliases proves detection spans canonical ∪ aliases (the
// probe's universe): an alias of one entity that loosely matches another entity's
// canonical name surfaces the two entities as a review pair.
func TestIdentityBackfillMatchesAliases(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()

	// Person 10 is canonically "Sean" but also known as "Diddy"; person 11 is "Did-dy".
	// The alias "Diddy" (loose key "diddy") near-matches canonical "Did-dy" (also "diddy").
	mustExec(t, db, `INSERT INTO people (id, name) VALUES (10,'Sean'), (11,'Did-dy')`)
	mustExec(t, db, `INSERT INTO entity_aliases (entity_type, entity_id, alias) VALUES ('person', 10, 'Diddy')`)

	if _, err := r.SeedIdentityReviewQueue(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	pairs := readReviewQueue(t, db)
	if !hasPair(pairs, reviewPair{"person", 10, 11, "punctuation"}) {
		t.Fatalf("alias near-miss not detected; got %+v", pairs)
	}
}

// rowCount returns COUNT(*) for a trusted table literal.
func rowCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// mustExec runs a statement, failing the test on error.
func mustExec(t *testing.T, db *sql.DB, q string) {
	t.Helper()
	if _, err := db.Exec(q); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}
