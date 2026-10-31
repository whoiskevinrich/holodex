-- F85 poster source slots (HOLODEX-599, media-ingest.md "Poster source slots").
-- One row per held local slot; the image itself is DATA_PATH/thumbnails/{id}-<kind>.jpg.
-- hash is the sha256 of the slot's bytes, the local:<kind>#<hash> decision token's
-- content half (field-resolution.md). frame_at is the picked time, frame only.
-- The file slot ({id}-file.jpg) has no row: it is never a local decision source.
CREATE TABLE video_poster_sources (
	video_id   INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
	kind       TEXT NOT NULL CHECK (kind IN ('frame', 'upload', 'automatic')),
	hash       TEXT NOT NULL,
	frame_at   REAL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (video_id, kind)
);
