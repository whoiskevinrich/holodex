-- HOLODEX-375 (F60, ADR-096 D2): one polymorphic external-id table for every entity
-- kind, replacing the two per-kind tables (studio_external_ids 0018, person_external_ids
-- 0038). Provider identity was stored in two places with two meanings and for two kinds
-- only; tags and films had no identity row at all, so "find film by TMDB id" did not
-- exist. Shaped like entity_aliases (0022): external_id is the namespace-qualified
-- "<provider>:<id>" string, and the PK makes it unique PER KIND -- the de-dup guarantee
-- resolveOrCreateByName relies on (an id owns exactly one entity of its kind), while
-- letting the same id string name a person and a studio.
--
-- The resolve order does not change: external id → nameKey → alias → create (ADR-061 D5),
-- now for all four kinds. The re-enrich memo (entity_enrichment.external_id) is NOT
-- dropped here -- it also serves video re-enrich, which has no row in this table.
CREATE TABLE entity_external_ids (
    entity_type TEXT    NOT NULL,   -- 'person' | 'studio' | 'tag' | 'film'
    entity_id   INTEGER NOT NULL,
    external_id TEXT    NOT NULL,   -- '<provider>:<id>'
    PRIMARY KEY (entity_type, external_id)
);
-- Reverse lookup (an entity's ids) and the trigger cleanup path; the id lookup is the PK.
CREATE INDEX idx_entity_external_ids_entity ON entity_external_ids(entity_type, entity_id);

INSERT INTO entity_external_ids (entity_type, entity_id, external_id)
    SELECT 'person', person_id, external_id FROM person_external_ids;
INSERT INTO entity_external_ids (entity_type, entity_id, external_id)
    SELECT 'studio', studio_id, external_id FROM studio_external_ids;

DROP INDEX IF EXISTS idx_person_external_ids_person;
DROP TABLE person_external_ids;
DROP INDEX IF EXISTS idx_studio_external_ids_studio;
DROP TABLE studio_external_ids;

-- An external id never outlives its entity. The table is polymorphic (no FK), so these
-- triggers do the cleanup the per-kind ON DELETE CASCADE used to -- the lesson 0022
-- recorded for entity_aliases. Merge paths repoint ids onto the survivor explicitly
-- before deleting the loser; these make raw/prune/orphan-sweep deletes safe regardless.
CREATE TRIGGER people_ad_external_ids  AFTER DELETE ON people  BEGIN
    DELETE FROM entity_external_ids WHERE entity_type = 'person' AND entity_id = old.id;
END;
CREATE TRIGGER studios_ad_external_ids AFTER DELETE ON studios BEGIN
    DELETE FROM entity_external_ids WHERE entity_type = 'studio' AND entity_id = old.id;
END;
CREATE TRIGGER tags_ad_external_ids    AFTER DELETE ON tags    BEGIN
    DELETE FROM entity_external_ids WHERE entity_type = 'tag'    AND entity_id = old.id;
END;
CREATE TRIGGER films_ad_external_ids   AFTER DELETE ON films   BEGIN
    DELETE FROM entity_external_ids WHERE entity_type = 'film'   AND entity_id = old.id;
END;
