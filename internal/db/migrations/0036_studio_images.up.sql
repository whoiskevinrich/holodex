-- F51 (ADR-079): generalizes the studio logo cache (studio_logos, migration 0020) into
-- three named, owner-editable image roles — 'icon' (studios list), 'logo' (detail page,
-- today's usage), 'poster' (no consumer yet). Unlike person_images (migration 0009),
-- every role here is core/single-slot: there is no gallery, so the unique index is a
-- plain composite, not partial, and there is no sort_order or 'promoted' source (nothing
-- to promote from without a gallery). Like every other image store, bytes live on disk at
-- DATA_PATH/studio-images/{studio_id}/{id}.jpg (ADR-014) and never in the DB; this table
-- is only the metadata index. ON DELETE CASCADE means an image never outlives its studio.
CREATE TABLE studio_images (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    studio_id   INTEGER NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    role        TEXT    NOT NULL,            -- 'icon' | 'logo' | 'poster'
    source      TEXT    NOT NULL,            -- 'upload' | 'enrichment'
    provider    TEXT    NOT NULL DEFAULT '',
    external_id TEXT    NOT NULL DEFAULT '',
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    byte_size   INTEGER NOT NULL,
    created_at  TEXT    NOT NULL
);

-- One image per (studio, role) — the single-slot invariant as a DB constraint. A
-- replace is delete + insert, so the row id changes -> the ?v={id} cache-buster changes.
CREATE UNIQUE INDEX idx_studio_images_slot ON studio_images(studio_id, role);
CREATE INDEX idx_studio_images_studio ON studio_images(studio_id);

-- Carry forward existing studio_logos rows as the studio's 'logo' role before dropping
-- the old table (RD3 in the F51 spec: studio_images replaces studio_logos outright, not
-- a coexisting second table). provider is preserved; external_id has no studio_logos
-- analogue and defaults to ''.
INSERT INTO studio_images (studio_id, role, source, provider, external_id, width, height, byte_size, created_at)
SELECT studio_id, 'logo', 'enrichment', provider, '', width, height, byte_size, created_at
FROM studio_logos;

DROP TABLE studio_logos;

-- The studio `logo` field is retired (ADR-079 RD4/§3): the resolved-field/decision model
-- is replaced by the asset-slot model above, so any standing decision on it is dead
-- weight, not a state this migration can meaningfully translate.
DELETE FROM field_source_decisions WHERE entity_type = 'studio' AND field_key = 'logo';
