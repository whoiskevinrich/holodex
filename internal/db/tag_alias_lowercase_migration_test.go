package db_test

import "testing"

// TestMigration0056LowercaseTagAliases (HOLODEX-507, F43 RD12): tag alias text is
// lowercased, other kinds keep their casing, and the shared FTS mirror follows the
// rewrite so a tag alias is still found by search.
func TestMigration0056LowercaseTagAliases(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(55); err != nil {
		t.Fatalf("migrate to 55: %v", err)
	}
	mustExec(t, db, `INSERT INTO tags (id, name) VALUES (1, 'sci-fi')`)
	mustExec(t, db, `INSERT INTO studios (id, name) VALUES (1, 'Warner Bros.')`)
	mustExec(t, db, `INSERT INTO entity_aliases (entity_type, entity_id, alias) VALUES
		('tag', 1, 'Science Fiction'), ('tag', 1, 'scifi'), ('studio', 1, 'WB')`)

	if err := m.Migrate(56); err != nil {
		t.Fatalf("migrate to 56: %v", err)
	}
	if c := count(t, db, `SELECT COUNT(*) FROM entity_aliases WHERE entity_type = 'tag' AND alias = 'science fiction'`); c != 1 {
		t.Errorf("tag alias not lowercased (%d rows)", c)
	}
	if c := count(t, db, `SELECT COUNT(*) FROM entity_aliases WHERE entity_type = 'studio' AND alias = 'WB'`); c != 1 {
		t.Errorf("studio alias casing changed (%d rows)", c)
	}
	if c := count(t, db, `SELECT COUNT(*) FROM entity_aliases_fts WHERE entity_aliases_fts MATCH 'science'`); c != 1 {
		t.Errorf("FTS lost the rewritten tag alias (%d hits)", c)
	}
	if err := m.Migrate(55); err != nil {
		t.Fatalf("down to 55: %v", err)
	}
}
