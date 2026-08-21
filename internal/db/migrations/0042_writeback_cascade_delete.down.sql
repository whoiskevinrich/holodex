-- Reverse 0042: drop the ON DELETE CASCADE, restoring the original bare FK.
CREATE TABLE file_writebacks_old (
    id         INTEGER PRIMARY KEY,
    video_id   INTEGER NOT NULL REFERENCES videos(id),
    field_key  TEXT    NOT NULL,
    tag_name   TEXT    NOT NULL,
    value      TEXT    NOT NULL,
    source     TEXT    NOT NULL,
    written_at TEXT    NOT NULL
);
INSERT INTO file_writebacks_old SELECT * FROM file_writebacks;
DROP TABLE file_writebacks;
ALTER TABLE file_writebacks_old RENAME TO file_writebacks;
CREATE INDEX idx_file_writebacks_video ON file_writebacks(video_id);

CREATE TABLE writeback_queue_old (
    id          INTEGER PRIMARY KEY,
    video_id    INTEGER NOT NULL REFERENCES videos(id),
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
