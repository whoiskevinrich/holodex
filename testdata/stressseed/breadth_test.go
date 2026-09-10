package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

// breadthTestCount is deliberately tiny. Every claim these tests make is about
// shape — how many, where, and whether the ladder moved — and none of them get
// truer at 2000, which would only add minutes of image encoding.
const breadthTestCount = 5

// countBulkRows is the breadth half's basic question, asked of the database rather
// than of the seeder's own bookkeeping: a pool that recorded five entities it
// never wrote would satisfy every claim made against the returned struct.
// A video's display string is `title`; every other seeded table calls it `name`.
var bulkNameColumn = map[string]string{"videos": "title"}

func countBulkRows(t *testing.T, database *sql.DB, table string) int {
	t.Helper()
	column, ok := bulkNameColumn[table]
	if !ok {
		column = "name"
	}
	var n int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM `+table+` WHERE `+column+` LIKE ?`, bulkPrefix+"%").Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// TestBreadthPopulatesEveryKind is the ticket's first acceptance criterion:
// -count of every entity type the app has a list page for, categories included.
//
// Counted through the database by name prefix rather than by trusting the
// manifest, because the two can disagree — and if they do, the manifest is the
// half that lies.
func TestBreadthPopulatesEveryKind(t *testing.T) {
	_, pool, database := seedCountInto(t, t.TempDir(), breadthTestCount)

	for _, table := range []string{"videos", "people", "studios", "tags", "films", categoriesTable} {
		t.Run(table, func(t *testing.T) {
			if got := countBulkRows(t, database, table); got != breadthTestCount {
				t.Errorf("%s: %d bulk rows in the database, want %d", table, got, breadthTestCount)
			}
			r, ok := pool.Tables[table]
			if !ok {
				t.Fatalf("%s: the manifest records no breadth range at all", table)
			}
			if r.N != breadthTestCount {
				t.Errorf("%s: manifest records %d, want %d", table, r.N, breadthTestCount)
			}
		})
	}
}

// TestBreadthStaysOutOfAddressedBlocks is the guarantee that makes it safe to add
// a breadth pool to a fixture whose addresses assertions are already written
// against: every bulk row is in the pool gap, and nothing is in a reserved block.
//
// Checked against the id ranges rather than a spot value, because the failure it
// guards is a steer that silently did nothing — which scatters bulk rows through
// a block rather than putting the whole run in the wrong place.
func TestBreadthStaysOutOfAddressedBlocks(t *testing.T) {
	_, pool, _ := seedCountInto(t, t.TempDir(), breadthTestCount)

	for table, r := range pool.Tables {
		if r.Lo < poolBase {
			t.Errorf("%s: bulk ids start at %d, inside the addressed range below %d",
				table, r.Lo, poolBase)
		}
		if r.Hi >= derivedBase {
			t.Errorf("%s: bulk ids reach %d, inside the derived blocks at %d",
				table, r.Hi, derivedBase)
		}
	}
}

// TestBreadthDoesNotMoveAddressedEntities is the one that would matter most if it
// broke, and the reason the bulk videos are seeded at the scene pool's moment
// rather than wherever was convenient.
//
// D4 promises an address never moves. A breadth pool is added to a fixture whose
// manifest is already in use, so if seeding one shifted a single addressed id,
// every assertion and every note written against the old fixture would silently
// point at a different entity. Compared entry by entry against a -count 0 run.
func TestBreadthDoesNotMoveAddressedEntities(t *testing.T) {
	bare, _ := seedInto(t, t.TempDir())
	withBulk, _, _ := seedCountInto(t, t.TempDir(), breadthTestCount)

	if len(bare) != len(withBulk) {
		t.Fatalf("seeding a breadth pool changed the addressed entity count: %d without, %d with",
			len(bare), len(withBulk))
	}
	for i := range bare {
		a, b := bare[i], withBulk[i]
		if a.ID != b.ID || a.Dimension != b.Dimension || a.Variant != b.Variant || a.Name != b.Name {
			t.Errorf("addressed entity %d moved: %s=%s at id %d became %s=%s at id %d",
				i, a.Dimension, a.Variant, a.ID, b.Dimension, b.Variant, b.ID)
		}
	}
}

// TestBreadthIsReproducible covers the spec's first acceptance criterion for the
// pool half: the same -count lands the same rows at the same addresses.
func TestBreadthIsReproducible(t *testing.T) {
	_, first, _ := seedCountInto(t, t.TempDir(), breadthTestCount)
	_, second, _ := seedCountInto(t, t.TempDir(), breadthTestCount)

	for table, a := range first.Tables {
		b, ok := second.Tables[table]
		if !ok {
			t.Errorf("%s: present in the first run, absent in the second", table)
			continue
		}
		if a != b {
			t.Errorf("%s: first run landed %+v, second landed %+v", table, a, b)
		}
	}
}

// TestReseedClearsCategories is the regression test for the one table the breadth
// pool added to the fixture's ownership.
//
// Categories are not derived and nothing cascades to them from the entity tables,
// so leaving them out of seededTables would not corrupt anything — the second run
// would simply fail on ErrNameTaken partway through, after having already cleared
// every other table. Re-seeding into the same directory is the only thing that
// catches it, which is why this test does that rather than asserting on the list.
func TestReseedClearsCategories(t *testing.T) {
	dir := t.TempDir()
	_, _, database := seedCountInto(t, dir, breadthTestCount)
	if err := database.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	_, pool, again := seedCountInto(t, dir, breadthTestCount)
	if got := countBulkRows(t, again, categoriesTable); got != breadthTestCount {
		t.Errorf("after a reseed: %d bulk categories, want %d — the first run's rows were "+
			"not cleared", got, breadthTestCount)
	}
	if r := pool.Tables[categoriesTable]; r.Lo < poolBase {
		t.Errorf("after a reseed: bulk categories start at %d, below the pool base %d", r.Lo, poolBase)
	}
}

// TestBreadthRefusesAnOversizedCount checks the up-front ceiling rather than
// leaving the run to fail on steer() after writing eleven thousand rows.
//
// The message has to state the limit: "too many" without a number leaves the
// operator bisecting -count by hand.
func TestBreadthRefusesAnOversizedCount(t *testing.T) {
	over := breadthCeiling() + 1
	_, err := seedBreadthVideos(context.Background(), nil, fixtureFields{}, imageWriter{}, over, newBreadthPool(over))
	if err == nil {
		t.Fatalf("-count %d was accepted; it does not fit the pool range", over)
	}
	if !strings.Contains(err.Error(), fmt.Sprint(breadthCeiling())) {
		t.Errorf("the refusal does not state the limit %d, so it cannot be acted on: %v",
			breadthCeiling(), err)
	}
}

// TestClaimCoversEverySeededTable ties the two halves of the safety guard
// together: reset() deletes seededTables, and inspect() refuses a database that
// has rows in contentTables. Anything in the first list but not the second is a
// table the fixture destroys without ever having looked at it.
//
// This is the invariant HOLODEX-350 broke by adding categories to seededTables
// alone. Categories are the reachable case — the owner creates them directly, so
// a library can hold them with no media at all — but the rule is general, which
// is why this asserts the relationship rather than spot-checking one table.
func TestClaimCoversEverySeededTable(t *testing.T) {
	checked := map[string]bool{}
	for _, table := range contentTables {
		checked[table] = true
	}
	for _, table := range seededTables {
		// The deliberate exemptions are argued for where they are declared, next to
		// seededTables — read from there rather than restated here, so a new one
		// cannot be waved through by editing this test.
		if _, exempt := notContentTables[table]; exempt {
			continue
		}
		if !checked[table] {
			t.Errorf("reset() deletes %q but inspect() never counts it, so a database "+
				"holding only %s rows would be claimed and wiped", table, table)
		}
	}
}

// TestAssertPooledBoundsBothEnds exercises assertPooled directly, because the
// upper bound is unreachable through a seed: the up-front ceiling refuses any
// -count that could produce it, so the only run that would trip it is one that
// writes eleven thousand rows first.
//
// It is kept rather than deleted as unreachable because the failure it names is
// one steer() would misdiagnose. An overrun does reach steer — the derived
// dimensions steer people to derivedBase and it refuses when a row already sits
// there — but its message sends the reader to the dimension ordering in
// ladder.go, which is not the cause. This one says -count is too large, which is.
func TestAssertPooledBoundsBothEnds(t *testing.T) {
	cases := []struct {
		name string
		kind entityKind
		id   int64
		want bool
	}{
		{"a bulk video in the addressed range", kindVideo, poolBase - 1, true},
		{"a bulk video in the pool", kindVideo, poolBase, false},
		{"a bulk person in a derived block", kindPerson, derivedBase, true},
		{"a bulk person just below one", kindPerson, derivedBase - 1, false},
		// Films and videos are numbered below the pool, so the derived ceiling does
		// not apply to them: a film above derivedBase is unusual but not wrong.
		{"a bulk film above the derived base", kindFilm, derivedBase + 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := assertPooled(tc.kind, tc.id)
			if got := err != nil; got != tc.want {
				t.Errorf("assertPooled(%s, %d) error = %v, want refusal = %v",
					tc.kind, tc.id, err, tc.want)
			}
		})
	}
}

// TestBulkFilmsCarryAScene guards the reason bulk films attach a video at all.
//
// A film's cast, studios and tags are each derived live from its attached videos,
// so a film with no scenes renders three empty sections and a bare tile. A list
// of two thousand of those would look like the films page at scale without
// measuring it, which is the failure mode this whole ticket exists to avoid.
func TestBulkFilmsCarryAScene(t *testing.T) {
	_, _, database := seedCountInto(t, t.TempDir(), breadthTestCount)

	var orphans int
	if err := database.QueryRow(`
SELECT COUNT(*) FROM films f
WHERE f.name LIKE ?
  AND NOT EXISTS (SELECT 1 FROM film_videos fv WHERE fv.film_id = f.id)`,
		bulkPrefix+"%").Scan(&orphans); err != nil {
		t.Fatalf("count sceneless bulk films: %v", err)
	}
	if orphans != 0 {
		t.Errorf("%d bulk films have no scene attached, so their cast, studio and tag "+
			"sections render empty", orphans)
	}
}

// TestBulkCategoriesCarryATag mirrors the film check for the entity the ladder
// never addresses. An empty category renders an empty detail page, so a list of
// them would exercise the list and nothing below it.
func TestBulkCategoriesCarryATag(t *testing.T) {
	_, _, database := seedCountInto(t, t.TempDir(), breadthTestCount)

	var empties int
	if err := database.QueryRow(`
SELECT COUNT(*) FROM categories c
WHERE c.name LIKE ?
  AND NOT EXISTS (SELECT 1 FROM category_tags ct WHERE ct.category_id = c.id)`,
		bulkPrefix+"%").Scan(&empties); err != nil {
		t.Fatalf("count empty bulk categories: %v", err)
	}
	if empties != 0 {
		t.Errorf("%d bulk categories hold no tag, so their detail page is empty", empties)
	}
}

// TestZeroCountSeedsNoPool is D2 applied to the breadth axis itself: the empty
// case is a state the fixture has to be able to express, and it is the one the
// ladder tests run in.
func TestZeroCountSeedsNoPool(t *testing.T) {
	_, pool, database := seedCountInto(t, t.TempDir(), 0)

	if len(pool.Tables) != 0 {
		t.Errorf("-count 0 recorded %d breadth ranges, want none", len(pool.Tables))
	}
	for _, table := range []string{"videos", "people", "studios", "tags", "films", categoriesTable} {
		if got := countBulkRows(t, database, table); got != 0 {
			t.Errorf("-count 0 still wrote %d bulk %s", got, table)
		}
	}
}
