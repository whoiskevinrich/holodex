-- HOLODEX-452 / spec F71 P1-2: the decision sheet for pairs that a provider says are one
-- record but the owner has ALREADY dismissed as deliberately distinct.
--
-- WHY THIS EXISTS, AND WHY IT IS NOT A QUEUE. ADR-061's keep-separate marker is a durable
-- NO: every detector, including F71's, refuses to re-propose a dismissed pair, and F71's
-- exclusion is asserted by name in both producers' tests. That rule is deliberate and stays.
-- But 4 of the 9 person pairs the host run found were dismissed BEFORE this signal existed —
-- judged against `provider-alias`/`alias`, the weakest evidence the queue produces. Owner's
-- call 2026-09-23: surface those once, OUT of queue, as a one-time reconciliation worked by
-- hand. At N=4 the tool is a probe, not a UI. Re-running this later is harmless; it writes
-- nothing and re-derives its own population.
--
-- WHAT IT DELIBERATELY DOES NOT PRINT (scripts/CLAUDE.md). No names, no provider names, no
-- external-id bytes — not even a tail, which is the rule that was learned the expensive way
-- when a slug-minting provider leaked a performer's name through `substr(external_id, -8)`.
-- Every column below is an internal id, a count or a boolean. **Reading the names is the
-- app's job**: each pair prints two ids, and `/people/<id>` opens the record.
--
-- HOW TO DECIDE A ROW, in the order the evidence actually settles it:
--   * `shared_videos > 0` is a flag to INVESTIGATE, not a verdict — and the first version of
--     this file got that wrong, so read this before trusting the column. Two credits on one
--     file usually mean two people in that scene, since a split identity rarely co-appears
--     with itself. But it also happens when **one file's own credit list names one performer
--     twice in two spellings**, which is a major way these duplicates get created here. The
--     only live pair that carried a co-appearance (2026-09-25) was still a real duplicate and
--     was merged. So: open the shared video and read the two credits. What tips it toward the
--     artifact is the other side looking like one — no enrichment, no aliases, and its only
--     video being the shared one.
--   * `providers` = 2 is the strongest POSITIVE: two independent providers minting the same
--     id for both sides is not one provider's bookkeeping error.
--   * `dissenting` > 0 is the quiet negative, and it is why `providers` alone is not enough:
--     a provider that holds an id for BOTH sides and keeps them APART has an opinion, and it
--     is the opposite one. `providers = 1, dissenting = 1` means the library's two providers
--     disagree with each other about this pair — a much weaker case than
--     `providers = 1, dissenting = 0`, where no second provider has a view at all.
--   * `spine_side` says which id the identity spine already believes owns the provider id.
--     After a merge the survivor inherits it (`UPDATE OR IGNORE entity_external_ids`), so
--     merging INTO the spine side is the cheaper direction — it is not evidence, only cost.
--   * `videos_a` / `videos_b` being wildly lopsided (e.g. 40 vs 1) is the classic split: one
--     real record plus a stub the enrich UI created by a mis-pick. Near-equal counts on two
--     records with no shared video is the shape that deserves the app.
--   * `aliases_a` / `aliases_b` show how much naming history a merge would fold in.
--
-- AFTER YOU DECIDE. Merging is done in the app (owner → the person page's Merge control); it
-- is IRREVERSIBLE (ADR-061) and nothing in this file does it. Two verified facts about that
-- path, because both matter here: (1) a kept-separate marker does NOT block a merge —
-- `Repo.IsKeptSeparate` has no production caller, only tests — so no marker has to be cleared
-- first; and (2) the merge repoints `entity_external_ids` to the survivor and drops any
-- review-queue row touching the loser, so the contest resolves itself. It leaves the
-- `entity_keep_separate` row behind (person/studio/tag have no AFTER DELETE cleanup for that
-- table; film got one in migration 0047) — inert, because `people.id` is AUTOINCREMENT so the
-- loser's id can never be handed out again. Tracked separately as hygiene, not a blocker.
-- If you decide a pair is genuinely two people, there is nothing to do: the marker already
-- says so and every detector already honors it.
--
-- Run read-only against the live library:
--   sqlite3 -readonly /path/to/holodex.db < scripts/review_kept_separate_shared_id_pairs.sql
--
-- VERIFIED 2026-09-25 against a throwaway database built by applying all 51 up migrations to
-- an empty file, seeded with seven cases: a kept-separate pair sharing one provider id and
-- NO video (the plain case); a kept-separate pair that CO-APPEARS in one video (must show
-- shared_videos = 1, the dismissal-was-right shape); a kept-separate pair sharing ids from
-- TWO providers (providers = 2, dissenting = 0); a kept-separate pair with no spine row on
-- either side (spine_side must be empty, not an arbitrary id); a shared-id pair that is NOT
-- kept-separate (must not appear at all — this probe's whole population is the intersection);
-- a pair whose id_lo owns a spine row for an UNRELATED provider id (spine_side must stay
-- empty); and a pair one provider asserts while a second provider that knows both sides keeps
-- them apart (dissenting = 1). The output is also asserted to contain no name and no id bytes.
--
-- The last two cases exist because the FIRST version of this file was wrong on both counts,
-- and the live run is what exposed it: `spine_side` was not correlated on the shared id, so it
-- reported whichever side owned any id at all, and there was no `dissenting` column, so a pair
-- two providers actively disagreed about looked identical to one no second provider had an
-- opinion on.

.mode box
.headers on

-- ── The newest non-empty memo per (entity, provider) ─────────────────────────────────────
-- ADR-107 D6 rule 2. Same shape as the detector and migration 0052: the newest memo wins,
-- and the non-empty test lives INSIDE the subquery so a group whose newest memo carries no
-- id still resolves to its newest real one rather than dropping out entirely.
CREATE TEMP VIEW memo AS
SELECT e.entity_type, e.entity_id, e.provider,
       (SELECT f.external_id FROM entity_enrichment f
         WHERE f.entity_type = e.entity_type AND f.entity_id = e.entity_id
           AND f.provider = e.provider AND f.external_id <> ''
         ORDER BY f.fetched_at DESC, f.external_id
         LIMIT 1) AS external_id
FROM entity_enrichment e
WHERE e.entity_type = 'person'
  AND e.external_id <> ''
GROUP BY e.entity_type, e.entity_id, e.provider;

-- ── Every claimant of an id: the spine's one owner ∪ every memo holder (ADR-107 D1) ──────
CREATE TEMP VIEW claimant AS
SELECT DISTINCT external_id, entity_id, provider FROM (
    SELECT x.external_id, x.entity_id,
           substr(x.external_id, 1, instr(x.external_id, ':') - 1) AS provider
      FROM entity_external_ids x
     WHERE x.entity_type = 'person'
       AND instr(x.external_id, ':') > 1
    UNION ALL
    SELECT m.external_id, m.entity_id, m.provider
      FROM memo m
     WHERE instr(m.external_id, ':') > 1
       AND instr(m.external_id, ':') < length(m.external_id)
       AND EXISTS (SELECT 1 FROM people p WHERE p.id = m.entity_id)
);

-- ── The population: shared-id pairs the owner has ALREADY dismissed ──────────────────────
-- The intersection is the point. A pair with no keep-separate row is the queue's job and is
-- already there; a kept-separate pair with no shared id is not this reconciliation's business.
--
-- `shared` keeps one row per (pair, external_id) rather than aggregating immediately, because
-- WHICH id is contested is needed further down: a person can hold a spine row for a provider
-- id that has nothing to do with this pair, and asking "does either side own a spine row?"
-- without naming the shared id answers a different question. That was a real bug in the first
-- version of this file (2026-09-25) — it reported the wrong side on the live library.
CREATE TEMP VIEW shared AS
SELECT a.entity_id AS id_lo, b.entity_id AS id_hi, a.external_id, a.provider
  FROM claimant a
  JOIN claimant b ON a.external_id = b.external_id AND a.entity_id < b.entity_id
 WHERE EXISTS (SELECT 1 FROM entity_keep_separate ks
                WHERE ks.entity_type = 'person'
                  AND ks.id_lo = a.entity_id AND ks.id_hi = b.entity_id);

CREATE TEMP VIEW pair AS
SELECT id_lo, id_hi,
       count(DISTINCT provider)    AS providers,
       count(DISTINCT external_id) AS ids
  FROM shared
 GROUP BY id_lo, id_hi;

-- ── 1. Scale, so the numbers below have a denominator ────────────────────────────────────
SELECT (SELECT count(*) FROM pair)                                    AS pairs_to_work,
       (SELECT count(*) FROM entity_keep_separate
         WHERE entity_type = 'person')                                AS person_keep_separate_total,
       (SELECT count(*) FROM pair WHERE providers > 1)                AS asserted_by_two_providers;

-- ── 2. The decision sheet, the rows needing a human eye first ────────────────────────────
-- shared_videos DESC puts any co-appearing pair at the top — not because it settles the row
-- but because it is the one that cannot be settled from counts at all.
SELECT p.id_lo,
       p.id_hi,
       p.providers,
       -- Which side the spine gives THE CONTESTED id to, or blank when neither holds it (the
       -- memo-to-memo case, which is invisible to a spine-anchored join and is why ADR-107
       -- D1 pairs the whole claimant set). Correlated on the shared external_id — a side may
       -- own an unrelated provider id, and that is not this pair's spine owner. group_concat
       -- because a pair contested on two ids can in principle have a different owner per id.
       coalesce((SELECT group_concat(DISTINCT x.entity_id)
                   FROM entity_external_ids x
                   JOIN shared s ON s.external_id = x.external_id
                  WHERE x.entity_type = 'person'
                    AND s.id_lo = p.id_lo AND s.id_hi = p.id_hi
                    AND x.entity_id IN (p.id_lo, p.id_hi)), '')       AS spine_side,
       -- Providers that hold an id for BOTH sides and give them DIFFERENT ones: a provider
       -- actively DISSENTING from the claim. Added 2026-09-25 after the live run, where two
       -- pairs turned out to be asserted by one provider while a second one that knew both
       -- sides had them apart — which reads very differently from one provider asserting
       -- while no other has an opinion. Derived as (providers holding both sides) minus the
       -- providers that agree, so a provider holding two ids for a side, one of which
       -- matches, counts as agreeing rather than dissenting.
       (SELECT count(DISTINCT ca.provider) FROM claimant ca
         WHERE ca.entity_id = p.id_lo
           AND EXISTS (SELECT 1 FROM claimant cb
                        WHERE cb.provider = ca.provider AND cb.entity_id = p.id_hi)
       ) - p.providers                                                AS dissenting,
       -- Co-appearance: usually two people in that scene, but ALSO what one file's credit list
       -- naming a performer twice produces. Investigate it, don't rule on it — see the header.
       (SELECT count(*) FROM video_people va
          JOIN video_people vb ON vb.video_id = va.video_id
         WHERE va.person_id = p.id_lo AND vb.person_id = p.id_hi)     AS shared_videos,
       (SELECT count(*) FROM video_people v WHERE v.person_id = p.id_lo) AS videos_a,
       (SELECT count(*) FROM video_people v WHERE v.person_id = p.id_hi) AS videos_b,
       (SELECT count(*) FROM entity_aliases a
         WHERE a.entity_type = 'person' AND a.entity_id = p.id_lo)    AS aliases_a,
       (SELECT count(*) FROM entity_aliases a
         WHERE a.entity_type = 'person' AND a.entity_id = p.id_hi)    AS aliases_b,
       -- A film credit both sides carry is the same signal as a shared video, one level up.
       (SELECT count(*) FROM film_people_roles fa
          JOIN film_people_roles fb ON fb.film_id = fa.film_id
         WHERE fa.person_id = p.id_lo AND fb.person_id = p.id_hi)     AS shared_films,
       -- Is the pair ALSO a name near-miss? If the name detector queued it too, the row the
       -- owner dismissed may have carried a second signal — worth knowing before re-deciding.
       (SELECT count(*) FROM identity_review_queue q
         WHERE q.entity_type = 'person'
           AND q.id_lo = p.id_lo AND q.id_hi = p.id_hi)               AS in_queue_now
  FROM pair p
 ORDER BY shared_videos DESC, p.providers DESC, p.id_lo;

-- ── 3. How many enriched fields each side is carrying ────────────────────────────────────
-- Counts only — never a `value`, which is free text and identifies by content. A side with
-- no fields at all is the mis-pick stub shape: created by the enrich UI, never populated.
SELECT p.id_lo, p.id_hi,
       (SELECT count(*) FROM entity_enrichment e
         WHERE e.entity_type = 'person' AND e.entity_id = p.id_lo)    AS fields_a,
       (SELECT count(*) FROM entity_enrichment e
         WHERE e.entity_type = 'person' AND e.entity_id = p.id_hi)    AS fields_b,
       (SELECT count(DISTINCT e.provider) FROM entity_enrichment e
         WHERE e.entity_type = 'person' AND e.entity_id = p.id_lo)    AS providers_a,
       (SELECT count(DISTINCT e.provider) FROM entity_enrichment e
         WHERE e.entity_type = 'person' AND e.entity_id = p.id_hi)    AS providers_b
  FROM pair p
 ORDER BY p.id_lo;
