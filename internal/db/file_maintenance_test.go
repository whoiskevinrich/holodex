package db_test

import "testing"

// Migration 0063 (F77, HOLODEX-556) adds writeback_queue.kind and held_originals. Both
// directions have to hold, and existing queue rows must land in the write lane.
func TestMigration0063FileMaintenanceUpAndDown(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(62); err != nil {
		t.Fatalf("migrate to 62: %v", err)
	}
	mustExec(t, db, `INSERT INTO videos (id, file_path, file_size, title, duration_sec, width, height, indexed_at, file_mtime, active) VALUES (1, '/m/a.mkv', 1, 'A', 1, 1, 1, '2026-10-10T00:00:00Z', '2026-10-10T00:00:00Z', 1)`)
	mustExec(t, db, `INSERT INTO writeback_queue (video_id, payload, status, enqueued_at, updated_at) VALUES (1, '[]', 'pending', '2026-10-10T00:00:00Z', '2026-10-10T00:00:00Z')`)

	if err := m.Migrate(63); err != nil {
		t.Fatalf("migrate to 63: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM writeback_queue WHERE kind = 'write'`); n != 1 {
		t.Fatalf("pre-existing queue row has kind != 'write' (%d write rows)", n)
	}
	mustExec(t, db, `INSERT INTO writeback_queue (video_id, kind, payload, status, enqueued_at, updated_at) VALUES (1, 'maintenance', '{}', 'pending', '2026-10-10T00:00:00Z', '2026-10-10T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO held_originals (video_id, held_path, op, state, created_at, expires_at) VALUES (1, '/m/a.mkv.holodex-orig', 'hflip', 'held', '2026-10-10T00:00:00Z', '2026-10-17T00:00:00Z')`)
	if _, err := db.Exec(`INSERT INTO held_originals (video_id, held_path, op, state, created_at, expires_at) VALUES (1, 'x', 'hflip', 'bogus', 'a', 'b')`); err == nil {
		t.Error("held_originals accepted an unknown state")
	}

	// The held row goes with its video, like every other video_* table.
	mustExec(t, db, `DELETE FROM videos WHERE id = 1`)
	if n := count(t, db, `SELECT COUNT(*) FROM held_originals`); n != 0 {
		t.Errorf("held_originals row survived its video's delete (%d rows)", n)
	}

	mustExec(t, db, `INSERT INTO videos (id, file_path, file_size, title, duration_sec, width, height, indexed_at, file_mtime, active) VALUES (2, '/m/b.mkv', 1, 'B', 1, 1, 1, '2026-10-10T00:00:00Z', '2026-10-10T00:00:00Z', 1)`)
	mustExec(t, db, `INSERT INTO writeback_queue (video_id, payload, status, enqueued_at, updated_at) VALUES (2, '[]', 'pending', '2026-10-10T00:00:00Z', '2026-10-10T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO writeback_queue (video_id, kind, payload, status, enqueued_at, updated_at) VALUES (2, 'maintenance', '{}', 'pending', '2026-10-10T00:00:00Z', '2026-10-10T00:00:00Z')`)

	// Down: maintenance rows are dropped (a pre-F77 worker would hand them to the tag
	// writer), write rows survive, the column and the table are gone.
	if err := m.Migrate(62); err != nil {
		t.Fatalf("migrate down to 62: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM writeback_queue`); n != 1 {
		t.Errorf("after down, %d queue rows; want only the write row", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM pragma_table_info('writeback_queue') WHERE name = 'kind'`); n != 0 {
		t.Error("writeback_queue.kind survived the down migration")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'held_originals'`); n != 0 {
		t.Error("held_originals survived the down migration")
	}
}
