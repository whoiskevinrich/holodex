-- Smart playlists (F75, ADR-121 D1/D8). A playlist is smart iff `query` IS NOT NULL: it
-- stores the canonical /media filter string (D2) and is re-evaluated on every read, so it
-- has no playlist_videos rows. query_version is NULL iff query is NULL (D2). Freeze runs the
-- query once, writes playlist_videos and nulls both, leaving an ordinary ADR-104 snapshot.
-- play_shuffled ("always shuffle", RD13) applies to any playlist, smart or not.
ALTER TABLE playlists ADD COLUMN query TEXT;
ALTER TABLE playlists ADD COLUMN query_version INTEGER;
ALTER TABLE playlists ADD COLUMN play_shuffled INTEGER NOT NULL DEFAULT 0;
