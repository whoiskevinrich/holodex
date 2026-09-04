-- HOLODEX-306 (ADR-088 D2/D4/D6, spec F58 P0-1): alternate names collapse to one store.
--
-- Provider `also_known_as` values previously landed in entity_enrichment and rendered as a
-- display-only "Also known as" row: not searchable, and never consulted by the scanner's
-- name -> alias -> create resolution. This migration prepares entity_aliases to be the single
-- store for owner-typed and provider-supplied names alike.
--
-- Three parts, in order: the provenance column, the suppression table the delete path needs,
-- and a one-time promotion of the curation the retired display row accumulated.

-- 1. Provenance, not privilege. Empty for owner-authored -- which is every row that exists
--    today, and the DEFAULT keeps every current INSERT correct without touching its call site.
--    Otherwise the provider namespace ('tmdb'). Nothing filters on this column except the
--    suppression bookkeeping in part 2 and the source badge in the UI; in particular the scan
--    resolve predicate stays source-blind, which is the whole point of the collapse.
ALTER TABLE entity_aliases ADD COLUMN source TEXT NOT NULL DEFAULT '';

-- 2. Deleting a provider alias has to be durable, or deleting it was pointless: the enrich path
--    skips any candidate whose key is suppressed for that entity, so a re-enrich cannot
--    resurrect a name the owner removed.
--
--    Keyed per-entity deliberately. The alternative -- a soft-deleted tombstone row inside
--    entity_aliases -- would have held the globally-unique alias_key hostage and blocked a
--    legitimate claim by another entity. Suppressing a name on person 12 must leave person 40
--    free to receive or add it.
--
--    Deleting an *owner-authored* alias writes nothing here; nothing would re-add it.
CREATE TABLE entity_alias_suppressions (
    entity_type TEXT    NOT NULL,          -- 'person' | 'studio' | 'tag'
    entity_id   INTEGER NOT NULL,
    alias_key   TEXT    NOT NULL,          -- entity_aliases.alias_key's fold, copied below
    PRIMARY KEY (entity_type, entity_id, alias_key)
);

-- A suppression never outlives its entity, for the same reason and by the same mechanism as
-- the alias it suppresses (0022's people_ad_aliases / studios_ad_aliases / tags_ad_aliases).
-- This table is polymorphic too, so no FK can express it and nothing else would ever prune
-- these rows. All three entity tables use AUTOINCREMENT, so a deleted id is never reissued
-- and an orphan cannot be inherited by a later entity -- these triggers are about the rows
-- not accumulating unreachably, and about the invariant holding uniformly across the spine
-- rather than for aliases only.
CREATE TRIGGER people_ad_alias_suppressions  AFTER DELETE ON people  BEGIN
    DELETE FROM entity_alias_suppressions WHERE entity_type = 'person' AND entity_id = old.id;
END;
CREATE TRIGGER studios_ad_alias_suppressions AFTER DELETE ON studios BEGIN
    DELETE FROM entity_alias_suppressions WHERE entity_type = 'studio' AND entity_id = old.id;
END;
CREATE TRIGGER tags_ad_alias_suppressions    AFTER DELETE ON tags    BEGIN
    DELETE FROM entity_alias_suppressions WHERE entity_type = 'tag'    AND entity_id = old.id;
END;

-- 3. Promote, don't drop. metadata_curation rows against the retired 'aliases' field carry real
--    owner intent -- 'add' means "they are also called that", 'suppress' means "the provider is
--    wrong about that one" -- so both become first-class rows here. An owner who curated the old
--    display-only row keeps the result, now searchable and scan-routing.
--
--    `value` is the reliable source for both the alias text and the suppression key: it is
--    always present and non-empty (setCurationLocked trims and rejects empty). `norm_value` is
--    deliberately NOT used for the key -- it carries curationNorm()'s fold, which is not
--    entity_aliases.alias_key's fold, and conflating the two would change what collides.
--
--    INSERT OR IGNORE is load-bearing, not defensive: a curated name that another entity already
--    holds violates UNIQUE (entity_type, alias_key), and that single row must not abort the
--    migration and take the whole upgrade down with it. The colliding value is simply not
--    promoted.
--
--    The entity_type guard keeps a stray non-entity curation row (nothing maps 'aliases' onto
--    video today, but the table is entity-generic and nothing stops it) from being promoted into
--    a spine that has no such entity. Such a row is still deleted below -- the field is gone
--    either way -- it just isn't given a meaningless alias row first.
INSERT OR IGNORE INTO entity_aliases (entity_type, entity_id, alias, source)
SELECT entity_type, entity_id, value, ''
  FROM metadata_curation
 WHERE field_key = 'aliases'
   AND action = 'add'
   AND entity_type IN ('person', 'studio', 'tag');

INSERT OR IGNORE INTO entity_alias_suppressions (entity_type, entity_id, alias_key)
SELECT entity_type, entity_id,
       CASE WHEN entity_type = 'tag'
            THEN replace(lower(trim(value)), ' ', '')
            ELSE lower(trim(value)) END
  FROM metadata_curation
 WHERE field_key = 'aliases'
   AND action = 'suppress'
   AND entity_type IN ('person', 'studio', 'tag');

-- Every remaining 'aliases' curation row goes, including any 'nowrite' ones. That action means
-- "don't push this value to the file", which has no meaning once the field is out of the
-- registry and the value lives in the identity spine -- person aliases were never a writeback
-- target. Dropping them is intentional, not an oversight in the two SELECTs above.
DELETE FROM metadata_curation WHERE field_key = 'aliases';
