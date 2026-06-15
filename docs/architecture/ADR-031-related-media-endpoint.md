# ADR-031: Related-media endpoint — one combined call, random selection seam

**Status**: Proposed
**Date**: 2026-06-14
**Deciders**: Project owner
**Extends**: ADR-006 (REST + chi), ADR-017 (query architecture), ADR-003 (SQLite + FTS5)
**Spec**: [Quick Wins — QW2](../specs/quick-wins.md)

---

## Context

The media detail page is a dead end: opening an item offers no path to "more like
this." The Quick Wins spec (QW2/QW3) adds two "More with …" shelves — one keyed to a
person in the item, one to the item's most distinctive tag — fed by a single new endpoint.

The query layer already has the pieces. `internal/repo/repo.go` builds filtered
video queries via `VideoFilter.build()`, which emits `EXISTS` subqueries over the
`video_people` / `video_tags` join tables (`0001_init`: `video_people(video_id,
person_id)` idx on `person_id`; `video_tags(video_id, tag_id)` idx on `tag_id`).
`ListPeople` / `ListTags` already surface a global `VideoCount` per entity via
`namedCountQuery`, and `attachAssociations()` does the batched IN-query attach of
people/tags onto a set of videos (no N+1). What does **not** exist yet is any
**randomly-ordered** query — every existing read is deterministically ordered
(`orderBy()`: title/added/duration/resolution). The "up to 5 random siblings"
requirement needs a new ordering seam.

## Decision

Add **one** endpoint that returns both related sets in a single response:

```
GET /api/v1/media/{id}/related      → 404 if the item is missing/inactive
```

```jsonc
{
  "person": { "id": 42, "name": "Jane Editor", "items": [ /* ≤5 Video */ ] } | null,
  "tag":    { "id": 7,  "name": "action",      "items": [ /* ≤5 Video */ ] } | null
}
```

**Selection (server-side, deterministic except for the item draw):**

- **Person key** — of the item's people, the one with the highest **global** video
  count; tie-break lowest `person_id`. `null` if the item has no people.
- **Tag key** — the item's **most *distinctive*** tag, not simply its highest-count tag.
  Score each of the item's tags by **`distinctiveness = c · (1 − c / N)`** where `c` is
  the tag's global video count and `N` is the total active-video count. This quadratic
  rewards tags that are *shared* (large `c`) but penalises near-**universal** tags
  (`c → N` drives the score to ~0), so a tag sitting on almost the whole library is
  demoted in favour of a popular-but-not-universal one. Pick the maximum score;
  tie-break by higher raw `c`, then lowest `tag_id`. `null` if the item has no tags.
- **Items** — for each chosen key independently, up to **5 active** videos sharing
  that person / tag, **excluding the current item**, ordered `RANDOM()`. Reuses the
  standard `Video` JSON and `attachAssociations()` (so cards carry their people/tags
  without N+1). `items: []` is valid and still returns the key's `id`/`name`.

**Implementation seam.** Add repo methods that (1) pick the key entity — person by
global count, tag by the `c · (1 − c / N)` score — and (2) run a
`... EXISTS(join) AND v.id != ? AND v.active = 1 ORDER BY RANDOM() LIMIT 5` select.
The tag score needs only each candidate tag's global count (the same `VideoCount`
`ListTags`/`namedCountQuery` already computes) and `N` (one `COUNT(*) WHERE active`);
the scoring is a closed-form arithmetic pick over the item's handful of tags — compute
it in SQL (`ORDER BY c*(1.0 - c*1.0/:N) DESC, c DESC, tag_id`) or in Go, no new table.
This is the project's **first use of `ORDER BY RANDOM()`** — kept local to these
methods rather than added to `VideoFilter`/`orderBy()`, so the deterministic-ordering
invariant of the general list path is untouched. The handler follows the existing
`getMedia`/`getPerson` pattern (`writeJSON` / `writeError` / `ErrNotFound` → 404),
registered in `Mount()` as `r.Get("/media/{id}/related", …)`.

No migration, no new persistent state.

## Rationale

- **One endpoint, not two.** Both shelves render together on the detail page, so a
  single `GET …/related` means one round-trip and a trivial client (no fan-out, no
  coordinating two loading states). The person/tag blocks are independent in the
  payload, so a future third shelf is an additive field, not a new route.
- **`ORDER BY RANDOM()` is right at this scale.** The randomised select runs over the
  *already filtered* set of videos sharing one person or one tag — a small set on a
  personal library — and takes `LIMIT 5`. SQLite full-scanning that filtered set is
  negligible. Using it keeps the query a single statement with no app-side shuffling.
- **Distinctiveness beats raw popularity for the tag shelf.** A pure "highest global
  count" pick degenerates when an item's top tag is near-universal (the shelf becomes
  "5 random items"). The `c · (1 − c / N)` score keeps the shelf *thematic* — it favours
  a tag shared by enough items to be meaningful but not by everything — while reusing the
  `VideoCount` the entity queries already compute (no new aggregation, just one `N`). The
  person shelf keeps the simpler highest-count rule: a person on many items is a feature,
  not noise, and people aren't subject to the same "universal tag" degeneration.
- **Stability is a client concern; the server stays per-request random.** The shelves
  are resolved **once per page view** (the client fetches `/related` on mount and holds
  it), so they don't reshuffle while the owner is on the page; a fresh page view (navigate
  back, revisit) draws anew. Keeping the *server* seedless (`RANDOM()` per call) means no
  seed state, no cache, and no migration — the client's fetch-and-hold provides the
  desired stability for free.
- **Reuse over new abstractions.** Selection leans on the existing join tables,
  indexes, `VideoCount`, and `attachAssociations()`; only the random-order select is
  genuinely new.

## Consequences

- **`ORDER BY RANDOM()` is now a recorded seam.** If a single person/tag ever matches
  a very large set (not expected on a personal library), random-ordering a big scan is
  the thing to revisit — switch to a random-offset window or precomputed sampling.
  Confined to the two related-media repo methods, so the change would be local.
- **The distinctiveness score peaks at `c = N/2`.** `c · (1 − c / N)` is maximised by a
  tag on about half the library, and demotes both very rare and near-universal tags. On
  a small/cold library (low `N`, sparse tags) the "best" tag may still be modestly shared
  — acceptable, and it never *fails* (it always picks the item's best-scoring tag). The
  curve is a deliberate, tunable heuristic, not a learned weight; if it skews toward
  too-popular tags in practice, adjust the shape (e.g. cap `c`, or shift the peak) — the
  change is one expression in the tag-selection method.
- **Stable-per-view, not stable-across-reload.** Because stability comes from the client
  holding the fetch, a hard browser reload is a new page view and re-draws the shelves.
  Reproducing an identical set across a reload would require threading a seed through the
  URL; deferred as not worth the complexity (Quick Wins QW2 note).
- **Empty/absent blocks are normal.** No people → `person: null`; people but no
  siblings → `items: []`. The client omits a shelf whose block is null or whose
  `items` is empty (QW3); the endpoint never 500s on "nothing related."
- **Bounded cost, no schema change.** Two key-selection reads + two `LIMIT 5` random
  selects + their association batches per call; no migration, no write path, nothing
  to retain.
- **Covered by tests** following the existing repo/handler patterns: key-selection
  tie-breaks, current-item exclusion, `active`-only, empty cases, and the 404 path.
