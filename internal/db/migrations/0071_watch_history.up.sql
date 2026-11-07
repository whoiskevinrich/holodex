-- 0071: F89 smart playlist rules, slice 4 (watch-history.md).
--
-- video_plays holds one row per counted play of the owner's: the video, when, and how far
-- playback got (SEC-10: nothing else, no IP, user agent or session). video_watch is the
-- per-video summary the query conditions read, written in the same transaction, and
-- rebuildable from the rows. manual_watched is the owner's mark: NULL for none, 1 watched,
-- 0 unwatched. A video's purge is the only delete path for either table.
CREATE TABLE video_plays (
    id        INTEGER PRIMARY KEY,
    video_id  INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    played_at TEXT NOT NULL,
    progress  REAL NOT NULL
);
CREATE INDEX idx_video_plays_video ON video_plays (video_id, played_at);

CREATE TABLE video_watch (
    video_id       INTEGER PRIMARY KEY REFERENCES videos(id) ON DELETE CASCADE,
    play_count     INTEGER NOT NULL DEFAULT 0,
    last_played_at TEXT,
    manual_watched INTEGER,
    manual_at      TEXT
);
