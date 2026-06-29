-- F34 (ADR-050): deduplicate enrichment photos by image CONTENT, not URL. Each
-- person image gains a content_hash — the hex sha256 of the stored (normalized)
-- JPEG bytes — so identity is the picture Holodex actually serves, independent of
-- the source URL, provider, or original encoding. An enrichment gallery 'extra'
-- whose hash already exists for the person (any role) is skipped at ingest, so a
-- re-enrich, a chatty provider, or a second source can't pile the same photo into
-- the gallery again.
--
-- The column is additive with a '' default: pre-existing rows are unhashed until a
-- one-time Go startup backfill (assets.go can't sha256 on-disk bytes from SQL) fills
-- them and collapses any already-duplicated extras. Uniqueness is enforced in the
-- repo layer, NOT here: the hash is computed in Go, core roles legitimately repeat a
-- hash across a single-slot replace / the F25.29 poster seed, and the backfill must
-- collapse dupes before any constraint could hold. The index is just a fast lookup
-- for the (person_id, content_hash) existence check.
ALTER TABLE person_images ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_person_images_hash ON person_images(person_id, content_hash);
