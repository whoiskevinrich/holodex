package db_test

import (
	"database/sql"
	"testing"
)

// TestMigration0055SmartPlaylists covers testing-strategy §23.2's migration row (F75,
// ADR-121 D1): up then down then up on a DB holding F69 playlists. Existing rows read
// query NULL, query_version NULL and play_shuffled 0 after each up, and their names,
// sorts and memberships come through the round trip untouched.
func TestMigration0055SmartPlaylists(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(54); err != nil { // one before smart playlists
		t.Fatalf("migrate to 54: %v", err)
	}
	mustExec(t, db, `INSERT INTO videos (id, file_path, file_size, title, file_mtime, indexed_at) VALUES
		(1, '/m/a.mkv', 1, 'A', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'), (2, '/m/b.mkv', 1, 'B', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO playlists (id, name, sort, visibility) VALUES
		(1, 'Mix', 'manual', 'public'), (2, 'Recent', 'added_desc', 'private')`)
	mustExec(t, db, `INSERT INTO playlist_videos (playlist_id, video_id, position) VALUES (1, 2, 1), (1, 1, 2)`)

	assertF69 := func(step string) {
		t.Helper()
		rows, err := db.Query(`SELECT id, query, query_version, play_shuffled FROM playlists ORDER BY id`)
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			var id int64
			var query sql.NullString
			var version sql.NullInt64
			var shuffled int
			if err := rows.Scan(&id, &query, &version, &shuffled); err != nil {
				t.Fatal(err)
			}
			if query.Valid || version.Valid || shuffled != 0 {
				t.Errorf("%s: playlist %d query %v version %v play_shuffled %d, want NULL NULL 0", step, id, query, version, shuffled)
			}
			n++
		}
		if n != 2 {
			t.Errorf("%s: %d playlists, want 2", step, n)
		}
		if c := count(t, db, `SELECT COUNT(*) FROM playlist_videos WHERE playlist_id = 1`); c != 2 {
			t.Errorf("%s: membership %d rows, want 2", step, c)
		}
		if c := count(t, db, `SELECT COUNT(*) FROM playlists WHERE (id = 1 AND name = 'Mix' AND sort = 'manual' AND visibility = 'public')
			OR (id = 2 AND name = 'Recent' AND sort = 'added_desc' AND visibility = 'private')`); c != 2 {
			t.Errorf("%s: F69 columns changed", step)
		}
	}

	if err := m.Migrate(55); err != nil {
		t.Fatalf("migrate to 55: %v", err)
	}
	assertF69("first up")
	mustExec(t, db, `INSERT INTO playlists (name, query, query_version, play_shuffled) VALUES ('Smart', 'tag=3', 1, 1)`)

	if err := m.Migrate(54); err != nil {
		t.Fatalf("migrate down to 54: %v", err)
	}
	for _, col := range []string{"query", "query_version", "play_shuffled"} {
		if _, err := db.Exec(`SELECT ` + col + ` FROM playlists`); err == nil {
			t.Errorf("playlists.%s survived the down migration", col)
		}
	}
	// The down migration's documented loss: a smart playlist reads as an empty snapshot.
	mustExec(t, db, `DELETE FROM playlists WHERE name = 'Smart'`)

	if err := m.Migrate(55); err != nil {
		t.Fatalf("migrate up to 55 again: %v", err)
	}
	assertF69("second up")
}
