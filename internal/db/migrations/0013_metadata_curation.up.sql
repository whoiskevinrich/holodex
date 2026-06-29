-- F30 (ADR-048): value-level metadata curation. One row per owner decision on a
-- single value of a single field. The resolver merges these with file + enrichment
-- sources at read time (pure re-interpretation, no I/O).
--
--   action = 'add'      → owner-added manual value (display form in `value`)
--   action = 'suppress' → tombstone: hide everywhere + never write (by norm_value)
--   action = 'nowrite'  → shown in Holodex but excluded from file writeback
--
-- norm_value is the dedup/match key (trim + case-fold); suppress/nowrite match on it
-- so an owner's decision survives a later re-scan/re-enrich that re-supplies the
-- value from any source. entity_type is "video" in v1 (generalizes to "person").
-- created_at is RFC3339 UTC, matching every other timestamp column.
CREATE TABLE metadata_curation (
    id          INTEGER PRIMARY KEY,
    entity_type TEXT    NOT NULL,
    entity_id   INTEGER NOT NULL,
    field_key   TEXT    NOT NULL,
    norm_value  TEXT    NOT NULL,
    value       TEXT    NOT NULL DEFAULT '',
    action      TEXT    NOT NULL,
    source      TEXT    NOT NULL DEFAULT 'manual',
    created_at  TEXT    NOT NULL,
    UNIQUE (entity_type, entity_id, field_key, norm_value, action)
);

CREATE INDEX idx_metadata_curation_entity ON metadata_curation(entity_type, entity_id);
