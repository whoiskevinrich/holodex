package db_test

import "testing"

// Migration 0031 (ADR-075 D2, HOLODEX-226) is append-only with a manual down,
// so both directions have to hold.
func TestMigration0031DeniedTagsUpAndDown(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(30); err != nil { // one before denied_tags
		t.Fatalf("migrate to 30: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'denied_tags'`); n != 0 {
		t.Fatal("denied_tags exists before its migration")
	}

	if err := m.Migrate(31); err != nil {
		t.Fatalf("migrate to 31: %v", err)
	}
	mustExec(t, db, `INSERT INTO denied_tags (term_key, term, created_at) VALUES ('gnome', 'Gnome', '2026-07-30T00:00:00Z')`)
	if n := count(t, db, `SELECT COUNT(*) FROM denied_tags WHERE term_key = 'gnome' AND term = 'Gnome'`); n != 1 {
		t.Fatal("denied term did not round-trip through the new table")
	}

	// Down drops the table entirely (unlike an ADD COLUMN migration, there's no
	// data to preserve across it -- the table itself is new).
	if err := m.Migrate(30); err != nil {
		t.Fatalf("migrate down to 30: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'denied_tags'`); n != 0 {
		t.Error("denied_tags survived the down migration")
	}
}
