-- F44 (ADR-062): in-app field promotion. One row promotes a single non-canonical
-- shadow field key for an entity *type* into a first-class, curatable field — an
-- owner-authored presentation override the resolver consults as a new tier-0, above
-- operator metadata-mappings.yaml (D3). It is the only remap surface person/studio have
-- at all (they have no YAML), and the in-app path video never had.
--
-- Scope is GLOBAL per (entity_type, field_key): the presentation (label/render/group/
-- order) is shared across every entity of that type that has the key. Value curation
-- stays PER-ENTITY on field_source_decisions / metadata_curation (keyed by field_key +
-- entity_id) — there is no per-entity presentation row here.
--
-- Empty presentation columns INHERIT from the lower tiers (provider hint → title-case),
-- so a promotion whose only purpose is "make this curatable" need not restate the label.
-- The row carries presentation only; the field's F36/F30 candidate sources are derived
-- at resolve time from the entity's shadow provenance (D-candidate) — no source list is
-- stored. created_at/updated_at are RFC3339 UTC, matching every other timestamp column.
-- The PRIMARY KEY makes "set promotion" an upsert and "de-promote" a delete → back to the
-- F39 auto-registered, display-only state.
CREATE TABLE field_promotions (
    entity_type TEXT    NOT NULL,               -- 'video' | 'person' | 'studio'
    field_key   TEXT    NOT NULL,               -- the non-canonical shadow key (lower-cased)
    label       TEXT    NOT NULL DEFAULT '',    -- '' ⇒ inherit tier-3/4 label
    render      TEXT    NOT NULL DEFAULT '',    -- '' ⇒ inherit; else text|long_text|chips|url|image_url
    hint_group  TEXT    NOT NULL DEFAULT '',    -- '' ⇒ inherit; else primary|attributes|extended
    ord         INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL,
    PRIMARY KEY (entity_type, field_key)
);
