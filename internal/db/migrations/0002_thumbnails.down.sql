DROP INDEX IF EXISTS idx_videos_thumbnail_pending;
ALTER TABLE videos DROP COLUMN thumbnail_state;
