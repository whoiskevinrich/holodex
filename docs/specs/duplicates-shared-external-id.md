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

Worse, the two can never look alike enough for the name detector to catch them by accident:
ADR-061's `UNIQUE` nameKey index means two people **cannot** share a canonical name, so a genuine
split identity necessarily wears two different names. The name detector and this one are looking
at disjoint populations by construction.

Measured on the live library 2026-09-22 (1591 people, 1000 enriched): **6 external ids are claimed
by more than one person**, none of them queued, and `same_xid = 0` across all 31 pairs currently in
the queue. Meanwhile all 31 queued pairs are `provider-alias` / `alias` — the weakest kind the
current detector produces — and 185 person pairs have already been dismissed keep-separate. The
owner is working a queue of near-certain false positives while the near-certain true positives are
invisible.

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
| Dropping `entity_enrichment.external_id` | ADR-107 D2 — that is ADR-096 D2's deferred action item; it needs its own migration and a re-homed video memo. Filed as a follow-up |
| Adding `UNIQUE` to the memo column | Converts a silent no-op into a failed enrich, the opposite of ADR-088's queue-don't-fail posture, and cannot express the legitimate video case |
| A compare panel for studio / film | F70 RD10 scoped it to person deliberately |
| Queueing videos | Two files of one movie legitimately share a provider id. Excluded by construction, with a named test |
| Backfilling the missing spine rows | Repairing `entity_external_ids` is a merge decision, not a sweep's to make. The probe sizes it (§7); it does not act on it |

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
- **RD7 — `entity_keep_separate` is honored identically.** A dismissed pair is never re-proposed,
  by either producer.

## User Stories

- As the owner, when a provider tells me two of my people are one record, I see that pair **above**
  the near-miss pairs, labelled with who asserted it, instead of never seeing it at all.
- As the owner, when I pick the wrong person in the enrich UI, the system tells me by queueing the
  pair, instead of silently discarding the identity write and leaving two half-enriched records.
- As the owner, a `shared-external-id` person pair opens into the same compare panel as every other
  pair, with no new controls to learn.

## Requirements

### Must-Have (P0)

**P0-1 — the detector query.** A pair is a finding when an entity's newest memo for a provider
carries an `external_id` that `entity_external_ids` assigns to a different entity of the same
kind. The four reading rules of ADR-107 D6 are mandatory:
`external_id <> ''` (excludes the whole `filename` extract population);
newest `fetched_at` per `(entity_type, entity_id, provider)`, ties broken toward the memo that
*agrees* with the spine so a tie yields no finding;
`entity_type IN ('person','studio','film')`;
and `entity_keep_separate` excluded.

*Acceptance*: given the fixture in `scripts/detect_shared_external_id.sql`'s header — an agreeing
memo, a disagreeing memo, a stale narrow re-enrich, a `filename` empty memo, a kept-separate pair,
and two videos sharing an id — exactly the disagreeing person pair and the disagreeing studio pair
are findings, and the kept-separate pair is a finding that the queue write suppresses.

**P0-2 — the queue row.** A finding inserts `identity_review_queue (entity_type, id_lo, id_hi,
variation = 'shared-external-id')` with `INSERT OR IGNORE`, so both producers and repeated runs are
idempotent. `detail` is left empty: unlike `provider-alias`, both sides of this pair are readable
from the entities themselves.

*Acceptance*: running the sweep twice inserts on the first pass and reports 0 on the second.

**P0-3 — the write-time guard.** `Repo.AttachExternalID` stops being a bare `INSERT OR IGNORE`.
Under `writeMu` it reads the current owner; if the id belongs to a different entity of the same
kind it queues the pair and returns without error. The enrich must still succeed — the value is
written, only the identity claim is contested.

*Acceptance*: enriching person B against an id person A already owns queues `(A, B)` as
`shared-external-id`, returns 200, and B's enrichment values are written. The private scan-path
`attachExternalID` is unchanged in behaviour because id-first resolve means the owner is already
the entity — asserted by a test, not assumed.

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

**P0-7 — the stale rationale is rewritten.** `internal/repo/identity.go:253`'s comment ("this only
ever records a genuinely new (id, entity) pair") and `AttachExternalID`'s doc comment are both
false once P0-3 lands and must be corrected in the same change.

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
- **P1-1 — size the orphaned memos.** Probe §7 counts memos whose id has *no* spine row at all —
  the other half of the same silent failure. Not a duplicate pair, but it says whether a repair
  pass is needed.

### Future Considerations (P2)

- ADR-096 D2's deferred drop of `entity_enrichment.external_id`, with the video re-enrich memo
  re-homed. Until it lands, this feature makes the drift observable rather than impossible
  (ADR-107 D2).
- A provider-side duplicate is the mirror case: one entity of ours matching two provider records.
  Nothing detects it and nothing here does either.

## Data model

No migration. `identity_review_queue` gains a fourth `variation` value in an existing `TEXT`
column; `detail` (0045) stays empty for it. One new `job_runs` kind constant.

## API

None. `GET /owner/duplicates` already returns `variation` verbatim, and the dismiss path is
unchanged.

## UI

One chip, keyed on `variation`, in `DuplicatePairRow`'s existing variation span — a sibling of
`MATCH_KIND_LABEL`, not an entry in it, because that map is keyed on the derived `match_kind`.
`web/src/lib/types.ts`'s `variation` gains the value. Full rationale, placement and the three-skin
QA list are in the [design handoff](../design/duplicates-shared-external-id-handoff.md).

## Success Metrics

- The 6 measured person collisions are queued and decidable; the number of undetected collisions
  stops growing.
- The set is disjoint from the name-based queue (probe §3 returns 0) — this is new coverage, not
  a re-cut of what is already there.
- No false positive from a stale narrow re-enrich (probe §4a > 0 with §1 unaffected by it).

## Open Questions

- **OQ1 — what are the real numbers for studio and film?** Unknown. The local
  `data/holodex.db` is a 72-person dev database that has not run migration 0046, so
  `scripts/detect_shared_external_id.sql` must be run **on the host** before implementation. If
  either is zero the code still ships (the hole is structural), but the probe output belongs in
  this spec.
- **OQ2 — how many memos have no spine row at all?** Probe §7. If it is large, the silent no-op has
  been dropping identity writes far more often than the 6 collisions suggest, and a repair pass
  becomes a P0 rather than a P1.
- **OQ3 — does any memo violate the `<ns>:<id>` grammar?** Nothing enforces it at the
  `entity_enrichment` write (`internal/enrich/service.go` passes the provider's string through);
  only `identityShaped()` checks it downstream. Probe §4c. A bare id would silently never match the
  spine, making the detector blind for that provider.

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
