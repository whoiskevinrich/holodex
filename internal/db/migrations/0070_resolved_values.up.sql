-- 0070: F89 smart playlist rules, slice 3 (field-resolution.md, "Winning values are indexed").
--
-- resolved_values holds each video's winning value per queryable field, one row per value
-- of a multi-value field. It is a cache rewritten by the completeness drain, never a source
-- of truth. Every video starts dirty, so the next drain fills it.
--
-- videos.changed_at is what `updated:` and the Updated sort read: indexed_at for existing
-- rows, the nearest record of a real change. videos.change_sig is the fingerprint of the
-- video's inputs that the drain compares to move it. NULL means not yet fingerprinted, and
-- taking the first one moves nothing, so this backfill doesn't mark the library as updated.
CREATE TABLE resolved_values (
    video_id   INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    field_key  TEXT NOT NULL,
    norm_value TEXT NOT NULL,
    value      TEXT NOT NULL,
    PRIMARY KEY (video_id, field_key, norm_value)
);
CREATE INDEX idx_resolved_values_field ON resolved_values (field_key, norm_value);

ALTER TABLE videos ADD COLUMN changed_at TEXT;
ALTER TABLE videos ADD COLUMN change_sig TEXT;
UPDATE videos SET changed_at = indexed_at;
CREATE INDEX idx_videos_changed_at ON videos (changed_at);

INSERT OR IGNORE INTO completeness_dirty (entity_type, entity_id) SELECT 'video', id FROM videos;
