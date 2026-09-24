-- ============================================================================
-- Person duplicate-pair EVIDENCE probe  —  READ-ONLY, ANONYMIZED OUTPUT.
--
--   sqlite3 -readonly /path/to/holodex.db ".read detect_person_duplicate_evidence.sql"
--
-- Answers: for the person pairs sitting in identity_review_queue, what evidence
-- is actually available to decide them? Written to size up a side-by-side
-- compare panel (headshots first), not to pick a winner.
--
-- Emits counts, buckets and classifications only. NO names, NO entity ids, NO
-- external-id values, NO dates, NO nationality strings. Pairs are opaque
-- `pair_no` ordinals.
--
-- Providers appear as stable `provider-N` aliases (scripts/CLAUDE.md). An
-- earlier version of this header argued a provider name is "schema vocabulary,
-- not library content" and printed it — that reasoning does not survive the
-- standing rule that this library's providers are never named, and every
-- provider column below was redacted by hand before the output could be shared.
-- Field keys stay in the clear: they are canonical mapping names (ADR-013),
-- fixed by the repo rather than by what the library happens to contain.
-- ============================================================================

.mode box
.headers on

-- Stable generic provider aliases, so "which provider?" stays answerable.
CREATE TEMP VIEW provider_alias AS
SELECT provider, 'provider-' || dense_rank() OVER (ORDER BY provider) AS alias
FROM (SELECT DISTINCT provider FROM entity_enrichment);

-- Loose key: lowercase, trim, strip spacing + punctuation. Same fold as the
-- S4/S5 detector and the older collision probe, so classifications agree.
CREATE TEMP VIEW pname AS
SELECT eid, kind,
  replace(replace(replace(replace(replace(replace(replace(replace(replace(
  replace(replace(replace(replace(replace(replace(
    lower(trim(nm)),
    ' ',''), '.',''), ',',''), '''',''), '-',''), '–',''), '—',''),
    '&',''), '(',''), ')',''), ':',''), '!',''), '?',''), '/',''), '’','') AS lkey
FROM (
  SELECT id AS eid, 'canonical' AS kind, name AS nm FROM people
  UNION ALL
  SELECT entity_id, 'alias', alias FROM entity_aliases WHERE entity_type = 'person'
);

-- The person half of the review queue, with a stable opaque ordinal so per-pair
-- rows can be discussed without ever printing an id or a name.
CREATE TEMP VIEW qpair AS
SELECT id_lo AS lo, id_hi AS hi, variation,
       row_number() OVER (ORDER BY variation, id_lo, id_hi) AS pair_no
FROM identity_review_queue
WHERE entity_type = 'person';

-- Which side of the collision is canonical vs alias — the strength of the
-- signal that put the pair in the queue at all.
CREATE TEMP VIEW qmatch AS
SELECT p.pair_no, p.lo, p.hi, p.variation,
  CASE
    WHEN EXISTS (SELECT 1 FROM pname a JOIN pname b ON a.lkey = b.lkey
                 WHERE a.eid = p.lo AND b.eid = p.hi
                   AND a.kind = 'canonical' AND b.kind = 'canonical') THEN 'canonical'
    WHEN EXISTS (SELECT 1 FROM pname a JOIN pname b ON a.lkey = b.lkey
                 WHERE a.eid = p.lo AND b.eid = p.hi
                   AND (a.kind = 'canonical' OR b.kind = 'canonical')) THEN 'mixed'
    ELSE 'alias'
  END AS match_kind
FROM qpair p;

CREATE TEMP VIEW pimg AS
SELECT person_id AS eid,
       sum(CASE WHEN role = 'headshot' THEN 1 ELSE 0 END) AS headshots,
       count(*) AS images
FROM person_images GROUP BY person_id;

CREATE TEMP VIEW pprov AS
SELECT entity_id AS eid, provider,
       max(CASE WHEN external_id <> '' THEN external_id END) AS xid
FROM entity_enrichment WHERE entity_type = 'person'
GROUP BY entity_id, provider;

CREATE TEMP VIEW pvid AS
SELECT person_id AS eid, count(*) AS vids FROM video_people GROUP BY person_id;

CREATE TEMP VIEW pfact AS
SELECT entity_id AS eid, provider, field_key, value
FROM entity_enrichment WHERE entity_type = 'person' AND trim(value) <> '';

-- ── 0. Scale ────────────────────────────────────────────────────────────────
SELECT '=== 0. QUEUE SCALE ===' AS "";
SELECT
  (SELECT count(*) FROM people)                                        AS people_total,
  (SELECT count(*) FROM qpair)                                         AS person_pairs_queued,
  (SELECT count(*) FROM identity_review_queue)                         AS all_pairs_queued,
  (SELECT count(*) FROM entity_keep_separate WHERE entity_type='person') AS person_keep_separate,
  (SELECT count(DISTINCT person_id) FROM person_images WHERE role='headshot') AS people_with_headshot,
  (SELECT count(DISTINCT entity_id) FROM entity_enrichment WHERE entity_type='person') AS people_enriched;

SELECT '--- queue shape (variation x match kind) ---' AS "";
SELECT variation, match_kind, count(*) AS pairs
FROM qmatch GROUP BY variation, match_kind ORDER BY pairs DESC;

-- ── 1. HEADSHOT COVERAGE — the make-or-break number for a compare panel ─────
SELECT '=== 1. HEADSHOT COVERAGE PER PAIR ===' AS "";
SELECT
  CASE WHEN coalesce(a.headshots,0) > 0 AND coalesce(b.headshots,0) > 0 THEN 'both sides'
       WHEN coalesce(a.headshots,0) > 0 OR  coalesce(b.headshots,0) > 0 THEN 'one side only'
       ELSE 'neither side' END AS headshots,
  count(*) AS pairs
FROM qmatch q
LEFT JOIN pimg a ON a.eid = q.lo
LEFT JOIN pimg b ON b.eid = q.hi
GROUP BY 1 ORDER BY pairs DESC;

SELECT '--- same, split by match kind (alias-only pairs are the hard ones) ---' AS "";
SELECT q.match_kind,
  sum(CASE WHEN coalesce(a.headshots,0) > 0 AND coalesce(b.headshots,0) > 0 THEN 1 ELSE 0 END) AS both,
  sum(CASE WHEN (coalesce(a.headshots,0) > 0) <> (coalesce(b.headshots,0) > 0) THEN 1 ELSE 0 END) AS one,
  sum(CASE WHEN coalesce(a.headshots,0) = 0 AND coalesce(b.headshots,0) = 0 THEN 1 ELSE 0 END) AS neither
FROM qmatch q
LEFT JOIN pimg a ON a.eid = q.lo
LEFT JOIN pimg b ON b.eid = q.hi
GROUP BY q.match_kind;

SELECT '--- ANY image (not just the headshot role) per pair ---' AS "";
SELECT
  CASE WHEN coalesce(a.images,0) > 0 AND coalesce(b.images,0) > 0 THEN 'both sides'
       WHEN coalesce(a.images,0) > 0 OR  coalesce(b.images,0) > 0 THEN 'one side only'
       ELSE 'neither side' END AS any_image,
  count(*) AS pairs
FROM qmatch q
LEFT JOIN pimg a ON a.eid = q.lo
LEFT JOIN pimg b ON b.eid = q.hi
GROUP BY 1 ORDER BY pairs DESC;

-- ── 2. Provider coverage ────────────────────────────────────────────────────
SELECT '=== 2. PROVIDER COVERAGE PER PAIR ===' AS "";
WITH pc AS (
  SELECT q.pair_no,
    (SELECT count(*) FROM pprov WHERE eid = q.lo) AS np_lo,
    (SELECT count(*) FROM pprov WHERE eid = q.hi) AS np_hi,
    (SELECT count(*) FROM pprov a JOIN pprov b ON a.provider = b.provider
      WHERE a.eid = q.lo AND b.eid = q.hi)        AS shared
  FROM qmatch q
)
SELECT
  CASE WHEN np_lo > 0 AND np_hi > 0 THEN 'both enriched'
       WHEN np_lo > 0 OR  np_hi > 0 THEN 'one enriched'
       ELSE 'neither enriched' END AS coverage,
  sum(CASE WHEN shared > 0 THEN 1 ELSE 0 END) AS share_a_provider,
  count(*) AS pairs
FROM pc GROUP BY 1 ORDER BY pairs DESC;

-- ── 3. External ids — is the "same id on both sides" rule even reachable? ───
-- NOTE entity_external_ids has PRIMARY KEY (entity_type, external_id), so two
-- people can never share one there BY CONSTRUCTION. The only place a shared id
-- could show up is entity_enrichment.external_id, which is not globally unique.
--
-- That observation became HOLODEX-452. The cross-check below counts colliding
-- MEMOS; it does NOT find the case where the memo holder and the spine's owner
-- are different entities and only one of them has a memo. Use
-- scripts/detect_shared_external_id.sql for that — it pairs the whole claimant
-- set and is the maintained detector (ADR-107 D1).
SELECT '=== 3. EXTERNAL-ID RELATIONSHIP (per shared provider) ===' AS "";
SELECT pa.alias AS provider,
  sum(CASE WHEN a.xid IS NOT NULL AND a.xid = b.xid THEN 1 ELSE 0 END) AS same_id,
  sum(CASE WHEN a.xid IS NOT NULL AND b.xid IS NOT NULL AND a.xid <> b.xid THEN 1 ELSE 0 END) AS different_ids,
  sum(CASE WHEN a.xid IS NULL OR b.xid IS NULL THEN 1 ELSE 0 END) AS id_missing_a_side,
  count(*) AS pairs_sharing_provider
FROM qmatch q
JOIN pprov a ON a.eid = q.lo
JOIN pprov b ON b.eid = q.hi AND b.provider = a.provider
JOIN provider_alias pa ON pa.provider = a.provider
GROUP BY pa.alias;

SELECT '--- cross-check: any external_id claimed by >1 person anywhere? ---' AS "";
SELECT 'entity_external_ids (PK forbids this — expect 0)' AS source,
       count(*) AS colliding_ids
FROM (SELECT external_id FROM entity_external_ids WHERE entity_type='person'
      GROUP BY external_id HAVING count(DISTINCT entity_id) > 1)
UNION ALL
SELECT 'entity_enrichment (not unique — this is the real number)',
       count(*)
FROM (SELECT provider, external_id FROM entity_enrichment
      WHERE entity_type='person' AND external_id <> ''
      GROUP BY provider, external_id HAVING count(DISTINCT entity_id) > 1);

-- ── 4. Bio facts — attributed, never adjudicated ────────────────────────────
-- Agreement is classified WITHIN a provider and ACROSS providers separately:
-- cross-provider disagreement says something about the providers, not the people
-- ("Nepal" vs "Federal Democratic Republic of Nepal"; 1990-01-01 vs 1990-03-17).
SELECT '=== 4a. FIELD-KEY INVENTORY (people only) ===' AS "";
SELECT field_key, count(DISTINCT eid) AS people, count(DISTINCT provider) AS providers
FROM pfact GROUP BY field_key ORDER BY people DESC LIMIT 40;

SELECT '=== 4b. FACT AGREEMENT ACROSS A QUEUED PAIR ===' AS "";
SELECT a.field_key,
  CASE WHEN a.provider = b.provider THEN 'same provider' ELSE 'cross provider' END AS provider_rel,
  CASE
    WHEN lower(trim(a.value)) = lower(trim(b.value))                      THEN 'exact'
    WHEN instr(lower(a.value), lower(b.value)) > 0
      OR instr(lower(b.value), lower(a.value)) > 0                        THEN 'one contains other'
    WHEN length(a.value) >= 4 AND substr(a.value,1,4) = substr(b.value,1,4) THEN 'same leading 4 chars'
    ELSE 'differ'
  END AS agreement,
  count(DISTINCT q.pair_no) AS pairs
FROM qmatch q
JOIN pfact a ON a.eid = q.lo
JOIN pfact b ON b.eid = q.hi AND b.field_key = a.field_key
GROUP BY 1,2,3 ORDER BY pairs DESC LIMIT 40;

-- ── 5. Co-appearance + thin/thick shape ─────────────────────────────────────
SELECT '=== 5. CO-APPEARANCE AND SIZE ASYMMETRY ===' AS "";
WITH sz AS (
  SELECT q.pair_no,
    coalesce(a.vids,0) AS v_lo, coalesce(b.vids,0) AS v_hi,
    EXISTS (SELECT 1 FROM video_people x JOIN video_people y ON x.video_id = y.video_id
            WHERE x.person_id = q.lo AND y.person_id = q.hi) AS co
  FROM qmatch q
  LEFT JOIN pvid a ON a.eid = q.lo
  LEFT JOIN pvid b ON b.eid = q.hi
)
SELECT
  sum(CASE WHEN co THEN 1 ELSE 0 END)                               AS co_appear_in_a_video,
  sum(CASE WHEN min(v_lo,v_hi) = 0 THEN 1 ELSE 0 END)               AS one_side_has_no_videos,
  sum(CASE WHEN min(v_lo,v_hi) = 1 THEN 1 ELSE 0 END)               AS thin_side_has_exactly_1,
  sum(CASE WHEN max(v_lo,v_hi) >= 5 * max(min(v_lo,v_hi),1) THEN 1 ELSE 0 END) AS lopsided_5x_or_more,
  count(*)                                                          AS pairs
FROM sz;

-- ── 6. Per-pair evidence shape (opaque ordinals — no names, no ids) ─────────
SELECT '=== 6. PER-PAIR EVIDENCE SHAPE ===' AS "";
SELECT q.pair_no, q.variation, q.match_kind,
  coalesce(a.headshots,0) AS hs_a, coalesce(b.headshots,0) AS hs_b,
  coalesce(a.images,0)    AS img_a, coalesce(b.images,0)   AS img_b,
  (SELECT count(*) FROM pprov WHERE eid=q.lo) AS prov_a,
  (SELECT count(*) FROM pprov WHERE eid=q.hi) AS prov_b,
  (SELECT count(*) FROM pprov x JOIN pprov y ON x.provider=y.provider
    WHERE x.eid=q.lo AND y.eid=q.hi AND x.xid IS NOT NULL AND x.xid=y.xid) AS same_xid,
  (SELECT count(*) FROM pprov x JOIN pprov y ON x.provider=y.provider
    WHERE x.eid=q.lo AND y.eid=q.hi AND x.xid IS NOT NULL AND y.xid IS NOT NULL
      AND x.xid<>y.xid) AS diff_xid,
  coalesce(v1.vids,0) AS vids_a, coalesce(v2.vids,0) AS vids_b,
  CASE WHEN EXISTS (SELECT 1 FROM video_people x JOIN video_people y ON x.video_id=y.video_id
                    WHERE x.person_id=q.lo AND y.person_id=q.hi) THEN 'yes' ELSE '' END AS co_appear
FROM qmatch q
LEFT JOIN pimg a  ON a.eid  = q.lo
LEFT JOIN pimg b  ON b.eid  = q.hi
LEFT JOIN pvid v1 ON v1.eid = q.lo
LEFT JOIN pvid v2 ON v2.eid = q.hi
ORDER BY q.pair_no;
