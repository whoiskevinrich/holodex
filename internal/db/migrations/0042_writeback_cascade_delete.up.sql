-- Fix: file_writebacks (F28/0011) and writeback_queue (F30/0014) were created
-- referencing videos(id) without ON DELETE CASCADE, unlike every other video_*
-- junction table (video_tags/video_metadata/video_studios/video_people). F24's
-- HardDelete (internal/repo/delete.go) assumes cascading FKs clean up every
-- child table, so a video with writeback history hits a FOREIGN KEY constraint
-- failure on every purge attempt, forever (confirmed in production: 3 videos
-- retried hourly for over a week). Rebuild both tables with ON DELETE CASCADE;
-- neither table is itself referenced by another table, so no dependent rebuild
-- is needed.
CREATE TABLE file_writebacks_new (
    id         INTEGER PRIMARY KEY,
    video_id   INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    field_key  TEXT    NOT NULL,
    tag_name   TEXT    NOT NULL,
    value      TEXT    NOT NULL,
    source     TEXT    NOT NULL,
    written_at TEXT    NOT NULL
);
INSERT INTO file_writebacks_new SELECT * FROM file_writebacks;
DROP TABLE file_writebacks;
ALTER TABLE file_writebacks_new RENAME TO file_writebacks;
CREATE INDEX idx_file_writebacks_video ON file_writebacks(video_id);

CREATE TABLE writeback_queue_new (
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
INSERT INTO writeback_queue_new SELECT * FROM writeback_queue;
DROP TABLE writeback_queue;
ALTER TABLE writeback_queue_new RENAME TO writeback_queue;
CREATE INDEX idx_writeback_queue_status ON writeback_queue(status, enqueued_at);
