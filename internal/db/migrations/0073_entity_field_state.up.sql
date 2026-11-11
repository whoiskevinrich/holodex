-- F90 (HOLODEX-626, field-resolution.md "Field presence and provider contribution are cached
-- in entity_field_state"). One row per cached field that is shown, enriched, or both, for
-- videos, people and studios. A cache like resolved_values, rewritten by the completeness
-- drain from one resolver pass; never a source of truth.
CREATE TABLE entity_field_state (
    entity_type TEXT NOT NULL,
    entity_id   INTEGER NOT NULL,
    field_key   TEXT NOT NULL,
    shown       INTEGER NOT NULL DEFAULT 0,
    enriched    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (entity_type, entity_id, field_key)
);
CREATE INDEX idx_entity_field_state_field ON entity_field_state (entity_type, field_key);

-- The poster is a field here, and two inputs the completeness triggers never covered decide
-- it: the held local slots and the file-slot lifecycle on videos.thumbnail_state.
CREATE TRIGGER cd_video_poster_sources_ai AFTER INSERT ON video_poster_sources BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
END;
CREATE TRIGGER cd_video_poster_sources_au AFTER UPDATE ON video_poster_sources BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
END;
CREATE TRIGGER cd_video_poster_sources_ad AFTER DELETE ON video_poster_sources BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', old.video_id WHERE EXISTS (SELECT 1 FROM videos WHERE id = old.video_id)
            AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = old.video_id);
END;
CREATE TRIGGER cd_videos_thumbnail_au AFTER UPDATE OF thumbnail_state ON videos
    WHEN old.thumbnail_state IS NOT new.thumbnail_state BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.id);
END;

-- No foreign key can span three entity tables, so a deleted entity takes its rows here.
CREATE TRIGGER efs_videos_ad AFTER DELETE ON videos BEGIN
    DELETE FROM entity_field_state WHERE entity_type = 'video' AND entity_id = old.id;
END;
CREATE TRIGGER efs_people_ad AFTER DELETE ON people BEGIN
    DELETE FROM entity_field_state WHERE entity_type = 'person' AND entity_id = old.id;
END;
CREATE TRIGGER efs_studios_ad AFTER DELETE ON studios BEGIN
    DELETE FROM entity_field_state WHERE entity_type = 'studio' AND entity_id = old.id;
END;

-- Everything starts dirty, so the next drain fills the cache.
INSERT OR IGNORE INTO completeness_dirty (entity_type, entity_id) SELECT 'video', id FROM videos;
INSERT OR IGNORE INTO completeness_dirty (entity_type, entity_id) SELECT 'person', id FROM people;
INSERT OR IGNORE INTO completeness_dirty (entity_type, entity_id) SELECT 'studio', id FROM studios;
