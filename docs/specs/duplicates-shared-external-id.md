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

Measured on the live library **2026-09-23** (`scripts/detect_shared_external_id.sql`; 1577 people,
556 studios, 48 films, 1000 enriched, 33 queued, 185 keep-separate):

| | pairs | not already queued or dismissed |
|---|---|---|
| person | 9 | 4 |
| studio | 6 | 6 |
| film | 0 | 0 |

**15 pairs, 10 new, and exactly one already in the name-based queue** — so this is near-disjoint
new coverage. Meanwhile all 33 queued pairs are `provider-alias` / `alias`, the weakest kind the
current detector produces. The owner is working a queue of near-certain false positives while the
near-certain true positives are invisible.

Two numbers from the same run set the shape of the work. Excluding `video` suppressed **23** ids
shared by multiple files — more wrong findings than there are right ones, which is why that
exclusion is a named test rather than a comment. And **1219 person memo/provider pairs carry an id
with no `entity_external_ids` row at all** (plus 28 for film): the same dropped write, in its
larger and quieter form.

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

- **HOLODEX-457** — ADR-096 D2's deferred drop of `entity_enrichment.external_id`, with the video
  re-enrich memo re-homed. Until it lands, this feature makes the drift observable rather than impossible
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

Probe run on the host 2026-09-23. OQ1 and OQ3 are closed; OQ2 is half-closed and got bigger.

- ~~**OQ1 — real numbers for studio and film?**~~ **Closed: studio 6 pairs (all new), film 0.**
  Film has only 48 rows and 28 memos with no spine row, so its zero is "not yet exercised", not
  "not affected" — it runs the same code path and stays in scope.
- **OQ2 — memos with no spine row: 1219 person, 28 film.** Against 1000 enriched people that is
  close to *every* enriched person, which is too large to read as ordinary loss and needs one more
  query to interpret before P1-1 is sized (see below). Either the enrich path almost never lands
  an identity row — in which case a repair pass is a P0 and **HOLODEX-457 must not drop the column
  that is currently the only record of those ids** — or the two stores disagree on the id string
  for some providers, in which case the detector is blind for them and P0-1 needs normalizing.
  The 15 findings prove exact-string matching works for at least two providers, so the second
  explanation cannot be the whole story.
- ~~**OQ3 — memo consistent per (entity, provider)?**~~ **Closed, clean.** Zero groups with more
  than one distinct memo id, zero mixing empty with non-empty, zero breaking `<ns>:<id>`. The
  newest-`fetched_at` rule stays as belt-and-braces (ADR-107 D6).
- **OQ4 (new) — does a stronger signal re-open a dismissed pair?** **4 of the 9 person findings
  are already `entity_keep_separate`.** Those dismissals were made against `provider-alias` /
  `alias` evidence — the weakest the queue produces — and `entity_keep_separate` records the
  *pair*, not the reason, so RD7 currently suppresses the strongest evidence in the system on the
  strength of a verdict reached without it. Options: honour the dismissal as-is (RD7 today);
  re-queue a dismissed pair when a *new, stronger* variation appears; or surface the 4 once,
  out-of-queue, as a one-time reconciliation. **Owner's call — blocks nothing else.**
- **OQ5 (new) — §6 returned 5 rows for 9 person pairs.** The co-appearance query should emit one
  row per pair. Either the pasted output was trimmed or something drops rows; re-run §6 alone
  before P1-0 is built on it. (Of the 5 shown, 3 pairs share a video — which is real supporting
  evidence, if the query is sound.)

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
