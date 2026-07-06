-- F43 (ADR-061): unified entity name-identity — a per-entity normalized nameKey
-- (unique across canonical names AND aliases) plus a shared alias / keep-separate /
-- review-queue spine across Person, Studio, and Tag. This migration lays the spine
-- (S1): the polymorphic alias store (person_aliases folds into it), the shared FTS
-- mirror, the keep-separate + review-queue tables, an in-SQL fold of the exact-case
-- canonical duplicates, and the canonical nameKey unique indexes.
--
-- Ordering is load-bearing. Bootstrap applies migrations BEFORE the Go one-time
-- backfills (cmd/holodex/main.go), so the unique-index build here would fail on any
-- residual "fox"/"Fox" canonical duplicate. The fold therefore runs first, in SQL,
-- data-driven — correct for any duplicates, not just today's probe count (14 pairs).

-- ─── Spine: polymorphic alias store (person_aliases migrates in, RD11) ──────────────
-- alias_key is the per-entity normalized identity key (ADR-061 D2): person/studio fold
-- case + edge whitespace; tag also folds internal whitespace. Computing it as a STORED
-- generated column keeps the key byte-identical to the resolve predicate and the unique
-- index — Go never computes it, so Go/SQLite lower()/trim() can never disagree. UNIQUE
-- is per entity_type over the key: one alias key belongs to exactly one entity (RD1).
CREATE TABLE entity_aliases (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type TEXT    NOT NULL,          -- 'person' | 'studio' | 'tag'
    entity_id   INTEGER NOT NULL,
    alias       TEXT    NOT NULL,          -- original casing, for display + FTS
    alias_key   TEXT    NOT NULL GENERATED ALWAYS AS (
                    CASE WHEN entity_type = 'tag'
                         THEN replace(lower(trim(alias)), ' ', '')
                         ELSE lower(trim(alias)) END) STORED,
    UNIQUE (entity_type, alias_key)
);
-- entity lookup (alias list, merge, collision check) is on the scan write path; index it.
CREATE INDEX idx_entity_aliases_entity ON entity_aliases(entity_type, entity_id);

-- Shared external-content FTS mirror of entity_aliases.alias, kept in sync by triggers —
-- identical in shape to the retired person_aliases_fts. Global search filters entity_type
-- by joining back to entity_aliases (ADR-017/036, RD11). Diacritic folding is for SEARCH
-- only, never for identity (the identity key above deliberately does not fold diacritics).
CREATE VIRTUAL TABLE entity_aliases_fts USING fts5(
    alias,
    content='entity_aliases',
    content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
CREATE TRIGGER entity_aliases_ai AFTER INSERT ON entity_aliases BEGIN
    INSERT INTO entity_aliases_fts(rowid, alias) VALUES (new.id, new.alias);
END;
CREATE TRIGGER entity_aliases_ad AFTER DELETE ON entity_aliases BEGIN
    INSERT INTO entity_aliases_fts(entity_aliases_fts, rowid, alias) VALUES('delete', old.id, old.alias);
END;
CREATE TRIGGER entity_aliases_au AFTER UPDATE ON entity_aliases BEGIN
    INSERT INTO entity_aliases_fts(entity_aliases_fts, rowid, alias) VALUES('delete', old.id, old.alias);
    INSERT INTO entity_aliases_fts(rowid, alias) VALUES (new.id, new.alias);
END;

-- An alias never outlives its entity. entity_aliases is polymorphic (no FK), so these
-- triggers do the cleanup person_aliases' ON DELETE CASCADE used to — and extend it to
-- studio prune-on-empty (studios.go) and tag deletes. Merge paths also clean aliases
-- explicitly before deleting; these triggers make raw/prune deletes safe regardless.
CREATE TRIGGER people_ad_aliases  AFTER DELETE ON people  BEGIN
    DELETE FROM entity_aliases WHERE entity_type = 'person' AND entity_id = old.id;
END;
CREATE TRIGGER studios_ad_aliases AFTER DELETE ON studios BEGIN
    DELETE FROM entity_aliases WHERE entity_type = 'studio' AND entity_id = old.id;
END;
CREATE TRIGGER tags_ad_aliases    AFTER DELETE ON tags    BEGIN
    DELETE FROM entity_aliases WHERE entity_type = 'tag'    AND entity_id = old.id;
END;

-- ─── Spine: keep-separate (RD5) — the durable negative of an alias ───────────────────
-- "these two ids are deliberately distinct"; the near-miss detector and the queue never
-- re-propose a kept-separate pair. id_lo/id_hi are ordered so a pair is stored once.
CREATE TABLE entity_keep_separate (
    entity_type TEXT    NOT NULL,
    id_lo       INTEGER NOT NULL,          -- min(id_a, id_b)
    id_hi       INTEGER NOT NULL,          -- max(id_a, id_b)
    PRIMARY KEY (entity_type, id_lo, id_hi)
);

-- ─── Spine: near-miss review queue (P1) — seeded by the backfill, appended by scan ──
CREATE TABLE identity_review_queue (
    entity_type TEXT    NOT NULL,
    id_lo       INTEGER NOT NULL,
    id_hi       INTEGER NOT NULL,
    variation   TEXT    NOT NULL,          -- 'internal-whitespace' | 'punctuation' | …
    PRIMARY KEY (entity_type, id_lo, id_hi)
);

-- ─── Person conformance (RD11): fold person_aliases into the shared store ────────────
-- INSERT OR IGNORE: the new key domain is global-per-type (was per-person), so on the
-- rare chance two people share an alias key today the duplicate is dropped rather than
-- violating the unique constraint (the probe found no such tangle in the collision set).
INSERT OR IGNORE INTO entity_aliases (entity_type, entity_id, alias)
    SELECT 'person', person_id, alias FROM person_aliases;

DROP TRIGGER IF EXISTS person_aliases_ai;
DROP TRIGGER IF EXISTS person_aliases_ad;
DROP TRIGGER IF EXISTS person_aliases_au;
DROP TABLE person_aliases_fts;
DROP TABLE person_aliases;

-- ─── Fold exact-nameKey canonical duplicates (RD10 hard pairs), per entity ──────────
-- Survivor = lowest id in each nameKey group; loser name → alias; associations move
-- (de-duped union); the loser's polymorphic shadow rows are dropped (survivor keeps its
-- own, matching MergePersons); the loser is deleted (FK CASCADE clears its junction /
-- image / external-id / logo rows). Data-driven so the unique index below sees a clean
-- set on any database.

-- People
CREATE TEMP TABLE _fold_people AS
    SELECT p.id AS loser, p.name AS loser_name,
           (SELECT MIN(q.id) FROM people q WHERE lower(trim(q.name)) = lower(trim(p.name))) AS survivor
    FROM people p
    WHERE p.id > (SELECT MIN(q.id) FROM people q WHERE lower(trim(q.name)) = lower(trim(p.name)));
UPDATE OR IGNORE video_people
   SET person_id = (SELECT survivor FROM _fold_people WHERE loser = video_people.person_id)
 WHERE person_id IN (SELECT loser FROM _fold_people);
INSERT OR IGNORE INTO entity_aliases (entity_type, entity_id, alias)
    SELECT 'person', survivor, loser_name FROM _fold_people;
UPDATE OR IGNORE entity_aliases
   SET entity_id = (SELECT survivor FROM _fold_people WHERE loser = entity_aliases.entity_id)
 WHERE entity_type = 'person' AND entity_id IN (SELECT loser FROM _fold_people);
-- Any collision-residue loser aliases are cleaned by people_ad_aliases on the delete below.
DELETE FROM entity_enrichment      WHERE entity_type = 'person' AND entity_id IN (SELECT loser FROM _fold_people);
DELETE FROM field_source_decisions WHERE entity_type = 'person' AND entity_id IN (SELECT loser FROM _fold_people);
DELETE FROM metadata_curation      WHERE entity_type = 'person' AND entity_id IN (SELECT loser FROM _fold_people);
DELETE FROM people WHERE id IN (SELECT loser FROM _fold_people);
DROP TABLE _fold_people;

-- Studios
CREATE TEMP TABLE _fold_studios AS
    SELECT s.id AS loser, s.name AS loser_name,
           (SELECT MIN(q.id) FROM studios q WHERE lower(trim(q.name)) = lower(trim(s.name))) AS survivor
    FROM studios s
    WHERE s.id > (SELECT MIN(q.id) FROM studios q WHERE lower(trim(q.name)) = lower(trim(s.name)));
UPDATE OR IGNORE video_studios
   SET studio_id = (SELECT survivor FROM _fold_studios WHERE loser = video_studios.studio_id)
 WHERE studio_id IN (SELECT loser FROM _fold_studios);
UPDATE OR IGNORE studio_external_ids
   SET studio_id = (SELECT survivor FROM _fold_studios WHERE loser = studio_external_ids.studio_id)
 WHERE studio_id IN (SELECT loser FROM _fold_studios);
INSERT OR IGNORE INTO entity_aliases (entity_type, entity_id, alias)
    SELECT 'studio', survivor, loser_name FROM _fold_studios;
DELETE FROM entity_enrichment      WHERE entity_type = 'studio' AND entity_id IN (SELECT loser FROM _fold_studios);
DELETE FROM field_source_decisions WHERE entity_type = 'studio' AND entity_id IN (SELECT loser FROM _fold_studios);
DELETE FROM metadata_curation      WHERE entity_type = 'studio' AND entity_id IN (SELECT loser FROM _fold_studios);
DELETE FROM studios WHERE id IN (SELECT loser FROM _fold_studios);
DROP TABLE _fold_studios;

-- Tags (nameKey also folds internal whitespace)
CREATE TEMP TABLE _fold_tags AS
    SELECT t.id AS loser, t.name AS loser_name,
           (SELECT MIN(q.id) FROM tags q
             WHERE replace(lower(trim(q.name)), ' ', '') = replace(lower(trim(t.name)), ' ', '')) AS survivor
    FROM tags t
    WHERE t.id > (SELECT MIN(q.id) FROM tags q
             WHERE replace(lower(trim(q.name)), ' ', '') = replace(lower(trim(t.name)), ' ', ''));
UPDATE OR IGNORE video_tags
   SET tag_id = (SELECT survivor FROM _fold_tags WHERE loser = video_tags.tag_id)
 WHERE tag_id IN (SELECT loser FROM _fold_tags);
INSERT OR IGNORE INTO entity_aliases (entity_type, entity_id, alias)
    SELECT 'tag', survivor, loser_name FROM _fold_tags;
DELETE FROM tags WHERE id IN (SELECT loser FROM _fold_tags);
DROP TABLE _fold_tags;

-- ─── Canonical nameKey uniqueness (replaces binary UNIQUE(name), RD1/P0-1) ───────────
CREATE UNIQUE INDEX ux_people_namekey  ON people  (lower(trim(name)));
CREATE UNIQUE INDEX ux_studios_namekey ON studios (lower(trim(name)));
CREATE UNIQUE INDEX ux_tags_namekey    ON tags    (replace(lower(trim(name)), ' ', ''));
