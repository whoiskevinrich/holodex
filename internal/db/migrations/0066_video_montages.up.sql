-- F83 hover montages (HOLODEX-578). A tier of its own beside the still thumbnail
-- (media-ingest.md, "Hover montages"): no column on videos, no value in
-- thumbnail_state. No row means never attempted; the file under
-- DATA_PATH/montages/{id}.mp4 is the montage itself.
CREATE TABLE video_montages (
	video_id   INTEGER PRIMARY KEY REFERENCES videos(id) ON DELETE CASCADE,
	state      TEXT NOT NULL,            -- 'generated' | 'failed'
	error      TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL
);
