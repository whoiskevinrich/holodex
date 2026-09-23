-- HOLODEX-452 / ADR-107 / spec F71 — shared-provider-external-id duplicate probe.
--
-- Read-only. Run against the LIVE library, not a dev database:
--
--     sqlite3 -readonly /path/to/holodex.db < scripts/detect_shared_external_id.sql
--
-- Sibling of scripts/detect_person_duplicate_evidence.sql, which measured the NAME-based
-- queue. This one measures the hole that queue cannot see: an entity whose re-enrich memo
-- (entity_enrichment.external_id) carries a provider id that entity_external_ids assigns to a
-- DIFFERENT entity of the same kind. That disagreement is the residue of the silent
-- `INSERT OR IGNORE` no-op in attachExternalID on the enrich path (ADR-107 Context).
--
-- Reading rules applied throughout, per ADR-107 decision 6:
--   1. external_id <> ''        — the whole `filename` extract population memoizes '' by design
--                                 (internal/extract/store.go:26).
--   2. newest fetched_at per (entity, provider) — a re-enrich that wrote a NARROWER field set
--                                 leaves older rows untouched, so only the newest memo counts.
--   3. person / studio / film only — video has no spine row at all and two files of one movie
--                                 legitimately share an id; tag is not enrichable.
--   4. entity_keep_separate is honored — a dismissed pair is never a finding.

.mode box
.headers on

-- ── The newest memo per (entity, provider), which is what "the memo says" means ─────────
-- max(fetched_at) picks the pass; the correlated max(external_id) inside that pass is a
-- tiebreak that only fires if one pass somehow wrote two ids, which UpsertEnrichment's
-- single stamped argument (internal/repo/enrichment.go:46) should make impossible.
CREATE TEMP VIEW memo AS
SELECT e.entity_type, e.entity_id, e.provider,
       (SELECT f.external_id FROM entity_enrichment f
         WHERE f.entity_type = e.entity_type AND f.entity_id = e.entity_id
           AND f.provider = e.provider AND f.external_id <> ''
         ORDER BY f.fetched_at DESC, f.external_id
         LIMIT 1) AS external_id,
       max(e.fetched_at) AS fetched_at
FROM entity_enrichment e
WHERE e.external_id <> ''
  AND e.entity_type IN ('person', 'studio', 'film')
GROUP BY e.entity_type, e.entity_id, e.provider;

-- ── The finding: the memo names an id the spine gives to somebody else ──────────────────
CREATE TEMP VIEW disagreement AS
SELECT m.entity_type,
       min(m.entity_id, x.entity_id) AS id_lo,
       max(m.entity_id, x.entity_id) AS id_hi,
       m.provider, m.external_id,
       m.entity_id AS memo_holder,
       x.entity_id AS spine_owner
FROM memo m
JOIN entity_external_ids x
  ON x.entity_type = m.entity_type AND x.external_id = m.external_id
WHERE x.entity_id <> m.entity_id;

SELECT '=== 0. scale ===' AS "";
SELECT (SELECT count(*) FROM people)                                   AS people,
       (SELECT count(*) FROM studios)                                  AS studios,
       (SELECT count(*) FROM films)                                    AS films,
       (SELECT count(DISTINCT entity_id) FROM entity_enrichment
         WHERE entity_type = 'person')                                 AS people_enriched,
       (SELECT count(*) FROM identity_review_queue)                    AS queued_now,
       (SELECT count(*) FROM entity_keep_separate)                     AS kept_separate;

SELECT '=== 1. HEADLINE — pairs this detector would queue, by kind ===' AS "";
-- Counted over DISTINCT pairs, not disagreement rows: one pair can collide on two providers,
-- and the queue stores it once (PK entity_type, id_lo, id_hi).
SELECT p.entity_type,
       count(*)          AS pairs,
       sum(p.is_new)     AS genuinely_new
FROM (SELECT DISTINCT d.entity_type, d.id_lo, d.id_hi,
             CASE WHEN NOT EXISTS (SELECT 1 FROM identity_review_queue q
                    WHERE q.entity_type = d.entity_type AND q.id_lo = d.id_lo AND q.id_hi = d.id_hi)
                   AND NOT EXISTS (SELECT 1 FROM entity_keep_separate k
                    WHERE k.entity_type = d.entity_type AND k.id_lo = d.id_lo AND k.id_hi = d.id_hi)
                  THEN 1 ELSE 0 END AS is_new
        FROM disagreement d) p
GROUP BY p.entity_type;

SELECT '=== 2. every finding, named ===' AS "";
SELECT d.entity_type, d.provider, d.external_id,
       d.memo_holder, d.spine_owner,
       CASE d.entity_type
            WHEN 'person' THEN (SELECT name FROM people  WHERE id = d.memo_holder)
            WHEN 'studio' THEN (SELECT name FROM studios WHERE id = d.memo_holder)
            WHEN 'film'   THEN (SELECT name FROM films   WHERE id = d.memo_holder) END
                                                                       AS memo_holder_name,
       CASE d.entity_type
            WHEN 'person' THEN (SELECT name FROM people  WHERE id = d.spine_owner)
            WHEN 'studio' THEN (SELECT name FROM studios WHERE id = d.spine_owner)
            WHEN 'film'   THEN (SELECT name FROM films   WHERE id = d.spine_owner) END
                                                                       AS spine_owner_name,
       EXISTS (SELECT 1 FROM identity_review_queue q
                WHERE q.entity_type = d.entity_type AND q.id_lo = d.id_lo AND q.id_hi = d.id_hi)
                                                                       AS already_queued,
       EXISTS (SELECT 1 FROM entity_keep_separate k
                WHERE k.entity_type = d.entity_type AND k.id_lo = d.id_lo AND k.id_hi = d.id_hi)
                                                                       AS kept_separate
FROM disagreement d
ORDER BY d.entity_type, d.provider, d.external_id;

SELECT '=== 3. is the new set DISJOINT from the name-based queue? (expect overlap 0) ===' AS "";
SELECT count(*) AS findings_already_in_queue
FROM disagreement d
JOIN identity_review_queue q
  ON q.entity_type = d.entity_type AND q.id_lo = d.id_lo AND q.id_hi = d.id_hi;

SELECT '=== 4. OQ3 — is the memo consistent per (entity, provider)? ===' AS "";
SELECT '4a. groups with >1 distinct NON-EMPTY memo id (the stale-narrow-re-enrich case)' AS check_;
SELECT entity_type, count(*) AS groups
FROM (SELECT entity_type, entity_id, provider
        FROM entity_enrichment WHERE external_id <> ''
       GROUP BY entity_type, entity_id, provider
      HAVING count(DISTINCT external_id) > 1)
GROUP BY entity_type;

SELECT '4b. groups mixing empty and non-empty memos' AS check_;
SELECT entity_type, count(*) AS groups
FROM (SELECT entity_type, entity_id, provider
        FROM entity_enrichment
       GROUP BY entity_type, entity_id, provider
      HAVING sum(external_id =  '') > 0
         AND sum(external_id <> '') > 0)
GROUP BY entity_type;

SELECT '4c. memos NOT of the form <ns>:<id> — nothing enforces the grammar at the write' AS check_;
SELECT entity_type, provider, count(*) AS rows_
FROM entity_enrichment
WHERE external_id <> '' AND instr(external_id, ':') = 0
GROUP BY entity_type, provider;

SELECT '=== 5. the memo self-join the ticket proposed — for comparison with §1 ===' AS "";
SELECT entity_type, count(*) AS colliding_ids
FROM (SELECT entity_type, external_id
        FROM entity_enrichment
       WHERE external_id <> '' AND entity_type IN ('person','studio','film')
       GROUP BY entity_type, external_id
      HAVING count(DISTINCT entity_id) > 1)
GROUP BY entity_type;

SELECT '--- 5b. and what it would WRONGLY find if video were not excluded ---' AS "";
SELECT count(*) AS video_ids_shared_by_multiple_files
FROM (SELECT external_id FROM entity_enrichment
       WHERE entity_type = 'video' AND external_id <> ''
       GROUP BY external_id HAVING count(DISTINCT entity_id) > 1);

SELECT '=== 6. co-appearance for each person finding (one indexed join) ===' AS "";
SELECT d.id_lo, d.id_hi,
       (SELECT count(*) FROM video_people a
          JOIN video_people b ON a.video_id = b.video_id
         WHERE a.person_id = d.id_lo AND b.person_id = d.id_hi) AS shared_videos
FROM disagreement d
WHERE d.entity_type = 'person'
GROUP BY d.id_lo, d.id_hi;

SELECT '=== 7. memos with NO spine row at all (the other half of the no-op) ===' AS "";
-- Not a duplicate pair, but the same silent failure: the entity was enriched and its identity
-- row never landed. Sizing it says whether the write-time guard needs a repair pass too.
SELECT m.entity_type, count(*) AS memos_without_a_spine_row
FROM memo m
WHERE NOT EXISTS (SELECT 1 FROM entity_external_ids x
                   WHERE x.entity_type = m.entity_type AND x.external_id = m.external_id)
GROUP BY m.entity_type;
