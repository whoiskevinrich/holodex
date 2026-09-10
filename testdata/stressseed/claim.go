package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// markerTable is how the fixture recognises its own database. It is created by
// this tool, not by a migration — nothing in the server knows or cares about it,
// which is the point: the marker exists only to answer "did I make this?".
const markerTable = "stress_fixture"

// contentTables are the tables whose rows mean the database belongs to someone
// else. Any one of them being non-empty in an unmarked database is enough to
// refuse: a real library has videos, and an enrichment-only or people-only
// database is still not ours to overwrite.
//
// This list has to cover everything reset() deletes, or the fixture destroys a
// table it never looked at. Categories are the case that makes the rule concrete:
// they are created by the owner directly rather than derived from media, so a
// library can hold categories and nothing else — and once the breadth pool made
// the fixture own that table (HOLODEX-350), an unchecked one would have been
// silently wiped. TestClaimCoversEverySeededTable keeps the two in step.
// entity_enrichment joined the list when the fixture started seeding the ADR-090
// precedence layer (HOLODEX-348). It is the case the comment above was already
// describing in the abstract: a database holding nothing but enrichment rows is
// somebody's shadow store, and once reset() deletes that table an unchecked one
// would be wiped by a seeder that never looked at it.
//
// denied_tags is the same shape as categories, one step further out: it is keyed by
// a folded *term* rather than by an entity id, so an owner can deny a term for a tag
// that was never created — a library can hold a deny list and nothing else. The
// other owner-decision tables reset() clears are all keyed to an entity this list
// already counts, so they are exempt; see notContentTables in generate.go.
var contentTables = []string{"videos", "people", "studios", "tags", "films", "categories", "entity_enrichment", "denied_tags"}

// claim is what inspect learned about a candidate database.
type claim struct {
	claimed bool     // the marker is present: this database is the fixture's
	seed    uint64   // the seed recorded in the marker; meaningful when claimed
	foreign []string // "videos: 1841" per non-empty content table, when unmarked
}

// inspect decides whether the database at dbPath is the fixture's to write.
//
// Three outcomes: no file yet (claimable), the marker is present (ours), or the
// marker is absent. In that last case an *empty* database is still claimable —
// starting the `backend-stress` server before the first seed creates one, and
// refusing that would be a trap with no upside — but any content row makes it
// somebody's real data.
func inspect(dbPath string) (claim, error) {
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		return claim{}, nil
	} else if err != nil {
		return claim{}, fmt.Errorf("stat %s: %w", dbPath, err)
	}

	// Read-only. An inspection that can write is not a guard: a database that
	// turns out to be a real library has to be left exactly as it was found,
	// including its journal files.
	database, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return claim{}, fmt.Errorf("open %s read-only: %w", dbPath, err)
	}
	defer database.Close()

	ctx := context.Background()
	tables, err := tableNames(ctx, database)
	if err != nil {
		return claim{}, fmt.Errorf("inspect %s: %w", dbPath, err)
	}

	if tables[markerTable] {
		var seed int64
		if err := database.QueryRowContext(ctx,
			`SELECT seed FROM `+markerTable+` WHERE id = 1`).Scan(&seed); err != nil {
			return claim{}, fmt.Errorf("read fixture marker in %s: %w", dbPath, err)
		}
		return claim{claimed: true, seed: uint64(seed)}, nil
	}

	var foreign []string
	for _, t := range contentTables {
		if !tables[t] {
			continue // schema predates that table, or is not Holodex's at all
		}
		var n int64
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+t).Scan(&n); err != nil {
			return claim{}, fmt.Errorf("count %s in %s: %w", t, dbPath, err)
		}
		if n > 0 {
			foreign = append(foreign, fmt.Sprintf("%s: %d", t, n))
		}
	}
	return claim{foreign: foreign}, nil
}

func tableNames(ctx context.Context, database *sql.DB) (map[string]bool, error) {
	rows, err := database.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names[name] = true
	}
	return names, rows.Err()
}

// markOwned writes (or refreshes) the marker, so every later run recognises this
// database as the fixture's rather than re-deciding from its contents. Both
// statements go in one transaction: a marker table with no row in it would fail
// every subsequent inspect, and there is no reason to leave that reachable.
func markOwned(ctx context.Context, database *sql.DB, seed uint64, count int) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin fixture marker: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op once committed

	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+markerTable+` (
    id         INTEGER PRIMARY KEY CHECK (id = 1),
    seed       INTEGER NOT NULL,
    count      INTEGER NOT NULL,
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL
)`); err != nil {
		return fmt.Errorf("create fixture marker: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, `INSERT INTO `+markerTable+`
    (id, seed, count, created_at, updated_at) VALUES (1, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET count = excluded.count, updated_at = excluded.updated_at`,
		int64(seed), count, now, now); err != nil {
		return fmt.Errorf("write fixture marker: %w", err)
	}
	return tx.Commit()
}
