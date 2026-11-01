-- F86 rename journal (HOLODEX-602, file-rename.md "The rename journal").
-- One append-only row per rename. Names are base names in the video's own folder.
-- A 'pending' row is written before the filesystem rename and is the crash journal;
-- via_name is set while a two-step rename is half done. token_values is JSON,
-- token -> the sanitised values written; template is the text that wrote the name.
CREATE TABLE file_renames (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	video_id     INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
	old_name     TEXT NOT NULL,
	new_name     TEXT NOT NULL,
	via_name     TEXT NOT NULL DEFAULT '',
	template     TEXT NOT NULL DEFAULT '',
	token_values TEXT NOT NULL DEFAULT '{}',
	trigger      TEXT NOT NULL CHECK (trigger IN ('manual', 'writeback', 'job', 'undo')),
	undo_of      INTEGER REFERENCES file_renames(id) ON DELETE SET NULL,
	state        TEXT NOT NULL CHECK (state IN ('pending', 'done')),
	created_at   TEXT NOT NULL
);
CREATE INDEX idx_file_renames_video ON file_renames(video_id, id);
CREATE INDEX idx_file_renames_pending ON file_renames(state) WHERE state = 'pending';

-- Per-video name state: the lock and the settle observation (desired_name, desired_since).
CREATE TABLE video_file_names (
	video_id      INTEGER PRIMARY KEY REFERENCES videos(id) ON DELETE CASCADE,
	locked        INTEGER NOT NULL DEFAULT 0,
	lock_source   TEXT NOT NULL DEFAULT '' CHECK (lock_source IN ('', 'auto', 'owner', 'undo')),
	locked_at     TEXT NOT NULL DEFAULT '',
	desired_name  TEXT NOT NULL DEFAULT '',
	desired_since TEXT NOT NULL DEFAULT ''
);
