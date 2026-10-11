-- F78 (HOLODEX-562): owner-initiated soft-delete for films, the same `deleted_at`
-- axis videos carry (F24, migration 0010). NULL = live, an ISO-8601 UTC timestamp =
-- in Trash. A trashed film keeps every row it owns (links, roles, images, aliases)
-- so a restore is lossless, and it keeps its (name, year) key: ux_films_namekey is
-- deliberately NOT made partial, so a create/rename onto a trashed film's key is
-- refused with a restore offer instead of colliding again on restore.
ALTER TABLE films ADD COLUMN deleted_at TEXT;

-- Backs the purge sweep (`WHERE deleted_at <= cutoff`) and the Trash list.
CREATE INDEX idx_films_deleted_at ON films(deleted_at);

-- A film entering or leaving Trash withdraws or restores its title/album candidate
-- on every linked video (spec RD9), so those videos' completeness is stale — the
-- 0058 film-name fan-out, keyed on deleted_at instead of name.
CREATE TRIGGER cd_films_deleted_au AFTER UPDATE OF deleted_at ON films
WHEN new.deleted_at IS NOT old.deleted_at
BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', fv.video_id FROM film_videos fv WHERE fv.film_id = new.id
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = fv.video_id);
END;

-- A permanently deleted film takes its FK-less per-entity rows with it (the film-side
-- 0060), plus every video's standing choice of it as a title/album source (spec
-- RD10: the choice is cleared, so the video falls back to its next source). Covers
-- every path that deletes a film row: purge job, purge-now, and a merge's loser.
CREATE TRIGGER films_ad_entity_rows AFTER DELETE ON films BEGIN
    DELETE FROM entity_enrichment      WHERE entity_type = 'film' AND entity_id = old.id;
    DELETE FROM field_source_decisions WHERE entity_type = 'film' AND entity_id = old.id;
    DELETE FROM metadata_curation      WHERE entity_type = 'film' AND entity_id = old.id;
    DELETE FROM facet_not_applicable   WHERE entity_type = 'film' AND entity_id = old.id;
    DELETE FROM enrichment_dismissals  WHERE entity_type = 'film' AND entity_id = old.id;
    DELETE FROM field_source_decisions WHERE entity_type = 'video' AND source = 'provider:film:' || old.id;
END;
