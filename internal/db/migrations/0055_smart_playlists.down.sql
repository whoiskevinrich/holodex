-- Reverts 0055. A smart playlist has no playlist_videos rows, so after this it reads as
-- an empty snapshot playlist; Freeze any you want to keep before rolling back.
ALTER TABLE playlists DROP COLUMN play_shuffled;
ALTER TABLE playlists DROP COLUMN query_version;
ALTER TABLE playlists DROP COLUMN query;
