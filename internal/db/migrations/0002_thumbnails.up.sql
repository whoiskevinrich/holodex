-- Phase 2: per-video thumbnail pipeline state (ADR-009).
--
-- thumbnail_state tracks the cover-image lifecycle. The image bytes live on disk
-- at DATA_PATH/thumbnails/{id}.jpg (ADR-014), never in the DB.
--   NULL         never attempted — a backfill candidate
--   'embedded'   Tier 1: cover art extracted from the container at index time
--   'generated'  Tier 2: a frame extracted by ffmpeg
--   'failed'     last attempt errored — retried by the startup backfill sweep
ALTER TABLE videos ADD COLUMN thumbnail_state TEXT;

-- Partial index over the backfill predicate (active rows still needing an image).
CREATE INDEX idx_videos_thumbnail_pending ON videos(active)
    WHERE thumbnail_state IS NULL OR thumbnail_state = 'failed';
