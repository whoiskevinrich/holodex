-- 0069: F89 smart playlist rules, slice 1 (docs/specs/smart-playlist-rules.md).
--
-- videos.added_at is when the video arrived, for `added:` (P0-1). UpsertVideo sets it on
-- the INSERT only, so it never moves. Existing rows take indexed_at, which is exact for a
-- file that hasn't changed since import and a little late for one that has: nothing
-- earlier records arrival time.
--
-- playlists.cap is a smart playlist's cap (P0-2): NULL for none, else 1..10,000 (SEC-7),
-- checked by the API.
ALTER TABLE videos ADD COLUMN added_at TEXT;
UPDATE videos SET added_at = indexed_at;
CREATE INDEX idx_videos_added_at ON videos (added_at);

ALTER TABLE playlists ADD COLUMN cap INTEGER;
