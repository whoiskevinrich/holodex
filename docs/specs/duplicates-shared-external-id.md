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

**Done** — `sharedExternalIDPairsSQL` in `internal/repo/shared_external_id.go`, a port of migration
0052's steps 1–3 rather than of the probe script, because only 0052's version carries the
tie-break-toward-agreement rule and the entity-exists guard. `claimant` is `MATERIALIZED` because it
is self-joined. Two of the three rules that could silently pass are **mutation-checked**: dropping
the spine half of the claimant union loses 3 of the 6 fixture pairs (it degenerates to a memo
self-join), and dropping the `DO UPDATE … WHERE` makes an unchanged second sweep report 6 instead
of 0.

**The video/tag exclusion is enforced twice, independently** — by the explicit
`entity_type IN (…)` on the memo scan and by the per-kind entity-exists guard, which enumerates
exactly those three kinds and so drops a video or tag row for having no branch to match. Mutation
testing showed either clause alone still excludes both kinds; removing **both** leaks exactly the
video pair and the tag pair. Recorded in the code comment so neither is deleted as redundant: the
`IN` list states the rule and keeps the correlated subquery off the video memos (the bulk of
`entity_enrichment`), while the exists guard is what makes the exclusion structural. The same
redundancy exists in migration 0052, by construction — it is the same query.

**P0-2 — the queue row.** A finding writes `identity_review_queue (entity_type, id_lo, id_hi,
variation = 'shared-external-id')` as an **upsert** —
`ON CONFLICT (entity_type, id_lo, id_hi) DO UPDATE SET variation = 'shared-external-id'`, RD8 — so
repeated runs are idempotent *and* a pair the name detector already queued is upgraded rather than
left wearing the weaker label. **`detail` carries the asserting provider's namespace** —
superseding this criterion's original "left empty", because P0-6's chip cites who made the claim
and that is the one fact the row cannot read off the two entities. Not the external id itself: it is
provider-internal, the owner cannot act on it, and the compare panel already renders it as a
provider-link badge. A pair colliding on two providers gets one of them deterministically (`min()`
over the namespace) rather than whichever row the join emitted first.

*Acceptance*: running the sweep twice changes nothing on the second pass; a pair pre-queued as
`punctuation` comes out as `shared-external-id`; a pair already `shared-external-id` is never
demoted by a later `SeedIdentityReviewQueue` run.

**Done** — `queueSharedExternalIDPair` in `internal/repo/shared_external_id.go`, the one writer both
producers call. It returns rows written (1 for a new or upgraded pair, 0 when the row already said
this) so the sweep can report an honest count.

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

**Done** — `Repo.SweepSharedExternalIDs` + `sweepSharedExternalIDs` in `cmd/holodex/main.go`, beside
`seedIdentityReviewQueue`, ungated per RD5, recorded as `model.JobKindSharedIDSweep`
(`"shared-id-sweep"`) with a bare count for detail. Best-effort: a failure is logged and never
blocks startup. It reuses `queueSharedExternalIDPair` per pair rather than re-deriving a bulk write,
so keep-separate and RD8's upgrade behave identically to the write-time guard by construction. Pairs
are read into memory before any write — the set is tiny (17 on the live library) and it keeps a read
cursor off a second pooled connection while writing.

**Reporting 0 on an unchanged pass needed one addition:** RD8's upsert counts an update as an
affected row, so a row already carrying `shared-external-id` would be re-reported on every boot. The
`DO UPDATE` therefore carries its own
`WHERE identity_review_queue.variation <> 'shared-external-id'`. Migration 0052 does not need it (on
a first run no row can already hold the value) and is deliberately left alone.

**The sweep queues; it never folds.** Repairing the spine was P0-9's job, and assigning a contested
id to one claimant would be an adjudication (ADR-107 D3) — asserted by a test that the sweep leaves
`entity_external_ids` untouched.

**P0-5 — the exclusions are tests.** A video pair sharing a provider id produces no finding. A tag
produces no finding. Both are named tests referencing ADR-107 D5, not comments.

**Done** — at both producers. `TestSweepSharedExternalIDs` seeds a video pair and a tag pair sharing
an id and asserts neither is queued; `TestAttachExternalIDGuardKindScope` asserts the guard queues
studio and film but not tag, and that a contested tag id keeps its old silent-ignore behaviour.
Video cannot reach the guard at all — `enrich.identityEntityType` stops it a layer up.

**P0-6 — the row renders the chip.** `variation = 'shared-external-id'` renders an accent chip
naming the provider (`tmdb says one person`) in place of the variation slug, in the row, for every
kind. Every other variation is untouched. Tokens only; QA in all three skins.

**Done.** `sharedIdChip` in `queue.ts` + the chip branch in `DuplicatePairRow`. Treatment is
**outlined** accent — `rounded-full border border-accent bg-accent/10 text-accent`, the same one
`ExtractionQueueRow`'s staged chips use — not the solid `bg-accent` the handoff's `--bg-accent`
wording implied: `app.css` reserves solid accent for a page's one primary action. A row whose
`detail` is empty reads `a provider says one person` rather than inventing a name.

*QA, on the live preview with a seeded contested pair.* **Screenshots are unavailable against this
preview** (they time out), so the verification is computed-style and geometry, and a human eyeball is
still wanted:

1. `[agent]` **Contrast, three skins — pass.** The chip resolves a *different* accent per skin, which
   is the real proof it is token-driven rather than hardcoded: Cinémathèque `rgb(232,163,61)`,
   Broadcast `rgb(54,224,208)`, Brutalist `rgb(214,255,63)`. Text-on-chip contrast, composited over
   the translucent `bg-accent/10` fill: **8.84 / 11.62 / 16.88** — all above AA and AAA.
2. `[agent]` **Row height unchanged at ≥ 640px — pass.** With two 39–42-character names, the chip row
   and a `punctuation` row both measure **49.0px** at 700px and at 1280px, with no truncation of the
   chip and no horizontal page overflow. On a phone (375px, where the row is designed to wrap) the
   chip costs **6px** over the slug — its 21.6px box against the slug's 16px line, isolated by
   swapping the chip for the slug in place; not an extra line.
3. `[agent]` **Sort order — pass, end to end.** The `shared-external-id` row renders above both
   `punctuation` rows even though its names sort last alphabetically, so it is the P0-8 rank doing it
   and not the name tiebreak.
4. `[human]` Does the chip read as *stronger* than the muted rows around it, not as an error?

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

**Done** — one `-2` branch ahead of the `-1` in `ListReviewPairs`' `ORDER BY`. Nothing else was
needed: a variation outside `fuzzyVariations` already passes the live-revalidation join untouched, so
these rows were listed correctly from the start and only their rank among the other non-fuzzy
variations was undefined. Mutation-checked (the test's shared-id pair carries the names that sort
LAST, so it cannot pass on the name tiebreak) and confirmed on the live preview.

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
  the tool is a probe, not a surface. Building one for four rows would be over-building — but that
  stops being true if the OQ2 repair pass lands and the number grows, at which point RD7's rejected
  option (reason-aware dismissals) is the one to reopen. Recorded here so the four are not silently
  re-suppressed on every sweep with nobody remembering why.

  **Tool, 2026-09-25: `scripts/review_kept_separate_shared_id_pairs.sql`.** The detector's §2 lists
  these pairs but not what decides them, so this one re-derives the population (shared-id pairs ∩
  `entity_keep_separate`, so it stays correct on a later run) and prints the evidence as **ids and
  counts only** — `scripts/CLAUDE.md` is explicit that a question needing a name is the wrong
  question for a probe, and the names are the app's job. It leads with **`shared_videos`**, which is
  the one column that can end the question outright: two credits on one file are two people in that
  scene, so a co-appearance means the provider conflated two performers and **the dismissal was
  right**. `providers = 2` is the converse — two independent providers minting one id for both sides
  is not one provider's bookkeeping error. Verified against a throwaway migrated database seeded with
  all five branches, including the pair that must *not* appear.

  **The four pairs, from the 2026-09-23 host run** (ids are the handle — `/people/<id>` opens each):

  | pair | asserted by | spine side | note |
  |---|---|---|---|
  | **167 ↔ 862** | **two providers** | 862 | The strongest of the four. One of the two rows is also where the anonymization leak was caught, so its id is never printed. |
  | 333 ↔ 911 | one | 333 | |
  | 836 ↔ 1429 | one | 1429 | |
  | 858 ↔ 1280 | one | 1280 | |

  None of the four was in the queue at probe time, which is the point: every detector honors the
  dismissal, so nothing but this pass will ever raise them again.

  **Two facts about acting on a decision, both verified 2026-09-25 rather than assumed.** A
  keep-separate marker does **not** block a merge — `Repo.IsKeptSeparate` has no production caller,
  only tests — so nothing has to be cleared first. And the merge does the right thing with this
  feature's own state: it repoints `entity_external_ids` to the survivor (`UPDATE OR IGNORE`) and
  drops any review-queue row touching the loser, so the contest resolves itself. It leaves the
  `entity_keep_separate` row standing, because person/studio/tag have no `AFTER DELETE` cleanup for
  that table while film got one in migration 0047 — **inert**, since `people.id` is `AUTOINCREMENT`
  and the loser's id can never be issued again. Filed as hygiene, not a blocker.

  Deciding a pair is genuinely two people needs **no action at all**: the marker already says so.
- ~~**P1-1 — size the orphaned memos.**~~ Promoted to **P0-9** on the measurement.

### Future Considerations (P2)

- **HOLODEX-457** — ADR-096 D2's deferred drop of `entity_enrichment.external_id`, with the video
  re-enrich memo re-homed. Until it lands, this feature makes the drift observable rather than impossible
  (ADR-107 D2).
- A provider-side duplicate is the mirror case: one entity of ours matching two provider records.
  Nothing detects it and nothing here does either.

## Data model

**No schema change.** `identity_review_queue` gains a fourth `variation` value in an existing
`TEXT` column, and `detail` (0045) carries the asserting provider's namespace for it — the first
variation whose detail has a reader. One new `job_runs` kind constant —
`model.JobKindSharedIDSweep` = `"shared-id-sweep"`. Nothing else registers a job kind: there is no
allowlist and no frontend label map, so the kind string renders as-is on the activity surface.

One **data-only migration**, `0052_backfill_entity_external_ids` — P0-9. It adds no table and no
column; it writes rows into `entity_external_ids` and `identity_review_queue`. (This section read
"No migration" until P1-1 was promoted to P0-9 on the host measurement.)

## API

**One added field**, superseding this section's original "None": `GET /owner/duplicates` now returns
`detail` alongside `variation`. The chip cites the asserting provider, and the row had no way to know
it — `ReviewPair` did not carry `detail` and `ListReviewPairs` did not select it, so the **queue's**
payload could not see the 0045 column at all. Owner's decision 2026-09-24, taken over a generic chip
that needed no plumbing. The dismiss path is unchanged, and no endpoint is added.

The column itself is not new to the API: `SkippedAliasesForEntity` already reads it for the Aliases
panel's collision line, behind `skippedAliases`' per-field `authorized` gate (corrected 2026-09-24 at
the security gate — an earlier note in this epic said the column had **no** reader anywhere, which was
wrong; what had no reader was the *review-queue row*). Two consequences worth stating: `detail` now
ships for **every** variation, not only this one — on a `provider-alias` row it carries the dropped
alias name — and both surfaces that read it are owner-gated, so the posture is unchanged.

## UI

One chip, keyed on `variation`, in `DuplicatePairRow`'s existing variation span — `sharedIdChip` in
`queue.ts`, a sibling of `MATCH_KIND_LABEL` and not an entry in it, because that map is keyed on the
derived `match_kind`, which `ListReviewPairs` leaves `''` for every non-fuzzy row (so anything driven
off that map would render nothing here). `web/src/lib/types.ts`'s `DuplicatePair` gains `detail` and
documents the new `variation` value. Full rationale, placement and the three-skin QA list are in the
[design handoff](../design/duplicates-shared-external-id-handoff.md).

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
| testing | **Done** — `docs/testing-strategy.md` **§17–17.3**: 8 Go tests + 4 SPA cases, 22 mutations run and 20 caught, the 2 survivors documented as survivors. The pass also pinned a rule nothing covered — rule 1 *inside* the winner subquery guards a **false negative** |
| security | **Done 2026-09-24** — no endpoint, no parameter, no new boundary; the owner gate verified (not assumed) and already pinned at 401 by `duplicates_test.go`; `detail` carries a provider **namespace**, never an external id. One finding fixed: `detail` is now projected **only** for this variation, because on a `provider-alias` row the same column holds a skipped person name whose correct reader is side-specific (§17.3) |
