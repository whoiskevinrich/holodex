-- HOLODEX-122 (ADR-054): give a studio entity a stable provider identifier so
-- enrichment can be refreshed deterministically, videos can hint their studio's
-- provider identity, and same-company duplicate spellings converge to one entity.
--
-- A join table, never a column on `studios` (mirrors person_aliases, F32 shape): a
-- studio may carry ids from multiple providers. external_id is namespace-qualified
-- ("tmdb:174") and is the PRIMARY KEY, so it is GLOBALLY UNIQUE — one company id maps
-- to exactly one studio, which is the de-dup guarantee. resolveOrCreateStudio consults
-- it BEFORE exact name (ADR-054 §4), so "Warner Bros." and "Warner Bros. Pictures"
-- that share TMDB id 174 resolve to the same studio.
--
-- ON DELETE CASCADE ties the id rows to their studio: ADR-053 prune-on-empty (delete a
-- studio with zero video_studios links) drops the id rows with it, so there are no
-- orphan ids. The id re-attaches on the next derivation from the persisted per-video
-- `_studio_external_ids` sidecar enrichment. video_studios is still the derived index;
-- this table only records identity, never links.
CREATE TABLE studio_external_ids (
    studio_id   INTEGER NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    external_id TEXT    NOT NULL,
    PRIMARY KEY (external_id)
);

-- Reverse lookup (a studio's ids) and the cascade path; the external_id lookup is
-- already served by the PK.
CREATE INDEX idx_studio_external_ids_studio ON studio_external_ids(studio_id);
