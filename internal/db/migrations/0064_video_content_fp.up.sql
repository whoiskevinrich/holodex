-- HOLODEX-568 / F79: a sampled content fingerprint (hex SHA-256 over the byte size plus three
-- 64 KiB samples) that identifies a video across a move or rename (media-ingest.md, "Moves
-- recognised by a sampled content hash"). NULL = not yet fingerprinted. No backfill here: the
-- scanner fills an unchanged file's row lazily on its next pass, and every insert or re-extract
-- writes it. A hidden row whose file is already gone stays NULL and matches on its recorded facts.
ALTER TABLE videos ADD COLUMN content_fp TEXT;

-- The scan holds a new file only when an existing row shares its byte size, and the fold pairs
-- rows by fingerprint; both look these up per file.
CREATE INDEX idx_videos_file_size ON videos(file_size);
CREATE INDEX idx_videos_content_fp ON videos(content_fp) WHERE content_fp IS NOT NULL;
