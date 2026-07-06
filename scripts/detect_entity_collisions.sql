-- ============================================================================
-- Entity collision probe  —  READ-ONLY, ANONYMIZED OUTPUT.
-- Emits counts / classifications / key-lengths only: no names, ids, keys, or
-- external-id values. Output is safe to paste/share.
--
--   sqlite3 -readonly /path/to/holodex.db ".read detect_entity_collisions.sql"
--
-- Covers case/whitespace collisions (Tier A, migration blockers) and
-- punctuation/spacing near-misses (Tier B, review-queue candidates) across
-- People (canonical ∪ alias), Studios, and Tags.
-- ============================================================================

.mode box
.headers on

CREATE TEMP VIEW keyed AS
SELECT
  entity, id, name, kind,
  lower(trim(name)) AS hkey,
  trim(name)        AS tname,
  replace(replace(replace(replace(replace(replace(replace(replace(replace(
  replace(replace(replace(replace(replace(replace(
    lower(trim(name)),
    ' ',''), '.',''), ',',''), '''',''), '-',''), '–',''), '—',''),
    '&',''), '(',''), ')',''), ':',''), '!',''), '?',''), '/',''), '’','') AS lkey
FROM (
  SELECT 'person' AS entity, id        AS id, name  AS name, 'canonical' AS kind FROM people
  UNION ALL SELECT 'person', person_id, alias, 'alias'     FROM person_aliases
  UNION ALL SELECT 'studio', id,        name,  'canonical' FROM studios
  UNION ALL SELECT 'tag',    id,        name,  'canonical' FROM tags
);

-- ── SUMMARY: the counts ─────────────────────────────────────────────────────
SELECT '=== SUMMARY (counts only) ===' AS "";
WITH totals AS (
  SELECT entity, count(DISTINCT id) AS entities_total FROM keyed GROUP BY entity
),
hardg  AS (
  SELECT entity, hkey, count(DISTINCT id) AS n
  FROM keyed GROUP BY entity, hkey HAVING count(DISTINCT id) > 1
),
harda  AS (
  SELECT entity, count(*) AS hard_groups, sum(n) AS entities_in_hard FROM hardg GROUP BY entity
),
looseb AS (
  SELECT entity, count(*) AS loose_groups FROM
    (SELECT entity, lkey FROM keyed GROUP BY entity, lkey HAVING count(DISTINCT hkey) > 1)
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
  row_number() OVER (ORDER BY entity, hkey)                             AS grp,
  entity,
  count(DISTINCT id)                                                    AS distinct_entities,
  group_concat(DISTINCT kind)                                          AS namespaces,
  CASE WHEN count(DISTINCT tname) > 1 THEN 'yes' ELSE 'no' END          AS case_variation,
  CASE WHEN count(DISTINCT name) > count(DISTINCT tname) THEN 'yes' ELSE 'no' END AS edge_whitespace,
  length(hkey)                                                         AS key_len
FROM keyed
GROUP BY entity, hkey
HAVING count(DISTINCT id) > 1
ORDER BY entity, distinct_entities DESC;

-- ── TIER B: punctuation/spacing near-misses (anonymized) ────────────────────
-- variation classifies the difference: pure internal whitespace vs punctuation.
SELECT '=== TIER B: punctuation/spacing near-misses (review-queue candidates) ===' AS "";
SELECT
  row_number() OVER (ORDER BY entity, lkey)                            AS grp,
  entity,
  count(DISTINCT hkey)                                                 AS distinct_forms,
  count(DISTINCT id)                                                   AS distinct_entities,
  CASE WHEN count(DISTINCT replace(hkey, ' ', '')) = 1
       THEN 'internal-whitespace' ELSE 'punctuation/other' END         AS variation,
  length(lkey)                                                         AS key_len
FROM keyed
GROUP BY entity, lkey
HAVING count(DISTINCT hkey) > 1
ORDER BY entity, distinct_forms DESC;

-- ── STUDIO refinement (anonymized): name collision vs external-id evidence ──
-- distinct_external_id_sets > 1 ⇒ probably different real companies (keep apart).
SELECT '=== STUDIO refinement: name collisions vs external-id evidence ===' AS "";
WITH s AS (
  SELECT s.id, lower(trim(s.name)) AS hkey,
         (SELECT group_concat(external_id) FROM studio_external_ids se WHERE se.studio_id = s.id) AS ext
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
