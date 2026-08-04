# Spec: Unified nav search — live, tabbed, in-place filtering

**Status**: Draft
**Phase**: Standalone frontend UX feature; no phase dependency
**Owner**: Project owner
**Date**: 2026-08-04

**Depends on**: Nothing new. Builds only on shipped surfaces — the global nav search box
and its history dropdown (`+layout.svelte`, `searchHistory.svelte.ts`), the existing
`/search` aggregation endpoint, the Media browse page's URL-synced `MediaFilters`
(`filters.ts`), the Tags page's client-side `filterByName` pattern, and the roving-tabindex
convention established by `EnrichPicker.svelte`.

**New ADRs required**: None. This extends existing, already-established patterns
(per-page query-param filtering on Media, client-side array filtering on Tags, the
existing `/search` aggregation contract) to more pages — it does not introduce a new data
model, a new cross-cutting infra decision, or a new API contract shape.

> **Routing note.** This is a user-facing UX surface change, so it must also go through
> **`/design-handoff`** (exact panel layout, spacing, empty/loading states, mobile
> breakpoints) and **`/testing-strategy`** before merge, per this repo's change-routing
> table. **`/security-review` is not required for the P0 scope** (no new backend
> endpoints — see Technical Notes); it becomes relevant only if the P1 detail-page item
> (NS6) ends up needing server-side pagination support.

---

## Problem Statement

Holodex's nav search and its per-page filter inputs evolved independently and now overlap
awkwardly. The Media browse page and the Tags page each carry their own standalone search
box, stacked directly under (or near) the always-visible global nav search box — two
visually similar inputs doing different jobs, which reads as cluttered and is especially
cramped on mobile, where the two inputs alone can consume a meaningful share of the
viewport before any content is visible. Separately, the People and Studios list pages have
no filtering at all today, forcing the owner to scroll a flat, unordered grid to find
anything. Both problems trace back to the same root cause: filtering was bolted on
per-page instead of being one consistent, reusable capability anchored to the nav search
box the owner already reaches for.

## Goals

1. **One visible search/filter affordance per screen.** No page shows two boxes doing
   overlapping jobs; the nav search box is the single entry point everywhere.
2. **Filtering exists on every list page.** Media, People, Studios, and Tags all support
   live, in-place filtering via the same mechanism — People and Studios gain a capability
   they have never had.
3. **Global "find anything" is preserved and improved.** Cross-entity search (today: type,
   press Enter, land on `/search`) becomes live and grouped as you type, without leaving
   the page, while still supporting a full deep-dive view for a given type.
4. **Consistent, predictable behavior.** The same box, the same tab row, and the same
   interaction pattern appear on every page — what changes is only what renders below,
   never what the box itself does.
5. **No regression to mobile usability.** The redesigned panel and tab row degrade
   gracefully at narrow widths — this is the primary pain point driving the feature and
   must be visibly better than today, not just functionally equivalent.

## Non-Goals

- **Folding resolution/duration/year facets into the search panel.** Those stay as
  Media's own small, page-scoped control, applied on top of whatever nav search is
  showing. *(Why: explicitly decided during scoping — facets are a different kind of
  control than text/entity search, and folding them in only makes sense for one tab out
  of five.)*
- **Removing or redesigning the recent-search-history dropdown.** It keeps working exactly
  as it does today (shown on focus while the box is empty) — this feature only changes
  what happens once the owner starts typing. *(Why: it's a separate, already-working
  capability; no reason to touch it.)*
- **Changing the Ctrl/Cmd-K trigger mechanism.** It still focuses the existing box; this
  spec is entirely about what the box *shows*, not how it's summoned. *(Why: out of
  scope, already works.)*
- **Fuzzy/typo-tolerant matching or search ranking improvements.** Both the in-place
  filters and the `/search` aggregation keep whatever matching logic they use today.
  *(Why: separate initiative — this spec is about surface consolidation, not search
  quality.)*
- **Multi-device or account-level search personalization.** Holodex is single-owner with
  no accounts; search state stays local to the browser exactly as it does today. *(Why:
  no accounts exist; would be premature infrastructure.)*

---

## User Stories

**In-place filtering**
- As the **library owner**, when I type in the nav search box while looking at the Media
  grid, I want the grid itself to filter down to matches so that I don't need a second,
  separate box to narrow what I'm already looking at.
- As the **library owner**, I want the same live filtering on the People and Studios
  pages, which have no filter today, so that I can find someone or a studio without
  scrolling a flat grid.
- As the **library owner**, I want the Tags page's existing filter behavior preserved but
  driven by the same top box, so I'm not relearning a page-specific input.

**Global search**
- As the **library owner**, when I'm on a page where my query doesn't match what that page
  shows (e.g. I search a person's name while on the Tags page), I want to see grouped,
  tabbed results across entity types without losing my place, so I can jump to what I
  actually meant without a full page navigation up front.
- As the **library owner**, I want a "view all" path from any group into a full results
  view for that entity type, so a quick glance can become a deep dive when I need it.

**Consistency**
- As the **library owner**, I want the People/Videos/Studios/Tags tab row to always be
  visible and behave the same way regardless of which page I'm on, so the box never
  surprises me by changing its own rules.
- As the **library owner**, I want this to feel good on my phone, not just my desktop,
  since that's where the current clutter bothers me most.

---

## Requirements

### Must-Have (P0)

#### NS1 — Live tabbed, grouped search panel

The nav search box's dropdown (today: recent-history only, shown on empty focus) extends
into a live results panel the moment the owner types, with an **All / People / Videos /
Studios / Tags** tab row always shown directly under the box.

- Panel data source: the existing `GET /search?q=` aggregation endpoint — no new backend
  call. Results render grouped by type (a short header per group, a handful of rows per
  group) with a **"View all N in <type>"** row when a group has more matches than shown.
- Debounced as-you-type querying (reuse whatever debounce pattern exists for other
  live-query surfaces in the codebase, or a simple ~200–300ms debounce if none exists).
- A visible clear (×) control appears in the box once it has text.
- The recent-history dropdown continues to show on empty+focus exactly as today; the
  panel described here only appears once there is a non-empty query.
- **Reuse, don't fork.** The `/search` results page and this panel should share one
  grouped/tabbed results component — the page renders it full-width as the page body, the
  nav box renders it as a dropdown. Avoid building two separate implementations of
  "grouped-by-type result rows."
- **Mobile.** At narrow widths, the panel must not require horizontal scrolling and must
  not push meaningful content below the fold before showing anything useful — validate
  this explicitly, since mobile clutter is the primary complaint driving this spec.

**Acceptance criteria — NS1**
- [ ] Given the box is empty and focused, then the existing recent-history dropdown shows
      (unchanged from today).
- [ ] Given I type a query, then the history dropdown is replaced by the grouped/tabbed
      panel within the debounce window, without a full page navigation.
- [ ] Given a group has more matches than shown, then a "View all N in <type>" row appears
      and navigates to the corresponding full list, query pre-filled.
- [ ] Given I clear the box (via the × control or deleting all text), then the panel closes
      and the history dropdown (if history exists) reappears.
- [ ] Given a narrow (mobile-width) viewport, then the panel and tab row render without
      horizontal scroll or layout overflow, in all three skins.

#### NS2 — Scope-matched in-place filtering, tab-mismatch overlay

Each page that shows a single entity-type list declares its own scope. When the
**selected tab matches the current page's scope**, the panel does not render — instead,
the query drives that page's own grid in place. When the selected tab is **"All" or does
not match** the page's scope, the NS1 panel renders instead.

- **Page scopes:** Media/browse (`/`) → Videos; People (`/people`) → People; Studios
  (`/studios`) → Studios; Tags (`/tags`) → Tags; person/studio/category detail pages → 
  Videos (their embedded video list — see NS6, P1).
- **Default tab on load:** matches the current page's own scope (e.g. landing on `/people`
  pre-selects the "People" tab). Typing immediately filters the page in place.
- **Tab switch while scoped:** tapping a non-matching tab (or "All") does **not** navigate
  away automatically — it shows the NS1 panel scoped to that tab's results, letting the
  owner preview before committing to a "View all" click-through. This keeps the box's
  behavior identical everywhere: matching tab = in-place, anything else = panel.
- Pages with no natural single-entity scope don't currently exist in this app (every page
  maps to one of the four scopes above); if one is added later, it defaults to panel-only
  behavior.

**Acceptance criteria — NS2**
- [ ] Given I land on `/people`, then the "People" tab is pre-selected and typing filters
      the People grid in place, panel closed.
- [ ] Given I'm on `/people` with "People" selected and I tap the "Videos" tab, then the
      People grid stops updating and the NS1 panel opens showing grouped results
      (Videos-focused), without navigating away from `/people`.
- [ ] Given I'm on `/people` and select "All", then the panel opens (never in-place),
      regardless of query content.
- [ ] Given I switch back to the matching tab after previewing another, then in-place
      filtering resumes on the current page's own grid.

#### NS3 — Per-page in-place filter mechanics

- **Media (`/`).** The nav search box becomes the source of the existing `MediaFilters`
  text field — same URL-synced (`filtersToParams`/`paramsToFilters`), same server-side
  `GET /media` query, same debounce/loading behavior already in place. No backend change.
- **People (`/people`) / Studios (`/studios`).** Both endpoints return their full list in
  one unpaged call (confirmed pattern, matching Tags). Filtering is **client-side**, over
  the already-fetched array — generalize the Tags page's existing `filterByName` into a
  small shared utility used by People, Studios, and Tags alike. No backend change.
- **Tags (`/tags`).** Same client-side mechanism as today, now driven by the top box
  instead of its own removed input (see NS4).
- None of the above require new backend endpoints or query params for P0.

**Acceptance criteria — NS3**
- [ ] Given I type on `/`, then the Media grid re-queries the server and updates, and the
      URL reflects the query (shareable/bookmarkable, matching today's Media filter
      behavior).
- [ ] Given I type on `/people` or `/studios`, then the grid filters client-side with no
      network request beyond the page's original full-list fetch.
- [ ] Given I type on `/tags`, then behavior matches today's Tags filter exactly, just
      driven by the top box.

#### NS4 — Remove standalone duplicate inputs

The Media page's own text input and the Tags page's own text input are deleted — their job
is now done entirely by the nav search box via NS2/NS3. Media's resolution/duration/year
facet controls remain untouched.

**Acceptance criteria — NS4**
- [ ] The Media page renders with exactly one text-search affordance on screen (the nav
      box) — its own inline text input is gone; facet controls (resolution/duration/year)
      remain.
- [ ] The Tags page renders with exactly one text-search affordance on screen.

#### NS5 — Keyboard and accessibility parity

The expanded panel and tab row are fully keyboard-operable and screen-reader-labeled,
following this repo's existing convention for selectable popup results.

- **Roving tabindex**, not `aria-activedescendant`, for result rows in the panel — Tab
  should reach and move through result rows, matching the pattern already established by
  `EnrichPicker.svelte`. (Note: today's history dropdown uses `aria-activedescendant`;
  since this panel is a substantial rebuild of that surface, bring it in line with the
  newer, preferred convention rather than extending the older pattern.)
- Tab row itself is keyboard-navigable (arrow keys or Tab, consistent with other tab/toggle
  controls in the app).
- Escape closes the panel and returns focus to the box without altering the current page's
  in-place filter state.
- Enter behavior: with a row highlighted, activates it (same as a click); with nothing
  highlighted, submits the current query as today's `runSearch` does for the fallback
  "view everything" case.

**Acceptance criteria — NS5**
- [ ] Given the panel is open, then Tab reaches each visible result row and each tab in the
      tab row, in a sensible visual order.
- [ ] Given I press Escape, then the panel closes and the in-place grid (if any) is
      unaffected.
- [ ] All new interactive elements have accessible names/labels (verified with a
      screen-reader pass or axe-style check).

### Nice-to-Have (P1)

- **NS6 — Detail-page embedded video lists (person/studio/category).** In-place filtering
  of the video list shown on `people/[id]`, `studios/[id]`, `categories/[id]` pages,
  scoped to Videos like Media. *(Deferred to P1 pending a quick technical check — see Open
  Questions — because it's unknown whether these embedded lists are paginated like Media
  or unpaged like People/Studios/Tags; if unpaged, this is a cheap client-side add and
  should ship alongside P0; if paginated, it needs the same server-side query-param
  treatment Media already has.)*
- **Shared `navSearch.svelte.ts` module** cleanly separating query state, debounce, and
  scope-matching logic from the panel's rendering, so future pages can opt in without
  duplicating NS2's routing logic. *(Recommended even in v1 if it falls out naturally —
  mirrors the existing `filters.ts`/`searchHistory.svelte.ts` module pattern.)*
- **Re-usable "View all" deep-link consistency** — ensure `/search?q=&type=X` (used by
  "View all" rows) and the People/Studios/Tags/Media in-place query state use one shared
  URL-param convention, so a shared link always reproduces the same filtered view.

### Future Considerations (P2)

- **Query syntax/operators** (e.g. `reso:1080`) if facets ever need to be searchable from
  the box — explicitly out of scope now (see Non-Goals) but the panel's architecture
  shouldn't preclude it later.
- **Extending scope-matched in-place filtering to any future list page** (e.g. a
  hypothetical Playlists page) — the NS2 scope-declaration mechanism should be designed
  generically enough that a new page opts in with a small config addition, not a bespoke
  implementation.

---

## Success Metrics

Single-user personal server, so metrics are qualitative / self-observed:
- **Clutter:** no page shows more than one text-search affordance at once, verified across
  Media, Tags (removed duplicates) and People, Studios, detail pages (never had one to
  begin with).
- **Mobile:** the owner's stated pain point — cramped stacked boxes on mobile — is
  visibly resolved; the panel and tab row don't require excessive scrolling or horizontal
  overflow on a phone-width viewport.
- **Coverage:** People and Studios, which had zero filtering before this spec, are
  filterable in place, same as Media and Tags.
- **No regressions:** Media's URL-shareable filter state keeps working; the recent-search
  history dropdown is untouched; `/search` still supports a full deep-dive view.

## Open Questions

- **RESOLVED — are detail-page embedded video lists paginated?** No. Confirmed against
  `EntityVideos.svelte` and its callers (`api.getPerson`/`api.getStudio`): both return the
  full `videos` array in one unpaged response, same shape as People/Studios/Tags. NS6 is a
  cheap client-side filter with no backend work — see
  [nav-search-live-filter-handoff.md Part F](../design/nav-search-live-filter-handoff.md#part-f--resolves-open-question-ns6-detail-page-video-lists),
  which recommends promoting NS6 from P1 to P0 on that basis.
- **RESOLVED — exact panel visual spec.** Pinned in
  [nav-search-live-filter-handoff.md Part B](../design/nav-search-live-filter-handoff.md#part-b--searchresultspanelsvelte-ns1)
  (3 rows/group before "View all" on the All tab, flat 8-row cap on a single-scope tab,
  skeleton/empty/error states, mobile fixed-sheet breakpoint at 640px).
- **RESOLVED — should `/search` adopt this spec's grouped/tabbed panel as its page body?**
  Yes, full reuse — see
  [nav-search-live-filter-handoff.md Part E](../design/nav-search-live-filter-handoff.md#part-e--resolves-open-question-search-page-reuse).
  `SearchResultsPanel.svelte` is shared verbatim between the nav dropdown and the
  `/search` page body, positioning-only differences.

## Timeline / Phasing

No hard deadline. Suggested order:
1. **`/design-handoff`** — pin the panel/tab visual spec and mobile breakpoints (resolves
   the design-tagged open questions above) before frontend work starts. **Done** — see
   [nav-search-live-filter-handoff.md](../design/nav-search-live-filter-handoff.md).
2. **NS1 + NS5** — build the shared grouped/tabbed panel component and its keyboard/a11y
   behavior first, since NS2/NS3 both depend on it existing.
3. **NS2 + NS3 + NS4** — wire scope-matched in-place filtering into Media, People,
   Studios, and Tags, and remove the two standalone inputs. These four can land together
   or be sequenced page-by-page (Media first, since it's highest-traffic and already has
   the most infrastructure to hook into).
4. **`/testing-strategy`** pass covering the tab-mismatch/in-place routing logic (NS2) and
   all three skins/mobile (per this repo's frontend-theming QA rule).
5. **NS6 (P1)** — once the engineering-tagged open question is resolved, fold in
   detail-page in-place filtering if it turns out to be cheap; otherwise treat as a
   fast-follow.
