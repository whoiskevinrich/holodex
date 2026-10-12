DROP INDEX IF EXISTS idx_videos_content_fp;
DROP INDEX IF EXISTS idx_videos_file_size;
ALTER TABLE videos DROP COLUMN content_fp;
