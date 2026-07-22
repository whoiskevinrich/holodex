package db_test

import "testing"

// Migration 0028 (ADR-070) is append-only with a manual down, so both directions
// have to hold: the up must add attribution to a job_runs table that already has
// rows, and the down must leave the table usable rather than half-dropped.
func TestMigration0028JobRunAttributionUpAndDown(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(27); err != nil { // one before the attribution columns
		t.Fatalf("migrate to 27: %v", err)
	}
	// A pre-existing run, so the up-migration is exercised against real data
	// rather than an empty table.
	mustExec(t, db, `INSERT INTO job_runs (kind, trigger, status, started_at, finished_at)
	                 VALUES ('scan','initial','success','2026-07-01T00:00:00Z','2026-07-01T00:00:01Z')`)

	if err := m.Migrate(28); err != nil {
		t.Fatalf("migrate to 28: %v", err)
	}
	// The old row survives and reads as unattributed via the column defaults —
	// no backfill, no sentinel.
	if n := count(t, db, `SELECT COUNT(*) FROM job_runs
	                      WHERE entity_type = '' AND entity_id = 0 AND batch_id = ''`); n != 1 {
		t.Fatalf("pre-migration rows defaulting to unattributed = %d, want 1", n)
	}
	// A newly attributed run round-trips, including a non-numeric batch id (the
	// merge-propagation shape the retired detail-line regex could not match).
	mustExec(t, db, `INSERT INTO job_runs (kind, trigger, status, started_at, finished_at,
	                                       entity_type, entity_id, batch_id)
	                 VALUES ('writeback','manual','success','2026-07-02T00:00:00Z','2026-07-02T00:00:01Z',
	                         'video', 412, 'merge-person-7-9')`)
	if n := count(t, db, `SELECT COUNT(*) FROM job_runs
	                      WHERE entity_type = 'video' AND entity_id = 412
	                        AND batch_id = 'merge-person-7-9'`); n != 1 {
		t.Fatal("attributed row did not round-trip through the new columns")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sqlite_master
	                      WHERE type = 'index' AND name = 'idx_job_runs_entity'`); n != 1 {
		t.Fatal("idx_job_runs_entity missing — entity-filtered reads would fall back to a scan")
	}

	// Down drops the index and all three columns, leaving job_runs intact.
	if err := m.Migrate(27); err != nil {
		t.Fatalf("migrate down to 27: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sqlite_master
	                      WHERE type = 'index' AND name = 'idx_job_runs_entity'`); n != 0 {
		t.Error("idx_job_runs_entity survived the down migration")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM pragma_table_info('job_runs')
	                      WHERE name IN ('entity_type','entity_id','batch_id')`); n != 0 {
		t.Errorf("%d attribution column(s) survived the down migration", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM job_runs`); n != 2 {
		t.Errorf("rows after down = %d, want 2 — the down must drop columns, not data", n)
	}
}
