# Spec: Sticky sort preferences + Random sort

**Status**: Draft
**Phase**: Post–Phase 2 backlog (standalone user-facing feature; no phase dependency)
**Owner**: Project owner
**Date**: 2026-06-27

**Depends on**: Nothing new. Builds only on shipped surfaces — the Media browse grid
(`SortDropdown`), the People and Tags pages (`SortToggle`), and the existing
media/people/tags list APIs.

**New ADRs required**:
- **[ADR-045](../architecture/ADR-045-seeded-random-ordering.md) (Proposed)** — Seeded
  random ordering for paginated list endpoints. The Media list is paged (offset/limit +
  "Load more"), so a naive `ORDER BY RANDOM()` would reshuffle on every page request and
  produce duplicate/skipped rows across pages. ADR-045 defines a **deterministic,
  seed-parameterized ordering** (a stable hash of row id + a client-supplied seed) so a
  single shuffle tiles correctly across pages, plus the `sort=random&seed=…` contract.
  *People/Tags return their full list in one call and are shuffled **client-side** — they
  need no ADR.*

> **Routing note.** Persisting the sort **choice** is client-only (localStorage) and
> carries no ADR. The **named-sort param unification** for `/people` and `/tags` (today a
> boolean `?sort=count`) and the seeded media ordering are the API/architecture pieces —
> both covered by ADR-045's contract section.

---

## Problem Statement

Holodex is a personal media server whose owner browses three catalog surfaces — **Media**,
**People**, and **Tags** — each with its own sort control. Two frictions stand out.
**(1) Sort has no memory.** Every visit resets to the hard-coded default: the People page
always opens A–Z even for an owner who always wants "Most videos," and the Media grid
always opens newest-first. The chosen order is component state that evaporates on
navigation or reload, so the owner re-selects the same sort over and over. **(2) Browsing
is deterministic to a fault.** A personal library has no recommendation engine; the only
way to rediscover forgotten items is to scroll the same fixed orders. There is no way to
*shuffle* the shelf and stumble onto something. Both are cheap to fix and directly serve
the core "rediscover my own library" loop.

## Goals

1. **Remember each page's sort independently.** The sort the owner last picked on Media,
   People, and Tags is restored on their next visit to that page — per-page, so "Most
   videos" on People and "A–Z" on Tags coexist.
2. **Add a Random sort to all three pages**, so the owner can shuffle the catalog and
   rediscover items that fixed orderings bury.
3. **Make Random feel stable, not flickery.** A shuffle holds for the session — it stays
   consistent across pagination ("Load more"), in-app Back navigation, and incidental
   re-renders — and only reshuffles on a new session or an explicit re-roll.
4. **Add zero backend persistence.** The sort preference is a local convenience in the
   owner's browser, with no server state and no privacy footprint.
5. **Keep all three skins clean.** The new Random option renders correctly in
   Cinémathèque, Broadcast, and Brutalist using semantic tokens only.

## Non-Goals

- **Server-side / cross-device sort sync.** The preference is `localStorage` only; it does
  not sync across devices or browsers and is not persisted server-side. *(Why: no accounts
  today; a personal server on one browser doesn't justify backend state.)*
- **Encoding the sort in the URL for People/Tags.** Media already round-trips `sort`
  through the URL for shareable/deep-linkable grids and **keeps doing so** (the URL remains
  the source of truth there; localStorage only supplies the *default* when no `sort` is in
  the URL — see SP1). People and Tags do **not** gain URL sort state in v1 — their
  preference lives only in localStorage. *(Why: those pages aren't built around shareable
  filtered URLs; adding URL state there is scope creep.)*
- **A reproducible Random order across a hard reload or a new tab.** A shuffle is stable
  *within* a session (SP2). A cold load / new tab legitimately draws a fresh shuffle.
  *(Why: reproducing an exact order across sessions would mean persisting the seed and a
  growing-stale "why is this the same random?" surprise — the session scope matches the
  "stumble onto something" intent.)*
- **Random as the persisted default that re-pins a fixed order.** Choosing Random is
  remembered as the *choice*; returning in a later session re-enters Random with a **new**
  seed (a fresh shuffle), not the previous order. *(Why: a sticky frozen "random" order is
  just a confusing fixed order; the value of Random is freshness.)*
- **New sort *dimensions*** beyond the existing ones + Random (e.g. "by duration" on
  People, "by date" on Tags). This spec adds **Random** and **persistence**; it does not
  expand the per-page option sets otherwise. *(Why: keep the change focused; new dimensions
  are separate backlog items.)*

---

## User Stories

**Sticky sort**
- As the **library owner**, when I set People to "Most videos" and come back later, I want
  it still sorted by "Most videos" so that I don't re-pick my preferred order every visit.
- As the **library owner**, I want Media, People, and Tags to each remember their **own**
  sort so that my Tags = A–Z preference doesn't override my People = Most-videos preference.
- As the **library owner**, I trust that my sort preference never leaves my browser so that
  it adds no server state or privacy surface.

**Random sort**
- As the **library owner**, I want a "Random" option on Media, People, and Tags so that I
  can shuffle the catalog and rediscover items I'd forgotten.
- As the **library owner**, when I shuffle Media and click "Load more", I want the next page
  to continue the *same* shuffle (no repeats, no jumping) so that paging through a random
  order is coherent.
- As the **library owner**, after I open a random-sorted item and press Back, I want the
  same shuffle still in place so that I don't lose where I was.
- As the **library owner**, I want a way to **re-roll** the shuffle so that I can get a new
  random order on demand without leaving the page.

---

## Requirements

### Must-Have (P0)

#### SP1 — Per-page sticky sort (client-only)

Each page's last-chosen sort is persisted to `localStorage` and restored on next visit,
independently per page.

- **Storage.** One namespaced key per page — e.g. `holodex:sort:media`,
  `holodex:sort:people`, `holodex:sort:tags` — each holding the raw sort value as a string.
  Mirrors the existing `holodex-theme` / `holodex:show-recently-added` localStorage
  patterns (single value, SSR-safe `typeof localStorage !== 'undefined'` guard, write on
  change).
- **Validation & fallback.** On read, the saved value is validated against that page's
  **current** allowed sort set; an unknown, malformed, or missing value falls back to the
  page's existing default (Media `added_desc`, People `name`, Tags `name`). A corrupt
  storage value never throws into the UI and never blocks rendering. *(This makes removing
  a sort option in future forward-safe — a stale saved value just falls back to default.)*
- **Media precedence (URL wins).** Media's sort already lives in the URL for
  shareability. Precedence on load: **URL `sort` param → saved localStorage value →
  default.** A `sort` present in the URL (e.g. a shared/deep link) always wins; the saved
  preference only supplies the default when the URL omits `sort`. Selecting a sort on Media
  continues to update the URL (existing `replaceState` behavior) **and** writes the
  preference.
- **People / Tags.** No URL sort state (per Non-Goals): the saved value (or default) drives
  the initial sort directly; selecting a sort writes the preference and re-queries.
- **Persisting Random.** `random` is a valid persisted value. Restoring a page whose saved
  sort is `random` re-enters Random with a **fresh session seed** (SP2) — i.e. it draws a
  new shuffle, not the prior order.

**Acceptance criteria — SP1**
- [ ] Given I set People to "Most videos", when I navigate away and return to `/people`,
      then it loads sorted by "Most videos" (not the A–Z default).
- [ ] Given I set People = "Most videos" and Tags = "A–Z", when I visit each, then each
      page reflects its own saved sort (the keys are independent).
- [ ] Given I reload the browser, then each of Media, People, Tags restores its last sort.
- [ ] Given a Media deep-link URL with `?sort=title_asc`, when I open it, then it sorts
      `title_asc` regardless of any saved Media preference (URL wins); with no `sort` in the
      URL, the saved preference (or `added_desc`) applies.
- [ ] Given a garbage value in `holodex:sort:people`, when I load `/people`, then it falls
      back to the default sort and the page renders normally (no thrown error).
- [ ] Given I last chose "Random" on a page, when I return in a new session, then the page
      opens in Random with a new shuffle.
- [ ] The sort preference never appears in any network request (verified client-only).

#### SP2 — Random sort option, stable per session

A "Random" entry on all three pages shuffles the list, holding the order stable for the
session and reshuffling only on a new session or an explicit re-roll.

- **Session seed.** A single integer seed is generated once per session and held in a
  module-scoped store, persisted to `sessionStorage` so it survives in-SPA navigation and a
  same-tab reload. A new browser session / new tab generates a new seed. The same seed is
  shared by all three pages (a coherent "this session's shuffle"); switching a page to a
  different sort and back within the session reuses the held seed (the order is stable),
  consistent with the fetch-and-hold pattern used by the related-media shelves.
- **Re-roll.** When Random is the active sort, the UI exposes a small **re-roll** affordance
  (e.g. a shuffle/↻ icon adjacent to the sort control) that regenerates the seed and
  re-applies the shuffle. Re-roll is only shown/active while Random is selected.
- **People / Tags (client-side shuffle).** These endpoints return the full list in one call
  (`ListPeople`/`ListTags` are unpaged), so Random is a **client-side seeded shuffle** of
  the fetched array using a small deterministic PRNG (e.g. mulberry32) keyed by the session
  seed. No backend change for the shuffle itself beyond accepting the named param (SP3). The
  same seed → the same order across re-renders.
- **Media (server-side seeded ordering).** The Media grid is paginated (offset/limit +
  "Load more"), so the shuffle must be **server-side and deterministic** for pages to tile
  without duplicates or gaps. The client sends `sort=random&seed=<seed>`; the server orders
  by a stable hash of (row id, seed) defined in **ADR-045**. The client holds the seed for
  the session so every "Load more" page and any re-fetch of the same view continues the same
  shuffle.
- **Stability scope.** "Stable per session" means: consistent across pagination, across
  in-app Back navigation, and across incidental re-renders (e.g. a skin switch). It does
  **not** mean reproducible across a new tab/session (Non-Goals) — a fresh session draws a
  fresh seed.

**Acceptance criteria — SP2**
- [ ] Given I select Random on People, then the list renders in a shuffled order; selecting
      A–Z and back to Random (same session) shows the **same** shuffled order (seed held).
- [ ] Given I select Random on Media and click "Load more", then the appended page continues
      the same shuffle — no item appears twice and none is skipped across the page boundary.
- [ ] Given I'm on random-sorted Media, when I open an item and press Back, then the same
      shuffled order (and, with SP-adjacent browse-state preservation, my place) is intact.
- [ ] Given Random is active, when I click the re-roll control, then the list reshuffles to
      a new order (new seed) on that page; pages still tile correctly under the new seed.
- [ ] Given a skin switch while Random is active, then the order does **not** change
      (re-render doesn't reshuffle).
- [ ] Given a new browser tab, when I select Random, then the order differs from the prior
      tab's shuffle (independent session seed) — i.e. Random is genuinely fresh per session.
- [ ] Given Random on Media, the request carries `sort=random&seed=<n>`; given Random on
      People/Tags, the shuffle is performed client-side (no seed needs to round-trip if the
      list is fetched whole).

#### SP3 — Named-sort param for People/Tags + Random support (backend)

The `/people` and `/tags` list endpoints move from a boolean `?sort=count` toggle to a
**named** sort param so they can express Random alongside A–Z / Most-videos; the `/media`
endpoint gains `random`. Contract finalized in **ADR-045**.

- **People / Tags param.** Accept `sort=name` (default, A–Z), `sort=count` (most videos),
  `sort=random`. The existing `sort=count` string continues to work (it's already the
  truthy value), so this is a backward-compatible widening from "is it `count`?" to a
  validated enum; unknown values fall back to the `name` default. For `sort=random`, the
  server may either (a) return the list in natural order and let the client shuffle
  (preferred — these lists are unpaged and small), or (b) apply `ORDER BY RANDOM()`; ADR-045
  picks (a) so the order is client-controlled and stable per the session seed.
- **Media param.** Extend the existing whitelisted sort set (`orderBy()` in `repo.go`) with
  `random`, gated on a `seed` query param. With `sort=random&seed=<int>`, ordering is
  deterministic in the seed (stable hash of id+seed per ADR-045); a missing/invalid seed
  falls back to a server-chosen seed for that single request (still internally consistent,
  but the client always supplies one so pages tile).
- **Validation.** All sort values are server-side whitelisted (no raw string into SQL);
  `seed` is parsed as a bounded integer. Invalid input degrades to defaults, never errors
  hard.
- **No new persistent state.** No tables, no migrations — purely query-shaping.

**Acceptance criteria — SP3**
- [ ] `GET /api/v1/people?sort=count` and `?sort=name` behave exactly as today
      (backward-compatible); `?sort=random` returns the full set (client shuffles).
- [ ] `GET /api/v1/tags` mirrors People for all three values.
- [ ] `GET /api/v1/media?sort=random&seed=123&limit=50&offset=0` then `offset=50` returns
      two disjoint pages whose union has no duplicates and no gaps (the shuffle tiles).
- [ ] `GET /api/v1/media?sort=random&seed=123` returns the **same** order on repeat calls;
      a different `seed` returns a (very likely) different order.
- [ ] An unknown `sort` value on any of the three endpoints falls back to that endpoint's
      default with a 200 (no 4xx/5xx).
- [ ] No SQL injection surface: `sort` is whitelisted and `seed` is integer-parsed
      (verified by `/security-review`, since this touches an API contract).

#### SP4 — Sort controls: add Random + re-roll (frontend, themed)

The existing sort controls gain a Random option and a re-roll affordance, themed across all
three skins.

- **Media (`SortDropdown`).** Add a `random` option to the dropdown's option list
  (label e.g. "Random"). When Random is selected, show the re-roll control beside the
  dropdown.
- **People / Tags (`SortToggle`).** Add a third segment, "Random" (or a shuffle icon
  segment), to the segmented control. When Random is active, show the re-roll control.
- **Re-roll affordance.** A small icon-button (shuffle/↻) using semantic tokens; visible
  only while Random is the active sort. Tooltip/`aria-label` "Shuffle again".
- **Theming.** Tokens only — `bg-accent`/`text-accent-ink` for the active state,
  `text-muted hover:text-ink`, `border-rule`, `rounded-theme` (matching the current
  `SortToggle`/`SortDropdown`). No hardcoded palette/radius. The widened `SortToggle` (now
  three segments) must not overflow or collide on narrow widths in any skin, including the
  `--radius: 0` square treatment in Broadcast/Brutalist.
- **Accessibility.** The Random segment/option is keyboard-reachable and labeled; the
  re-roll button is a real focusable button with an accessible name.

**Acceptance criteria — SP4**
- [ ] Media's sort dropdown lists "Random"; selecting it shuffles the grid and reveals the
      re-roll control.
- [ ] People's and Tags' sort toggles show a third "Random" option; selecting it shuffles
      the list and reveals re-roll.
- [ ] The re-roll control appears only while Random is selected and disappears when another
      sort is chosen.
- [ ] All sort controls (including the new Random state and re-roll) render correctly —
      tokens, radius, contrast, active-state legibility, no overflow/collision — in
      **Cinémathèque, Broadcast, and Brutalist**.
- [ ] Random option and re-roll are keyboard-operable and screen-reader-labeled.

### Nice-to-Have (P1)
- **Re-roll keyboard shortcut** (e.g. press `r` while a Random-sorted list is focused) for
  fast reshuffling. *(Fast follow; the button covers the core need.)*
- **Subtle "shuffled" indicator** on the grid header when Random is active, so the
  non-deterministic order is self-explanatory. *(Polish; the active control already implies
  it.)*
- **Shared `sortPreference.svelte.ts` module** abstracting read/validate/write so all three
  pages (and future sortable surfaces) use one tested seam rather than per-page inline
  localStorage calls. *(Cheap consolidation; recommended even in v1 if it lands naturally.)*

### Future Considerations (P2)
- **Server-synced preferences** if/when multi-user lands — relocate the per-page sort
  behind a small profile/settings store; keep the client read/write behind one module so
  the backend can swap in.
- **Reproducible-shuffle deep links** — a `?sort=random&seed=…` Media URL is already
  reproducible server-side; a "copy this shuffle" affordance could expose it. Deliberately
  out of scope now (Non-Goals: no cross-session reproducibility by default).
- **Random on other listy surfaces** (search results, related shelves already random) reuse
  the same seeded-shuffle module if they ever gain explicit sort controls.

---

## Success Metrics

Single-user personal server, so metrics are qualitative / self-observed:
- **Sticky sort:** the owner stops re-selecting their preferred sort each visit — each page
  opens in the last-chosen order (per-page keys holding).
- **Random:** the owner uses Random to rediscover items; paging through a random Media order
  is coherent (no dupes/gaps), and the order stays put across Back/re-render within a
  session, reshuffling only on re-roll or a new session.
- **No regressions:** no new backend state; `/people` & `/tags` stay backward-compatible;
  Media deep-links still reproduce sort from the URL; all three skins remain clean.

## Open Questions

*All resolved 2026-06-27 (via spec intake) — recorded for traceability.*

- **RESOLVED — per-page independent persistence.** Media, People, and Tags each persist
  their own sort under a separate `holodex:sort:<page>` key (not one global sort), so
  page-appropriate orders like People = "Most videos" and Tags = "A–Z" coexist. *(See SP1.)*
- **RESOLVED — Random is stable per session (seeded).** A session seed (held in a module
  store, persisted to `sessionStorage`) keeps the shuffle consistent across pagination,
  in-app Back, and re-renders, reshuffling only on a new session or an explicit re-roll —
  not a re-shuffle on every visit, and not a frozen order persisted across sessions.
  *(See SP2.)*
- **RESOLVED — scope is all three pages.** Random + persistence apply to Media, People, and
  Tags. The seeded mechanism splits by pagination: server-side for paginated Media
  (ADR-045), client-side for the unpaged People/Tags lists. *(See SP2/SP3.)*

## Timeline / Phasing

No hard deadline. Suggested order:
1. **ADR-045** (Proposed → Accepted) — pin the seeded-ordering expression and the
   `sort`/`seed` contract before backend work.
2. **SP3 backend** — named sort param for People/Tags (backward-compatible) + `random`/`seed`
   on Media; `/security-review` on the contract change.
3. **SP1 sticky persistence** — the `sortPreference` module + wiring all three pages
   (client-only; independent of the backend work for People/Tags A–Z/count).
4. **SP2 + SP4** — session-seed store, client shuffle for People/Tags, seed plumbing for
   Media, and the Random option + re-roll in `SortDropdown` / `SortToggle`, QA'd in all
   three skins.
