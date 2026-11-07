ALTER TABLE playlists DROP COLUMN cap;
DROP INDEX idx_videos_added_at;
ALTER TABLE videos DROP COLUMN added_at;
