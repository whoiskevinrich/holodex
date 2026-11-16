-- 0074 down: restore entity_completeness and 0049's delete triggers. The table comes back
-- empty, so every entity is marked dirty for the next drain to refill it.

CREATE TABLE entity_completeness (
    entity_type TEXT    NOT NULL,
    entity_id   INTEGER NOT NULL,
    required    INTEGER,
    extras      INTEGER,
    computed_at TEXT    NOT NULL,
    PRIMARY KEY (entity_type, entity_id)
);

DROP TRIGGER IF EXISTS cd_videos_ad;
DROP TRIGGER IF EXISTS cd_people_ad;
DROP TRIGGER IF EXISTS cd_studios_ad;

CREATE TRIGGER cd_videos_ad AFTER DELETE ON videos BEGIN
    DELETE FROM entity_completeness         WHERE entity_type = 'video' AND entity_id = old.id;
    DELETE FROM entity_completeness_missing WHERE entity_type = 'video' AND entity_id = old.id;
    DELETE FROM completeness_dirty          WHERE entity_type = 'video' AND entity_id = old.id;
END;
CREATE TRIGGER cd_people_ad AFTER DELETE ON people BEGIN
    DELETE FROM entity_completeness         WHERE entity_type = 'person' AND entity_id = old.id;
    DELETE FROM entity_completeness_missing WHERE entity_type = 'person' AND entity_id = old.id;
    DELETE FROM completeness_dirty          WHERE entity_type = 'person' AND entity_id = old.id;
END;
CREATE TRIGGER cd_studios_ad AFTER DELETE ON studios BEGIN
    DELETE FROM entity_completeness         WHERE entity_type = 'studio' AND entity_id = old.id;
    DELETE FROM entity_completeness_missing WHERE entity_type = 'studio' AND entity_id = old.id;
    DELETE FROM completeness_dirty          WHERE entity_type = 'studio' AND entity_id = old.id;
END;

-- SQLite fires the most recently created trigger first. 0060's cascade deletes a video's
-- shadow rows, whose own triggers mark the video dirty again, so cd_videos_ad must run after
-- it, as it did when 0049 created it. Recreating the cascade restores that order.
DROP TRIGGER IF EXISTS videos_ad_entity_rows;
CREATE TRIGGER videos_ad_entity_rows AFTER DELETE ON videos BEGIN
    DELETE FROM entity_enrichment      WHERE entity_type = 'video' AND entity_id = old.id;
    DELETE FROM field_source_decisions WHERE entity_type = 'video' AND entity_id = old.id;
    DELETE FROM metadata_curation      WHERE entity_type = 'video' AND entity_id = old.id;
    DELETE FROM facet_not_applicable   WHERE entity_type = 'video' AND entity_id = old.id;
END;

INSERT OR IGNORE INTO completeness_dirty (entity_type, entity_id) SELECT 'video', id FROM videos;
INSERT OR IGNORE INTO completeness_dirty (entity_type, entity_id) SELECT 'person', id FROM people;
INSERT OR IGNORE INTO completeness_dirty (entity_type, entity_id) SELECT 'studio', id FROM studios;
