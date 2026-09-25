-- ============================================================================
-- Entity collision probe  —  READ-ONLY, ANONYMIZED OUTPUT.
-- Emits counts / classifications / key-lengths only: no names, ids, keys, or
-- external-id values, and no provider names. Output is safe to paste/share
-- as-is (scripts/CLAUDE.md).
--
--   sqlite3 -readonly /path/to/holodex.db ".read detect_entity_collisions.sql"
--
-- Covers case/whitespace collisions (Tier A, migration blockers) and
-- punctuation/spacing near-misses (Tier B, review-queue candidates) across
-- People (canonical ∪ alias), Studios, Tags and Films.
--
-- FILM KEYS ON (nameKey, year), not nameKey alone: ux_films_namekey, migration
-- 0047, per ADR-096 D3 — two films sharing a title across different years are
-- legal (Superman II 1980 / 2006). So film carries a `ykey` through Tier A and
-- Tier B, which for every other kind is the constant '' and changes nothing.
-- Grouping film by name alone would report every remake as a hard collision.
--
-- Two film-specific wrinkles the generic tiers cannot express, so §FILM below
-- handles them:
--   * A NULL year is DISTINCT from every other NULL under a SQLite unique
--     index, so two films with the same title and NO year satisfy both
--     constraints and can coexist — a real duplicate the index cannot stop.
--   * queueFilmSameTitle (internal/repo/films.go) queues same-title pairs under
--     ANY year as the non-fuzzy 'same-title' variation. That is a tracked
--     signal, not noise, and it is invisible to a (nameKey, year) grouping.
--
-- Verified 2026-09-23 against a database built by applying all 50 up migrations
-- to an empty file, seeded with: Superman II 1980 + 2006 (same title, both
-- years known — must NOT be a Tier A collision); two films titled Ghost with
-- NULL years (must BE one, since the unique index treats NULLs as distinct);
-- Blade Runner vs Blade␣␣Runner in 1982 (Tier B internal-whitespace); Alien
-- 1979 + Alien with no year (same title, one year unknown); plus a person and a
-- tag near-miss confirming the ykey column leaves the other three kinds' output
-- byte-for-byte unchanged.
-- ============================================================================

.mode box
.headers on

CREATE TEMP VIEW keyed AS
SELECT
  entity, id, name, kind, ykey,
  lower(trim(name)) AS hkey,
  trim(name)        AS tname,
  replace(replace(replace(replace(replace(replace(replace(replace(replace(
  replace(replace(replace(replace(replace(replace(
    lower(trim(name)),
    ' ',''), '.',''), ',',''), '''',''), '-',''), '–',''), '—',''),
    '&',''), '(',''), ')',''), ':',''), '!',''), '?',''), '/',''), '’','') AS lkey
FROM (
  SELECT 'person' AS entity, id        AS id, name  AS name, 'canonical' AS kind, '' AS ykey FROM people
  UNION ALL SELECT 'person', entity_id, alias, 'alias', '' FROM entity_aliases WHERE entity_type = 'person'
  UNION ALL SELECT 'studio', id,        name,  'canonical', '' FROM studios
  UNION ALL SELECT 'studio', entity_id, alias, 'alias', '' FROM entity_aliases WHERE entity_type = 'studio'
  UNION ALL SELECT 'tag',    id,        name,  'canonical', '' FROM tags
  UNION ALL SELECT 'tag',    entity_id, alias, 'alias', '' FROM entity_aliases WHERE entity_type = 'tag'
  -- A film alias carries no year of its own; it inherits the film's, which is
  -- what makes an alias comparable to a canonical title under the same key.
  UNION ALL SELECT 'film',   id,        name,  'canonical', coalesce(CAST(year AS TEXT), '(none)') FROM films
  UNION ALL SELECT 'film',   a.entity_id, a.alias, 'alias', coalesce(CAST(f.year AS TEXT), '(none)')
              FROM entity_aliases a JOIN films f ON f.id = a.entity_id WHERE a.entity_type = 'film'
);

-- ── SUMMARY: the counts ─────────────────────────────────────────────────────
SELECT '=== SUMMARY (counts only) ===' AS "";
WITH totals AS (
  SELECT entity, count(DISTINCT id) AS entities_total FROM keyed GROUP BY entity
),
hardg  AS (
  SELECT entity, hkey, ykey, count(DISTINCT id) AS n
  FROM keyed GROUP BY entity, hkey, ykey HAVING count(DISTINCT id) > 1
),
harda  AS (
  SELECT entity, count(*) AS hard_groups, sum(n) AS entities_in_hard FROM hardg GROUP BY entity
),
looseb AS (
  SELECT entity, count(*) AS loose_groups FROM
    (SELECT entity, lkey, ykey FROM keyed GROUP BY entity, lkey, ykey HAVING count(DISTINCT hkey) > 1)
  GROUP BY entity
)
SELECT t.entity,
       t.entities_total,
       coalesce(h.hard_groups, 0)      AS hard_collision_groups,
       coalesce(h.entities_in_hard, 0) AS entities_in_hard_collisions,
       coalesce(l.loose_groups, 0)     AS nearmiss_groups
FROM totals t
LEFT JOIN harda  h USING (entity)
LEFT JOIN looseb l USING (entity)
ORDER BY t.entity;

-- ── TIER A: case/whitespace collisions (anonymized) ─────────────────────────
-- namespaces shows whether a canonical name collides with an alias.
-- case_variation / edge_whitespace tell you WHICH axis drove the collision.
SELECT '=== TIER A: case/whitespace collisions (migration blockers) ===' AS "";
SELECT
  row_number() OVER (ORDER BY entity, hkey, ykey)                       AS grp,
  entity,
  count(DISTINCT id)                                                    AS distinct_entities,
  group_concat(DISTINCT kind)                                          AS namespaces,
  CASE WHEN count(DISTINCT tname) > 1 THEN 'yes' ELSE 'no' END          AS case_variation,
  CASE WHEN count(DISTINCT name) > count(DISTINCT tname) THEN 'yes' ELSE 'no' END AS edge_whitespace,
  length(hkey)                                                         AS key_len,
  CASE WHEN entity = 'film' AND ykey = '(none)' THEN 'both years NULL'
       WHEN entity = 'film' THEN 'same year'
       ELSE '' END                                                     AS film_year
FROM keyed
GROUP BY entity, hkey, ykey
HAVING count(DISTINCT id) > 1
ORDER BY entity, distinct_entities DESC;

-- ── TIER B: punctuation/spacing near-misses (anonymized) ────────────────────
-- variation classifies the difference: pure internal whitespace vs punctuation.
SELECT '=== TIER B: punctuation/spacing near-misses (review-queue candidates) ===' AS "";
SELECT
  row_number() OVER (ORDER BY entity, lkey, ykey)                      AS grp,
  entity,
  count(DISTINCT hkey)                                                 AS distinct_forms,
  count(DISTINCT id)                                                   AS distinct_entities,
  CASE WHEN count(DISTINCT replace(hkey, ' ', '')) = 1
       THEN 'internal-whitespace' ELSE 'punctuation/other' END         AS variation,
  length(lkey)                                                         AS key_len
FROM keyed
GROUP BY entity, lkey, ykey
HAVING count(DISTINCT hkey) > 1
ORDER BY entity, distinct_forms DESC;

-- ── FILM: same title, DIFFERENT year — the 'same-title' variation ───────────
-- Invisible to Tier A/B, which group by (name, year) precisely so a remake is
-- not called a collision. queueFilmSameTitle queues these anyway, under any
-- year, as a non-fuzzy pair the owner must settle — so they are counted here.
-- `one year unknown` is the actionable bucket: FillFilmYear (F59/ADR-089 D3)
-- can fill a missing year from a provider, which either splits the pair
-- cleanly or confirms it.
SELECT '=== FILM: same title, different year (queueFilmSameTitle candidates) ===' AS "";
WITH fpair AS (
  SELECT DISTINCT
    min(a.id, b.id) AS lo, max(a.id, b.id) AS hi,
    CASE WHEN a.ykey = '(none)' OR b.ykey = '(none)' THEN 'one year unknown'
         ELSE 'both years known' END AS year_shape
  FROM keyed a
  JOIN keyed b ON a.entity = 'film' AND b.entity = 'film'
              AND a.hkey = b.hkey AND a.id < b.id AND a.ykey <> b.ykey
)
SELECT year_shape,
       count(*) AS pairs,
       sum(EXISTS (SELECT 1 FROM identity_review_queue q
                    WHERE q.entity_type = 'film' AND q.id_lo = fpair.lo AND q.id_hi = fpair.hi))
                 AS already_queued,
       sum(EXISTS (SELECT 1 FROM entity_keep_separate k
                    WHERE k.entity_type = 'film' AND k.id_lo = fpair.lo AND k.id_hi = fpair.hi))
                 AS kept_separate
FROM fpair GROUP BY year_shape ORDER BY pairs DESC;

-- ── STUDIO refinement (anonymized): name collision vs external-id evidence ──
-- distinct_external_id_sets > 1 ⇒ probably different real companies (keep apart).
SELECT '=== STUDIO refinement: name collisions vs external-id evidence ===' AS "";
-- studio_external_ids was folded into the polymorphic entity_external_ids and
-- DROPPED by migration 0046 (ADR-096 D2), so this section errored on any
-- current database until it was repointed.
WITH s AS (
  SELECT s.id, lower(trim(s.name)) AS hkey,
         (SELECT group_concat(se.external_id) FROM entity_external_ids se
           WHERE se.entity_type = 'studio' AND se.entity_id = s.id) AS ext
  FROM studios s
)
SELECT
  row_number() OVER (ORDER BY hkey) AS grp,
  count(*)                          AS n_studios,
  count(DISTINCT ext)               AS distinct_external_id_sets,
  CASE WHEN count(DISTINCT ext) > 1
       THEN 'different ext ids -> likely NOT a dupe (keep separate)'
       ELSE 'no conflicting ids -> likely a real dupe (safe to merge)' END AS verdict
FROM s
GROUP BY hkey
HAVING count(*) > 1
ORDER BY n_studios DESC;

DROP VIEW keyed;
