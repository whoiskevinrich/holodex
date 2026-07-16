-- F48.4 (ADR-067): pending/dismissed/resolved field-level extraction conflicts.
-- One row per (video, field) where filename-derived and tag-derived data disagree,
-- or where the confidence-scored candidate didn't clear its field tier's
-- AutoApplyThreshold (F48.3/F48.4a-b) — a candidate that clears both the threshold
-- and, for entity fields, the exact-match gate never creates a row here at all; it
-- writes directly. suggested_entity_id is the Jaro-Winkler-ranked advisory match
-- (F48.3d) — never itself a resolution, only a pre-fill for the owner to confirm.
--
-- status transitions: pending -> resolved (owner picked a value, F48.4c) or
-- dismissed (F48.4d, durable until the owner re-triggers extraction for the file,
-- which is free to open a fresh pending row for the same field). The partial unique
-- index means only ONE pending row can exist per (video, field) at a time — a
-- re-run while already pending updates that row in place (F48.4b) rather than
-- duplicating it.
CREATE TABLE metadata_extraction_review (
    id                   INTEGER PRIMARY KEY,
    video_id             INTEGER NOT NULL,
    field_key            TEXT    NOT NULL,
    filename_value       TEXT    NOT NULL DEFAULT '',
    tag_value            TEXT    NOT NULL DEFAULT '',
    confidence           REAL    NOT NULL,
    suggested_entity_id  INTEGER,
    status               TEXT    NOT NULL DEFAULT 'pending',
    created_at           TEXT    NOT NULL,
    resolved_at          TEXT
);

CREATE UNIQUE INDEX ux_extraction_review_pending
    ON metadata_extraction_review(video_id, field_key)
    WHERE status = 'pending';

CREATE INDEX idx_extraction_review_status ON metadata_extraction_review(status);

-- A deleted video's pending review is meaningless; cascade like enrichment_dismissals (0024).
CREATE TRIGGER videos_ad_extraction_review AFTER DELETE ON videos BEGIN
    DELETE FROM metadata_extraction_review WHERE video_id = old.id;
END;
