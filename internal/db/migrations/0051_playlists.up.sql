-- Video playlists (F69, ADR-104 D1/D2): an owner-made container of videos with a
-- sort — a first-class page, NOT an entity on the ADR-061/096 spine (no baseline, no
-- aliases, no external ids, no decisions, no completeness). A playlist is membership
-- + a sort: playlist_videos is a set (one row per video per playlist) carrying a
-- position that only the 'manual' sort reads. sort/visibility domains are validated
-- by the handler, not a CHECK (the film_images.role precedent), so a new value is a
-- code change, not a migration.
CREATE TABLE playlists (
  id          INTEGER PRIMARY KEY,
  name        TEXT    NOT NULL,
  sort        TEXT    NOT NULL DEFAULT 'added_desc',
  visibility  TEXT    NOT NULL DEFAULT 'private',
  created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- Purge (hard delete, ADR-037) cascades here; soft delete does not — playlist reads
-- join through the same `deleted_at IS NULL` seam every list surface uses, so a
-- trashed video vanishes from its playlists and returns on restore (ADR-104 D5).
CREATE TABLE playlist_videos (
  playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
  video_id    INTEGER NOT NULL REFERENCES videos(id)    ON DELETE CASCADE,
  position    INTEGER NOT NULL,
  PRIMARY KEY (playlist_id, video_id)
);
CREATE INDEX playlist_videos_order ON playlist_videos(playlist_id, position);
