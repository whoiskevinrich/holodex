-- HOLODEX-547: a permanently deleted video takes its per-entity rows with it. These
-- tables key on (entity_type, entity_id) with no foreign key, so the videos row's
-- ON DELETE CASCADEs never reach them and they outlived the purge as orphans. A
-- trigger, like 0024's dismissals and 0049's completeness rows, covers every path
-- that deletes a video (purge job, purge-now, Delete permanently).
CREATE TRIGGER videos_ad_entity_rows AFTER DELETE ON videos BEGIN
    DELETE FROM entity_enrichment      WHERE entity_type = 'video' AND entity_id = old.id;
    DELETE FROM field_source_decisions WHERE entity_type = 'video' AND entity_id = old.id;
    DELETE FROM metadata_curation      WHERE entity_type = 'video' AND entity_id = old.id;
    DELETE FROM facet_not_applicable   WHERE entity_type = 'video' AND entity_id = old.id;
END;

-- One-time sweep of the orphans earlier purges left behind.
DELETE FROM entity_enrichment      WHERE entity_type = 'video' AND entity_id NOT IN (SELECT id FROM videos);
DELETE FROM field_source_decisions WHERE entity_type = 'video' AND entity_id NOT IN (SELECT id FROM videos);
DELETE FROM metadata_curation      WHERE entity_type = 'video' AND entity_id NOT IN (SELECT id FROM videos);
DELETE FROM facet_not_applicable   WHERE entity_type = 'video' AND entity_id NOT IN (SELECT id FROM videos);
