package db_test

import "testing"

// TestMigration0046FoldsExternalIDsUpAndDown exercises ADR-096 D2 (F60 / HOLODEX-375):
// the per-kind person_external_ids / studio_external_ids rows fold into the one
// polymorphic entity_external_ids table tagged by kind, the per-kind tables are gone,
// the per-kind AFTER DELETE trigger does the cleanup the old FK cascade did, and the
// down migration restores both tables with their rows.
func TestMigration0046FoldsExternalIDsUpAndDown(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(45); err != nil {
		t.Fatalf("migrate to 45: %v", err)
	}
	mustExec(t, db, `INSERT INTO people (id, name) VALUES (1,'Denis Villeneuve'),(2,'Brad Pitt')`)
	mustExec(t, db, `INSERT INTO studios (id, name) VALUES (1,'Warner Bros.')`)
	mustExec(t, db, `INSERT INTO person_external_ids (person_id, external_id) VALUES (1,'tmdb:137'),(1,'imdb:nm0898288'),(2,'tmdb:287')`)
	// The same id string under both kinds must survive the fold: uniqueness is per kind.
	mustExec(t, db, `INSERT INTO studio_external_ids (studio_id, external_id) VALUES (1,'tmdb:137')`)

	if err := m.Migrate(46); err != nil {
		t.Fatalf("migrate to 46: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM entity_external_ids`); n != 4 {
		t.Fatalf("entity_external_ids rows = %d, want 4", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM entity_external_ids WHERE entity_type='person' AND entity_id=1`); n != 2 {
		t.Fatalf("person 1 ids = %d, want 2", n)
	}
	if n := count(t, db, `SELECT entity_id FROM entity_external_ids WHERE entity_type='studio' AND external_id='tmdb:137'`); n != 1 {
		t.Fatalf("studio tmdb:137 → entity %d, want 1", n)
	}
	for _, old := range []string{"person_external_ids", "studio_external_ids"} {
		if n := count(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='`+old+`'`); n != 0 {
			t.Fatalf("%s still exists after fold", old)
		}
	}

	// Cleanup trigger replaces the FK cascade: deleting person 1 drops only its rows.
	mustExec(t, db, `DELETE FROM people WHERE id = 1`)
	if n := count(t, db, `SELECT COUNT(*) FROM entity_external_ids WHERE entity_type='person'`); n != 1 {
		t.Fatalf("person ids after delete = %d, want 1 (Brad Pitt's)", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM entity_external_ids WHERE entity_type='studio'`); n != 1 {
		t.Fatalf("studio ids after person delete = %d, want 1 (untouched)", n)
	}

	// Down: both per-kind tables come back carrying their rows; film/tag rows (no
	// pre-0046 home) are dropped with the table.
	mustExec(t, db, `INSERT INTO entity_external_ids (entity_type, entity_id, external_id) VALUES ('film', 7, 'tmdb:603')`)
	if err := m.Migrate(45); err != nil {
		t.Fatalf("migrate down to 45: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM person_external_ids WHERE person_id = 2 AND external_id = 'tmdb:287'`); n != 1 {
		t.Fatalf("person_external_ids after down = %d, want 1", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM studio_external_ids WHERE studio_id = 1 AND external_id = 'tmdb:137'`); n != 1 {
		t.Fatalf("studio_external_ids after down = %d, want 1", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='entity_external_ids'`); n != 0 {
		t.Fatalf("entity_external_ids still exists after down")
	}
}
