-- 0059: one orphan stamp for people, studios and tags (HOLODEX-535,
-- docs/architecture/entity-relationships.md "one orphan stamp and one sweep").
--
-- A studio used to be deleted in the same transaction that removed its last
-- video link, taking its aliases, images, decisions and enrichment with it; tags
-- were never removed at all; and a person orphaned by a purge cascade was never
-- stamped. Studios and tags now carry orphaned_at like people, and these triggers
-- own the stamp for all three, so every path that adds or removes a link keeps it
-- honest: reconcile, tag attach/detach, rescan, merge, and the FK cascade of a
-- purged video (cascade deletes fire child-table triggers, as 0049/0058 rely on).
-- The orphan sweep (repo.SweepOrphans) deletes rows stamped longer ago than the
-- grace period that carry no authored data.
--
-- The stamp is RFC3339 UTC, the layout the repo writes (repo.timeLayout). The
-- delete triggers keep the earliest stamp (orphaned_at IS NULL guard).
ALTER TABLE studios ADD COLUMN orphaned_at TEXT;
ALTER TABLE tags    ADD COLUMN orphaned_at TEXT;

-- The name FTS mirrors re-index on ANY update of their row (0001/0017), so every
-- orphan stamp below would rewrite the FTS entry, and tags_ai_orphan's update
-- would run before tags_ai has indexed the new row ("database disk image is
-- malformed"). FTS indexes only `name`; scope the update mirrors to it.
DROP TRIGGER people_au;
CREATE TRIGGER people_au AFTER UPDATE OF name ON people BEGIN
    INSERT INTO people_fts(people_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO people_fts(rowid, name) VALUES (new.id, new.name);
END;
DROP TRIGGER studios_au;
CREATE TRIGGER studios_au AFTER UPDATE OF name ON studios BEGIN
    INSERT INTO studios_fts(studios_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO studios_fts(rowid, name) VALUES (new.id, new.name);
END;
DROP TRIGGER tags_au;
CREATE TRIGGER tags_au AFTER UPDATE OF name ON tags BEGIN
    INSERT INTO tags_fts(tags_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO tags_fts(rowid, name) VALUES (new.id, new.name);
END;
-- Same for 0049's completeness trigger, which dirties EVERY video on any tag update:
-- unscoped, a rescan that briefly empties a tag would recompute the whole library.
-- Scope it to the columns that existed when it was written (the stamp is not an input).
DROP TRIGGER cd_tags_au;
CREATE TRIGGER cd_tags_au AFTER UPDATE OF name, parent_tag_id, writeback_enabled ON tags BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id) SELECT 'video', id FROM videos WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video');
END;

CREATE TRIGGER video_people_ai_orphan AFTER INSERT ON video_people BEGIN
    UPDATE people SET orphaned_at = NULL WHERE id = new.person_id AND orphaned_at IS NOT NULL;
END;
CREATE TRIGGER video_people_ad_orphan AFTER DELETE ON video_people BEGIN
    UPDATE people SET orphaned_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
     WHERE id = old.person_id AND orphaned_at IS NULL
       AND NOT EXISTS (SELECT 1 FROM video_people WHERE person_id = old.person_id);
END;

CREATE TRIGGER video_studios_ai_orphan AFTER INSERT ON video_studios BEGIN
    UPDATE studios SET orphaned_at = NULL WHERE id = new.studio_id AND orphaned_at IS NOT NULL;
END;
CREATE TRIGGER video_studios_ad_orphan AFTER DELETE ON video_studios BEGIN
    UPDATE studios SET orphaned_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
     WHERE id = old.studio_id AND orphaned_at IS NULL
       AND NOT EXISTS (SELECT 1 FROM video_studios WHERE studio_id = old.studio_id);
END;

CREATE TRIGGER video_tags_ai_orphan AFTER INSERT ON video_tags BEGIN
    UPDATE tags SET orphaned_at = NULL WHERE id = new.tag_id AND orphaned_at IS NOT NULL;
END;
CREATE TRIGGER video_tags_ad_orphan AFTER DELETE ON video_tags BEGIN
    UPDATE tags SET orphaned_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
     WHERE id = old.tag_id AND orphaned_at IS NULL
       AND NOT EXISTS (SELECT 1 FROM video_tags WHERE tag_id = old.tag_id);
END;
-- A tag can be created bare on /tags; it starts orphaned. A tag created for a
-- link is cleared by video_tags_ai_orphan when the link lands.
CREATE TRIGGER tags_ai_orphan AFTER INSERT ON tags BEGIN
    UPDATE tags SET orphaned_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = new.id;
END;

-- Backfill: start the grace period now for every entity with no link today
-- (long-orphaned and bare tags, studios and people left behind by a purge).
UPDATE studios SET orphaned_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
 WHERE NOT EXISTS (SELECT 1 FROM video_studios WHERE studio_id = studios.id);
UPDATE tags SET orphaned_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
 WHERE NOT EXISTS (SELECT 1 FROM video_tags WHERE tag_id = tags.id);
UPDATE people SET orphaned_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
 WHERE orphaned_at IS NULL
   AND NOT EXISTS (SELECT 1 FROM video_people WHERE person_id = people.id);
