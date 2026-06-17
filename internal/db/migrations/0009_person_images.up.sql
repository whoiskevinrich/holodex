-- F25 (ADR-038): per-person images (headshot / banner / poster + a gallery of
-- extras). Like thumbnails (migration 0002), the image BYTES live on disk at
-- DATA_PATH/person-images/{person_id}/{id}.jpg (ADR-014) and never in the DB; this
-- table is only the metadata index the API serves and orders from.
--
-- role is one of 'headshot' | 'banner' | 'poster' | 'extra'. source records how the
-- row arrived ('upload' | 'enrichment' | 'promoted') for provenance, mirroring the
-- entity_enrichment separation. provider/external_id are populated only for
-- enrichment-sourced rows (the upstream record the bytes came from). width/height/
-- byte_size describe the stored (re-encoded) JPEG. sort_order positions a row within
-- its person's gallery. created_at is RFC3339 UTC, matching every other timestamp.
-- ON DELETE CASCADE means an image never outlives its person.
CREATE TABLE person_images (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id   INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role        TEXT    NOT NULL,
    source      TEXT    NOT NULL,
    provider    TEXT    NOT NULL DEFAULT '',
    external_id TEXT    NOT NULL DEFAULT '',
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    byte_size   INTEGER NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL
);

-- A person has at most one image per core role (headshot/banner/poster); the
-- 'extra' gallery is unbounded by this index (the 20-cap is enforced in the repo
-- transaction). The partial unique index lets a replace be "delete + insert".
CREATE UNIQUE INDEX idx_person_images_core_slot
    ON person_images(person_id, role) WHERE role <> 'extra';
-- All of a person's images are fetched together (detail view + gallery ordering).
CREATE INDEX idx_person_images_person ON person_images(person_id);
