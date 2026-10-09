package db_test

import (
	"fmt"
	"testing"
)

// TestMigration0060VideoEntityRows covers HOLODEX-547: a video's per-entity rows
// (no FK to videos) are swept once when orphaned, then go with every later delete.
func TestMigration0060VideoEntityRows(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(59); err != nil { // one before the trigger and sweep
		t.Fatalf("migrate to 59: %v", err)
	}

	// Video 1 is live; video 9 was purged before 0060 and left its rows behind.
	mustExec(t, db, `INSERT INTO videos (id, file_path, indexed_at, file_mtime) VALUES
		(1,'/m/a.mkv','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)
	tables := map[string]string{
		"entity_enrichment":      `INSERT INTO entity_enrichment (entity_type, entity_id, provider, field_key, value, fetched_at) VALUES ('%s', %d, 'tmdb', 'title', 'x', 't')`,
		"field_source_decisions": `INSERT INTO field_source_decisions (entity_type, entity_id, field_key, source, created_at) VALUES ('%s', %d, 'title', 'manual', 't')`,
		"metadata_curation":      `INSERT INTO metadata_curation (entity_type, entity_id, field_key, norm_value, action, created_at) VALUES ('%s', %d, 'tags', 'x', 'hide', 't')`,
		"facet_not_applicable":   `INSERT INTO facet_not_applicable (entity_type, entity_id, canonical_field, created_at) VALUES ('%s', %d, 'studio', 't')`,
	}
	for _, ins := range tables {
		mustExec(t, db, fmt.Sprintf(ins, "video", 1))
		mustExec(t, db, fmt.Sprintf(ins, "video", 9))
		mustExec(t, db, fmt.Sprintf(ins, "person", 9)) // same id, other kind: never touched
	}
	rows := func(table, entityType string, id int) int {
		return count(t, db, fmt.Sprintf(`SELECT count(*) FROM %s WHERE entity_type = '%s' AND entity_id = %d`, table, entityType, id))
	}

	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	for table := range tables {
		if n := rows(table, "video", 9); n != 0 {
			t.Errorf("%s: orphan of purged video survived the sweep (%d rows)", table, n)
		}
		if n := rows(table, "video", 1); n != 1 {
			t.Errorf("%s: live video's row = %d, want 1", table, n)
		}
	}

	mustExec(t, db, `DELETE FROM videos WHERE id = 1`)
	for table := range tables {
		if n := rows(table, "video", 1); n != 0 {
			t.Errorf("%s: deleted video's row survived (%d rows)", table, n)
		}
		if n := rows(table, "person", 9); n != 1 {
			t.Errorf("%s: person row = %d, want 1", table, n)
		}
	}
}
