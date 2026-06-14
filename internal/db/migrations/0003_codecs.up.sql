-- Phase 2 (F12.4): per-video codec / container / bitrate, captured from ffprobe
-- stream + format metadata at index time. Existing rows backfill on the next
-- scan (or an admin rescan); the NOT NULL defaults keep pre-backfill rows valid.
ALTER TABLE videos ADD COLUMN video_codec  TEXT    NOT NULL DEFAULT '';
ALTER TABLE videos ADD COLUMN audio_codec  TEXT    NOT NULL DEFAULT '';
ALTER TABLE videos ADD COLUMN bitrate_kbps INTEGER NOT NULL DEFAULT 0;
ALTER TABLE videos ADD COLUMN container    TEXT    NOT NULL DEFAULT '';
