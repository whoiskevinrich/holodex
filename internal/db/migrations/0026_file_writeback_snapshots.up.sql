-- F48.9 (ADR-067): pre-write snapshot of a field's on-disk value, captured
-- immediately before a writeback job overwrites it — the rollback foundation
-- extraction auto-apply (F48.4) needs before it can run unattended at scale.
-- Amends ADR-041's deferred "prior-value capture / undo" non-goal.
--
-- batch_id groups every snapshot taken by one write operation (today: one
-- writequeue job — one video, N fields written in a single tool pass; a
-- future multi-video operation such as merge propagation, F48.8, could share
-- one batch_id across videos so a single Revert restores all of them).
-- prior_value is "" when the field had no value on disk before the write.
--
-- field_key (not the container tag name) is stored deliberately, mirroring
-- writeback_queue's JobField payload: a revert re-resolves the tag name from
-- the video's current container at write time rather than trusting a stored
-- one (security C2, see internal/writequeue/writequeue.go buildBatch).
--
-- Revert is a forward write through the same queue (F48.9b/c): it re-enqueues
-- each snapshotted field's prior_value as the new value, so the revert itself
-- is snapshotted too (undo-of-undo) — no special-cased write path.
CREATE TABLE file_writeback_snapshots (
    id          INTEGER PRIMARY KEY,
    video_id    INTEGER NOT NULL,
    batch_id    TEXT    NOT NULL,
    field_key   TEXT    NOT NULL,
    prior_value TEXT    NOT NULL DEFAULT '',
    written_at  TEXT    NOT NULL
);

CREATE INDEX idx_writeback_snapshots_batch ON file_writeback_snapshots(batch_id);
CREATE INDEX idx_writeback_snapshots_video ON file_writeback_snapshots(video_id, written_at);

-- A deleted video's snapshots are meaningless; cascade like extraction_review (0025).
CREATE TRIGGER videos_ad_writeback_snapshots AFTER DELETE ON videos BEGIN
    DELETE FROM file_writeback_snapshots WHERE video_id = old.id;
END;
