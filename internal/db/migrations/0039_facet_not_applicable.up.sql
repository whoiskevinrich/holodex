-- F55 (ADR-081 D2): owner-asserted "this facet doesn't apply to this entity"
-- exclusion for the entity completeness score. A row's existence *is* the
-- fact — no other columns. Mirrors person_image_suppressions (0012), the
-- closest existing shape for an owner-asserted exclusion with no value of its
-- own. Deliberately not a 4th field_source_decisions.source value: that
-- table's UNIQUE key and every reader assume every row names a live source of
-- truth, which not-applicable has none of. The completeness scorer checks
-- this table first and short-circuits before consulting field_source_decisions
-- for the same field — the two tables answer different questions and can't
-- contradict. entity_type/entity_id follow the same untyped (no FK)
-- convention as field_source_decisions, entity-generic from day one
-- (video/person/studio), unlike 0012's person-only shape. created_at is
-- RFC3339 UTC, matching every other timestamp column.
CREATE TABLE facet_not_applicable (
    entity_type     TEXT    NOT NULL,
    entity_id       INTEGER NOT NULL,
    canonical_field TEXT    NOT NULL,
    created_at      TEXT    NOT NULL,
    PRIMARY KEY (entity_type, entity_id, canonical_field)
);
