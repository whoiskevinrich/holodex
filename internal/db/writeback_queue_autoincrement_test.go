package db_test

import "testing"

// TestMigration0057WritebackQueueAutoincrement covers HOLODEX-510: a job with no
// caller-supplied batch id snapshots under its own id, so an id SQLite hands out
// twice (plain INTEGER PRIMARY KEY reuses the max rowid once the row is deleted on
// success) made a later write find the earlier job's snapshot and skip its own.
// After the up: live rows survive, a deleted job's id is never reissued, and the
// sequence starts above every numeric batch id already recorded in snapshots or
// job_runs — not just above the queue's surviving rows.
func TestMigration0057WritebackQueueAutoincrement(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(55); err != nil {
		t.Fatalf("migrate to 55: %v", err)
	}
	mustExec(t, db, `INSERT INTO videos (id, file_path, file_size, title, file_mtime, indexed_at) VALUES
		(1, '/m/a.mkv', 1, 'A', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO writeback_queue (id, video_id, payload, status, enqueued_at, updated_at, batch_id) VALUES
		(3, 1, '[]', 'failed', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '')`)
	// Batch "40" came from a long-deleted job; "merge-…" and "tag-…" are caller ids
	// that must not be mistaken for numbers.
	mustExec(t, db, `INSERT INTO file_writeback_snapshots (video_id, batch_id, field_key, prior_value, written_at) VALUES
		(1, '40', 'title', 'Old', '2026-01-01T00:00:00Z'),
		(1, 'merge-person-1-2', 'title', 'Old', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO job_runs (kind, trigger, status, started_at, finished_at, batch_id) VALUES
		('writeback', 'manual', 'ok', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '57'),
		('writeback', 'manual', 'ok', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', 'tag-writeback-sync-999999')`)

	if err := m.Migrate(57); err != nil {
		t.Fatalf("migrate to 57: %v", err)
	}
	if c := count(t, db, `SELECT COUNT(*) FROM writeback_queue WHERE id = 3 AND status = 'failed'`); c != 1 {
		t.Errorf("live queue row lost in the rebuild")
	}

	enqueue := func() int64 {
		t.Helper()
		res, err := db.Exec(`INSERT INTO writeback_queue (video_id, payload, enqueued_at, updated_at) VALUES
			(1, '[]', '2026-01-02T00:00:00Z', '2026-01-02T00:00:00Z')`)
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	first := enqueue()
	if first <= 57 {
		t.Errorf("first id after the migration = %d, want > 57 (the highest numeric batch id in use)", first)
	}
	if _, err := db.Exec(`DELETE FROM writeback_queue WHERE id = ?`, first); err != nil { // FinishWriteback on success
		t.Fatalf("delete: %v", err)
	}
	if second := enqueue(); second == first {
		t.Errorf("deleted job's id %d was reissued", first)
	}

	if err := m.Migrate(55); err != nil {
		t.Fatalf("migrate down to 55: %v", err)
	}
	if c := count(t, db, `SELECT COUNT(*) FROM writeback_queue WHERE id = 3`); c != 1 {
		t.Errorf("down migration lost a live queue row")
	}
	if err := m.Migrate(57); err != nil {
		t.Fatalf("migrate back up to 57: %v", err)
	}
}
