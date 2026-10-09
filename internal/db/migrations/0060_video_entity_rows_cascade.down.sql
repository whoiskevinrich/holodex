-- Reverse 0060. The swept orphans are not restored: they pointed at videos that no
-- longer exist.
DROP TRIGGER IF EXISTS videos_ad_entity_rows;
