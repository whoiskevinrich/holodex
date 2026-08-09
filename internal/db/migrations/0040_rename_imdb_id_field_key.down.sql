-- Reverse of 0040.up: rename external_provider_id back to imdb_id and strip a
-- leading "imdb:" namespace prefix from values, but ONLY for rows whose value
-- is empty or already "imdb:"-prefixed -- those are the rows this migration's
-- up side could have produced from genuine legacy imdb_id data. A row
-- namespace-qualified under a different provider (e.g. "tmdb:603") has no
-- imdb_id-era equivalent to revert to (ADR-082) and is deliberately left
-- under external_provider_id, un-renamed -- this down is a best-effort
-- reversal of this migration's own effect, not a guarantee that every
-- external_provider_id row predates it.

UPDATE entity_enrichment
SET value = CASE WHEN value LIKE 'imdb:%' THEN substr(value, 6) ELSE value END,
    field_key = 'imdb_id'
WHERE field_key = 'external_provider_id' AND (value = '' OR value LIKE 'imdb:%');

UPDATE metadata_curation
SET value = CASE WHEN value LIKE 'imdb:%' THEN substr(value, 6) ELSE value END,
    norm_value = CASE WHEN norm_value LIKE 'imdb:%' THEN substr(norm_value, 6) ELSE norm_value END,
    field_key = 'imdb_id'
WHERE field_key = 'external_provider_id'
  AND (value = '' OR value LIKE 'imdb:%')
  AND (norm_value = '' OR norm_value LIKE 'imdb:%');

UPDATE metadata_extraction_review
SET filename_value = CASE WHEN filename_value LIKE 'imdb:%' THEN substr(filename_value, 6) ELSE filename_value END,
    tag_value = CASE WHEN tag_value LIKE 'imdb:%' THEN substr(tag_value, 6) ELSE tag_value END,
    field_key = 'imdb_id'
WHERE field_key = 'external_provider_id'
  AND (filename_value = '' OR filename_value LIKE 'imdb:%')
  AND (tag_value = '' OR tag_value LIKE 'imdb:%');

UPDATE file_writeback_snapshots
SET prior_value = CASE WHEN prior_value LIKE 'imdb:%' THEN substr(prior_value, 6) ELSE prior_value END,
    field_key = 'imdb_id'
WHERE field_key = 'external_provider_id' AND (prior_value = '' OR prior_value LIKE 'imdb:%');

UPDATE file_writebacks
SET value = CASE WHEN value LIKE 'imdb:%' THEN substr(value, 6) ELSE value END,
    field_key = 'imdb_id'
WHERE field_key = 'external_provider_id' AND (value = '' OR value LIKE 'imdb:%');

UPDATE field_source_decisions
SET manual_value = CASE WHEN manual_value LIKE 'imdb:%' THEN substr(manual_value, 6) ELSE manual_value END,
    field_key = 'imdb_id'
WHERE field_key = 'external_provider_id' AND (manual_value = '' OR manual_value LIKE 'imdb:%');

UPDATE provider_field_hints SET field_key = 'imdb_id' WHERE field_key = 'external_provider_id';
UPDATE field_promotions SET field_key = 'imdb_id' WHERE field_key = 'external_provider_id';

UPDATE field_claims SET canonical = 'imdb_id' WHERE canonical = 'external_provider_id';
