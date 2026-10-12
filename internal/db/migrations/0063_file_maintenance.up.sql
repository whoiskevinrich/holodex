-- F77 video file maintenance (HOLODEX-556, docs/architecture/file-maintenance.md).
--
-- writeback_queue gains a `kind`: 'write' (every existing row and every tag write) or
-- 'maintenance' (a picture re-encode, or a held original's restore/discard). The two kinds
-- are claimed by separate worker lanes, and neither lane claims a row whose video already has
-- a running row of either kind — the index serves that lane-scoped claim.
ALTER TABLE writeback_queue ADD COLUMN kind TEXT NOT NULL DEFAULT 'write';
CREATE INDEX idx_writeback_queue_kind_status ON writeback_queue(kind, status, enqueued_at);

-- held_originals journals the two-rename swap and then the hold itself: one row per video
-- whose pre-maintenance original sits beside it as `<file>.holodex-orig`. held_path is always
-- derived server-side (file_path + '.holodex-orig') and re-checked before every use.
-- ON DELETE CASCADE matches every other video_* table; the purge job reads this row before
-- it deletes the video, because what happens to the held file depends on delete_remove_files.
CREATE TABLE held_originals (
    video_id   INTEGER PRIMARY KEY REFERENCES videos(id) ON DELETE CASCADE,
    held_path  TEXT    NOT NULL,
    op         TEXT    NOT NULL,
    state      TEXT    NOT NULL CHECK (state IN ('swapping', 'held', 'restoring', 'discarding')),
    created_at TEXT    NOT NULL,
    expires_at TEXT    NOT NULL
);
CREATE INDEX idx_held_originals_expiry ON held_originals(state, expires_at);
