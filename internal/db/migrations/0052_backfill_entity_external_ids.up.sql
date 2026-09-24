-- HOLODEX-452 (spec F71 P0-9, ADR-107 §3b): fold the re-enrich memo
-- (entity_enrichment.external_id) into the identity spine (entity_external_ids) for
-- person / studio / film.
--
-- WHY THIS EXISTS. No migration has ever read the memo into the spine -- 0018, 0038 and
-- 0046 each declined -- and the enrich-path attach that would have written it is only as
-- old as 28e2540 (F60, 2026-09-15). Measured on the live library 2026-09-23: only 489 of
-- 1000 enriched people and 17 of 45 films hold ANY entity_external_ids row, while 1700+
-- memos name a provider id. Studio's 100% coverage comes from a different writer
-- (ReconcileVideoStudios -> resolveOrCreateByName -> attachExternalID, ADR-054) that
-- re-derives on every relink. So the gap is HISTORICAL, not a write that is still being
-- dropped, and it is a backfill rather than a bug fix.
--
-- ORDERING IS LOAD-BEARING. This runs before F71's write-time guard on
-- Repo.AttachExternalID (spec P0-3): a guard switched on first would find almost nothing,
-- because nearly every collision already exists. It must also land before HOLODEX-457
-- (ADR-096 D2's drop of entity_enrichment.external_id), which would delete the only
-- record roughly 500 people and 28 films have that their provider id exists at all.
--
-- ADR-107 D6's four reading rules apply, the same four scripts/detect_shared_external_id.sql
-- probed with:
--   1. external_id <> '' -- the whole `filename` extract population memoizes '' by design
--      (internal/extract/store.go).
--   2. newest fetched_at per (entity_type, entity_id, provider) -- a re-enrich that wrote a
--      NARROWER field set leaves older rows untouched, so only the newest memo counts. Ties
--      break toward the memo that AGREES with the spine, so a tie yields no finding.
--   3. person / studio / film only -- video has no spine row and two files of one movie
--      legitimately share a provider id; tag is not enrichable.
--   4. entity_keep_separate is honored -- a dismissed pair is never re-proposed (ADR-061).

-- ── 1. The newest memo per (entity, provider) ────────────────────────────────────────
-- Rule 2. The correlated subquery picks the winner over ALL non-empty memos of the group;
-- the shape check in step 2 is applied to that winner only, never used to fall back to an
-- older memo -- the newest assertion is the one the owner made, shaped or not.
-- OQ3 measured zero groups on the host carrying more than one distinct memo id, so this is
-- belt-and-braces; it stays because UpsertEnrichment is per-key and never deletes.
CREATE TEMP TABLE _memo_winner AS
SELECT e.entity_type, e.entity_id, e.provider,
       (SELECT f.external_id
          FROM entity_enrichment f
         WHERE f.entity_type = e.entity_type
           AND f.entity_id   = e.entity_id
           AND f.provider    = e.provider
           AND f.external_id <> ''
         ORDER BY f.fetched_at DESC,
                  EXISTS (SELECT 1 FROM entity_external_ids x
                           WHERE x.entity_type = f.entity_type
                             AND x.external_id = f.external_id
                             AND x.entity_id   = f.entity_id) DESC,
                  f.external_id
         LIMIT 1) AS external_id
FROM entity_enrichment e
WHERE e.entity_type IN ('person', 'studio', 'film')   -- rule 3
  AND e.external_id <> ''                             -- rule 1
GROUP BY e.entity_type, e.entity_id, e.provider;

-- ── 2. Only namespace-qualified ids, only live entities ──────────────────────────────
-- The shape test is identityShaped (internal/enrich/service.go): "<ns>:<id>" with both
-- halves present, cut at the FIRST colon -- so a slug id whose own half contains a colon
-- ("<ns>:performer:Some_Name") passes, which is the shape one live provider actually
-- mints. entity_external_ids is polymorphic and carries no FK, so the entity-exists guard
-- is explicit here rather than delegated to a constraint: a memo for an entity deleted
-- before its enrichment rows were swept must not become an orphan spine row.
CREATE TEMP TABLE _memo AS
SELECT DISTINCT w.entity_type, w.entity_id, w.external_id
FROM _memo_winner w
WHERE instr(w.external_id, ':') > 1                           -- namespace non-empty
  AND instr(w.external_id, ':') < length(w.external_id)       -- id non-empty
  AND ((w.entity_type = 'person' AND EXISTS (SELECT 1 FROM people  p WHERE p.id = w.entity_id))
    OR (w.entity_type = 'studio' AND EXISTS (SELECT 1 FROM studios s WHERE s.id = w.entity_id))
    OR (w.entity_type = 'film'   AND EXISTS (SELECT 1 FROM films   f WHERE f.id = w.entity_id)));

-- ── 3. Everyone who claims an id ─────────────────────────────────────────────────────
-- ADR-107 D1: the claimant set is the spine's one owner UNION every memo holder, NOT a
-- memo-to-spine join. The host run found one studio id claimed by three studios -- a join
-- anchored on the spine owner emits the two pairs that touch it and silently drops the
-- memo-to-memo pair between the other two, a duplicate the owner could never reach.
-- DISTINCT because an entity that both owns the spine row and memoizes the id is one
-- claimant, not two.
CREATE TEMP TABLE _claimant AS
SELECT DISTINCT entity_type, external_id, entity_id FROM (
    SELECT entity_type, external_id, entity_id FROM entity_external_ids
     WHERE entity_type IN ('person', 'studio', 'film')
    UNION ALL
    SELECT entity_type, external_id, entity_id FROM _memo
);

CREATE TEMP TABLE _contested AS
SELECT entity_type, external_id
FROM _claimant
GROUP BY entity_type, external_id
HAVING count(*) > 1;

-- ── 4. Fold the uncontested memos into the spine ─────────────────────────────────────
-- A contested id is deliberately left UNOWNED. The spine PK gives an id exactly one owner
-- per kind, so writing a row for one of two claimants would decide which entity the
-- provider record belongs to -- and id-first resolve (resolveOrCreateByName step 1) would
-- then route every future credit carrying that id to whichever side this migration picked.
-- That is an adjudication, and ADR-107 D3 is explicit that this feature queues and never
-- adjudicates: the colliding data was produced by an owner picking the wrong entity in the
-- enrich UI, so the newest memo is not evidence of the right answer. Step 5 records the
-- pair instead, and the owner's merge assigns the id.
--
-- OR IGNORE covers the agreeing case: an uncontested claimant set of one can be an entity
-- that already owns its spine row, whose memo simply repeats it.
INSERT OR IGNORE INTO entity_external_ids (entity_type, entity_id, external_id)
SELECT m.entity_type, m.entity_id, m.external_id
FROM _memo m
WHERE NOT EXISTS (SELECT 1 FROM _contested c
                   WHERE c.entity_type = m.entity_type
                     AND c.external_id = m.external_id);

-- ── 5. Queue every contested pair ────────────────────────────────────────────────────
-- Not an INSERT OR IGNORE into the spine that drops the loser: this backfill is F71's
-- single largest producer of review rows, and discarding a contested claim would discard
-- exactly the signal the feature exists to surface. It is also the only moment the
-- evidence is durable -- the queue row outlives HOLODEX-457's drop of the memo column,
-- which the boot sweep reads.
--
-- Pairs over the whole claimant set (every pair of a 3-claimant id, not just the two that
-- touch the spine owner). OR IGNORE on the (entity_type, id_lo, id_hi) PK keeps the pass
-- idempotent and leaves a pair the name-based detector already queued under its existing
-- variation. `detail` stays '' (spec P0-2): unlike provider-alias, both sides of this pair
-- are readable from the entities themselves.
INSERT OR IGNORE INTO identity_review_queue (entity_type, id_lo, id_hi, variation)
SELECT a.entity_type, a.entity_id, b.entity_id, 'shared-external-id'
FROM _claimant a
JOIN _claimant b ON a.entity_type = b.entity_type
                AND a.external_id = b.external_id
                AND a.entity_id   < b.entity_id
WHERE NOT EXISTS (SELECT 1 FROM entity_keep_separate ks     -- rule 4
                   WHERE ks.entity_type = a.entity_type
                     AND ks.id_lo = a.entity_id
                     AND ks.id_hi = b.entity_id);

DROP TABLE _contested;
DROP TABLE _claimant;
DROP TABLE _memo;
DROP TABLE _memo_winner;
