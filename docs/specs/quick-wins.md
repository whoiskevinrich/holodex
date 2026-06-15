# Spec: Quick Wins batch — Search history & "More with …" shelves

**Status**: Draft
**Phase**: Post–Phase 2 backlog (standalone user-facing features; no phase dependency)
**Owner**: Project owner
**Date**: 2026-06-14

**Depends on**: Nothing. Both features build only on shipped surfaces (the header search form, the media detail page, the existing media/person/tag read APIs).

**New ADRs required**:
- **[ADR-031](../architecture/ADR-031-related-media-endpoint.md) (Proposed)** — Related-media query & endpoint (`GET /api/v1/media/{id}/related`): random selection seam (`ORDER BY RANDOM()`), most-popular-tag selection, and the response contract. Search history is client-only and needs **no ADR**.
- **[ADR-032](../architecture/ADR-032-browse-state-preservation.md) (Proposed)** — Browse-state preservation across SPA navigation (the QW4 "fluid Back" mechanism): a module-scoped client cache of the loaded grid + filters + pagination offset + scroll position, restored on return.

> **Also in this batch (not specced here):** the **QA overlay bug** — the `.app-atmosphere::after` scan/atmosphere overlay (`position:fixed; z-index:40`, worst in **Broadcast**) stays visible over the `<video>` on the media detail page once playback starts. That is a pure frontend bugfix (suppress the atmosphere overlay while a media video is playing) routed through the theming QA discipline, not a functional change — so it carries no spec or ADR. Acceptance for it lives with the design/theming QA: video unobstructed during playback in **all three skins**, overlay restored on pause/end.

---

## Problem Statement

Holodex is a personal media server whose owner browses and rediscovers their own library. Three small frictions stand out today. **(1) Search has no memory** — every query is typed from scratch, and the search supports structured syntax (`field:value`, FTS), so re-running a recent search means re-typing something fiddly with nothing to jog the memory. **(2) The media detail page is a dead end** — once you open an item there is no path to "more like this," so discovering related content means going back to browse and re-filtering by hand. **(3) Going Back loses your place** — the browse grid is an SPA view that re-fetches from scratch on return, so after scrolling (and "Load more"-ing) through results, opening an item, and hitting Back, you land at the top of a freshly-loaded first page — your scroll position *and* the extra pages you'd loaded are gone. All three are cheap to fix and directly serve the core "rediscover my own library" loop.

## Goals

1. **Let the owner re-run a recent search in one click**, without retyping query syntax — recent queries are remembered locally and surfaced at the point of search.
2. **Turn the media detail page into a jumping-off point** — from any item, surface up to two shelves of genuinely related items (same person, same popular tag) so the owner keeps discovering.
3. **Add zero backend state for search history** — it is a local convenience, not synced data, with no privacy footprint beyond the user's own browser.
4. **Reuse one component and one endpoint for both shelves** — the person shelf and tag shelf share a single shelf component and a single related-media endpoint, keeping the surface small.
5. **Make Back instant and place-accurate** — returning to the browse grid restores exactly what the owner left: the same scroll position, the same loaded results (including extra "Load more" pages), and the same filters — with no re-fetch flash.

## Non-Goals

- **Server-side / cross-device search history.** History is `localStorage` only; it does not sync across devices or browsers and is not persisted server-side. *(Why: no accounts today; a personal server on one browser doesn't justify a backend table or the privacy surface of stored query logs.)*
- **Search autocomplete / suggestions from indexed metadata.** Suggesting completions as you type is a separate, larger backlog item (needs a suggestions endpoint). History only replays *queries you already ran*. *(Why: distinct feature, distinct backend work — explicitly out of scope here.)*
- **Curated / algorithmic recommendations.** "More with …" is deliberately simple: random items sharing one chosen person or the single most-popular tag. No ranking model, no "because you watched," no collaborative filtering. *(Why: a personal library has no engagement signal to rank on; randomness keeps it fresh and cheap.)*
- **More than two shelves, or configurable shelves.** v1 is exactly one person shelf + one tag shelf. No per-tag shelves, no "more with this container/codec," no user-arranged shelves. *(Why: keep the page legible and the endpoint single-purpose; revisit if it proves useful.)*
- **Paging / "see all" from a shelf.** Each shelf shows up to 5 items and stops. Clicking the person/tag heading can deep-link to the existing person/tag page (which already lists everything). *(Why: the existing entity pages already do the full-list job; don't duplicate it.)*
- **Persisting browse state across a full page reload (F5 / cold load) or across browser sessions.** "Fluid Back" (QW4) preserves grid state only for **in-app client navigation** (open item → Back). A hard refresh legitimately starts fresh from the URL. *(Why: the URL already encodes filters for shareable/reload state; preserving an in-memory grid across reloads would mean serializing a large result set for marginal benefit.)*
- **Restoring scroll on the media detail page or entity pages in v1.** QW4 targets the browse grid (the one place with deep scroll + "Load more"). The same mechanism is designed to generalize later, but only the browse grid is in scope now. *(Why: the browse grid is where the friction is felt; keep the change focused.)*

---

## User Stories

**Search history**
- As the **library owner**, when I focus the search box I want to see my recent searches so that I can re-run one without retyping its syntax.
- As the **library owner**, I want my most recent search at the top and duplicates collapsed so that the list stays short and useful.
- As the **library owner**, I want to clear my search history (and/or remove a single entry) so that I can tidy it or drop a one-off query I won't repeat.
- As the **library owner**, I trust that my search history never leaves my browser so that nothing about what I search is sent anywhere.

**"More with …" shelves**
- As the **library owner**, on a media item's page I want a "More with `<person>`" shelf so that I can jump to other items featuring someone in this one.
- As the **library owner**, I want a "More with `<tag>`" shelf keyed to the item's most notable tag so that I can explore more of that theme.
- As the **library owner**, when an item has no people, no tags, or no related items, I want the page to simply omit the empty shelf so that I never see a broken or empty rail.

**Fluid Back navigation**
- As the **library owner**, after I scroll through the grid and open an item, I want pressing Back to return me to the exact same scroll position so that I don't lose my place.
- As the **library owner**, when I'd loaded several "Load more" pages before opening an item, I want all of those still loaded on return so that the item I came from is still on screen where I left it.
- As the **library owner**, I want Back to feel instant (no spinner, no flash of a reloading grid) so that browsing feels continuous rather than like reloading a website.

---

## Requirements

### Must-Have (P0)

#### QW1 — Search history (client-only)

A locally-stored, most-recent-first list of past search queries, surfaced as a dropdown under the header search input.

- **Recording.** A query string is recorded **on submit** (when the search is actually run from the header form → `/search?q=…`), not on keystroke. The *raw query text the user submitted* is stored (preserving any `field:value` / FTS syntax). Empty/whitespace-only queries are not recorded.
- **Dedupe & ordering.** Case-insensitive dedupe on the trimmed query; re-submitting an existing query **moves it to the top** rather than adding a duplicate. List is strictly most-recent-first.
- **Cap.** Retain at most **10** entries; submitting an 11th evicts the oldest.
- **Storage.** A single `localStorage` key (e.g. `holodex-search-history`) holding a JSON array of strings. Reads/writes are defensive: malformed or oversized JSON is treated as empty (never throws into the UI). Mirrors the existing `holodex-theme` localStorage pattern.
- **Surface.** Focusing the search input **while it is empty** (including via the existing Ctrl/Cmd-K shortcut) opens a dropdown listing the recent queries. **The dropdown hides the moment the user types a character** — history is for recalling past queries from an empty box, not for filtering as you type; this leaves the "typing" state free for a future autocomplete surface to own without reworking history *(resolves the coexistence Open Question)*. Clicking an entry populates the input **and runs the search** (navigates to `/search?q=…`). Keyboard: arrow keys move through entries, Enter runs the highlighted one, Esc closes the dropdown. When history is empty, no dropdown (or a quiet empty hint) — never an empty floating box.
- **Management.** A "Clear history" affordance empties the list; each entry has a small remove (×) control to drop just that one.
- **Theming.** The dropdown uses semantic tokens only (`bg-surface`, `text-ink`, `text-muted`, `border-rule`, `bg-accent`/`text-accent-ink` for the active row, `rounded-theme`) and is QA'd in all three skins — including the Broadcast/Brutalist `--radius: 0` square treatment.

**Acceptance criteria — QW1**
- [ ] Given I run a search "amv `editor:foo`", when I next focus the search box, then "amv `editor:foo`" appears at the top of the recent list.
- [ ] Given a query already in history, when I run it again, then it moves to the top and is not duplicated (case-insensitive match).
- [ ] Given 10 queries in history, when I run an 11th distinct query, then the oldest is evicted and the list length stays 10.
- [ ] Given the dropdown is open, when I click an entry, then the input fills with that query and the search runs (URL becomes `/search?q=<that query>`).
- [ ] Given the dropdown is open, when I press ↓/↑ then Enter, then the highlighted query runs; when I press Esc, then the dropdown closes without running anything.
- [ ] Given the dropdown is open on an empty box, when I type a character, then the dropdown closes (history does not filter-as-I-type); clearing the box and re-focusing reopens it.
- [ ] Given I click "Clear history", then the list empties and the `localStorage` key is removed/emptied; reopening shows no entries.
- [ ] Given I click an entry's × control, then only that entry is removed and the rest of the list is preserved in order.
- [ ] Given a corrupted/garbage value in `localStorage`, when the page loads, then search still works and history reads as empty (no thrown error).
- [ ] History never appears in any network request — verified there is no new backend call.
- [ ] The dropdown renders correctly (tokens, radius, contrast, active-row legibility) in **Cinémathèque, Broadcast, and Brutalist**.

#### QW2 — Related-media endpoint (`GET /api/v1/media/{id}/related`)

One backend endpoint returns both the person-based and tag-based related sets for a media item in a single call.

- **Route.** `GET /api/v1/media/{id}/related`. 404 if the item doesn't exist / isn't active (consistent with `GET /api/v1/media/{id}`).
- **Person selection.** From the item's people, pick **one** person to key the shelf on. Selection rule: the person with the **highest global video count** (most-connected → most likely to yield a populated shelf); ties broken deterministically by lowest person id. If the item has no people, the `person` block is omitted/null.
- **Tag selection.** Pick the **single most *distinctive* tag** on the item — the tag that is popular **but not near-universal** — rather than simply the highest-count tag. Selection scores each of the item's tags to reward sharing while penalizing universality (the exact score is defined in ADR-031; it demotes a tag that sits on almost the whole library so the shelf stays *thematic* instead of degenerating into "5 random items"). Ties broken deterministically (higher raw count, then lowest tag id). If the item has no tags, the `tag` block is omitted/null. *(Resolves the near-universal-tag Open Question.)*
- **Item sets.** For the chosen person and the chosen tag independently: up to **5** active items that share that person / that tag, **excluding the current item**, selected **randomly** (`ORDER BY RANDOM()`) server-side. Each item reuses the standard `Video` JSON shape (id, title, thumbnail, etc.) already returned by list/detail endpoints, including attached people/tags via the existing batched-association path (no N+1).
- **Stability — stable per page view.** The client fetches `/related` **once when the media page mounts and holds the result**, so the shelves do **not** reshuffle while the owner is on the page (no jarring mid-view changes). Opening the item again — navigating back to it, or revisiting later — mounts a fresh page view and draws a new set. The server query stays per-request random; stability is a pure client concern (fetch-and-hold, keyed to the item id). *(Resolves the randomness Open Question. A hard browser reload is a new page view and therefore re-draws; reproducing an identical set across a reload would require a URL-carried seed and is deferred as not worth the complexity.)*
- **Response contract** (shape illustrative; finalized in ADR-031):

```jsonc
{
  "person": {                      // omitted or null if the item has no people
    "id": 42,
    "name": "Jane Editor",
    "items": [ /* up to 5 Video */ ]
  },
  "tag": {                         // omitted or null if the item has no tags
    "id": 7,
    "name": "action",
    "items": [ /* up to 5 Video */ ]
  }
}
```

- **Empty results.** A present-but-empty `items: []` is valid (the chosen person/tag exists on this item but no *other* item shares it). The block is still returned with its `id`/`name` so the client can decide to render or omit; the client omits a shelf whose `items` is empty.
- **Cost.** Bounded: at most one small query to choose the person, one to choose the tag, and two `LIMIT 5` random selects + their association batches. No new persistent state.

**Acceptance criteria — QW2**
- [ ] Given an item with people and tags that have siblings, when I GET `/related`, then `person.items` and `tag.items` each contain ≤5 active items, none of which is the current item.
- [ ] Given an item whose only people/tags are unique to it, when I GET `/related`, then the corresponding `items` array is empty (block still carries the chosen `id`/`name`).
- [ ] Given an item with no people, when I GET `/related`, then `person` is null/omitted and the request still succeeds with the `tag` block.
- [ ] Given an item with no tags, when I GET `/related`, then `tag` is null/omitted and the request still succeeds with the `person` block.
- [ ] Given an item with multiple tags — one near-universal (on almost every item) and one mid-frequency — when I GET `/related`, then the chosen tag is the **mid-frequency (most distinctive)** one, not the near-universal one (deterministic per the ADR-031 score + tie-break).
- [ ] Given an item with multiple people, when I GET `/related`, then the chosen person is the one with the highest global video count (deterministic tie-break by id).
- [ ] Given I am on a media page, when an incidental re-render occurs (e.g. a thumbnail regenerate, a skin switch), then the related shelves do **not** reshuffle — they were resolved once on mount.
- [ ] Given a non-existent or inactive item id, when I GET `/related`, then the response is 404.
- [ ] Returned items include their attached people/tags (verified no N+1 — same association batch as list/detail).

#### QW3 — "More with …" shelf component (frontend)

A single reusable shelf component, instantiated twice on the media detail page (person, then tag), fed by the `/related` response.

- **Placement.** Below the existing detail content on `web/src/routes/media/[id]/+page.svelte`, person shelf first, then tag shelf.
- **Heading.** "More with `<name>`" where `<name>` is the chosen person or tag. The heading links to that entity's existing page (person/tag) for the full list.
- **Items.** Reuses the existing media-card presentation (thumbnail, title, the standard `.video-frame`/skin flourishes) so cards look identical to the browse grid. Each card links to that item's detail page.
- **Omission.** A shelf renders **only** when its block is present **and** `items` is non-empty. No skeleton-forever, no "nothing here" text — an empty/absent shelf is simply not rendered.
- **States.** Loading (while `/related` is in flight) shows the existing card shimmer; error (request fails) silently omits the shelves (the page's primary content is unaffected). The shelves are non-blocking — the detail page renders fully without waiting on `/related`.
- **Theming.** Tokens only; QA'd in all three skins. Confirm the Brutalist catalog-counter and Broadcast scanline flourishes on `.video-frame` read correctly inside a shelf, and that shelf headings use `.skin-title` where the page already does.

**Acceptance criteria — QW3**
- [ ] Given a `/related` response with a non-empty person block, when the page loads, then a "More with `<person>`" shelf shows up to 5 cards, each linking to its item.
- [ ] Given a `/related` response with an empty/absent tag block, when the page loads, then no tag shelf is rendered (and no empty rail / placeholder text).
- [ ] Given `/related` is still loading, when I view the page, then primary detail content is fully usable and the shelves show shimmer (not a blocked page).
- [ ] Given `/related` fails, when I view the page, then the shelves are omitted and the rest of the page is unaffected (no error UI dominating the page).
- [ ] Clicking a shelf heading navigates to the corresponding person/tag page.
- [ ] Shelves render correctly (cards, flourishes, headings, contrast) in **Cinémathèque, Broadcast, and Brutalist**.

#### QW4 — Fluid Back navigation (browse-state preservation)

Returning to the browse grid via in-app navigation (open item → Back) restores the
grid exactly as it was left. Mechanism is architectural — see
**[ADR-032](../architecture/ADR-032-browse-state-preservation.md)**.

- **What is preserved.** The loaded `videos` array (including every "Load more" page),
  `total`, the pagination `offset`, the active filters/sort, and the **window scroll
  position** — captured when navigating away from the browse grid and restored on
  return.
- **Restore must be synchronous & flash-free.** On return, the grid renders from the
  cached state **without** re-fetching, so the content height is correct immediately and
  scroll is restored to the saved Y with no spinner and no "jump to top then settle."
- **Cache scope & invalidation.** The cache lives in client memory (a module-scoped
  `.svelte.ts` store, per existing `theme`/`activity` convention) for the lifetime of
  the SPA session. It is **invalidated** (fresh fetch from page 0) when: the user changes
  any filter/sort (the result set legitimately changed), or on a full page reload/cold
  load (state is gone; rebuild from the URL). It is **reused** only when the return
  navigation lands on the browse grid with the **same filter signature** that produced
  the cached set.
- **Scope.** Browse grid (`/`) only in v1. The store is shaped so the same pattern can
  later cover entity pages, but those are out of scope here.
- **No backend, no URL change.** Filters already round-trip through the URL
  (`history.replaceState`); QW4 adds **only** the in-memory grid/scroll cache. The
  pagination `offset` is deliberately *not* added to the URL (paging stays a client
  concern, per the existing design).

**Acceptance criteria — QW4**
- [ ] Given I scroll down the grid and open an item, when I press Back, then I return to the **same scroll position** (within a few px) without a visible jump-to-top.
- [ ] Given I clicked "Load more" twice (150 items) then opened an item, when I press Back, then all 150 items are still rendered and the item I opened is on screen where I left it.
- [ ] Given I return to the grid, when it restores, then **no loading spinner / "Loading…"** flash appears and no network request re-fetches page 0.
- [ ] Given I change a filter or sort, then the cache is discarded and the grid re-fetches from page 0 (scroll resets to top — expected).
- [ ] Given I hard-reload the browse page, then it rebuilds from the URL filters at page 0, top of grid (no stale restore).
- [ ] Given I open an item from a **filtered** grid and press Back, then the same filtered+paged set and scroll position are restored (filter signature matches).
- [ ] No regression to deep-linking/sharing: a pasted `/?…` URL still reproduces the filter state (QW4 does not touch URL semantics).

### Nice-to-Have (P1)
- **Search-history "did you mean recent" inline hint** when a typed query closely matches a recent one. *(Fast follow; not required for the core re-run loop.)*
- **Generalize browse-state preservation to entity pages** (`/people/{id}`, `/tags/{id}`) reusing the QW4 store. *(Cheap once the store exists; not needed for the headline friction.)*
- **Shelf heading shows the count** ("More with Jane Editor · 12 total") using the entity's video count. *(Cheap polish once the entity counts are on hand.)*

### Future Considerations (P2)
- **Autocomplete/suggestions** sharing the search dropdown surface (separate backlog item; design the dropdown so suggestions could slot in above/below history).
- **Additional shelves** ("More with `<container>`", "Recently added in this tag") reusing the same shelf component — keep the component prop-driven so new sources are cheap.
- **Server-synced history** if/when multi-user lands — would relocate history behind an endpoint; keep the client read/write behind a small module so the storage backend can swap.

---

## Success Metrics

This is a personal single-user server, so metrics are qualitative / self-observed rather than instrumented:
- **Search history:** the owner re-runs searches from the dropdown instead of retyping (observed in normal use); history stays short and relevant (cap + dedupe holding).
- **"More with …":** the detail page produces onward navigation — opening an item leads to opening a related item — where before it was a dead end. Shelves are populated (non-empty) for items whose people/tags have siblings, and gracefully absent otherwise.
- **No regressions:** no new backend state for history; `/related` adds bounded query cost; all three skins remain clean (the overlay bug fixed, dropdown + shelves themed).

## Open Questions

*All resolved 2026-06-14 — recorded here for traceability.*

- **(eng) — RESOLVED: stable per page view.** `/related` is **not** re-randomized on every render. The client fetches it once on media-page mount and holds the result, so shelves don't reshuffle while viewing; navigating back to the item (a fresh page view) draws a new set. The server query stays per-request `ORDER BY RANDOM()`; stability is a client fetch-and-hold concern. *(A hard reload re-draws — making a reload identical would need a URL-carried seed; deferred. See QW2 "Stability".)*
- **(eng/design) — RESOLVED: most *distinctive* tag, not most popular.** Tag selection scores the item's tags to reward sharing while penalizing universality, so a near-universal tag is demoted in favor of a popular-but-not-universal one — the tag shelf stays thematic. Score defined in [ADR-031](../architecture/ADR-031-related-media-endpoint.md). *(See QW2 "Tag selection".)*
- **(design) — RESOLVED: history hides once typing starts.** The dropdown opens on focus of an *empty* search box and closes the instant the user types, leaving the "typing" state free for a future autocomplete surface to own without reworking history. *(See QW1 "Surface".)*

## Timeline / Phasing
No hard deadline. Suggested order within the batch:
1. **QA overlay bugfix** (P1·S, no spec) — smallest, unblocks clean 3-skin QA for everything that follows on the media page.
2. **QW1 Search history** (P2·S) — client-only, independent, no backend.
3. **QW4 Fluid Back** (P2·M, ADR-032) — client-only; pairs naturally with the "More with…" work since both are about the open-item → Back loop.
4. **QW2 + QW3 "More with …"** (P2·M) — endpoint + ADR-031 first, then the shared shelf component on the detail page.
