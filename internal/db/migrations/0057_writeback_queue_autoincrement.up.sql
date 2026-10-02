-- HOLODEX-510: writeback_queue.id was a plain INTEGER PRIMARY KEY, and a
-- successful job's row is deleted (FinishWriteback), so SQLite reissued the
-- same id to the next job. A job with no caller-supplied batch id snapshots
-- under its own id (snapshotBeforeWrite, F48.9a), so the later job found the
-- earlier one's snapshot, skipped its own capture, and a Revert of that batch
-- restored a mix of both writes. AUTOINCREMENT never reissues an id.
--
-- Rebuilt the same way as 0042 (no other table references writeback_queue).
CREATE TABLE writeback_queue_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    video_id    INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    payload     TEXT    NOT NULL,
    status      TEXT    NOT NULL DEFAULT 'pending',
    attempts    INTEGER NOT NULL DEFAULT 0,
    error       TEXT    NOT NULL DEFAULT '',
    enqueued_at TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL,
    batch_id    TEXT    NOT NULL DEFAULT ''
);
INSERT INTO writeback_queue_new SELECT * FROM writeback_queue;
DROP TABLE writeback_queue;
ALTER TABLE writeback_queue_new RENAME TO writeback_queue;
CREATE INDEX idx_writeback_queue_status ON writeback_queue(status, enqueued_at);

-- The copy seeds the sequence from the surviving rows only, which sit below
-- ids already spent by deleted jobs. Start above every all-digit batch id a
-- snapshot or job run still carries, so no new job lands on an old batch.
-- sqlite_sequence has no unique key on name, so replace by delete + insert.
DELETE FROM sqlite_sequence WHERE name = 'writeback_queue';
INSERT INTO sqlite_sequence (name, seq)
SELECT 'writeback_queue', MAX(n) FROM (
    SELECT COALESCE(MAX(id), 0) AS n FROM writeback_queue
    UNION ALL
    SELECT COALESCE(MAX(CAST(batch_id AS INTEGER)), 0) FROM file_writeback_snapshots
        WHERE batch_id <> '' AND batch_id NOT GLOB '*[^0-9]*'
    UNION ALL
    SELECT COALESCE(MAX(CAST(batch_id AS INTEGER)), 0) FROM job_runs
        WHERE batch_id <> '' AND batch_id NOT GLOB '*[^0-9]*'
);
