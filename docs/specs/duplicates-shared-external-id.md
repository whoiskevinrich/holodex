# Spec: Shared provider external id — detect a duplicate the name-based queue structurally cannot see (F71)

**Status**: Draft
**Phase**: Phase 2 (identity) — extends the F43 duplicates queue (ADR-061) and lands on F70's
compare panel; adds one `variation`, two producers and no new endpoint
**Owner**: Project owner
**Date**: 2026-09-23
**Jira**: [HOLODEX-452](https://whoiskevinrich.atlassian.net/browse/HOLODEX-452)
**ADR**: [ADR-107](../architecture/ADR-107-shared-external-id-duplicate-detection.md)
**Design**: [handoff](../design/duplicates-shared-external-id-handoff.md) ·
[mockup](../design/duplicates-shared-external-id-mockup.svg)
**Feature block**: **F71** — two entities of the same kind that carry the same provider external
id are queued as a duplicate pair with `variation = 'shared-external-id'`, detected as the
re-enrich memo disagreeing with the identity spine. A boot sweep drains the historical backlog and
a write-time guard stops the silent `INSERT OR IGNORE` no-op from creating new ones.

## Problem Statement

Every duplicate the system can see, it sees by name. `SeedIdentityReviewQueue` folds canonical
names ∪ aliases to a loose key; `FlagNearMiss`, `queueProviderAliasPair` and `queueFilmSameTitle`
are all name-driven. Nothing can see two entities carrying the **same provider external id**,
which is the strongest positive evidence available — the provider has already asserted they are
one record.

And the two rarely look alike: ADR-061's `UNIQUE` nameKey index means two entities **cannot**
share a canonical name, so a split identity always wears two different names. The name detector
reaches such a pair only if those names happen to differ by punctuation or whitespace.

Measured on the live library **2026-09-23** (`scripts/detect_shared_external_id.sql`; 1578 people,
556 studios, 48 films, 1000 enriched, 33 queued, 185 keep-separate):

| | pairs | not already queued or dismissed |
|---|---|---|
| person | 10 | 5 |
| studio | 7 | 7 |
| film | 0 | 0 |

**17 pairs, 12 new, and exactly one already in the name-based queue** — so this is near-disjoint
new coverage. **Two of those pairs are visible only because P0-1 pairs the whole claimant set**:
they have no spine owner on either side and were found memo-to-memo, which is not an edge case but
a direct consequence of how sparse the spine turns out to be (below).

Meanwhile all 33 queued pairs are `provider-alias` / `alias`, the weakest kind the
current detector produces. The owner is working a queue of near-certain false positives while the
near-certain true positives are invisible.

Excluding `video` suppressed **23** ids shared by multiple files — more wrong findings than there
are right ones, which is why that exclusion is a named test rather than a comment.

And the identity spine is **far sparser than the memo layer**:

| kind | provider | memos | id also in `entity_external_ids` |
|---|---|---|---|
| studio | provider-3 | 436 | **436 — 100%** |
| person | provider-3 | 960 | 491 — 51% |
| person | provider-1 | 851 | **101 — 12%** |
| film | provider-3 | 45 | 17 — 38% |

Only **489 of 1000** enriched people hold any spine row at all. This is a **separate problem from
the silent no-op below, and a historical one** — not the same write being dropped at scale. The
enrich-path attach did not exist until `28e2540` (F60, **2026-09-15**), eight days before the
probe. Studio's 100% comes from a *different writer*: `ReconcileVideoStudios` →
`resolveOrCreateByName` → `attachExternalID`, fed by the `_studio_external_ids` sidecar since
`746a5ac` (ADR-054, **2026-07-02**), which re-derives on every relink — and a studio with no video
link is pruned, so every surviving studio has been through it. Person has the same mirror
(`ReconcileVideoPeople`'s `extIDByName`) but its contract is explicitly optional, and an id-less
person is orphan-stamped rather than pruned, so it persists indefinitely. Film's 38% is the same
cutoff: it had no identity table before 0046, which backfilled nothing for it.

**No migration has ever read `entity_enrichment.external_id` into the spine** — 0018, 0038 and 0046
each declined. That is P0-9, and it has to run before everything else.

The cost of not fixing it compounds: the collisions are produced by a **silent no-op**
(`internal/repo/identity.go:258`, see ADR-107 Context), so their number only grows, and every one
is a split identity whose videos, films and provider facts are permanently divided between two
records.

## Goals

1. **Surface the strongest evidence the system has**, on the surface that already adjudicates
   duplicates, at the rank it deserves.
2. **Stop producing new collisions.** A dropped identity write becomes a review row instead of
   silence.
3. **Cover every kind that has the hole** — person, studio and film run the identical code path.
4. **Add no new review surface.** A pair inherits F70's compare panel; the queue gains one chip.

## Non-Goals

| Not doing | Why |
|---|---|
| Auto-merging on a shared id | ADR-107 D3 — irreversible (ADR-061), providers carry their own duplicates, and the colliding data came from an owner's mis-click in the enrich UI |
| Dropping `entity_enrichment.external_id` | ADR-107 D2 — that is ADR-096 D2's deferred action item; it needs its own migration and a re-homed video memo. Filed as **HOLODEX-457** |
| Adding `UNIQUE` to the memo column | Converts a silent no-op into a failed enrich, the opposite of ADR-088's queue-don't-fail posture, and cannot express the legitimate video case |
| A compare panel for studio / film | F70 RD10 scoped it to person deliberately |
| Queueing videos | Two files of one movie legitimately share a provider id. Excluded by construction, with a named test |
| ~~Backfilling the missing spine rows~~ | **No longer a non-goal.** Written when the gap was assumed small; the host probe measured 511 of 1000 enriched people with no spine row and promoted it to **P0-9**. The reasoning that survives is narrower and still binding: repairing a **contested** id is a merge decision, so the backfill folds only the uncontested ones |

## Resolved Decisions

- **RD1 — the signal is memo ⋈ spine disagreement, not a memo self-join.** ADR-107 D1. The spine's
  PK guarantees one owner per id, so the disagreement names which side the system already believes;
  a self-join cannot. Measured on the fixture, the self-join also produces a false positive the
  disagreement suppresses (a stale narrow re-enrich).
- **RD2 — it queues, it never merges.** ADR-107 D3. F70's rule governs: positive evidence can be
  decisive, but the panel reports and never adjudicates.
- **RD3 — person, studio and film. Video and tag are excluded by construction**, with named tests,
  not comments. ADR-107 D5.
- **RD4 — two producers.** A write-time guard at `Repo.AttachExternalID` closes the source; a boot
  sweep drains the backlog. ADR-107 D4.
- **RD5 — the boot sweep is not gated on `HasSuccessfulJobRun`.** The identity backfill's gate
  exists because the F43 fold was a one-time historical normalization. This is an idempotent
  reconciliation whose input keeps changing, so it runs every boot.
- **RD6 — the row names its asserter in an accent chip**, not the raw variation slug: the queue's
  `text-warn` already means *weak*, and this is the strongest signal in the list. Design handoff.
- **RD7 — `entity_keep_separate` is honored identically, and the backlog it hides is reconciled
  once by hand.** A dismissed pair is never re-proposed by either producer — ADR-061's durable-no
  invariant is untouched. But 4 of the 9 person findings are pairs dismissed against
  `provider-alias` / `alias` evidence *before this signal existed*, and `entity_keep_separate`
  records the pair, not the reason. Owner's decision 2026-09-23: **surface those once,
  out-of-queue, as a one-time reconciliation** — see P1-2. Rejected: making keep-separate
  reason-aware, which would change an ADR-061 invariant and need a migration to record which
  variation each dismissal answered; revisit only if the number stops being small.

- **RD8 — a shared-id finding UPGRADES an existing weaker `variation`, in every producer.**
  Owner's decision 2026-09-24. A pair can be both a shared-id finding and a name near-miss (1 of the
  host's 15 was); under a plain `INSERT OR IGNORE` it would keep a label that means *weak* and sort
  in the fuzzy band, which is the exact failure P0-8 exists to prevent. So the queue write is
  `ON CONFLICT (entity_type, id_lo, id_hi) DO UPDATE SET variation = 'shared-external-id'` — a
  **one-way ratchet**, because `SeedIdentityReviewQueue` writes with `INSERT OR IGNORE` and can
  never demote a row back. Accepted cost: the queue stores one variation per pair, so "the names
  also nearly match" stops being recorded for that pair. **All three producers must agree** — the
  migration (P0-9), the boot sweep (P0-4) and the write-time guard (P0-3). This amends P0-2's
  "`INSERT OR IGNORE`", which was written before the collision with the name detector was measured;
  idempotence is unaffected, since the upsert is a no-op on a row already carrying the value.

## User Stories

- As the owner, when a provider tells me two of my people are one record, I see that pair **above**
  the near-miss pairs, labelled with who asserted it, instead of never seeing it at all.
- As the owner, when I pick the wrong person in the enrich UI, the system tells me by queueing the
  pair, instead of silently discarding the identity write and leaving two half-enriched records.
- As the owner, a `shared-external-id` person pair opens into the same compare panel as every other
  pair, with no new controls to learn.

## Requirements

### Must-Have (P0)

**P0-1 — the detector query.** For each `(entity_type, external_id)`, the claimant set is the
entity `entity_external_ids` assigns the id to **union** every entity whose newest memo carries it;
two or more claimants yields one finding per pair (ADR-107 D1). A memo⇔spine join is **not**
sufficient — the host run found one studio id with three claimants, whose memo⇔memo pair such a
join drops. The four reading rules of ADR-107 D6 are mandatory:
`external_id <> ''` (excludes the whole `filename` extract population);
newest `fetched_at` per `(entity_type, entity_id, provider)`, ties broken toward the memo that
*agrees* with the spine so a tie yields no finding;
`entity_type IN ('person','studio','film')`;
and `entity_keep_separate` excluded.

*Acceptance*: given the seven-case fixture in `scripts/detect_shared_external_id.sql`'s header —
an agreeing memo, a disagreeing memo, a stale narrow re-enrich, a `filename` empty memo, a
kept-separate pair, two videos sharing an id, and one pair colliding on two providers — exactly
the disagreeing person pairs and the disagreeing studio pair are findings; the kept-separate pair
is a finding the queue write suppresses; and the two-provider pair yields one queue row, not two.

**P0-2 — the queue row.** A finding writes `identity_review_queue (entity_type, id_lo, id_hi,
variation = 'shared-external-id')` as an **upsert** —
`ON CONFLICT (entity_type, id_lo, id_hi) DO UPDATE SET variation = 'shared-external-id'`, RD8 — so
repeated runs are idempotent *and* a pair the name detector already queued is upgraded rather than
left wearing the weaker label. `detail` is left empty: unlike `provider-alias`, both sides of this
pair are readable from the entities themselves.

*Acceptance*: running the sweep twice changes nothing on the second pass; a pair pre-queued as
`punctuation` comes out as `shared-external-id`; a pair already `shared-external-id` is never
demoted by a later `SeedIdentityReviewQueue` run.

**P0-3 — the write-time guard.** `Repo.AttachExternalID` (`internal/repo/identity.go:274`) stops
being a bare `INSERT OR IGNORE`. Under the `writeMu` it already takes, it reads the current owner;
if the id belongs to a different entity of the same kind it queues the pair and returns without
error. The enrich must still succeed — the value is written, only the identity claim is contested.

Three constraints, each of which the obvious implementation gets wrong:

1. **`RowsAffected() == 0` is not the signal.** `attachExternalID` returns `nil` for three
   different outcomes — inserted, *already owned by this same entity*, and owned by another. The
   middle one is the common case on every re-enrich and refresh sweep, so queueing on
   `RowsAffected() == 0` alone would flood the queue. The guard must follow up with
   `externalIDSelect` and queue only when the owner is a **different** entity.
2. **Guard `Repo.AttachExternalID` only — never the shared `attachExternalID`.** The
   `resolveOrCreateByName` call sites legitimately depend on silent-ignore and run inside the
   caller's scan transaction, where a review row would be written under a transaction that may
   roll back. **Correction found while implementing:** ADR-107 D4 also gives "a queue insert
   there would fire on every relink" as a reason, and that half does not hold —
   `resolveOrCreateByName`'s step 1 looks the id up and **returns the owner before ever reaching
   the private writer** (`internal/repo/identity.go:166`), so a contested id cannot arrive there
   at all and a guard placed there would be dead code on the scan path rather than a flood. The
   constraint stands; the transactional reason is the one that carries it. Recorded in the
   `attachExternalID` doc comment so nobody "simplifies" the guard back down into the shared
   writer. It also means **no test can distinguish a guard placed there by its queue output** —
   what `TestScanPathAttachStillSilent` asserts is that the relink path still resolves id-first
   and still writes no review row.
3. **The check and the queue write must stay in one critical section.** `Repo.AttachExternalID`
   takes `r.writeMu` itself and uses `r.db` rather than a tx, so a check-then-queue that leaves and
   re-enters races. There is no `AttachExternalIDLocked` variant today — contrast
   `ReconcileVideoPeopleLocked`.

*Acceptance*: enriching person B against an id person A already owns queues `(A, B)` as
`shared-external-id`, returns 200, and B's enrichment values are written. **Re-enriching person A
against the id A already owns queues nothing** — the anti-flood case, and a named test. The
private scan-path `attachExternalID` is unchanged in behaviour because id-first resolve means the
owner is already the entity — asserted by a test, not assumed.

**Done** — `internal/repo/shared_external_id_test.go`, four named tests: the contest (pair queued,
call returns nil, the spine keeps its owner, no second spine row), the anti-flood re-attach, the
kind scope (studio and film queued, **tag not**, per ADR-107 D5), keep-separate honored, RD8's
upgrade of a pre-queued `punctuation` pair, and the scan path unchanged. The anti-flood case is
**mutation-checked**: dropping the owner-equality branch makes even a *free* attach queue a
self-pair, so the branch is load-bearing rather than incidentally satisfied. The shared writer is
`queueSharedExternalIDPair`, which P0-4's sweep reuses.

**P0-4 — the boot sweep.** Wired beside `seedIdentityReviewQueue` in `cmd/holodex/main.go`,
recorded as its own `job_runs` kind so it appears on the activity surface (ADR-028), and run every
boot (RD5).

*Acceptance*: a boot on a library carrying a historical collision queues it and records one job
run; the next boot records a run that inserted 0.

**P0-5 — the exclusions are tests.** A video pair sharing a provider id produces no finding. A tag
produces no finding. Both are named tests referencing ADR-107 D5, not comments.

**P0-6 — the row renders the chip.** `variation = 'shared-external-id'` renders an accent chip
naming the provider (`tmdb says one person`) in place of the variation slug, in the row, for every
kind. Every other variation is untouched. Tokens only; QA in all three skins.

**P0-7 — the stale rationale is rewritten.** `internal/repo/identity.go`'s comment ("this only
ever records a genuinely new (id, entity) pair") and `AttachExternalID`'s doc comment are both
false once P0-3 lands and must be corrected in the same change.

**Done**, in the same commit as P0-3. `attachExternalID`'s comment now says *why* its silence is
correct in that one place (step 1's early return, plus the caller's transaction) rather than
asserting something untrue about what it records; `AttachExternalID`'s doc comment states the
queue-on-conflict behaviour, that it still returns nil so the enrich succeeds, and why the owner is
re-read instead of the guard keying on `RowsAffected`.

**P0-9 — the spine backfill, and it runs FIRST.** 511 of 1000 enriched people and 28 of 45 films
hold a memo whose id the spine has never recorded, because no migration ever folded the memo in
(OQ2). A detector that pairs claimants cannot see a duplicate whose *other* side was never written
down, so this is coverage, not tidy-up.

Migration **`0052_backfill_entity_external_ids`** (data only, no schema change) folds
`entity_enrichment.external_id` into `entity_external_ids` for `entity_type IN
('person','studio','film')` where `identityShaped` holds, taking the newest memo per
`(entity, provider)` (P0-1's rule 2). **An id that two entities claim goes to the review queue as
`shared-external-id` rather than being dropped** — the backfill is the single largest producer of
P0-2 rows, and an `INSERT OR IGNORE` here would silently discard exactly the signal this feature
exists to surface. It is also the one moment that evidence is made durable: the queue row outlives
HOLODEX-457's drop of the memo column, which is what the boot sweep reads.

**Ordering is load-bearing: P0-9 lands before P0-3.** The gap is overwhelmingly historical, so a
write-time guard switched on first would find almost nothing. It must also land before
**HOLODEX-457**, which would delete the only record those ids have.

Four rules the implementation settled, each a named case in
`internal/db/external_ids_backfill_test.go`:

1. **A contested id is left UNOWNED.** The spine PK gives an id exactly one owner per kind, so
   writing a row for one of two claimants decides which entity the provider record names — and
   id-first resolve (`resolveOrCreateByName` step 1) would then route every future credit carrying
   that id to whichever side the migration picked. That is an adjudication, and ADR-107 D3 forbids
   it: the colliding data came from an owner's mis-click, so the newest memo is not evidence of the
   right answer. The pair is queued instead and the owner's merge assigns the id. This is the
   named rule the acceptance criterion's second clause allows.
2. **The entity-exists guard is explicit.** `entity_external_ids` is polymorphic and carries no FK,
   so a memo whose entity was deleted before its enrichment rows were swept would become an orphan
   spine row. Checked per kind against `people` / `studios` / `films`.
3. **The shape test matches `identityShaped`, cut at the first colon** — `<ns>:<id>` with both
   halves present. A slug id whose own half contains a colon (`<ns>:performer:Some_Name`, which one
   live provider mints) passes, as it does in Go.
4. **The down migration is asymmetric, on 0044's precedent.** It deletes the
   `shared-external-id` queue rows — that variation did not exist before 0052, so all of them are
   this feature's — and **leaves the folded spine rows**: once written they are indistinguishable
   from the rows `attachExternalID` produces on the scan path (same shape, same meaning, no
   provenance column), so reconstructing which came from the memo would have to guess, and guessing
   wrong deletes an identity the owner asserted. Re-applying is a no-op on them. A pair RD8
   **upgraded** loses its row entirely on the way down rather than being restored to a variation
   nothing recorded — which self-heals, because `SeedIdentityReviewQueue` is ungated and re-derives
   that pair from the names it was found by.

*Acceptance*: after the pass, §8b's `with_any_spine_row` equals `enriched_entities` for person and
film, or every remaining gap is explained by a named, tested rule (rules 1–3 above are those
rules). Down-then-up leaves the spine byte-identical and restores exactly the queue rows the down
removed — no duplicate spine row, and no pair whose existing variation was overwritten.

**P0-8 — the sort tiebreak.** `shared-external-id` sorts above `provider-alias` and `same-title`,
which today share the non-fuzzy `-1` slot with no tiebreak
(`internal/repo/review_queue.go:193`). The strongest signal must not sort below the weakest by
accident of insertion order.

### Nice-to-Have (P1)

- **P1-0 — the co-appearance read.** `SELECT count(*) FROM video_people a JOIN video_people b ON
  a.video_id = b.video_id WHERE a.person_id = ? AND b.person_id = ?` — one indexed query over the
  existing PK. It is §6 of the probe already; promoting it to a repo method and onto the F70 panel
  closes HOLODEX-451's P1-0, which was demoted only because the frontend path cost two paged calls
  per pair.
- **P1-3 — align `identityShaped` with `sanitizePeople`.** The enrich attach's `identityShaped`
  (`internal/enrich/service.go:928`) accepts an id containing whitespace, which `sanitizePeople`
  (`:750`) rejects on the sidecar path for a documented security reason. The attach applies the
  weaker check, so P0-3 could report a contested id the sidecar path would have refused outright.
  Align the two, or the queue inherits the discrepancy.
- **P1-2 — reconcile the 4 dismissed-but-now-evidenced pairs, once.** No code and no UI: at N = 4
  the tool is the probe itself, whose §2 already lists them with `kept_separate = 1`. Work them by
  hand, and either merge or leave the dismissal standing. Building a surface for four rows would be
  over-building — but this stops being true if the OQ2 repair pass lands and the number grows, at
  which point RD7's rejected option (reason-aware dismissals) is the one to reopen. Recorded here
  so the four are not silently re-suppressed on every sweep with nobody remembering why.
- ~~**P1-1 — size the orphaned memos.**~~ Promoted to **P0-9** on the measurement.

### Future Considerations (P2)

- **HOLODEX-457** — ADR-096 D2's deferred drop of `entity_enrichment.external_id`, with the video
  re-enrich memo re-homed. Until it lands, this feature makes the drift observable rather than impossible
  (ADR-107 D2).
- A provider-side duplicate is the mirror case: one entity of ours matching two provider records.
  Nothing detects it and nothing here does either.

## Data model

**No schema change.** `identity_review_queue` gains a fourth `variation` value in an existing
`TEXT` column; `detail` (0045) stays empty for it. One new `job_runs` kind constant.

One **data-only migration**, `0052_backfill_entity_external_ids` — P0-9. It adds no table and no
column; it writes rows into `entity_external_ids` and `identity_review_queue`. (This section read
"No migration" until P1-1 was promoted to P0-9 on the host measurement.)

## API

None. `GET /owner/duplicates` already returns `variation` verbatim, and the dismiss path is
unchanged.

## UI

One chip, keyed on `variation`, in `DuplicatePairRow`'s existing variation span — a sibling of
`MATCH_KIND_LABEL`, not an entry in it, because that map is keyed on the derived `match_kind`.
`web/src/lib/types.ts`'s `variation` gains the value. Full rationale, placement and the three-skin
QA list are in the [design handoff](../design/duplicates-shared-external-id-handoff.md).

## Success Metrics

- The 10 measured person collisions and 7 studio ones are queued and decidable; the number of
  undetected collisions stops growing.
- The set is **near**-disjoint from the name-based queue — probe §3 returned **1**, not 0, so this
  is new coverage rather than a re-cut, and that one pair is both a shared-id finding and a
  punctuation near-miss. It renders the chip, not the near-miss label — see RD8.
- No false positive from a stale narrow re-enrich (probe §4a > 0 with §1 unaffected by it).

## Open Questions

Probe run on the host 2026-09-23. OQ1 and OQ3 are closed; OQ2 is half-closed and got bigger.

- ~~**OQ1 — real numbers for studio and film?**~~ **Closed: studio 6 pairs (all new), film 0.**
  Film has only 48 rows and 28 memos with no spine row, so its zero is "not yet exercised", not
  "not affected" — it runs the same code path and stays in scope.
- ~~**OQ2 — why is the spine so much sparser than the memo?**~~ **Closed: it is historical.** The
  enrich attach is 8 days old (`28e2540`, 2026-09-15); studio's scan-time feeder is 2.5 months old
  (`746a5ac`, 2026-07-02) and self-healing. Not a string mismatch, not a confidence gate, and not
  `identityShaped` — which accepts a slug id like `<provider>:performer:Taylor_Luna` cleanly. So
  **P0-9 is a backfill, not a bug fix**, and it must run *before* P0-3's guard, which would
  otherwise surface almost nothing from 1700+ stale memos. Provider-1's 12% is consistent with a
  person-only provider that never emits video `people[]` credits and so never had the scan-time
  writer at all. **HOLODEX-457 stays blocked** behind P0-9.
- ~~**OQ3 — memo consistent per (entity, provider)?**~~ **Closed, clean.** Zero groups with more
  than one distinct memo id, zero mixing empty with non-empty, zero breaking `<ns>:<id>`. The
  newest-`fetched_at` rule stays as belt-and-braces (ADR-107 D6).
- ~~**OQ4 — does a stronger signal re-open a dismissed pair?**~~ **Closed 2026-09-23: no — the
  queue never re-proposes, and the 4 already-dismissed findings are reconciled once, by hand, from
  the probe's own output.** RD7 and P1-2.
- ~~**OQ5 — §6 returned 5 rows for 9 person pairs.**~~ **Closed: the query is sound**, the earlier
  paste was trimmed. The re-run returns one row per pair, 10 for 10, and **3 of the 10 person pairs
  share a video** — real supporting evidence, and P1-0 can ride it.

## Timeline / routing

| Gate | State |
|---|---|
| spec | this document (F71) |
| architecture | [ADR-107](../architecture/ADR-107-shared-external-id-duplicate-detection.md) |
| design | [handoff](../design/duplicates-shared-external-id-handoff.md) + committed mockup |
| backend | P0-1 … P0-5, P0-7, P0-8 |
| frontend | P0-6 |
| testing | `docs/testing-strategy.md` — the exclusions (P0-5), the write-time guard (P0-3), sweep idempotence (P0-2) |
| security | owner-gated surface, no new endpoint; expected to be a short pass |
