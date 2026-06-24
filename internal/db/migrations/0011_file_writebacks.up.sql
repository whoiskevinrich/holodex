-- F28 (ADR-041): audit log for operator-triggered metadata writes back to media
-- files. One row per successful write; a write that fails never inserts here.
-- written_at is RFC3339 UTC, matching every other timestamp column.
-- tag_name is the actual exiftool tag target (e.g. "QuickTime:Title", "GENRE").
-- source mirrors ResolvedField.WinningSource so a later reader knows where the
-- written value came from (e.g. "tmdb:title").
CREATE TABLE file_writebacks (
    id         INTEGER PRIMARY KEY,
    video_id   INTEGER NOT NULL REFERENCES videos(id),
    field_key  TEXT    NOT NULL,
    tag_name   TEXT    NOT NULL,
    value      TEXT    NOT NULL,
    source     TEXT    NOT NULL,
    written_at TEXT    NOT NULL
);

CREATE INDEX idx_file_writebacks_video ON file_writebacks(video_id);
