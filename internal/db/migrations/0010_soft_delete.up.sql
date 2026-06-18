-- F24 (ADR-037): user-initiated soft-delete. `deleted_at` is an axis orthogonal
-- to `active` (ADR-018, which tracks disk presence): NULL = live, an ISO-8601 UTC
-- timestamp = the owner soft-deleted it. A row is library-visible only when
-- `active = 1 AND deleted_at IS NULL`. Storing this in `active` would be undone by
-- the #26 reactivation fast-path on the next scan of a still-present file, so it
-- needs its own column. Additive + nullable — every existing row defaults to live.
ALTER TABLE videos ADD COLUMN deleted_at TEXT;

-- Backs the purge sweep (`WHERE deleted_at < cutoff`) and the Trash list
-- (`WHERE deleted_at IS NOT NULL`).
CREATE INDEX idx_videos_deleted_at ON videos(deleted_at);
