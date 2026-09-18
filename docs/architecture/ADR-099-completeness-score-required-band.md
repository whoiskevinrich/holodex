# ADR-099: Completeness Score v2 — Required-Band Score, Extras Overfill, and a Trigger-Invalidated Materialized Store

**Status:** Proposed
**Date:** 2026-09-18
**Deciders:** Kevin Rich
**Supersedes:** ADR-081 **D3** (score formula and compute-on-read for lists) and **D4** (list-wide full-scan consumption) — only those two; D1 (criticality on `registry.FieldDef`, as revisited by ADR-096 D4), D2 (not-applicable table) and D5 (as superseded by ADR-082) stand
**Extends:** ADR-051 (per-field source-of-truth decisions), ADR-063 (derived fields — the "twin of `Derive`" framing), ADR-096 D4 (the `optional` criticality value)
**Relates to:** ADR-013 (field mapping reload), ADR-068/073 (post-write derived-state hooks), ADR-045 (owner-session gating)
**Contract:** owner-only `completeness` object on list items (video / person / studio); detail `completeness.score` re-defined as the required band and `completeness.extras` added — see D5
**Spec:** [entity-completeness-score.md](../specs/entity-completeness-score.md) (F55, amended in place — F65 is the change, not a new spec)
**Issue:** HOLODEX-412

## Context

ADR-081 shipped the F55 completeness score as `round(100 × Σ(weight × tier) / Σ(weight))` over every
scored facet, with critical facets weighted 3, nice-to-have 1, and a source tier of 0 / 0.7 / 1.0
(missing / provider / curated). The owner uses the score for one job — sort the library to find what
still needs work — and after a month of use it is not doing that job:

- **Aggregate weight inverts the per-facet intent.** The video table has 4 critical and 12
  nice-to-have facets, so 4 × 3 = 12 = 12 × 1: the two bands carry *equal* aggregate weight even
  though each critical facet is worth three nice-to-haves. Every required facet curated and nothing
  else scores **50**; a missing poster with everything else curated scores **88**. Seven missing
  extras cost 29 points; a missing *critical* costs 12. The owner's report — "a media file can have
  all required facets and still sit at 69%" — is this inversion, not a facet-list problem.
- **The provider tier smuggles provenance into a doneness number.** A video whose four critical
  facets are all TMDB-resolved scores 70 × (its share) even though, for the job of filling gaps,
  provider-resolved *is* done. Provenance already has a home — the breakdown panel's
  `ProvenanceBadge` (ADR-081 design handoff DD7) — and does not need a second, lossy one in the sort.
- **Facets that never fill tax every entity equally.** TMDB-shaped video facets (`status`,
  `tagline`, `homepage`, `collection`, `original_title`, `original_language`) are structurally
  empty for non-theatrical content. A constant tax cancels out of the *order* but not the *number*,
  and the number is about to be drawn on every card.
- **There is no at-a-glance surface.** The score exists only as a sort order. The list path
  (`listMediaByCompleteness`, `writeCompletenessList`) computes every score and then maps each
  scored item back to its plain entity before writing the response — the value is discarded.
- **"Always on" breaks D4's cost assumption.** The owner wants the score visible on every entity
  list page in owner mode, not only under the completeness sort. D4's full-scan-per-request was
  bounded by *"only when the owner asks for that sort"*; on every owner list page, it is a full
  resolve of the library per page load, per pagination step, per filter change.

### Forces

- The resolver stays pure and the sole merge point (ADR-033/051/052); the scorer stays a pure
  post-pass over `ResolvedField` (ADR-081 D3's shape survives even though its formula does not).
- **Staleness was D3's stated reason for rejecting a stored score**: "a stored score needs
  invalidation hooks scattered across every mutation path … and a missed hook silently shows a
  stale number." The write surface that can change a score is genuinely wide — an inventory for
  this ADR found ten input groups and ~30 API/scanner/boot call sites (file layer, enrichment,
  decisions, curation, not-applicable, `video_people`/`video_studios`, `film_videos`,
  person/studio images, `video_tags`, promotions/claims), all behind per-method `writeMu`
  acquisitions with **no single choke point**. Any stored design must answer D3's objection
  *by construction*, not by promising to remember every hook.
- The codebase already maintains derived read-structures with **SQL triggers** — the FTS tables
  (`videos_fts`, `people_fts`, `tags_fts`, migrations 0001/0007/0017). D4's "a background-job /
  trigger seam this codebase doesn't otherwise have" was true for *recompute-in-trigger*; it is
  not true for *notify-in-trigger*.
- Owner-only. Nothing here is visible to a visitor; a visitor list page must pay nothing.
- Personal-library scale, single process, single writer. No multi-node cache coherence.
- Three skins, tokens only. The badge must use existing tokens.

## Decision

### D1: The score is the required band alone; extras are a separate number, never blended

`resolver.Complete` returns two integers, each 0–100:

```
required = round(100 × present_critical / applicable_critical)
extras   = round(100 × present_nice_to_have / applicable_nice_to_have)
```

where *present* is **binary** — `WinningSource != ""` — and *applicable* excludes facets in
`facet_not_applicable` (ADR-081 D2, unchanged). The 0.7 provider tier is gone from scoring;
`FacetScore.Tier` is still emitted for the panel, which keeps rendering provenance.

`required` is **the score**: the number in the panel, the ring on the card, the primary sort key.
`extras` is the tiebreaker and the overfill. They are never summed, averaged or weighted together —
there is no single blended number anywhere in the payload.

Edge rules:

| Case | Rule |
|---|---|
| No applicable critical facets (studios today — "everything is nice to have") | `required` is reported as `null`; `extras` **is** the ring and the primary sort key for that entity type. Not vacuously 100 — a studio with nothing filled must not draw a full ring. |
| No applicable nice-to-have facets | `extras` is `null`; no overfill is ever drawn. |
| `optional` facets (ADR-096 D4) | Listed, never in either band — unchanged. |
| Actionability | Unchanged (ADR-081 D3): missing-with-cached-candidate over missing, queue-only, computed live in the queue path. |

**Chosen over:** a 75/25 band split (`0.75 × required + 0.25 × extras`). It fixes the inversion
(all-required floors at 75) but keeps one blended number, so the ring still cannot say "the
required set is done" without a legend. **Chosen over:** keeping the formula and only demoting the
noise facets — with 6 nice-to-haves left the aggregate is 12 vs 6, still enough for extras to
out-vote a missing critical.

### D2: The completeness sort is the composite key `(required, extras)`

`completeness_asc` / `completeness_desc` order by `required` (with the D1 `null` rule per entity
type) and break ties by `extras`, then by the type's default order. No new sort values; the
existing `SortDropdown` option keeps its name.

### D3: The score is materialized in an entity-generic store — superseding ADR-081 D4

Two new tables, keyed the way `facet_not_applicable` is:

```sql
entity_completeness(entity_type, entity_id, required INTEGER NULL, extras INTEGER NULL,
                    computed_at TEXT, PRIMARY KEY (entity_type, entity_id))
entity_completeness_missing(entity_type, entity_id, canonical TEXT, band TEXT,
                            PRIMARY KEY (entity_type, entity_id, canonical))
```

`entity_completeness` backs the badge and the sort (`ORDER BY required, extras` in SQL, so the
list path returns to `LIMIT`/`OFFSET` paging like every other sort). `entity_completeness_missing`
backs the "Missing facet" filter chip and its per-facet counts (`GET /completeness/facets` becomes
a `GROUP BY canonical`), which keeps F55.6's "the chip's counts and what selecting a facet filters
to come from the same data" — now the same *rows* rather than the same pass. The remediation queue
(F55.7) keeps its live full-scan: it needs actionability, whose inputs (cached candidates) are not
in the store, and it is one owner page, not every owner page.

Rows are a **cache of `resolver.Complete`'s output**, never a source of truth. The detail page
keeps computing live (the panel is always right) and, when its live result differs from the stored
row, writes the row — a self-heal on view, so a stale badge can never outlive a look at the entity.

**Chosen over:** columns on `videos` / `people` / `studios`. One generic table is what the
resolver's entity-generic model wants, is what D2 and the review queues already do, and adds a
fourth entity type without a migration per table.

### D4: Invalidation is by SQL trigger into a dirty set, drained lazily on the next owner read

The staleness objection in ADR-081 D3 is answered by not asking Go to remember:

- A migration adds `completeness_dirty(entity_type, entity_id, PRIMARY KEY (entity_type, entity_id))`
  and `AFTER INSERT / UPDATE / DELETE` triggers on **every table `Complete`'s inputs are read
  from**, each doing `INSERT OR IGNORE INTO completeness_dirty` for the entity the row belongs to
  (`video_metadata.video_id`, `entity_enrichment.(entity_type, entity_id)`, `video_people.video_id`,
  `person_images.person_id`, …). Link tables dirty the *video* side only — no person or studio
  facet reads a link. The migration is the exhaustive list; a test enumerates the input tables and
  asserts that one write to each leaves a dirty row, so a future input table cannot be added
  without extending the trigger set.
- **Denominator changes dirty everything** with one `INSERT OR IGNORE … SELECT id FROM <type>`
  per entity type: at boot (registry criticality is compiled in, so a build that re-tags a facet is
  a boot), on `mapping.Store.Reload` (the fields list), and in `SetPromotion` / `ClearPromotion` /
  `SetClaim` / `ClearClaim` — the four writes that change *which* fields exist rather than a
  value. These four are the only Go-side hooks, and they sit beside the atomic-pointer cache drop
  those methods already do (`repo.go:40-45`).
- **Draining** happens at the top of the owner-gated paths that read the store — list with
  `completeness` requested, `/completeness/facets`, the detail page — under `writeMu`: select the
  dirty set, resolve + `Complete` each entity (the existing batch loaders from
  `completenessForVideos` / people / studios, restricted to the dirty ids), upsert the rows, delete
  the drained ids. Cost is O(entities mutated since the last owner read), which during a curation
  session is one entity per click. A visitor request never drains and never pays.

Why not recompute at write time in Go: that is exactly the scattered-hook design D3 rejected, and
the inventory confirms the fear (ten groups, ~30 sites, some inside scanner and boot backfills).
Why not recompute *in* the trigger: the score is a resolver output — precedence, decisions,
curation, mapping — and cannot be expressed in SQL without duplicating the resolver, which
ADR-051/052 forbid. Why not a global generation counter with an in-process cache: it is the
smallest possible version of this design (one trigger per table, no per-entity rows) but it throws
away the whole cache on every write, so every click in a curation session is followed by a full
library resolve on the next list load — the per-page cost this ADR exists to remove, moved one
request later.

### D5: The list payload carries `completeness` for the owner; the badge rides it

Video / person / studio list items gain an owner-only object:

```json
"completeness": { "required": 75, "extras": 100 }
```

(`null` for a band with no applicable facets, per D1). It is populated only when the requester
passes the owner gate — the same `redactFileMetadataForVisitors` seam that strips file metadata
today drops it for a visitor — and is present on **every** owner list response, not only under the
completeness sort, because that is what "always on" means. No per-card fetch; no new endpoint.

The detail `completeness` object keeps `score` as its name for the required band (so the panel and
the deep-link code path do not churn) and adds `extras`. `facets[]` is unchanged.

### D6: Noise facets are muted by demoting them to `optional` in the registry

The lever is ADR-096 D4's `CriticalityOptional` — listed, never scored, never queued. The exact
list is a spec decision (F55 § Facet tables, amended by F65), not an architecture one. **Chosen
over:** a per-deployment `criticality:` override key in `metadata-mappings.yaml` (the smallest
form of F55.14) — a real option, deferred with F55.14 rather than shipped piecemeal, because the
D1 formula removes the pressure that made per-deployment tuning attractive: an extra that never
fills no longer touches the score. **Chosen over:** auto-muting facets that are missing
library-wide — the denominator would move as the library grows, and a badge whose scale changes
under the owner cannot be explained.

## Options Considered

### D1: aggregation

| Option | Fixes the inversion | One honest number for the ring | Sort still discriminates within "required done" |
|---|---|---|---|
| Keep Σ(weight × tier), demote noise facets | No — 12 vs 6 aggregate | No | Yes |
| 75/25 band split | Yes (floor 75) | No — still blended | Yes |
| **Required band + separate extras (chosen)** | Yes | Yes — the ring is the required band | Yes — extras is the tiebreaker |
| Required band only, extras unscored | Yes | Yes | No — ~5 buckets per entity type |

### D3/D4: storage and invalidation

| Option | New seams | Coverage of write paths | Cost per owner list page | Cost per mutation |
|---|---|---|---|---|
| Compute on read, full scan on every owner list page (D4 as-is, widened) | None | N/A | Full library resolve | None |
| In-process cache + global generation via one trigger per input table | Triggers | Complete | Free between writes | Full library resolve on the next read |
| Materialized rows, recomputed on write in Go | ~30 hooks | Whatever we remembered | Free | One entity |
| **Materialized rows, trigger-fed dirty set, lazy drain (chosen)** | Triggers + 3 tables + 4 Go hooks | Complete, test-enforced | Free (plus the drain) | One entity, deferred to the next owner read |

### D5: badge data path

| Option | Requests per grid | Owner gating |
|---|---|---|
| **Field on the list item (chosen)** | 0 extra | Existing redaction seam |
| Batch endpoint `GET /completeness?ids=` | 1 extra per grid | New endpoint to gate |
| Per-card fetch | N | New endpoint to gate |

## Trade-off Analysis

- **Coverage vs. novelty.** Triggers are the only mechanism that covers every current *and future*
  write path without a Go author remembering it, and the FTS triggers mean they are not a new kind
  of thing in this schema. The price is that the input-table list lives in SQL, where a Go reader
  of `Complete` will not see it — hence the enumerating test and the "the migration is the list"
  rule.
- **Lazy drain vs. write-time recompute.** Draining on read keeps the write paths untouched and
  makes the cost proportional to mutations, but it puts a resolve inside a read request. The drain
  runs only on owner paths, under `writeMu`, on the dirty subset; the worst case (boot, reload —
  everything dirty) is one full-library resolve on the first owner read after, which is today's
  per-request cost paid once.
- **`null` over vacuous 100.** Reporting `required = null` for studios pushes a per-type branch
  into the sort and the badge, but the alternative — a full ring on an empty studio — is exactly
  the lie D1 exists to stop.
- **Binary tier loses information in the sort.** Two videos with identical presence but different
  provenance now tie. That is intended: provenance is the panel's job, and the extras tiebreaker
  still orders within a band.
- **Self-heal on the detail page** is belt-and-braces over the triggers, not a substitute. It
  costs one compare per detail view and closes the only gap the triggers cannot: a bug in the
  trigger set itself.

## Consequences

- **Easier:** the list path drops its full-scan branch — completeness becomes an ordinary SQL sort
  with normal paging; the badge is free on every owner list; the facet counts are a `GROUP BY`.
- **Easier:** the number means one thing. "Required done" reads as a full ring on every entity
  type that has a required band.
- **Harder:** every new input to `Complete` needs a trigger, and the enumerating test will fail
  until it gets one — that is the mechanism working.
- **Harder:** person rings are 0 or 100 (`photo` is the only critical person facet). If that
  proves too coarse, the fix is promoting a facet to critical in the registry — a boot-time
  mark-all-dirty, not a schema change.
- **Changed behavior to state in the spec:** the completeness sort and the breakdown panel's
  `score` both change value for every entity on upgrade; the queue's grouping is unaffected.
- **Revisit:** if a fourth scored entity type arrives (films are not scored today), it is one
  more `entity_type` value in three tables plus its own trigger set. If the drain ever shows up in
  request latency, moving it to a post-write goroutine keyed off the same dirty table is a local
  change — the dirty set is the contract, the drain site is not.

## Action Items

1. [ ] Amend `docs/specs/entity-completeness-score.md` (F65): scoring model, the `null` rules,
       the composite sort, the demoted-facet list, the badge, the owner-only list field.
2. [ ] Migration `0048`: `entity_completeness`, `entity_completeness_missing`,
       `completeness_dirty`, and the trigger set over every input table; manual down drops all.
3. [ ] `resolver.Complete`: `Required` / `Extras` (`*int`), binary presence, per-band
       not-applicable exclusion; keep `Facets[]` and actionability.
4. [ ] Repo: dirty-set drain + upsert; mark-all on boot, `mapping.Store.Reload`, promotions and
       claims writes; enumerating trigger-coverage test.
5. [ ] API: list paths read the store (SQL sort + paging), owner-only `completeness` on items,
       `/completeness/facets` from `entity_completeness_missing`, detail self-heal.
6. [ ] Registry: demote the spec's noise-facet list to `CriticalityOptional`.
7. [ ] Design handoff (`/design-handoff`): ring badge, O2 second-lap overfill, committed SVG.
8. [ ] `/testing-strategy`, `/security-review` (owner gating of the new list field; trigger
       migration), README index row, ADR-081 index row cross-reference.
