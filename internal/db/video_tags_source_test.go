package db_test

import "testing"

// Migration 0030 (ADR-075 D3, HOLODEX-225) is append-only with a manual down, so
// both directions have to hold: the up must add provenance to a video_tags table
// that already has rows, and the down must leave the table usable rather than
// half-dropped.
func TestMigration0030VideoTagsSourceUpAndDown(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(29); err != nil { // one before the source column
		t.Fatalf("migrate to 29: %v", err)
	}
	// A pre-existing video/tag pair, so the up-migration is exercised against
	// real data rather than an empty table.
	mustExec(t, db, `INSERT INTO videos (file_path, file_size, title, duration_sec,
	                                     width, height, indexed_at, file_mtime, active)
	                 VALUES ('/m/a.mkv', 1000, 'A', 60, 1920, 1080,
	                         '2026-07-01T00:00:00Z', '2026-07-01T00:00:00Z', 1)`)
	mustExec(t, db, `INSERT INTO tags (name) VALUES ('documentary')`)
	mustExec(t, db, `INSERT INTO video_tags (video_id, tag_id) VALUES (1, 1)`)

	if err := m.Migrate(30); err != nil {
		t.Fatalf("migrate to 30: %v", err)
	}
	// The old row survives and backfills as file-derived — no ambiguity, since
	// every row that could exist pre-migration came only from the scanner.
	if n := count(t, db, `SELECT COUNT(*) FROM video_tags WHERE source = 'file'`); n != 1 {
		t.Fatalf("pre-migration rows defaulting to source='file' = %d, want 1", n)
	}
	// A newly-sourced row round-trips.
	mustExec(t, db, `INSERT INTO tags (name) VALUES ('curated')`)
	mustExec(t, db, `INSERT INTO video_tags (video_id, tag_id, source) VALUES (1, 2, 'manual')`)
	if n := count(t, db, `SELECT COUNT(*) FROM video_tags WHERE source = 'manual'`); n != 1 {
		t.Fatal("manually-sourced row did not round-trip through the new column")
	}

	// Down drops the column, leaving video_tags intact.
	if err := m.Migrate(29); err != nil {
		t.Fatalf("migrate down to 29: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM pragma_table_info('video_tags')
	                      WHERE name = 'source'`); n != 0 {
		t.Error("source column survived the down migration")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM video_tags`); n != 2 {
		t.Errorf("rows after down = %d, want 2 — the down must drop the column, not data", n)
	}
}
