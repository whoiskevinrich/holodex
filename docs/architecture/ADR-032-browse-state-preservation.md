# ADR-032: Browse-state preservation across SPA navigation

**Status**: Proposed
**Date**: 2026-06-14
**Deciders**: Project owner
**Extends**: ADR-002 (SvelteKit SPA, `ssr=false`), ADR-021/025 (frontend conventions)
**Spec**: [Quick Wins — QW4](../specs/quick-wins.md)

---

## Context

The browse grid ([`web/src/routes/+page.svelte`](../../web/src/routes/+page.svelte)) is
the app's primary surface. Holodex is a **client-only SPA** (`ssr=false`,
adapter-static fallback — ADR-002), so every route is a Svelte component that is
**destroyed on navigation away and re-created on return**. The grid:

- holds its result set in component-local `$state` (`videos`, `total`, `offset`);
- paginates with a **"Load more"** button that *appends* the next 50-item page into
  `videos` (the `offset` is intentionally **not** in the URL — paging is a client
  concern);
- on mount, an `$effect` reads filters from the URL and calls `loadPage(true)`, which
  fetches **page 0 only**.

So the round-trip "scroll the grid → open an item → press Back" loses two things at
once: **(1) scroll position**, and **(2) every "Load more" page beyond the first**.
These are coupled — SvelteKit's native scroll restoration can't restore a Y position
that lives inside items which no longer exist after a page-0-only re-fetch. Restoring
scroll *requires* restoring the loaded set first. Today the result is a jarring "land
at the top of a reloading grid," which makes browsing feel like reloading a website.

Two mechanisms were considered:

- **SvelteKit `snapshot` export** — serializes component state to `sessionStorage`,
  keyed per history entry, restored on back/forward. Purpose-built for "navigate away
  and come back," but the docs steer it toward **small** state; our `videos` array can
  be hundreds of objects, so serializing it on every navigation is the wrong shape.
- **Module-scoped in-memory store** — a `.svelte.ts` singleton (the pattern already used
  by [`theme.svelte.ts`](../../web/src/lib/theme.svelte.ts) and
  [`activity.svelte.ts`](../../web/src/lib/activity.svelte.ts)) that survives client
  navigation because the JS module is not torn down. No serialization, any size.

## Decision

Add a **module-scoped browse-state cache** (`web/src/lib/browse.svelte.ts`) that
preserves the grid across in-app navigation and is restored synchronously on return.

**Cached shape** (one entry — there is a single browse grid):

```ts
{ signature: string;   // filter/sort identity that produced this set
  videos: Video[];      // every loaded page, in order
  total: number;
  offset: number;       // last fetched page offset
  scrollY: number }     // window scroll at navigate-away
```

- **Signature** = the shareable filter param string the page already computes
  (`activeParams.toString()`), so it captures filters + sort but not paging.
- **Capture** scroll + current state in `beforeNavigate` (SvelteKit lifecycle) when
  leaving the grid.
- **Restore** on mount: if the cache exists **and** its `signature` matches the current
  URL's filter signature, seed `videos`/`total`/`offset` from it **synchronously and
  skip the page-0 fetch**; then in `afterNavigate` (DOM is laid out) `window.scrollTo`
  to `scrollY`. Otherwise fetch page 0 as today.
- **Invalidate** (drop the cache, fetch fresh from offset 0) whenever the filter/sort
  signature changes — the existing re-fetch `$effect` already fires on exactly those
  changes, so invalidation hangs off it.
- **Scope**: only the browse grid (`/`) in v1. The store is shaped (single keyed entry)
  so it can later hold per-route entries for entity pages without changing consumers.

No backend, no migration, no URL change. Native SvelteKit scroll restoration is left
on; the explicit `scrollTo` in `afterNavigate` is the authoritative restore once the
cached content guarantees correct page height.

## Rationale

- **It fixes the actual coupled problem.** Preserving the loaded set is a *precondition*
  for scroll restoration past page 1; the module store does both in one move.
- **No serialization, any size.** A few hundred `Video` objects stay as live JS
  references — no `sessionStorage` quota, no JSON round-trip cost on every click. This is
  why the module store beats `snapshot` here.
- **Matches an established convention.** `theme` and `activity` are already module
  `.svelte.ts` singletons; a reviewer reads `browse.svelte.ts` with zero new concepts.
- **Flash-free by construction.** Seeding from cache before render means the grid never
  shows a `Loading…` state or re-requests page 0 on Back — the fluidity requirement.
- **URL semantics untouched.** Filters still round-trip through the URL for
  share/reload; the cache is a pure in-memory accelerator layered on top, so
  deep-linking and hard-reload behavior are unchanged.

## Consequences

- **Cache is session-scoped, not durable.** A full reload / cold load rebuilds from the
  URL at page 0 (by design — QW4 non-goal). Acceptable: the URL already encodes the
  shareable/reproducible state.
- **Single-entry cache is a deliberate limitation.** Because filter changes use
  `replaceState` (one history entry for browse), one keyed entry suffices. If browse ever
  becomes multiple history entries (e.g. paging *does* push history), this generalizes to
  a keyed map — the shape already anticipates it.
- **Memory cost is bounded and transient.** It holds at most the one currently-loaded
  grid; it is replaced on every filter change and never accumulates.
- **Restore correctness depends on synchronous seeding.** If a future refactor makes the
  grid fetch even the cached first page asynchronously, the height-at-restore guarantee
  breaks; the seeding path must stay synchronous. Called out for maintainers.
- **Testable**: the store (signature match/mismatch, invalidation) is unit-testable;
  the scroll-restore behavior is a Playwright case (scroll → open → Back → assert
  `scrollY` and item count). Added to the testing strategy with the rest of the batch.
- **Generalization is opt-in later**, not now — entity-page preservation is a P1 spec
  item reusing this store.
