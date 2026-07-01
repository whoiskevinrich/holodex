-- F36 (ADR-051): per-field source-of-truth decisions. One row per standing,
-- owner-made decision about which source is *true* for a single replace (scalar)
-- field of one entity. The resolver consults this map before mapping order at read
-- time (pure re-interpretation, no I/O) — a decision short-circuits precedence and
-- drives both display and writeback.
--
--   source = 'file'            → the file/baseline layer is the truth (the default)
--   source = 'provider:<name>' → that matched provider's shadow value is the truth
--   source = 'manual'          → a frozen owner literal, stored in manual_value
--
-- The decision pins the *source*, not the value: a 'file'/'provider' decision follows
-- the live layer (a later refresh re-extract or re-enrich flows straight through);
-- only 'manual' is a frozen literal. entity_type is "video" in v1 (generalizes to
-- "person"/"studio" with no schema change). created_at is RFC3339 UTC, matching every
-- other timestamp column. The UNIQUE makes "set decision" an upsert and "clear" a
-- delete → back to the file-first default.
CREATE TABLE field_source_decisions (
    id            INTEGER PRIMARY KEY,
    entity_type   TEXT    NOT NULL,
    entity_id     INTEGER NOT NULL,
    field_key     TEXT    NOT NULL,
    source        TEXT    NOT NULL,
    manual_value  TEXT    NOT NULL DEFAULT '',
    created_at    TEXT    NOT NULL,
    UNIQUE (entity_type, entity_id, field_key)
);

CREATE INDEX idx_field_decisions_entity ON field_source_decisions(entity_type, entity_id);
