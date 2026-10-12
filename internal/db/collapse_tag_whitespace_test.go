package db_test

import "testing"

// TestMigration0061CollapseTagWhitespace covers HOLODEX-557 (F43 RD12): a run of spaces
// in a tag name or tag alias collapses to one; other entities' aliases are untouched.
func TestMigration0061CollapseTagWhitespace(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(60); err != nil {
		t.Fatalf("migrate to 60: %v", err)
	}
	mustExec(t, db, `INSERT INTO tags (id, name) VALUES (1, 'sci   fi'), (2, 'film noir'), (3, 'a  b  c')`)
	mustExec(t, db, `INSERT INTO entity_aliases (entity_type, entity_id, alias) VALUES
		('tag', 2, 'noir  film'), ('person', 7, 'John  Doe')`)

	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	for id, want := range map[int]string{1: "sci fi", 2: "film noir", 3: "a b c"} {
		var got string
		if err := db.QueryRow(`SELECT name FROM tags WHERE id = ?`, id).Scan(&got); err != nil || got != want {
			t.Errorf("tag %d name = %q (err %v), want %q", id, got, err, want)
		}
	}
	for typ, want := range map[string]string{"tag": "noir film", "person": "John  Doe"} {
		var got string
		if err := db.QueryRow(`SELECT alias FROM entity_aliases WHERE entity_type = ?`, typ).Scan(&got); err != nil || got != want {
			t.Errorf("%s alias = %q (err %v), want %q", typ, got, err, want)
		}
	}
}
