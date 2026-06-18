DROP INDEX IF EXISTS idx_videos_deleted_at;
ALTER TABLE videos DROP COLUMN deleted_at;
