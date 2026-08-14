-- ADR-081 D5 (rename) + ADR-082 (value shape, supersedes D5's bare-value
-- assumption): imdb_id -> external_provider_id, and every stored value for
-- the field becomes namespace-qualified ("<provider>:<id>") rather than a
-- bare id string, since production enrichment is not limited to one
-- provider (ADR-033's declared-not-compiled-in provider registry). A value
-- that already contains ":" is left untouched (already namespace-qualified,
-- or from a non-IMDb provider that got here ahead of this migration); an
-- empty value is left untouched. Any bare legacy value can only have come
-- from the field's sole historical meaning (IMDb), so it is prefixed
-- "imdb:". No schema change: field_key/equivalent columns are already
-- untyped TEXT, per D5.
--
-- Value columns are rewritten BEFORE the field_key rename in each table, so
-- the "field_key = 'imdb_id'" filter still identifies the right rows.
--
-- field_claims is the one table in this list keyed by `canonical`, not
-- `field_key` (F49/ADR-074) -- it records which canonical field a claimed
-- provider key feeds, not a value, so there is nothing to namespace-qualify.
--
-- provider_field_hints and field_promotions structurally never hold a
-- canonical key's field_key (both are scoped to non-canonical shadow keys
-- by their own tables' rules) -- their UPDATEs below are no-ops today, kept
-- only so this migration mirrors D5's full nine-table list exactly.

UPDATE entity_enrichment
SET value = 'imdb:' || value
WHERE field_key = 'imdb_id' AND value != '' AND instr(value, ':') = 0;
UPDATE entity_enrichment SET field_key = 'external_provider_id' WHERE field_key = 'imdb_id';

UPDATE metadata_curation
SET value = CASE WHEN value != '' AND instr(value, ':') = 0 THEN 'imdb:' || value ELSE value END,
    norm_value = CASE WHEN norm_value != '' AND instr(norm_value, ':') = 0 THEN 'imdb:' || norm_value ELSE norm_value END
WHERE field_key = 'imdb_id';
UPDATE metadata_curation SET field_key = 'external_provider_id' WHERE field_key = 'imdb_id';

UPDATE metadata_extraction_review
SET filename_value = CASE WHEN filename_value != '' AND instr(filename_value, ':') = 0 THEN 'imdb:' || filename_value ELSE filename_value END,
    tag_value = CASE WHEN tag_value != '' AND instr(tag_value, ':') = 0 THEN 'imdb:' || tag_value ELSE tag_value END
WHERE field_key = 'imdb_id';
UPDATE metadata_extraction_review SET field_key = 'external_provider_id' WHERE field_key = 'imdb_id';

UPDATE file_writeback_snapshots
SET prior_value = 'imdb:' || prior_value
WHERE field_key = 'imdb_id' AND prior_value != '' AND instr(prior_value, ':') = 0;
UPDATE file_writeback_snapshots SET field_key = 'external_provider_id' WHERE field_key = 'imdb_id';

UPDATE file_writebacks
SET value = 'imdb:' || value
WHERE field_key = 'imdb_id' AND value != '' AND instr(value, ':') = 0;
UPDATE file_writebacks SET field_key = 'external_provider_id' WHERE field_key = 'imdb_id';

UPDATE field_source_decisions
SET manual_value = 'imdb:' || manual_value
WHERE field_key = 'imdb_id' AND manual_value != '' AND instr(manual_value, ':') = 0;
UPDATE field_source_decisions SET field_key = 'external_provider_id' WHERE field_key = 'imdb_id';

UPDATE provider_field_hints SET field_key = 'external_provider_id' WHERE field_key = 'imdb_id';
UPDATE field_promotions SET field_key = 'external_provider_id' WHERE field_key = 'imdb_id';

UPDATE field_claims SET canonical = 'external_provider_id' WHERE canonical = 'imdb_id';
