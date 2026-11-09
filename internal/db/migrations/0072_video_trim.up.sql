-- F88 video trimming (HOLODEX-613, file-maintenance.md). job_runs is pruned after 30 days, too
-- short for either of these.
--
-- video_trim_results: the last trim's outcome on the page of the video it ran from, until the
-- owner dismisses it. skipped is JSON, one {"end","reason"} per frame the screen check skipped.
-- edition_video_id is set when the trim saved an edition.
CREATE TABLE video_trim_results (
	video_id         INTEGER PRIMARY KEY REFERENCES videos(id) ON DELETE CASCADE,
	mode             TEXT NOT NULL CHECK (mode IN ('replace', 'edition')),
	edition_name     TEXT NOT NULL DEFAULT '',
	edition_video_id INTEGER REFERENCES videos(id) ON DELETE SET NULL,
	skipped          TEXT NOT NULL DEFAULT '[]',
	created_at       TEXT NOT NULL
);

-- video_trimmed_from: an edition's first history entry. It outlives the source, which may be
-- purged, so the source id carries no foreign key; a reader finds no title and shows no link.
CREATE TABLE video_trimmed_from (
	video_id        INTEGER PRIMARY KEY REFERENCES videos(id) ON DELETE CASCADE,
	source_video_id INTEGER NOT NULL,
	created_at      TEXT NOT NULL
);
