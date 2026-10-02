-- Reverse 0057: drop AUTOINCREMENT, restoring 0042's plain INTEGER PRIMARY KEY
-- (and with it HOLODEX-510's id reuse).
CREATE TABLE writeback_queue_old (
    id          INTEGER PRIMARY KEY,
    video_id    INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    payload     TEXT    NOT NULL,
    status      TEXT    NOT NULL DEFAULT 'pending',
    attempts    INTEGER NOT NULL DEFAULT 0,
    error       TEXT    NOT NULL DEFAULT '',
    enqueued_at TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL,
    batch_id    TEXT    NOT NULL DEFAULT ''
);
INSERT INTO writeback_queue_old SELECT * FROM writeback_queue;
DROP TABLE writeback_queue;
ALTER TABLE writeback_queue_old RENAME TO writeback_queue;
CREATE INDEX idx_writeback_queue_status ON writeback_queue(status, enqueued_at);
DELETE FROM sqlite_sequence WHERE name = 'writeback_queue';
