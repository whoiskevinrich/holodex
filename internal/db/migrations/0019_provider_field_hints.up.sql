-- HOLODEX-128 (F39, ADR-056): persist a provider's advertised per-field render hints
-- (label / render mode / ordering group) so the read path can render its non-canonical
-- fields first-class WITHOUT contacting the provider. The manifest (GET /describe) is
-- read only during owner actions; this table is the durable copy the person/studio/
-- media detail GETs consult.
--
-- Keyed by (provider, field_key): hints are per-provider, per-key — NOT per entity —
-- so they are stored once, not denormalized across every entity's enrichment rows.
-- The whole provider's rows are replaced (delete-then-insert) each time /describe is
-- read, so the table always reflects the provider's current manifest.
--
-- Only NON-CANONICAL keys are stored (a provider may not relabel a canonical field,
-- and `_`-prefixed reserved sidecars never display — both dropped on ingest). The
-- render/group values are normalized to the internal/registry vocabulary before write.
CREATE TABLE provider_field_hints (
    provider   TEXT    NOT NULL,
    field_key  TEXT    NOT NULL,
    label      TEXT    NOT NULL DEFAULT '',
    render     TEXT    NOT NULL DEFAULT '',
    hint_group TEXT    NOT NULL DEFAULT '',
    ord        INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT    NOT NULL,
    PRIMARY KEY (provider, field_key)
);
