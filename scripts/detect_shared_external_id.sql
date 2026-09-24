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
--
-- OUTPUT IS ANONYMIZED (scripts/CLAUDE.md): no entity names, no provider names, no full provider
-- ids. Pairs are internal ids — /people/1679 opens the record — providers are stable `provider-N`
-- aliases, and an external id shows only its last 8 characters, which is enough to group rows that
-- share one. Every result here is safe to paste into a session, an issue or a PR as-is.
--
-- Verified 2026-09-23 against a database built by applying all 50 up migrations to an empty
-- file and seeded with seven cases: a memo that agrees with the spine (no finding); a memo the
-- spine gives to somebody else (finding); a stale narrow re-enrich whose OLD memo points at
-- another owner (no finding — rule 2); a `filename` memo of '' (no finding — rule 1); a
-- kept-separate pair (found, but excluded from `genuinely_new`); two videos sharing tmdb:841
-- (never a finding — rule 3, and §5b counts what the naive query would have taken); and one
-- pair colliding on TWO providers (counted once, in both columns of §1).

.mode box
.headers on

-- ── The newest memo per (entity, provider), which is what "the memo says" means ─────────
-- The correlated subquery takes the newest non-empty memo; ORDER BY external_id is only a
-- determinism tiebreak for rows sharing a fetched_at, which UpsertEnrichment's single stamped
-- argument (internal/repo/enrichment.go:46) should already make impossible within one pass.
-- max(e.fetched_at) is carried alongside so a reader can see which pass was chosen.
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

-- ── Stable generic provider aliases, so "which provider?" is answerable without naming one ──
CREATE TEMP VIEW provider_alias AS
SELECT provider, 'provider-' || dense_rank() OVER (ORDER BY provider) AS alias
FROM (SELECT DISTINCT provider FROM entity_enrichment);

-- ── Everyone who claims an id: the spine's one owner, plus every memo holder ───────
-- NOT just "memo disagrees with the spine". The host run found ONE studio id claimed by three
-- studios (two memo holders against one spine owner): a memo⇔spine join emits the two pairs
-- that touch the owner and silently drops the memo⇔memo pair between the other two. Pairing
-- over the whole claimant set is the only shape that closes a clique. DISTINCT because an
-- entity that both owns the spine row and memoizes the id is one claimant, not two.
CREATE TEMP VIEW claimant AS
SELECT DISTINCT entity_type, external_id, entity_id FROM (
    SELECT entity_type, external_id, entity_id FROM entity_external_ids
     WHERE entity_type IN ('person', 'studio', 'film')
    UNION ALL
    SELECT entity_type, external_id, entity_id FROM memo
);

CREATE TEMP VIEW disagreement AS
SELECT a.entity_type, a.entity_id AS id_lo, b.entity_id AS id_hi, a.external_id,
       EXISTS (SELECT 1 FROM entity_external_ids x
                WHERE x.entity_type = a.entity_type AND x.external_id = a.external_id
                  AND x.entity_id = a.entity_id)                 AS lo_owns_spine,
       EXISTS (SELECT 1 FROM entity_external_ids x
                WHERE x.entity_type = b.entity_type AND x.external_id = b.external_id
                  AND x.entity_id = b.entity_id)                 AS hi_owns_spine
FROM claimant a
JOIN claimant b ON a.entity_type = b.entity_type
               AND a.external_id = b.external_id
               AND a.entity_id < b.entity_id;

-- ── One publishable label per external id: aliased provider + the tail of the NATIVE id ──────
-- The namespace is stripped BEFORE taking the tail: substr(x, -8) alone still spells the provider
-- out for a short id like `tmdb:287`, which is the exact leak scripts/CLAUDE.md exists to stop.
-- COALESCE covers an id with no ':' at all (§4c counts those).
CREATE TEMP VIEW xid_label AS
SELECT x.external_id,
       coalesce(pa.alias, 'provider-?') || ':…' ||
       substr(substr(x.external_id, instr(x.external_id, ':') + 1), -8) AS label
FROM (SELECT DISTINCT external_id FROM claimant) x
LEFT JOIN provider_alias pa
       ON pa.provider = substr(x.external_id, 1, instr(x.external_id, ':') - 1);

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

SELECT '=== 2. every finding (ids only — look names up in the app) ===' AS "";
SELECT d.entity_type,
       (SELECT label FROM xid_label l WHERE l.external_id = d.external_id) AS xid,
       d.id_lo, d.id_hi,
       CASE WHEN d.lo_owns_spine THEN d.id_lo
            WHEN d.hi_owns_spine THEN d.id_hi END                AS spine_owner,
       EXISTS (SELECT 1 FROM identity_review_queue q
                WHERE q.entity_type = d.entity_type AND q.id_lo = d.id_lo AND q.id_hi = d.id_hi)
                                                                 AS already_queued,
       EXISTS (SELECT 1 FROM entity_keep_separate k
                WHERE k.entity_type = d.entity_type AND k.id_lo = d.id_lo AND k.id_hi = d.id_hi)
                                                                 AS kept_separate
FROM disagreement d
ORDER BY d.entity_type, d.external_id, d.id_lo;

SELECT '--- 2b. ids claimed by MORE than two entities (a clique, not a pair) ---' AS "";
SELECT c.entity_type,
       (SELECT label FROM xid_label l WHERE l.external_id = c.external_id) AS xid,
       count(*) AS claimants
FROM claimant c
GROUP BY c.entity_type, c.external_id
HAVING count(*) > 2;

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
SELECT e.entity_type, pa.alias AS provider, count(*) AS rows_
FROM entity_enrichment e JOIN provider_alias pa ON pa.provider = e.provider
WHERE e.external_id <> '' AND instr(e.external_id, ':') = 0
GROUP BY e.entity_type, pa.alias;

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

SELECT '=== 8. OQ2 disambiguation — is the spine sparse, or do the stores disagree on the string? ===' AS "";
-- §7 returned 1219 person memos with no spine row against 1000 enriched people, which is close to
-- every enriched person. Two very different explanations, and 8c separates them: if the misses
-- cluster in one provider, the two stores write that provider's id differently and the detector is
-- blind for it; if they are spread evenly, AttachExternalID is simply not landing.
SELECT entity_type, count(*) AS spine_rows, count(DISTINCT entity_id) AS entities
FROM entity_external_ids GROUP BY entity_type;

SELECT '--- 8b. enriched entities holding at least one spine row ---' AS "";
SELECT m.entity_type,
       count(DISTINCT m.entity_id) AS enriched_entities,
       count(DISTINCT CASE WHEN EXISTS (SELECT 1 FROM entity_external_ids x
                                         WHERE x.entity_type = m.entity_type
                                           AND x.entity_id = m.entity_id)
                           THEN m.entity_id END) AS with_any_spine_row
FROM memo m GROUP BY m.entity_type;

SELECT '--- 8c. per provider: memos, and how many of those ids the spine knows ---' AS "";
SELECT m.entity_type, pa.alias AS provider, count(*) AS memos,
       sum(EXISTS (SELECT 1 FROM entity_external_ids x
                    WHERE x.entity_type = m.entity_type AND x.external_id = m.external_id))
                                                        AS id_known_to_spine
FROM memo m JOIN provider_alias pa ON pa.provider = m.provider
GROUP BY m.entity_type, pa.alias;
