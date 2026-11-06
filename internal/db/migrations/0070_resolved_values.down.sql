DROP INDEX idx_videos_changed_at;
ALTER TABLE videos DROP COLUMN change_sig;
ALTER TABLE videos DROP COLUMN changed_at;
DROP TABLE resolved_values;
