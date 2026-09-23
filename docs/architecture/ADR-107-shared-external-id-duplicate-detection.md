# ADR-107: A shared provider external id is detected as the memo disagreeing with the identity spine

**Status:** Proposed (extends ADR-061 D5 and ADR-088's queue-don't-fail posture; **amends the
timing, not the substance, of ADR-096 D2's "drop the memo column"**; relates ADR-028/033/051;
spec F71; HOLODEX-452)

**Date:** 2026-09-23

## Context

Duplicate detection is entirely name-based. `SeedIdentityReviewQueue`
(`internal/repo/identity_queue.go`) folds every canonical name ∪ alias to a loose key and pairs
entities whose keys collide; the three incremental producers — `FlagNearMiss`
(`internal/repo/review_queue.go:89`), `queueProviderAliasPair`
(`internal/repo/provider_aliases.go:174`) and `queueFilmSameTitle` (`internal/repo/films.go:297`)
— are all name-driven too. Nothing in the system can see two entities that carry the **same
provider external id**, which is the strongest positive evidence available: the provider has
already asserted the two are one record.

Measured on the live library 2026-09-22 (1591 people, 1000 enriched, 31 queued person pairs,
`scripts/detect_person_duplicate_evidence.sql`): **6 external ids are claimed by more than one
person**, none of them queued, and `same_xid = 0` across all 31 pairs currently in the queue. The
detector finds a **disjoint and much higher-confidence set** than the name-based one — which
matters, because all 31 queued pairs are `provider-alias` / `alias`, the weakest kind the current
detector produces, and 185 person pairs have already been dismissed keep-separate.

### The hole is a silent no-op on the enrich path, not a missing query

Provider identity lives in two places with two meanings (ADR-096's inventory):

| | Store | Uniqueness |
|---|---|---|
| Identity — consulted **before** name | `entity_external_ids` (0046) | `PRIMARY KEY (entity_type, external_id)` — an id owns exactly one entity per kind |
| Re-enrich memo | `entity_enrichment.external_id` (0005) | none; one row per `(entity_type, entity_id, provider, field_key)` |

`resolveOrCreateByName` is **id-first** (`internal/repo/identity.go:162`), so on the scan path a
provider id that already has an owner returns that owner and the name is never consulted. The
writer, `attachExternalID` (`internal/repo/identity.go:258`), is `INSERT OR IGNORE`, and its own
comment says why that is safe: *"the id-first lookup in `resolveOrCreateByName` would already have
returned that owner, so this only ever records a genuinely new (id, entity) pair."*

**That premise does not hold on the enrich path.** `runEnrich` calls the public
`Repo.AttachExternalID` (`internal/repo/identity.go:274`, from
`internal/enrich/service.go:695`) with an entity id the **owner chose in the UI**, not one
id-first resolve produced. When that id already belongs to another entity of the same kind the
`INSERT OR IGNORE` silently does nothing — no error, no warning, no review row — while
`UpsertEnrichment` (`internal/repo/enrichment.go:48`) stamps the colliding id onto every field row
of the second entity regardless. The 6 collisions are the residue of that no-op.

Studio and film run the identical `runEnrich` → `AttachExternalID` path, so the hole is theirs
too. Video is different in kind: it is enrichable but excluded from `identityEntityType`
(`internal/enrich/service.go:921`), so it has no spine row at all, and two files of the same movie
**legitimately** share `tmdb:841`. Tag is not enrichable (`internal/model/model.go:482`).

### ADR-096 D2 predicted exactly this drift

D2 chose *"one polymorphic `entity_external_ids` table replaces two per-kind tables **and the memo
column**"*, and explicitly rejected option C — *keep the memo column as a per-provider
"last enriched" cache* — on the grounds that **"two copies drift — the confusion this ADR exists
to remove."** Migration 0046 then deferred the drop: *"The re-enrich memo is NOT dropped here — it
also serves video re-enrich, which has no row in this table."*

So the 6 collisions are not an unforeseen bug. They are the drift D2 named, kept alive by a
deferral D2 left open (ADR-096's own action item: *"decide in-PR whether
`entity_enrichment.external_id` drops in the same migration"*). Any decision here has to say what
the memo column **is**, or it entrenches the thing D2 condemned.

### What is already true, and cheap

- The memo is **already namespace-qualified** — `tmdb:1100`, the same grammar
  `entity_external_ids` stores. A memo value is directly comparable to the identity table with no
  string building, and the probe's `GROUP BY (provider, external_id)` is redundant.
- A merge **deletes the loser's enrichment rows** (`internal/repo/identity_ops.go:451`), so a
  merge can never strand a memo. The only way a memo can be stale is a re-enrich that wrote a
  *narrower* field set than a previous pass, leaving older rows untouched — resolved by reading
  the newest `fetched_at` per `(entity, provider)`.
- `entity_enrichment` carries `external_id = ''` for the whole `filename` extract population
  (`internal/extract/store.go:35`), which every query must exclude.
- A new `variation` value falls through `ListReviewQueue`'s non-fuzzy branch untouched
  (`internal/repo/review_queue.go:140`) — no name re-validation, which is correct here — and sorts
  at `-1`, top of its group, sharing that slot with `provider-alias` and `same-title`.

## Decision

### 1. The signal is the memo disagreeing with the spine, not a memo self-join

A pair is queued when an entity's newest memo for a provider carries an `external_id` that
`entity_external_ids` assigns to a **different entity of the same kind**. One `LEFT JOIN` against
a `PRIMARY KEY`, no self-join, no grouping.

This is the whole of the ticket's evidence, because the spine's PK guarantees a colliding id has
exactly one owner: if A and B both memoize `tmdb:287`, one of them owns the spine row and the
other is the no-op victim. Reading the disagreement names the survivor and the victim, which a
memo self-join does not.

**Rejected: the memo self-join** (`GROUP BY external_id HAVING count(DISTINCT entity_id) > 1`,
the ticket as filed, and what the probe measures). It works, but it makes the memo column a
*source of truth about identity* — precisely ADR-096 D2's rejected option C — and it cannot tell
which side the spine already believes.

### 2. The memo column is a witness, not a second source of truth — and its drop stays deferred

`entity_enrichment.external_id` keeps its ADR-096 meaning: **a re-enrich memo, and nothing else.**
This ADR does not resurrect option C, because the detector never treats the memo as an assertion
about who an entity *is*; it treats a memo/spine disagreement as evidence that **a write was
silently dropped**. The memo is the witness to the no-op, not a rival identity store.

The drop itself stays deferred, for the reason 0046 recorded: video re-enrich has no spine row and
still needs the memo. Closing it is a separate change with its own migration and a re-homed video
memo — filed as a follow-up, not folded in here, because doing both at once would couple a
detector the owner wants now to a migration touching every enriched entity.

**Consequence to state plainly:** until that drop lands, this ADR makes the drift *observable*
rather than *impossible*. Decision 4 closes the source; decision 3 drains what the source already
produced.

### 3. It queues. It never merges.

A shared external id inserts an `identity_review_queue` row with a new
`variation = 'shared-external-id'`. It never merges, and it never auto-applies.

Three reasons, in order of weight: the merge is irreversible (ADR-061); providers carry their own
duplicate entries, so even the strongest positive evidence is not proof; and — decisively — **the
colliding data was produced by an owner picking the wrong entity in the enrich UI**, so
auto-merging would let one mis-click fold two real people together with no review step. F70's rule
governs the surface it lands on: *positive evidence can be decisive, but the panel reports, it
never adjudicates.*

The pair inherits the F70 compare panel for free on person rows, so this ships **no new review
surface** — only a label. `entity_keep_separate` is honored exactly as the name-based detector
honors it: a dismissed pair is never re-proposed.

Because a `shared-external-id` row is the strongest signal in the queue and the existing
`text-warn` emphasis means *weak*, the row names its asserter in an accent chip
(`tmdb says one person`) rather than printing the raw variation slug. See the F71 spec and
`docs/design/duplicates-shared-external-id-mockup.svg`.

### 4. Two producers: close the source, then drain the backlog

**Write-time (the source).** `Repo.AttachExternalID` stops being a silent `INSERT OR IGNORE`.
Under `writeMu` it reads the current owner first; if the id belongs to a different entity of the
same kind it queues the pair instead of discarding the write. The private scan-path
`attachExternalID` can share the implementation — after id-first resolve the owner *is* the
entity, so the collision branch simply never fires there. This converts a data-losing no-op into a
review row, which is the same posture ADR-088 took for an alias another entity already holds:
**queue it, never fail the enrich and never merge silently.**

**Boot sweep (the backlog).** A sweep over the memo/spine disagreement, wired beside
`seedIdentityReviewQueue` in `cmd/holodex/main.go` and recorded as its own `job_runs` kind so the
activity surface (ADR-028) shows it. Unlike the identity backfill it is **not** gated on
`HasSuccessfulJobRun`: that gate exists because the F43 fold was a one-time historical
normalization, whereas this sweep is a cheap idempotent reconciliation whose input keeps changing.
`INSERT OR IGNORE` on the queue PK makes re-running it free.

### 5. Person, studio and film. Video and tag are excluded by construction.

All three enrichable spine kinds run the same path and have the same hole, so scoping to person
would knowingly leave two open. **Video is excluded** — it has no `entity_external_ids` row, and
two files of one movie sharing a provider id is correct, not a duplicate; a detector that queued
those would be wrong by construction, not merely noisy. **Tag is excluded** because it is not
enrichable. Both exclusions are named tests, not comments.

### 6. Reading rules the detector must apply

1. `external_id <> ''` — excludes the whole `filename` extract population.
2. Newest `fetched_at` per `(entity_type, entity_id, provider)` — a narrower re-enrich leaves
   older rows behind; a tie is broken toward the memo that **agrees** with the spine, so a tie
   yields no finding.
3. `entity_type IN ('person','studio','film')`.
4. Skip pairs in `entity_keep_separate`.

## Options Considered

| | Uses one source of truth | Names which side the spine believes | Stops new collisions | Honors ADR-096 D2 |
|---|---|---|---|---|
| **A. Memo ⋈ spine disagreement + write-time guard (chosen)** | ✅ spine is truth, memo is a witness | ✅ | ✅ | ✅ timing amended, substance kept |
| B. Memo self-join (ticket as filed) | ❌ memo becomes an identity store | ❌ | ❌ detector only | ❌ this is rejected option C |
| C. Auto-merge on a shared id | ✅ | ✅ | ✅ | ✅ |
| D. Drop the memo column in this change | ✅ | ✅ | ✅ | ✅ fully closes it |
| E. Add `UNIQUE` to the memo column | ❌ | ❌ | ⚠️ by failing the enrich | ❌ |

- **C** is rejected on decision 3's grounds: irreversible, and it automates a fold on data whose
  provenance is an owner's mis-click.
- **D** is right eventually and is the follow-up, but it couples a detector wanted now to a
  migration over every enriched entity plus a re-homing of the video memo.
- **E** would make the drift impossible but converts a silent no-op into a **failed enrich**,
  which is the opposite of the queue-don't-fail posture ADR-088 established, and it cannot express
  the legitimate video case at all.

## Trade-off Analysis

**What this buys.** The strongest merge evidence in the system becomes visible for the first time,
on a surface that already knows how to adjudicate it, at the cost of one query, one variation
string and one chip. The write-time guard means the number of undetected collisions stops growing
the moment it ships.

**What it costs.** The memo column survives a little longer in a role ADR-096 wanted retired, and
this ADR is the thing that makes that survival defensible — which is a risk, because "temporarily
justified" is how a rejected option comes back. Decision 2 is written to bound it: the memo is
never read as identity, and the drop is a filed follow-up, not a vague intention.

**Failure mode if the reading rules are wrong.** A stale narrow-field memo would queue a pair that
is genuinely two entities. The cost is one keep-separate click and a durable dismissal — the same
cost the name-based queue already imposes 185 times over — so the rules fail safe.

## Consequences

- `identity_review_queue` gains a fourth non-fuzzy variation. It sorts at `-1` alongside
  `provider-alias` and `same-title` with no tiebreak (`internal/repo/review_queue.go:193`); F71
  adds one so the strongest signal sorts above the weakest.
- `Repo.AttachExternalID` gains a side effect. Its doc comment and
  `internal/repo/identity.go:253`'s "this only ever records a genuinely new pair" rationale both
  become wrong as written and must be rewritten in the same change.
- `web/src/lib/types.ts`'s `match_kind` union and the `MATCH_KIND_LABEL` map
  (`web/src/lib/components/duplicates/queue.ts:35`) gain the new value; `labelPlacement` already
  routes person → panel, every other kind → row, so studio and film get the chip in the row for
  free.
- The activity surface gains one job kind.
- A follow-up ticket carries ADR-096 D2's deferred drop of `entity_enrichment.external_id`,
  including re-homing the video re-enrich memo.

## Action Items

1. `scripts/detect_shared_external_id.sql` — a host-runnable probe measuring the disagreement for
   all three kinds, plus the OQ3 consistency check. **Must run on the host before implementation**:
   the local `data/holodex.db` is a 72-person dev database that has not run migration 0046.
2. Spec F71 (`docs/specs/duplicates-shared-external-id.md`) with acceptance criteria per decision.
3. Named tests for the two exclusions (video, tag) and for the write-time guard replacing the
   no-op.
4. File the ADR-096 D2 drop follow-up before this merges, so the deferral has an owner.
