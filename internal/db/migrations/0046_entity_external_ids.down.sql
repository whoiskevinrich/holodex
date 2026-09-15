-- Restore the per-kind tables (0018 / 0038 shapes) from the polymorphic rows. Film and
-- tag ids have no pre-0046 home and are dropped with the table.
CREATE TABLE person_external_ids (
    person_id   INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    external_id TEXT    NOT NULL,
    PRIMARY KEY (external_id)
);
CREATE INDEX idx_person_external_ids_person ON person_external_ids(person_id);
CREATE TABLE studio_external_ids (
    studio_id   INTEGER NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    external_id TEXT    NOT NULL,
    PRIMARY KEY (external_id)
);
CREATE INDEX idx_studio_external_ids_studio ON studio_external_ids(studio_id);

INSERT INTO person_external_ids (person_id, external_id)
    SELECT entity_id, external_id FROM entity_external_ids WHERE entity_type = 'person';
INSERT INTO studio_external_ids (studio_id, external_id)
    SELECT entity_id, external_id FROM entity_external_ids WHERE entity_type = 'studio';

DROP TRIGGER IF EXISTS people_ad_external_ids;
DROP TRIGGER IF EXISTS studios_ad_external_ids;
DROP TRIGGER IF EXISTS tags_ad_external_ids;
DROP TRIGGER IF EXISTS films_ad_external_ids;
DROP INDEX IF EXISTS idx_entity_external_ids_entity;
DROP TABLE entity_external_ids;
