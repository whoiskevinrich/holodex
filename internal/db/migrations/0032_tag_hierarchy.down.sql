DROP INDEX IF EXISTS idx_tags_parent;
ALTER TABLE tags DROP COLUMN parent_tag_id;
