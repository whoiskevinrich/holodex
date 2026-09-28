# ADR-114: One list-state model — the URL holds what you see, storage holds only preferences

**Status:** Proposed
**Date:** 2026-09-27
**Deciders:** Project owner

**Spec:** [F73 — list toolbar](../specs/list-toolbar.md) R7/R8 (HOLODEX-472).
**Supersedes:** [sort-persistence](../specs/sort-persistence.md) **SP5** (sticky filters), whose
storage posture this ADR replaces. SP1's "URL wins" rule is generalised here from Media to every
list page.
**Extends:** [ADR-032](ADR-032-browse-state-preservation.md), whose snapshots (`browseCache`,
`listScroll`) are now keyed by the list's URL query string.
**Relates to:** [ADR-045](ADR-045-seeded-random-ordering.md) (seed stays per session, never in
the URL); HOLODEX-41 (media delete's `history.back()` exit, generalised by D4).

---

## Context

Five list pages (Media, People, Studios, Films, Tags) each store their state differently.

| Page | Sort | Filters | URL |
|---|---|---|---|
| Media | `holodex:sort:media` + URL | `holodex:filters:media` + URL | full query string, `replaceState` |
| People, Studios | `holodex:sort:*` | `holodex:filters:*` (completeness dir, missing facet) | none |
| Films | `holodex:sort:films` | none | none |
| Tags | `holodex:sort:tags` | `holodex:filters:tags` (type) | none |

This inconsistency causes four problems:
- **Hidden filtered state.** SP5 restores filters on *every* visit, including a plain nav click, so
  a list can open filtered with nothing obvious to say so. F73 makes chips the filter indicator,
  but a filter nobody chose this visit is still a surprise.
- **Nothing on People, Studios, Films or Tags can be shared.** They have no URL state, so a view
  can't be linked, bookmarked or reloaded faithfully except through localStorage.
- **Two tabs clobber each other.** Two tabs sharing one storage key means the last write wins
  for both.
- **SP5 was built to fix Back, and solved it with the wrong tool.** Back from a detail page lost
  People filters because they lived only in component state. SP5 put them in localStorage, which
  fixed Back but also restores them everywhere else.

F73 moves every list onto one shared toolbar. The toolbar needs one state contract behind it, or
the inconsistency moves from the markup into its props.

Constraints:
- SPA only (`ssr=false`, ADR-002).
- No accounts for visitors and no server-side preference store.
- SvelteKit's `replaceState` must be used, never raw `history.replaceState`, which wipes router
  state and breaks Back (see the note in `routes/+page.svelte`).

## Decision

### D1. Split list state into *preferences* and *queries*

| Kind | Keys | Home |
|---|---|---|
| **Preference** ("how I like to browse") | `sort`, view (List/Poster), density | localStorage **and** (for `sort`) the URL |
| **Query** ("what I'm looking at now") | every filter, entity-scope chips (`person`, `tag`, `studio_id`, `category`), Tags `type`, `q` | **URL only** |
| **Session** | Random seed | session store (ADR-045 / SP2), never the URL |

View and density stay out of the URL. They're display settings, and a shared link should show
the recipient's own layout.

### D2. The URL is the single source of truth for what is on screen

On every navigation into a list route, including a same-route navigation such as clicking
**People** while already on `/people?…`, the page derives its state from `page.url`:

1. **Query keys** come from the URL, or are absent. Nothing else fills them.
2. **`sort`** comes from the URL `sort` param if it's valid, otherwise from localStorage if valid,
   otherwise the page default. An owner-only value (completeness) is dropped for a visitor.
3. If the resolved `sort` isn't the default and the URL lacked it, the page `replaceState`s it in,
   so the address bar always reproduces the screen. This uses the macrotask deferral Media already
   uses on a hard load.

The derivation is **reactive to `page.url`**, not run once in `onMount`, because SvelteKit
reuses the component on a same-route navigation. A nav click to bare `/people` must clear the
filters on a mounted page.

### D3. Only an explicit user choice writes to storage

- localStorage is written in the control's change handler: picking a sort, switching the view,
  moving density.
- It is **never** written by an `$effect` that watches derived state. Such an effect would fire on
  URL arrival and let a shared link overwrite the recipient's preference.
- Every state change goes through **one** module function, which calls SvelteKit
  `replaceState(url)`. Filter changes don't create history entries (F73 non-goal). Keeping this in
  one place means switching to `pushState` later is a one-line change (F73 P2).

### D4. Leaving a list after a destructive action is `history.back()` when in-app, else the bare list

This generalises HOLODEX-41's media-delete rule to every entity. Any action that removes the
entity you're viewing and so must leave the page does this:

```ts
if (cameFromInApp) history.back(); else goto(listHref);   // listHref = the bare list, e.g. '/people'
```

`cameFromInApp` is `afterNavigate`'s `type !== 'enter'`, exactly as in `media/[id]`.

- Back returns to the list entry, whose URL D3's `replaceState` kept current. D2 then restores
  filters and sort from it, and the ADR-032 snapshot restores scroll.
- Merges that keep you on the page (list-page merge, and detail-page merge into the current
  entity) reload in place. The URL and in-memory state survive untouched, so no redirect rule is
  needed. **If a detail-page merge ever absorbs the entity being viewed, it takes this D4 exit.**
- The helper is shared (one `exitAfterRemoval(listHref)`). Pages don't each hand-roll the branch.

### D5. Snapshots are keyed by the list's URL query string

ADR-032's `browseCache` (Media) is already keyed by the query string. `listScroll`
(People/Studios, and under F73 Films and Tags) moves from the ad-hoc "sort plus missing-facet"
key to the same **canonical query string**: sorted params, defaults omitted. A snapshot then
restores only when you return to exactly the same view, whatever mix of filters produced it.

### D6. One parser/serialiser per page, one shared module

- `web/src/lib/listState.ts` owns the contract. Each page declares a schema: its keys, allowed
  values and defaults for sort, and which keys are queries versus preferences.
- The module provides `parse(url)`, `serialize(state)` (canonical: defaults omitted, keys
  sorted), `commit(state)` (the single `replaceState`, D3) and `exitAfterRemoval` (D4).
- Media's existing `filtersToParams`/`paramsToFilters` become its schema, not a second
  implementation.
- Legacy `holodex:filters:*` keys are never read (HOLODEX-474 deletes them later).

## Options considered

### Option A: URL as source of truth, storage for preferences only (chosen)

| Dimension | Assessment |
|---|---|
| Complexity | Medium. One module and five schemas; the precedence and reactivity rules need care |
| Cost | Frontend only, no backend or API change |
| Shareability | Full: every visible view is a link |
| Familiarity | Media already works this way for everything except SP5's filter restore |

**Pros:** Back, reload and share all "just work" from one mechanism. Nothing is hidden, because a
nav click is always clean. Tabs are independent. It retires SP5's second restore path.
**Cons:** Filters no longer come back on a fresh visit. That's intended, and accepted by the owner
in the F73 brainstorm, since the missing-facet work queue has its own owner page. URLs get longer.

### Option B: Storage as source of truth, URL as a mirror (People today, generalised)

**Pros:** Least change on four pages; preferences and filters both stick.
**Cons:** A shared link fights stored state, so you need precedence anyway, which is just
Option A with more ways to lose. Hidden filtered state and tab clobbering remain. Rejected.

### Option C: Everything in both (Media today, generalised)

**Pros:** Maximum stickiness, and shareable.
**Cons:** Hidden filtered state is the defining defect. A bare path silently restores filters.
It's also the reason for SP5's "pristine `/`" special case. Rejected.

### Option D: URL only, no storage at all

**Pros:** Simplest, with no precedence rules.
**Cons:** Loses sticky sort, which SP1 shipped deliberately and the owner wants kept. Rejected.

### D4 alternatives: how a removal redirect finds the list

| | `history.back()` if in-app (chosen) | Per-tab `sessionStorage` "last list URL" | `?returnTo=` param |
|---|---|---|---|
| New state | none | a second record of what history already holds | a param on every detail link |
| Correct target | whatever you came from (a list, a person's filmography, a playlist) | always a list, even when you came from a filmography | whatever the link carried |
| Risks | none new; proven by HOLODEX-41 | goes stale; per-tab keys to clean up | open-redirect validation; URL noise |

`history.back()` wins because it's already in production and returns you to where you actually
were, not just "a list". The sessionStorage option would send someone who deleted a media item
from a person's filmography to the Media grid instead.

## Consequences

- **Easier:** one toolbar state contract. Adding a filter means adding a schema key, and Back,
  reload, share and chips follow automatically. The multi-sort bug (HOLODEX-473) can't recur,
  because `sort` is one key.
- **Easier:** SP5's special cases disappear: the "pristine `/`" restore and "owner-only filters
  stored regardless".
- **Harder:** the reactivity rule in D2. A page that reads the URL once in `onMount` will
  silently ignore a same-route nav click. Tests must cover it (see Action Items).
- **Behaviour change the owner will notice:** a nav click no longer restores the last filters.
  Production confirmation of this is the gate for HOLODEX-474.
- **Revisit if:**
  - Filter undo is wanted: swap `commit` to `pushState` for discrete changes.
  - A server-side preference store ever exists: sort, view and density could move there without
    touching the URL half.

## Action items

1. [ ] `web/src/lib/listState.ts`: schema type, `parse`, `serialize` (canonical), `commit`,
       `exitAfterRemoval`. Unit tests for precedence, canonical form, visitor stripping of
       owner-only sort, and that arriving by URL never writes storage.
2. [ ] Migrate Media's `filtersToParams`/`paramsToFilters` to a schema. Drop the SP5
       `holodex:filters:media` read and the "restored" branch.
3. [ ] People, Studios, Films, Tags: derive from `page.url` reactively, with sort in the URL and
       filters/`type` URL-only. Drop the `holodex:filters:*` reads.
4. [ ] Re-key `listScroll` on the canonical query string (D5).
5. [ ] Replace `media/[id]`'s inline delete exit with `exitAfterRemoval('/')`, and use it for any
       future entity removal.
6. [ ] Testing strategy: a same-route nav click clears filters; Back restores filters and scroll;
       a shared link doesn't write storage; two tabs stay independent.
7. [ ] HOLODEX-474 (after production confirmation): delete `holodex:filters:*` once on load.
