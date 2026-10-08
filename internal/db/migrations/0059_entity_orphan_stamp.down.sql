-- Reverse 0059. The people backfill stamps are left in place: they are valid
-- 0037-era orphan stamps for people with no link. After this down, studio and tag
-- orphans are no longer stamped and ReconcileVideoPeople no longer stamps people
-- (the triggers did), so pair it with the matching code rollback.
DROP TRIGGER IF EXISTS tags_ai_orphan;
DROP TRIGGER IF EXISTS video_tags_ad_orphan;
DROP TRIGGER IF EXISTS video_tags_ai_orphan;
DROP TRIGGER IF EXISTS video_studios_ad_orphan;
DROP TRIGGER IF EXISTS video_studios_ai_orphan;
DROP TRIGGER IF EXISTS video_people_ad_orphan;
DROP TRIGGER IF EXISTS video_people_ai_orphan;
DROP TRIGGER cd_tags_au;
CREATE TRIGGER cd_tags_au AFTER UPDATE ON tags BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id) SELECT 'video', id FROM videos WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video');
END;
DROP TRIGGER tags_au;
CREATE TRIGGER tags_au AFTER UPDATE ON tags BEGIN
    INSERT INTO tags_fts(tags_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO tags_fts(rowid, name) VALUES (new.id, new.name);
END;
DROP TRIGGER studios_au;
CREATE TRIGGER studios_au AFTER UPDATE ON studios BEGIN
    INSERT INTO studios_fts(studios_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO studios_fts(rowid, name) VALUES (new.id, new.name);
END;
DROP TRIGGER people_au;
CREATE TRIGGER people_au AFTER UPDATE ON people BEGIN
    INSERT INTO people_fts(people_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO people_fts(rowid, name) VALUES (new.id, new.name);
END;
ALTER TABLE tags    DROP COLUMN orphaned_at;
ALTER TABLE studios DROP COLUMN orphaned_at;
