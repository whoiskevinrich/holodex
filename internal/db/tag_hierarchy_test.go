package db_test

import "testing"

// Migration 0032 (ADR-075 D1, HOLODEX-227) is append-only with a manual down,
// so both directions have to hold.
func TestMigration0032TagHierarchyUpAndDown(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(31); err != nil { // one before parent_tag_id
		t.Fatalf("migrate to 31: %v", err)
	}
	mustExec(t, db, `INSERT INTO tags (name) VALUES ('animal')`)
	mustExec(t, db, `INSERT INTO tags (name) VALUES ('dog')`)

	if err := m.Migrate(32); err != nil {
		t.Fatalf("migrate to 32: %v", err)
	}
	// A pre-existing row backfills parent_tag_id NULL — no ambiguity, since no
	// tag could have a parent before this column existed.
	if n := count(t, db, `SELECT COUNT(*) FROM tags WHERE parent_tag_id IS NULL`); n != 2 {
		t.Fatalf("pre-migration rows with NULL parent_tag_id = %d, want 2", n)
	}
	mustExec(t, db, `UPDATE tags SET parent_tag_id = (SELECT id FROM tags WHERE name = 'animal') WHERE name = 'dog'`)
	if n := count(t, db, `SELECT COUNT(*) FROM tags WHERE parent_tag_id IS NOT NULL`); n != 1 {
		t.Fatal("parent_tag_id did not round-trip through the new column")
	}

	// Down drops the column, leaving tags intact.
	if err := m.Migrate(31); err != nil {
		t.Fatalf("migrate down to 31: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM pragma_table_info('tags')
	                      WHERE name = 'parent_tag_id'`); n != 0 {
		t.Error("parent_tag_id column survived the down migration")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM tags`); n != 2 {
		t.Errorf("rows after down = %d, want 2 — the down must drop the column, not data", n)
	}
}
