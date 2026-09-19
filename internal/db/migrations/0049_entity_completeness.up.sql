-- 0049: materialized completeness score + trigger-fed dirty set (F65, ADR-099 D3/D4).
--
-- entity_completeness / entity_completeness_missing are a CACHE of
-- resolver.Complete's output, never a source of truth. The detail page still
-- computes live and rewrites a stale row (self-heal). completeness_dirty is the
-- invalidation contract: every table Complete reads from carries AFTER
-- INSERT/UPDATE/DELETE triggers that insert the owning entity into it, and the
-- next owner-gated read drains the set (resolve, upsert, delete). Trigger bodies
-- use INSERT ... SELECT ... WHERE NOT EXISTS rather than INSERT OR IGNORE: SQLite
-- lets the firing statement's conflict clause (an INSERT OR REPLACE, an upsert)
-- override the one written in the trigger, which turned OR IGNORE into an abort.
--
-- THE TRIGGER SET BELOW IS THE EXHAUSTIVE LIST OF Complete's INPUT TABLES.
-- internal/db/completeness_triggers_test.go enumerates it: an input table without a
-- trigger fails that test. Extend both together (ADR-099 D4).

CREATE TABLE entity_completeness (
    entity_type TEXT    NOT NULL,          -- 'video' | 'person' | 'studio'
    entity_id   INTEGER NOT NULL,
    required    INTEGER,                   -- NULL = no applicable critical facet (ADR-099 D1)
    extras      INTEGER,                   -- NULL = no applicable nice_to_have facet
    computed_at TEXT    NOT NULL,
    PRIMARY KEY (entity_type, entity_id)
);

CREATE TABLE entity_completeness_missing (
    entity_type TEXT    NOT NULL,
    entity_id   INTEGER NOT NULL,
    canonical   TEXT    NOT NULL,
    band        TEXT    NOT NULL,          -- 'critical' | 'nice_to_have'
    PRIMARY KEY (entity_type, entity_id, canonical)
);
-- GET /completeness/facets is a GROUP BY canonical per entity type.
CREATE INDEX idx_entity_completeness_missing_canonical
    ON entity_completeness_missing (entity_type, canonical);

CREATE TABLE completeness_dirty (
    entity_type TEXT    NOT NULL,
    entity_id   INTEGER NOT NULL,
    PRIMARY KEY (entity_type, entity_id)
);

-- Everything starts dirty: the first owner read after upgrade fills the store.
INSERT INTO completeness_dirty (entity_type, entity_id) SELECT 'video', id FROM videos WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video');
INSERT INTO completeness_dirty (entity_type, entity_id) SELECT 'person', id FROM people WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'person');
INSERT INTO completeness_dirty (entity_type, entity_id) SELECT 'studio', id FROM studios WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'studio');

-- ---------------------------------------------------------------------------
-- Entity rows. Insert dirties; delete drops every derived row (the drain cannot
-- score an entity that no longer exists). videos: the scanner's per-scan upsert
-- rewrites every column on every row (UpsertVideo's ON CONFLICT DO UPDATE), so
-- the update trigger fires only when a column Complete actually reads (title,
-- the baseline's one videos column) or the row's visibility changes; without
-- the WHEN clause every scan would dirty the whole library.
-- ---------------------------------------------------------------------------
CREATE TRIGGER cd_videos_ai AFTER INSERT ON videos BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.id);
END;
CREATE TRIGGER cd_videos_au AFTER UPDATE OF title, active, deleted_at ON videos
WHEN new.title IS NOT old.title OR new.active IS NOT old.active OR new.deleted_at IS NOT old.deleted_at
BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.id);
END;
CREATE TRIGGER cd_videos_ad AFTER DELETE ON videos BEGIN
    DELETE FROM entity_completeness         WHERE entity_type = 'video' AND entity_id = old.id;
    DELETE FROM entity_completeness_missing WHERE entity_type = 'video' AND entity_id = old.id;
    DELETE FROM completeness_dirty          WHERE entity_type = 'video' AND entity_id = old.id;
END;

CREATE TRIGGER cd_people_ai AFTER INSERT ON people BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'person', new.id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'person' AND entity_id = new.id);
END;
CREATE TRIGGER cd_people_ad AFTER DELETE ON people BEGIN
    DELETE FROM entity_completeness         WHERE entity_type = 'person' AND entity_id = old.id;
    DELETE FROM entity_completeness_missing WHERE entity_type = 'person' AND entity_id = old.id;
    DELETE FROM completeness_dirty          WHERE entity_type = 'person' AND entity_id = old.id;
END;

CREATE TRIGGER cd_studios_ai AFTER INSERT ON studios BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'studio', new.id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'studio' AND entity_id = new.id);
END;
CREATE TRIGGER cd_studios_ad AFTER DELETE ON studios BEGIN
    DELETE FROM entity_completeness         WHERE entity_type = 'studio' AND entity_id = old.id;
    DELETE FROM entity_completeness_missing WHERE entity_type = 'studio' AND entity_id = old.id;
    DELETE FROM completeness_dirty          WHERE entity_type = 'studio' AND entity_id = old.id;
END;

-- ---------------------------------------------------------------------------
-- Entity-typed shadow tables (entity_type, entity_id): the enrichment shadow
-- store, curation, per-field source decisions, not-applicable marks, aliases.
-- Only the three scored types are ever flagged: a film's shadow rows or a tag's
-- aliases would otherwise leave dirty rows that exist only to be cleared.
-- ---------------------------------------------------------------------------
CREATE TRIGGER cd_entity_enrichment_ai AFTER INSERT ON entity_enrichment BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT new.entity_type, new.entity_id WHERE new.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = new.entity_type AND entity_id = new.entity_id);
END;
CREATE TRIGGER cd_entity_enrichment_au AFTER UPDATE ON entity_enrichment BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT old.entity_type, old.entity_id WHERE old.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = old.entity_type AND entity_id = old.entity_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT new.entity_type, new.entity_id WHERE new.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = new.entity_type AND entity_id = new.entity_id);
END;
CREATE TRIGGER cd_entity_enrichment_ad AFTER DELETE ON entity_enrichment BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT old.entity_type, old.entity_id WHERE old.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = old.entity_type AND entity_id = old.entity_id);
END;

CREATE TRIGGER cd_metadata_curation_ai AFTER INSERT ON metadata_curation BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT new.entity_type, new.entity_id WHERE new.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = new.entity_type AND entity_id = new.entity_id);
END;
CREATE TRIGGER cd_metadata_curation_au AFTER UPDATE ON metadata_curation BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT old.entity_type, old.entity_id WHERE old.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = old.entity_type AND entity_id = old.entity_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT new.entity_type, new.entity_id WHERE new.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = new.entity_type AND entity_id = new.entity_id);
END;
CREATE TRIGGER cd_metadata_curation_ad AFTER DELETE ON metadata_curation BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT old.entity_type, old.entity_id WHERE old.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = old.entity_type AND entity_id = old.entity_id);
END;

CREATE TRIGGER cd_field_source_decisions_ai AFTER INSERT ON field_source_decisions BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT new.entity_type, new.entity_id WHERE new.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = new.entity_type AND entity_id = new.entity_id);
END;
CREATE TRIGGER cd_field_source_decisions_au AFTER UPDATE ON field_source_decisions BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT old.entity_type, old.entity_id WHERE old.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = old.entity_type AND entity_id = old.entity_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT new.entity_type, new.entity_id WHERE new.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = new.entity_type AND entity_id = new.entity_id);
END;
CREATE TRIGGER cd_field_source_decisions_ad AFTER DELETE ON field_source_decisions BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT old.entity_type, old.entity_id WHERE old.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = old.entity_type AND entity_id = old.entity_id);
END;

CREATE TRIGGER cd_facet_not_applicable_ai AFTER INSERT ON facet_not_applicable BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT new.entity_type, new.entity_id WHERE new.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = new.entity_type AND entity_id = new.entity_id);
END;
CREATE TRIGGER cd_facet_not_applicable_au AFTER UPDATE ON facet_not_applicable BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT old.entity_type, old.entity_id WHERE old.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = old.entity_type AND entity_id = old.entity_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT new.entity_type, new.entity_id WHERE new.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = new.entity_type AND entity_id = new.entity_id);
END;
CREATE TRIGGER cd_facet_not_applicable_ad AFTER DELETE ON facet_not_applicable BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT old.entity_type, old.entity_id WHERE old.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = old.entity_type AND entity_id = old.entity_id);
END;

-- alternate_names is an optional facet today (listed, unscored) but it is an
-- input Complete lists, so a future promotion to a band needs no new trigger.
CREATE TRIGGER cd_entity_aliases_ai AFTER INSERT ON entity_aliases BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT new.entity_type, new.entity_id WHERE new.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = new.entity_type AND entity_id = new.entity_id);
END;
CREATE TRIGGER cd_entity_aliases_au AFTER UPDATE ON entity_aliases BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT old.entity_type, old.entity_id WHERE old.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = old.entity_type AND entity_id = old.entity_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT new.entity_type, new.entity_id WHERE new.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = new.entity_type AND entity_id = new.entity_id);
END;
CREATE TRIGGER cd_entity_aliases_ad AFTER DELETE ON entity_aliases BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT old.entity_type, old.entity_id WHERE old.entity_type IN ('video', 'person', 'studio')
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = old.entity_type AND entity_id = old.entity_id);
END;

-- ---------------------------------------------------------------------------
-- Video-keyed tables: the file layer, and the link tables. video_people and
-- video_studios dirty BOTH sides: no person or studio facet reads a link
-- (ADR-099 D4), but *listability* does — a person with no active video has no
-- store row (the drain clears it), so re-linking must flag the person again or
-- it would list with no ring until some unrelated input write.
-- ---------------------------------------------------------------------------
CREATE TRIGGER cd_video_metadata_ai AFTER INSERT ON video_metadata BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
END;
CREATE TRIGGER cd_video_metadata_au AFTER UPDATE ON video_metadata BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', old.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = old.video_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
END;
CREATE TRIGGER cd_video_metadata_ad AFTER DELETE ON video_metadata BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', old.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = old.video_id);
END;

CREATE TRIGGER cd_video_people_ai AFTER INSERT ON video_people BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'person', new.person_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'person' AND entity_id = new.person_id);
END;
CREATE TRIGGER cd_video_people_au AFTER UPDATE ON video_people BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', old.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = old.video_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'person', old.person_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'person' AND entity_id = old.person_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'person', new.person_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'person' AND entity_id = new.person_id);
END;
CREATE TRIGGER cd_video_people_ad AFTER DELETE ON video_people BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', old.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = old.video_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'person', old.person_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'person' AND entity_id = old.person_id);
END;

CREATE TRIGGER cd_video_studios_ai AFTER INSERT ON video_studios BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'studio', new.studio_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'studio' AND entity_id = new.studio_id);
END;
CREATE TRIGGER cd_video_studios_au AFTER UPDATE ON video_studios BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', old.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = old.video_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'studio', old.studio_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'studio' AND entity_id = old.studio_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'studio', new.studio_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'studio' AND entity_id = new.studio_id);
END;
CREATE TRIGGER cd_video_studios_ad AFTER DELETE ON video_studios BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', old.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = old.video_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'studio', old.studio_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'studio' AND entity_id = old.studio_id);
END;

CREATE TRIGGER cd_video_tags_ai AFTER INSERT ON video_tags BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
END;
CREATE TRIGGER cd_video_tags_au AFTER UPDATE ON video_tags BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', old.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = old.video_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
END;
CREATE TRIGGER cd_video_tags_ad AFTER DELETE ON video_tags BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', old.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = old.video_id);
END;

CREATE TRIGGER cd_film_videos_ai AFTER INSERT ON film_videos BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
END;
CREATE TRIGGER cd_film_videos_au AFTER UPDATE ON film_videos BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', old.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = old.video_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', new.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = new.video_id);
END;
CREATE TRIGGER cd_film_videos_ad AFTER DELETE ON film_videos BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', old.video_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = old.video_id);
END;

-- ---------------------------------------------------------------------------
-- Image assets: person `photo` and studio `branding_image` are scored off these.
-- ---------------------------------------------------------------------------
CREATE TRIGGER cd_person_images_ai AFTER INSERT ON person_images BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'person', new.person_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'person' AND entity_id = new.person_id);
END;
CREATE TRIGGER cd_person_images_au AFTER UPDATE ON person_images BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'person', old.person_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'person' AND entity_id = old.person_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'person', new.person_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'person' AND entity_id = new.person_id);
END;
CREATE TRIGGER cd_person_images_ad AFTER DELETE ON person_images BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'person', old.person_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'person' AND entity_id = old.person_id);
END;

CREATE TRIGGER cd_studio_images_ai AFTER INSERT ON studio_images BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'studio', new.studio_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'studio' AND entity_id = new.studio_id);
END;
CREATE TRIGGER cd_studio_images_au AFTER UPDATE ON studio_images BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'studio', old.studio_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'studio' AND entity_id = old.studio_id);
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'studio', new.studio_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'studio' AND entity_id = new.studio_id);
END;
CREATE TRIGGER cd_studio_images_ad AFTER DELETE ON studio_images BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'studio', old.studio_id WHERE NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'studio' AND entity_id = old.studio_id);
END;

-- ---------------------------------------------------------------------------
-- Library-wide inputs. A tag's parent/writeback flag feeds the genre writeback
-- union through the recursive ancestor query, which a trigger body cannot run
-- (no WITH in triggers), so a tag change dirties every video; a denylist change
-- likewise. These are rare, owner-driven edits.
-- ---------------------------------------------------------------------------
CREATE TRIGGER cd_tags_au AFTER UPDATE ON tags BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id) SELECT 'video', id FROM videos WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video');
END;
CREATE TRIGGER cd_tags_ad AFTER DELETE ON tags BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id) SELECT 'video', id FROM videos WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video');
END;

CREATE TRIGGER cd_denied_tags_ai AFTER INSERT ON denied_tags BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id) SELECT 'video', id FROM videos WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video');
END;
CREATE TRIGGER cd_denied_tags_ad AFTER DELETE ON denied_tags BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id) SELECT 'video', id FROM videos WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video');
END;

-- ---------------------------------------------------------------------------
-- Denominator changes: promotions and claims change WHICH fields exist for an
-- entity type, so every entity of that type is dirtied (ADR-099 D4, done here
-- in SQL rather than beside the Go cache drop so any future write to these
-- tables is covered without a hook). Provider field hints fold into every
-- type's field list.
-- ---------------------------------------------------------------------------
CREATE TRIGGER cd_field_promotions_ai AFTER INSERT ON field_promotions BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', id FROM videos  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video') AND new.entity_type = 'video'
        UNION ALL SELECT 'person', id FROM people  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'person') AND new.entity_type = 'person'
        UNION ALL SELECT 'studio', id FROM studios WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'studio') AND new.entity_type = 'studio';
END;
CREATE TRIGGER cd_field_promotions_au AFTER UPDATE ON field_promotions BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', id FROM videos  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video') AND 'video'  IN (old.entity_type, new.entity_type)
        UNION ALL SELECT 'person', id FROM people  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'person') AND 'person' IN (old.entity_type, new.entity_type)
        UNION ALL SELECT 'studio', id FROM studios WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'studio') AND 'studio' IN (old.entity_type, new.entity_type);
END;
CREATE TRIGGER cd_field_promotions_ad AFTER DELETE ON field_promotions BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', id FROM videos  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video') AND old.entity_type = 'video'
        UNION ALL SELECT 'person', id FROM people  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'person') AND old.entity_type = 'person'
        UNION ALL SELECT 'studio', id FROM studios WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'studio') AND old.entity_type = 'studio';
END;

CREATE TRIGGER cd_field_claims_ai AFTER INSERT ON field_claims BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', id FROM videos  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video') AND new.entity_type = 'video'
        UNION ALL SELECT 'person', id FROM people  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'person') AND new.entity_type = 'person'
        UNION ALL SELECT 'studio', id FROM studios WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'studio') AND new.entity_type = 'studio';
END;
CREATE TRIGGER cd_field_claims_au AFTER UPDATE ON field_claims BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', id FROM videos  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video') AND 'video'  IN (old.entity_type, new.entity_type)
        UNION ALL SELECT 'person', id FROM people  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'person') AND 'person' IN (old.entity_type, new.entity_type)
        UNION ALL SELECT 'studio', id FROM studios WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'studio') AND 'studio' IN (old.entity_type, new.entity_type);
END;
CREATE TRIGGER cd_field_claims_ad AFTER DELETE ON field_claims BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', id FROM videos  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video') AND old.entity_type = 'video'
        UNION ALL SELECT 'person', id FROM people  WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'person') AND old.entity_type = 'person'
        UNION ALL SELECT 'studio', id FROM studios WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'studio') AND old.entity_type = 'studio';
END;

CREATE TRIGGER cd_provider_field_hints_ai AFTER INSERT ON provider_field_hints BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', id FROM videos WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video')
        UNION ALL SELECT 'person', id FROM people WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'person')
        UNION ALL SELECT 'studio', id FROM studios WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'studio');
END;
CREATE TRIGGER cd_provider_field_hints_au AFTER UPDATE ON provider_field_hints BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', id FROM videos WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video')
        UNION ALL SELECT 'person', id FROM people WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'person')
        UNION ALL SELECT 'studio', id FROM studios WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'studio');
END;
CREATE TRIGGER cd_provider_field_hints_ad AFTER DELETE ON provider_field_hints BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', id FROM videos WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video')
        UNION ALL SELECT 'person', id FROM people WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'person')
        UNION ALL SELECT 'studio', id FROM studios WHERE id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'studio');
END;
