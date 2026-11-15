-- 0074: retire the Completeness score (F90 P0-6, field-resolution.md).
--
-- entity_completeness held each entity's required and extras bands for the ring, the
-- Completeness sorts and the list summary. All three are gone, so the table goes. Kept:
-- entity_completeness_missing (the owner queue and missing_facet), completeness_dirty (the
-- drain's invalidation set, which also feeds entity_field_state and resolved_values) and
-- every input trigger. The three entity delete triggers named the dropped table, so they are
-- recreated without it.

DROP TRIGGER IF EXISTS cd_videos_ad;
DROP TRIGGER IF EXISTS cd_people_ad;
DROP TRIGGER IF EXISTS cd_studios_ad;

CREATE TRIGGER cd_videos_ad AFTER DELETE ON videos BEGIN
    DELETE FROM entity_completeness_missing WHERE entity_type = 'video' AND entity_id = old.id;
    DELETE FROM completeness_dirty          WHERE entity_type = 'video' AND entity_id = old.id;
END;
CREATE TRIGGER cd_people_ad AFTER DELETE ON people BEGIN
    DELETE FROM entity_completeness_missing WHERE entity_type = 'person' AND entity_id = old.id;
    DELETE FROM completeness_dirty          WHERE entity_type = 'person' AND entity_id = old.id;
END;
CREATE TRIGGER cd_studios_ad AFTER DELETE ON studios BEGIN
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

DROP TABLE IF EXISTS entity_completeness;
