# Spec: One list toolbar — shared sort, filter and view across list pages (F73)

**Status**: Draft
**Epic**: HOLODEX-472 · children HOLODEX-473 (multi-sort bug), HOLODEX-474 (legacy filter-key purge, fast-follow)
**Owner**: Project owner
**Date**: 2026-09-27

**Amends**: [`sort-persistence.md`](sort-persistence.md) — SP1's precedence rules; **supersedes
SP5** (sticky filters). Media's "URL →
saved → default" rule becomes the rule for **every** list page, and "People / Tags have no URL sort
state" is withdrawn. See [Amendments to sort-persistence](#amendments-to-sort-persistence).
**Supersedes in part**: [`entity-completeness-handoff.md`](../design/entity-completeness-handoff.md)
§9 is *reinstated*. Completeness is a sort entry, as that handoff specified, not the separate
`CompletenessSortToggle` that People and Studios actually built.

**ADR**: [ADR-114 — list-state model](../architecture/ADR-114-list-state-model.md). It decides
which list state lives in the URL, in localStorage or in the session, and how arrival, Back and
removal exits restore it.

**Design handoff required**: `docs/design/list-toolbar-handoff.md`, with committed SVG mockups
(desktop + 375px, all three skins, owner + visitor).

---

## Problem Statement

Holodex has five list pages: Media (the home grid), People, Studios, Films and Tags. They don't
agree on how you sort, filter or change the view.
- **Media** uses a filter bar with a caption on each control and a `<select>` for sort.
- **The four entity pages** use segmented controls in the header.

Moving between them is jarring, and the inconsistency has already caused a real bug. On People and
Studios, completeness is a *second* sort control with its own state (HOLODEX-473):
- "A–Z" and "Completeness ↓" can both show as active, while only completeness actually orders the
  list.
- Clicking A–Z doesn't undo it.

On a phone, every control wraps into a wall that sits above the first row of data:
- On Media, even a visitor sees about nine wrapped control groups before the first card.
- On People, the owner sees three or four rows of controls plus a 27-letter A–Z bar that wraps.

List state is also stored inconsistently. Media keeps sort and filters in the URL *and*
localStorage. People and Studios keep them only in localStorage. So a filtered People view can't be
shared or bookmarked, and saved filters can come back invisibly on a later visit.

## Goals

1. **One toolbar, one set of words.** Every list page draws its sort, filter and view controls
   from the same shared toolbar, in the same order and style. A page only omits slots it has no
   use for.
2. **Sort is always a single choice.** Two sort controls can never both show as active. This
   fixes HOLODEX-473 by construction.
3. **Data comes first on a phone.** At 375px, every list page shows the title, a single toolbar
   row and at most one row of active-filter chips before the first row of data, for owner and
   visitor alike.
4. **List state is predictable.**
   - Your sort and view come back on every visit.
   - Filters come back only when you return to where you were (Back, or a redirect after an
     action).
   - A plain nav click always shows the full, unfiltered list.
   - Anything visible is in the URL, so any view can be shared.
5. **Tokens only, three skins.** Everything renders cleanly in Cinémathèque, Broadcast and
   Brutalist, with no hardcoded styling.

## Non-Goals

- **Undoing filters with Back.** Filter changes use `replaceState`. Back leaves the list, it
  doesn't step through each chip you added. *(Why: pressing Back from a detail page and then
  having to press it again for every filter reads as a trap. Nobody asked for undo.)*
- **Seeds in URLs.** A shared link with Random sort reshuffles for whoever opens it, as
  sort-persistence SP2 already says. *(Why: reproducing an exact shuffle is a separate feature,
  HOLODEX-31.)*
- **Saved or named views.** *(Why: bookmarking the URL already covers this once filters live in
  it. Named views need server persistence, which doesn't exist.)*
- **The Missing (completeness facet) filter on list pages.** It's removed from the People,
  Studios and Media toolbars. *(Why: it's a work queue, and the owner page already has a
  missing-facet queue. HOLODEX-262's "missing-facet filter chip" is superseded for list pages.)*
- **Tag, studio or person pickers on Media.** *(Why: the entity pages already answer "show me
  this studio". Media receives that scope as a chip, see R5.)*
- **New filters.** This epic moves and restyles existing filters. It adds no new filterable
  fields.

## User Stories

**Visitor browsing on a phone**
- As a visitor on my phone, I want to see media or people as soon as a list page opens, so I'm
  not scrolling past controls to reach content.
- As a visitor, I want to see at a glance which filters are narrowing the list, and remove one
  with a single tap, so a filtered list never looks like the whole catalog.

**Anyone moving between pages**
- As a user, I want sort, filter and view to look and work the same on Media, People, Studios,
  Films and Tags, so I don't relearn each page.
- As a user, I want to go from a studio's page to its media, narrow by resolution, and see "Studio:
  Acme" as a removable chip, so I can combine an entity with file properties.
- As a user, I want to open a person, press Back and land on the list exactly as I left it, with
  the same filters and sort, so browsing is non-destructive.
- As a user, I want clicking **People** in the nav to show me all people in my usual sort, so
  yesterday's filter doesn't silently hide results.
- As a user, I want to copy the URL of a filtered list and send it to someone, and have them see
  the same list.

**Owner**
- As the owner, I want completeness in the same sort list as A–Z, so choosing it obviously
  replaces the other sort.
- As the owner, I want page actions (Merge people, Manage tags) in a `⋯` menu instead of the
  control row, so owner tooling doesn't push content down.
- As the owner, after I delete or merge something and get sent back to the list, I want my
  filters still applied, so I can keep working through the same slice.

**Edge cases**
- As a user opening a shared link, I want to see the sender's view without it replacing my own
  saved sort.
- As a user, when a filter matches nothing, I want an empty state that names the active filters
  and lets me clear them in one tap.

## Requirements

### P0: Must-have

**R1. Shared list toolbar component.** One component is used by all five list pages. It has three
slots, in this order, and a slot only appears when the page gives it something:

| Slot | Media | People | Studios | Films | Tags |
|---|---|---|---|---|---|
| **Sort ▾** | ✓ | ✓ | ✓ | ✓ | ✓ |
| **Filters (n)** | ✓ | ✓ (only if any filter remains after R6) | ✓ (only if any remains) | — | — |
| **View** (List/Poster, density) | density | List/Poster (+ density in Poster) | — | — | — |

- The shuffle-again control (`SortReroll`) appears next to Sort when Random is active. That's
  unchanged.
- The page title row gets a `⋯` menu holding owner page actions: Merge people (People), Manage
  tags (Tags). Visitors see no `⋯` when it would be empty.

*Acceptance:*
- [ ] Every list page renders sort, filter and view through the shared component. None of them
      still render `SortToggle` or `CompletenessSortToggle`, or hand-roll a segmented sort.
- [ ] The slot order is identical on every page, and an empty slot leaves no gap or placeholder.
- [ ] At 375px the toolbar is a single row that doesn't wrap and doesn't scroll sideways, for the
      owner on every page.

**R2. Sort is a single choice (fixes HOLODEX-473).**
- Each page has one sort value, picked from one list.
- Direction is part of the entry, as in Media's `MEDIA_SORTS` (for example `title_asc`).
- Completeness (most first, least first) is an **owner-only entry in the same list** on Media,
  People and Studios. Visitors never see it.

*Acceptance:*
- [ ] Given the owner picks "Completeness (most first)" on People, only that entry shows as
      selected, and A–Z does not.
- [ ] Given completeness is the sort, when the owner picks A–Z, then the list re-sorts A–Z and
      the A–Z jump index appears.
- [ ] A saved completeness sort restored for a visitor session (for example on a shared device
      after logout) falls back to the page default.

**R3. Filters sheet and active-filter chips.**
- **Filters (n)** opens every filter the page has.
  - Below the small-screen breakpoint it opens as a sheet.
  - On desktop, it's a popover or inline row; the design handoff decides.
  - `n` is the number of active filters.
- Every active filter shows as a removable chip under the toolbar, plus a **Clear** action when
  two or more are active. Chips are the only thing that tells you a list is narrowed, so they're
  mandatory, not decorative.
- Media's filters in the sheet: Resolution, Duration, Year, and the mapped facets (for example
  Collection).

*Acceptance:*
- [ ] Given any filter is active, a chip naming it is visible without opening the sheet.
- [ ] Removing a chip re-queries and updates the URL. Clear removes all chips.
- [ ] The empty result state lists the active filters and offers Clear.

**R4. Tags page: All / Tags / Categories become tabs.** They render as tabs under the title, not
as a segmented control in the toolbar. The tab is a filter, stored in the URL as `?type=tags` or
`?type=categories`, with `all` as the default and omitted from the URL.

*Acceptance:*
- [ ] The Tags toolbar contains Sort only, and the tabs sit between the title and the toolbar.
- [ ] `/tags?type=categories` opens on the Categories tab. Back from a tag detail page returns to
      the same tab.

**R5. Entity scope chips on Media.**
- Media's `person`, `tag`, `studio_id` and `category` URL params render as scope chips, for
  example "Studio: Acme ×".
- They combine with the R3 filters.
- There's no picker for adding them: they arrive only from an entity page link.
- The Tags type-ahead is removed from Media.

*Acceptance:*
- [ ] Following a studio's "all media" link lands on Media with a Studio chip, and adding 4K
      narrows further.
- [ ] Removing the chip widens the list to the whole library and removes the param.

**R6. Missing filter removed from list pages.** The `FacetFilter` "Missing" control is removed
from the Media, People and Studios toolbars. The owner-page missing-facet queue stays the home for
that workflow.

**R7. The list-state model.** This is the contract the ADR will formalise.

| State | Where it lives | How it's restored |
|---|---|---|
| Sort (including Random) | localStorage `holodex:sort:<page>` **and** the URL `sort` param (omitted when it's the default) | Every visit. A URL value wins over a saved one. |
| View (List/Poster) and density | localStorage | Every visit |
| Filters, scope chips, Tags tab, search text `q`¹ | **URL only** | From the URL itself: Back navigation, a redirect back to the originating list (R8), or a shared link |
| Random seed | Session (unchanged from sort-persistence SP2) | Never from the URL |

¹ Making the entity pages *read* `?q=` is [nav-search-live-filter](nav-search-live-filter.md)
NS3's job, not this epic's. F73 only fixes where `q` lives: in the URL, never in storage, the same
as every other filter.

- **Only an explicit user choice writes to storage.** Arriving by URL (a shared link, Back, or a
  redirect) never writes to localStorage.
- **Every change updates the URL with `replaceState`**, so the address bar always matches what's
  on screen. On arrival at a bare path, the restored sort is written into the URL.
- **A plain nav link (`/people`) shows no filters**: the saved sort, no filters, the full list.

*Acceptance:*
- [ ] Given People is sorted "Most videos" and filtered, when I click **People** in the nav, then
      it opens "Most videos" with no filters, and the URL shows `?sort=count`.
- [ ] Given I filter Media to 4K, open a video and press Back, then Media is still filtered to 4K
      with the same sort.
- [ ] Given my saved People sort is A–Z, when I open a shared `/people?sort=count` link, then I
      see "Most videos". When I later click **People** in the nav, I get A–Z again, because the
      link didn't overwrite my sort.
- [ ] Given a garbage `sort` value in the URL or in storage, the page falls back to its default
      without throwing (unchanged from SP1).
- [ ] Given a filtered list, when I reload, the filters persist, because they're in the URL.
- [ ] Legacy saved filters (`holodex:filters:*`) are **ignored** on read from this release. Deleting
      them is HOLODEX-474.

**R8. Redirects after actions return to the list you came from.**
- When an action sends you to a list page, you return to the list URL (sort and filters) you left
  from in this tab, if you came from that list. Otherwise you get the bare list.
- Mechanism: [ADR-114](../architecture/ADR-114-list-state-model.md) D4.
  - The exit is `history.back()` when you arrived from within the app, otherwise the bare list.
    This generalises the media-delete exit from HOLODEX-41. It's per tab by construction.
  - Merges that keep you on the page reload in place, so they need no redirect.

*Acceptance:*
- [ ] Given Media is filtered and sorted, when I open a video and delete it, then Media comes back
      with the same filters, sort and scroll.
- [ ] Given I opened a video from a deep link in a new tab, when I delete it, then I land on the
      bare `/`.
- [ ] Given People is filtered, when I merge people from the list, then the list reloads with the
      same filters and sort.
- [ ] Two tabs on different People views each return to their own view.

**R9. Mobile A–Z index.** Below the small-screen breakpoint, the People A–Z jump bar becomes a
compact rail on the right edge, one column that doesn't wrap. At or above the breakpoint it stays
the sticky horizontal bar. It shows only when the sort is A–Z, and it no longer hides behind a
separate completeness state (see R2).

*Acceptance:*
- [ ] At 375px no A–Z letters sit above the first row. The rail doesn't overlap row content that
      you can tap.

**R10. Three skins, tokens only, accessible controls.**
- Tokens only (`.claude/rules/frontend-theming.md`), QA'd in all three skins.
- The sort list is a single-choice control with radio semantics.
- The Filters sheet traps focus and closes on Escape.
- Chips have accessible names ("Remove filter: Resolution 4K").
- Tabs use tab semantics.

### P1: Nice-to-have

- **P1-a. Result count in the toolbar area** ("412 people"), consistent across pages. Media has it
  today and the entity pages don't.
- **P1-b. The Filters sheet shows a live result count** ("Show 38 videos") where the API can
  answer cheaply.
- **P1-c. Consolidate stray segmented-control class copies** (`PersonViewToggle`, the Tags
  `typeCls` helper) onto `segmentedToggle.ts`, where they survive the redesign.

### P2: Future considerations (design for, don't build)

- **Filters on entity pages** (People by nationality, and so on, via F46 queryable attributes).
  The sheet and chip design must take new filter types without layout changes.
- **Undo for filter changes** (`pushState`). Keep all URL writes in one function so this could
  change in one place.
- **Removing legacy keys** is HOLODEX-474, gated on production confirmation.

## Amendments to sort-persistence

This spec changes these clauses of [`sort-persistence.md`](sort-persistence.md) SP1. A pointer
note is added there.

- **"Media precedence (URL wins)"** now applies to **every** list page, not just Media.
- **"People / Tags. No URL sort state"** is withdrawn. People, Studios, Films and Tags mirror sort
  into `?sort=`, with the default omitted.
- **"Selecting a sort … writes the preference"** stays, and gets the R7 corollary: arriving by
  URL never writes.
- **SP5 (per-page sticky filters, HOLODEX-25) is superseded.**
  - SP5 existed because Back from a detail page lost People and Studios filters, which lived only
    in component state. R7 solves the same problem through the URL: the Back entry carries the
    filters.
  - The `holodex:filters:*` keys stop being read in this release. HOLODEX-474 deletes them.
  - SP5's knock-on fix, the `listScroll` snapshot, stays. It is re-keyed on the list's URL query
    string (see ADR-114).
- SP2 (seed) and SP3 (seeded Media ordering, ADR-045) are unchanged.

## Success Metrics

This is a single-owner instance with no analytics, so the metrics are checked in QA against a
seeded instance (the HOLODEX-342 stress fixture) and then in production.

**Leading (checked at merge)**
- **Controls before the first row at 375px.** Target: the title, 1 toolbar row and ≤ 1 chip row
  on every list page, for owner and visitor. Baseline: Media visitor about 9 wrapped groups;
  People owner 4 rows plus the wrapped A–Z bar.
- **Two sorts active at once.** Target: 0 on every page. Baseline: reproducible on People and
  Studios.
- **Separate sort/filter implementations.** Target: 1 shared toolbar. Baseline: `SortDropdown`,
  `SortToggle`, `CompletenessSortToggle`, the Films inline control and the Tags hand copy.

**Lagging (production, about 2 weeks)**
- The owner confirms that "I got a filtered list I didn't expect" no longer happens. That
  confirmation is the gate for HOLODEX-474.
- No regression reports on Back or redirect restoring state (R7, R8).

## Open Questions

*Non-blocking for the spec; each is answered in the named artifact.*

- **[design] Desktop Filters surface.** Should it be a popover, or an always-visible inline row
  when the page has ≤ 3 filters? → design handoff.
- **[design] Where does density go?** It's a view setting on Media and on People's Poster view.
  Should it sit inside the View slot's menu or beside it? → design handoff.
- ~~**[engineering] Redirect mechanism for R8.**~~ Resolved by ADR-114 D4: `history.back()` when
  in-app, otherwise the bare list.
- **[engineering] Which entity pages link to Media with a scope param today?** R5 assumes Person,
  Studio and Tag detail pages do; any that don't need the link added. → frontend gate, non-blocking.

## Timeline Considerations

- **Order:** spec → ADR (list-state model) → design handoff → `/implement` → frontend → testing
  strategy.
- **No backend change is expected.** Every param already exists on the list APIs. If `q` or
  `type` turns out to need server support on an entity list, that becomes a `backend` gate row.
- **HOLODEX-474** runs after production confirmation (see Lagging metrics), not in this epic's PR.
- **Related, not blocking:**
  - HOLODEX-404 (`/?q=` deep link shows no results): R7 puts `q` in every list URL, so that fix
    should land first or with this epic.
  - HOLODEX-436 (375px toolbar overflow): already released; R1's acceptance guards against it
    coming back.
