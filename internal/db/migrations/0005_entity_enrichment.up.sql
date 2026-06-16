-- F22 (ADR-033): shadow layer for plugin-sourced metadata. Kept separate from the
-- file-extracted tables (videos / people / video_metadata) so enrichment is purely
-- additive — a re-scan rebuilds the file-sourced rows and never touches these. One
-- row per (entity, provider, canonical field); multi-valued fields store their
-- values newline-joined and are split on read (mirrors the mapping resolver).
--
-- entity_type is "person" in v1 (ADR-033 People slice); "series"/"video" reuse the
-- same table when the design generalizes — no schema change. external_id records
-- the matched upstream record so a re-enrich skips identity (F22.4b). fetched_at is
-- RFC3339 UTC, matching every other timestamp column.
CREATE TABLE entity_enrichment (
    id          INTEGER PRIMARY KEY,
    entity_type TEXT    NOT NULL,
    entity_id   INTEGER NOT NULL,
    provider    TEXT    NOT NULL,
    field_key   TEXT    NOT NULL,
    value       TEXT    NOT NULL,
    external_id TEXT    NOT NULL DEFAULT '',
    fetched_at  TEXT    NOT NULL,
    UNIQUE (entity_type, entity_id, provider, field_key)
);

CREATE INDEX idx_entity_enrichment_lookup ON entity_enrichment (entity_type, entity_id);
