# ADR-045: Seeded random ordering for paginated list endpoints

**Status**: Proposed
**Date**: 2026-06-27
**Deciders**: Project owner
**Extends**: ADR-006 (REST + chi), ADR-003 (SQLite + FTS5), ADR-017 (query architecture)
**Relates to**: ADR-031 (related-media `ORDER BY RANDOM()` seam — *seedless* by design)
**Spec**: [Sticky sort preferences + Random sort — SP2/SP3](../specs/sort-persistence.md)

---

## Context

The [sort-persistence spec](../specs/sort-persistence.md) adds a **Random** sort to the
Media, People, and Tags pages and asks that a shuffle be **stable per session** — it must
not reshuffle as the user pages through results, navigates Back, or triggers an incidental
re-render. It reshuffles only on a new session or an explicit re-roll.

The three surfaces split cleanly by how they read data:

- **People / Tags** — `ListPeople` / `ListTags` (`internal/repo/repo.go:780,817`) return the
  **entire** list in one call (unpaged). The client already holds the whole array.
- **Media** — `ListVideos` (`repo.go:330`) is **paginated**: the browse grid fetches
  `limit`/`offset` windows and appends via "Load more". Its ordering goes through the
  whitelisted `orderBy()` (`repo.go:287–303`: title/added/duration/resolution).

This split is decisive. For the unpaged lists, "random" can be a client concern. For the
paginated Media grid it cannot: **`ORDER BY RANDOM()` re-randomises on every query**, and
each "Load more" page is a *separate* query. Page 2 would be drawn from a fresh shuffle
independent of page 1 — producing **duplicate rows across pages and skipped rows that never
appear**. SQLite's `random()` is non-deterministic per row per statement and cannot be
seeded to reproduce an order across requests. ADR-031 sidesteps this precisely because it is
**not** paged — it does a single `LIMIT 5` fetch-and-hold, so a seedless per-request
`RANDOM()` is correct there. A paginated shuffle needs a different mechanism: an order that
is **deterministic given a seed**, so one shuffle tiles across all pages.

We also need to widen the People/Tags sort param. Today the handlers test a single boolean
(`r.URL.Query().Get("sort") == "count"`, `handlers.go:599,630`). Adding a third option
(`random`) means moving to a small **validated enum**, kept backward-compatible with the
existing `?sort=count`.

## Decision

### 1. Media — seeded deterministic ordering via a custom SQLite function

Register **one deterministic scalar SQL function**, `holo_shuffle(id, seed) → INTEGER`, on
the SQLite connection. It returns a well-distributed 64-bit hash of the row id mixed with
the session seed (a `splitmix64`-style integer mix of `uint64(id) ^ mix(seed)`), computed in
Go where hash quality is easy to get right. `modernc.org/sqlite` (our driver, ADR-003)
supports this via `RegisterDeterministicScalarFunction`; marking it **deterministic** lets
SQLite treat equal inputs as equal and is sound because the function is pure.

Add `random` to the whitelisted `orderBy()` set. When `sort=random`, ordering becomes:

```sql
ORDER BY holo_shuffle(v.id, :seed), v.id
```

The trailing `, v.id` is a total-order tie-break so the sort is fully determined even on the
(astronomically unlikely) hash collision — guaranteeing stable pagination. Because the order
is a pure function of `(id, seed)` and the row set, `LIMIT`/`OFFSET` windows over it **tile
exactly**: no duplicates, no gaps, page after page, for the life of the seed.

### 2. The `sort` / `seed` query contract

```
GET /api/v1/media?sort=random&seed=<int>&limit=&offset=
      → seeded deterministic shuffle; pages tile under a fixed seed
GET /api/v1/people?sort=name|count|random
GET /api/v1/tags?sort=name|count|random
```

- **`seed`** — parsed as a bounded integer (`int64`; values outside range or non-numeric are
  rejected). It is only meaningful with `sort=random`. If `sort=random` arrives **without** a
  valid seed, the server picks one for that single request (the response is still internally
  consistent) — but the client **always** supplies a seed so that successive "Load more"
  pages share it. The client generates the seed once per session (held in a module store +
  `sessionStorage`, per the spec) and re-rolls by generating a new one.
- **`sort` is a validated enum per endpoint.** Media: the existing whitelist + `random`.
  People/Tags: `{name, count, random}`, default `name`. The legacy `?sort=count` keeps
  working unchanged (it remains the truthy "by count" value); unknown values fall back to the
  endpoint default with a `200` — never a 4xx/5xx.

### 3. People / Tags — client-side seeded shuffle

These lists are unpaged and small, so **`random` is performed in the client**, not in SQL.
On `sort=random` the server returns the list in its canonical `name` order; the client
shuffles that array with a tiny deterministic PRNG (e.g. `mulberry32`) keyed by the **same
session seed** used for Media. Same seed → same order across re-renders; re-roll → new order.

This deliberately keeps the seeded-SQL path (#1) confined to the **one** endpoint that
actually paginates. The unpaged endpoints stay out of the custom-function path entirely —
they only gain enum validation (#2).

No migration, no new persistent state, no write path.

## Options Considered

### Option A — Custom deterministic scalar function `holo_shuffle(id, seed)` (CHOSEN, for Media)
| Dimension | Assessment |
|-----------|------------|
| Complexity | Low–Med — one Go function + one registration hook |
| Cost | Negligible — pure arithmetic per row, fully indexed scan unaffected |
| Distribution | High — proper integer mix (splitmix64) avoids the clumping of `id % p` |
| Pagination | **Tiles exactly** — order is a pure function of (id, seed) |

**Pros:** deterministic and seedable (the whole point); strong shuffle quality kept in Go,
testable in isolation; SQL stays a one-line `ORDER BY`; driver-supported.
**Cons:** introduces the project's **first** custom SQL function (a new seam to register on
every connection); the shuffle is a full sort of the filtered set (fine at personal-library
scale, same scan SQLite already does for `title`/`duration` sorts).

### Option B — Inline arithmetic hash in SQL (no custom function)
e.g. `ORDER BY (v.id * :a + :b) % 2147483647`.
**Pros:** zero registration; pure stock SQL.
**Cons:** weak, lumpy distribution unless multi-step bit-mixing is written inline (gnarly,
unreadable SQL); easy to get a non-permutation (collisions/clustering) wrong; the quality is
exactly what a Go function makes trivial. Rejected — saves a registration hook at the cost of
a worse, harder-to-test shuffle living in SQL strings.

### Option C — Materialize a per-seed shuffle (random column / temp ordering table)
**Pros:** any ordering expressible.
**Cons:** stateful — needs creation, seed-keying, and cleanup/eviction; a cache-invalidation
problem for a cosmetic feature. Overkill. Rejected.

### Option D — Client-side shuffle for Media too (fetch all ids, page in the client)
**Pros:** symmetric with People/Tags; no SQL change.
**Cons:** breaks the paginated API model — either ship the entire id set to the client to
shuffle (defeats pagination, unbounded payload) or page a client-held id list through a new
"by id list" query path (a bigger API change than a seed param). Rejected for Media;
*accepted* for People/Tags precisely because those are already fully materialized.

## Trade-off Analysis

The core tension is **where the shuffle lives**. A seedless `ORDER BY RANDOM()` (ADR-031) is
the simplest possible thing and is correct for a single fetch-and-hold draw — but it is
*structurally* wrong for pagination, where independent page queries must agree on one order.
Determinism-by-seed is the minimal property that makes paged random coherent, and a custom
scalar function (Option A) buys that property with the least SQL complexity and the best
shuffle quality, at the cost of one new registration seam. The unpaged lists don't share this
constraint, so pushing their shuffle to the client (Option D, scoped to them) avoids touching
their queries at all. The result: seeded SQL **only** where pagination forces it; plain enum
validation everywhere else.

## Consequences

- **First custom SQL function in the project.** `holo_shuffle` must be registered on **every**
  connection (a connection hook / driver registration at DB-open), and the registration must
  be in place before any `sort=random` query runs. This is a new operational seam to keep in
  mind (e.g. for any future raw-`database/sql` opens or test harnesses) — but it is confined
  to the DB-open path and the one `orderBy()` branch.
- **Pagination under a fixed seed is exact.** Disjoint `(limit, offset)` windows union to the
  full set with no duplicate and no gap — the property `ORDER BY RANDOM()` cannot provide and
  the reason this ADR exists. Verified directly by test (two adjacent pages, same seed).
- **Stable per session, fresh per session.** The client holds the seed for the session, so
  the order survives "Load more", Back, and re-renders; a new tab/session draws a new seed
  (the spec's intent). Reproducing an order across sessions is possible *in principle* (the
  seed fully determines it) but is deliberately **not** surfaced by default — a future "copy
  this shuffle" deep link could expose `?sort=random&seed=…` (spec P2).
- **`seed` is an untrusted integer, `sort` is whitelisted.** `seed` is parsed/bounded as
  `int64` and only ever passed as a **bound parameter** to `holo_shuffle` — never
  concatenated into SQL. `sort` is validated against a fixed per-endpoint set before reaching
  `orderBy()` (no raw string into the `ORDER BY` clause). This keeps the widened contract free
  of injection surface (see the security note below).
- **People/Tags stay backward-compatible.** `?sort=count` is unchanged; the only behavioral
  change is that previously-ignored unknown `sort` values now explicitly fall back to `name`
  (already the effective behavior of the boolean check). No client of the old boolean breaks.
- **ADR-031's seedless seam stands.** That endpoint remains `ORDER BY RANDOM()` — it is
  fetch-and-hold, not paged, so it needs no seed. The two random seams coexist with a clear
  rule: **seedless for single-draw fetch-and-hold, seeded for paginated.**
- **Scale caveat (shared with ADR-031).** Random-ordering is a full sort of the *filtered*
  set. On a personal library this is negligible; if a filtered Media set ever grew very large,
  a seeded random-offset window would be the thing to revisit. Confined to one `orderBy()`
  branch, so the change would be local.

## Action Items
1. [ ] Register `holo_shuffle(id, seed)` as a deterministic scalar function at DB-open
       (`modernc.org/sqlite` `RegisterDeterministicScalarFunction`); unit-test the mix for
       distribution and determinism.
2. [ ] Add `random` to `orderBy()` with `ORDER BY holo_shuffle(v.id, :seed), v.id`; thread a
       bounded `int64` seed through `VideoFilter`.
3. [ ] Widen `/people` & `/tags` handlers from the boolean `sort==count` check to a validated
       `{name,count,random}` enum (default `name`); keep `count` behavior identical.
4. [ ] Parse/validate `seed` and `sort` in the Media handler; bound the seed; ensure both are
       bound parameters / whitelisted (no string interpolation into SQL).
5. [ ] Tests: two adjacent Media pages under one seed union with no dup/gap; same seed →
       same order, different seed → different order; unknown `sort` → default `200`;
       People/Tags `count` unchanged and `random` returns canonical order for the client.
6. [ ] `/security-review` the contract change (param validation, injection surface).
7. [ ] Add ADR-045 to the index (`docs/architecture/README.md`).
