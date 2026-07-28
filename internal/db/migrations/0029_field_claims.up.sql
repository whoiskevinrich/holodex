-- F49 (ADR-074): claimed provider keys. One row states that a canonical field
-- *claims* a differently-named provider key — the key contributes its value as a
-- candidate source of that field and stops auto-registering as its own display-only
-- row (F39/ADR-056). It is the in-app half of a mechanism operators express in
-- metadata-mappings.yaml as a field's `sources:` list; both halves materialize into
-- the same mapping.Source before anything reads them (D2), so there is one
-- suppression path and no way for the two to disagree.
--
-- Scope is GLOBAL per (entity_type, provider, field_key), like field_promotions —
-- a claim is a statement about what a key *is*, which does not vary by entity.
--
-- The PRIMARY KEY carries `provider`, and that is the one place this table's grain
-- deliberately differs from field_promotions (D1). A promotion is keyed
-- (entity_type, field_key) because presentation is shared across providers; identity
-- is not: `provA:synopsis` and `provB:synopsis` are different assertions, and one row
-- must never speak for both. It is also what lets `provA:rating` (an age certificate)
-- be claimed while `provB:rating` (a 1-10 score) keeps its own row.
--
-- `canonical` is the whole payload: there is no precedence column (D3). A claim
-- appends at the END of the target field's candidate list, below every YAML source,
-- because adding a claim states identity and must never move the resolved winner —
-- precedence is ADR-051's per-entity source decisions, the instrument built for it.
-- Several claims on one canonical append sorted by (provider, field_key), so
-- resolution is reproducible from the table's contents rather than from edit history.
--
-- A claim whose `canonical` is absent from the effective field set is INERT and is
-- never pruned (D4): it appends nothing and — because suppression reads the
-- materialized field set, not this table — suppresses nothing either, so the key
-- simply auto-registers again exactly as it did pre-F49. Target absence is usually
-- transient (a YAML edit awaiting reload-config, a promotion about to be re-made),
-- and pruning on a transient state would destroy owner intent that was never
-- withdrawn. created_at/updated_at are RFC3339 UTC, matching every other ts column.
CREATE TABLE field_claims (
    entity_type TEXT NOT NULL,  -- 'video' | 'person' | 'studio'
    provider    TEXT NOT NULL,  -- enrichment namespace (entity_enrichment.provider)
    field_key   TEXT NOT NULL,  -- the non-canonical provider key being claimed (lower-cased)
    canonical   TEXT NOT NULL,  -- the field that claims it
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (entity_type, provider, field_key)
);
