# Spec: Paged entity lists — People, Studios, Tags and Films load a page at a time (F80)

**Status**: Draft
**Story**: HOLODEX-354 · subtasks HOLODEX-586 (pickers and datalists → bounded search), HOLODEX-587
(owner queues paged)
**Owner**: Project owner
**Date**: 2026-10-18

**Amends**: [`sort-persistence.md`](sort-persistence.md) (Random on these four pages is no longer a
client-side shuffle), [`nav-search-live-filter.md`](nav-search-live-filter.md) NS3 (in-place filtering
on People, Studios and Tags queries the server), and [`people-poster-view.md`](people-poster-view.md)
(its "no pagination change" non-goal no longer holds). See
[Amendments to other specs](#amendments-to-other-specs).

**Design handoff**: to come (`/design-handoff`). It covers the Load more control, the A–Z restart
state and the Tags stream.

---

## Problem Statement

The People, Studios, Tags and Films pages fetch their entire library and draw every entry at once.
The Media grid is the only list that loads a page at a time. At 2,000 of each kind (the stress
fixture), `/people` draws 2,064 rows and 2,064 images, about 12,500 page elements on a 112,000px
page. `/tags` draws 4,037 pills on a 452,000px page. Each page takes about 2 seconds to settle, and
the cost grows with the library. Every visit pays it, even a visitor who only wants the first
screen. The server answers in milliseconds; the cost is in sending everything and drawing everything.

## Goals

1. **First screen is bounded.** Opening any of the four pages draws at most one page of entries,
   whatever the library size.
2. **Nothing becomes unreachable.** Every entry is still reachable by Load more, search, or (on
   People and Studios) the A–Z index.
3. **Same behaviour, new plumbing.** Sort, Random, search, counts and Back keep the results the owner
   gets today. The only visible changes are the Load more control and the A–Z restart.
4. **One paging pattern.** The four pages page the way the Media grid and entity video grids already
   do: a fixed page, then an explicit Load more.

## Non-Goals

- **Infinite scroll or virtualization.** *(Why: the owner chose explicit Load more to match the Media
  grid. A whole-list virtual scroller still sends the whole library.)*
- **Alias matching in search.** Search matches what it matches today: name and display name. *(Why:
  a parity port. Alias matching is its own change.)*
- **A search or letter in the URL.** As today, only the sort goes in the URL; a shared link opens at
  the start. *(Why: unchanged from [`list-toolbar.md`](list-toolbar.md); seeds and queries stay out of
  links.)*
- **Pickers, datalists and owner queues.** *(Why: HOLODEX-586 and HOLODEX-587 cover them.)*
- **Tag hierarchy display.** The Tags page stays one flat pill cloud. *(Why: separate concern.)*

## User Stories

**Visitor**
- As a visitor, I want People to open quickly on a large library, so that I'm not waiting for
  thousands of rows I won't scroll to.
- As a visitor, I want to click "M" in the A–Z index and land on the M names, so that I can find
  someone without loading A through L.
- As a visitor, I want search on People, Studios and Tags to find any match in the library, not just
  among what's loaded.
- As a visitor, I want Random to keep its order as I load more, without repeats or gaps.
- As a visitor, I want Back from a detail page to return me to the same spot, with the same entries
  loaded.

**Owner**
- As the owner, I want merge, manage and bulk actions to keep working across everything I've loaded.
- As the owner, I want the "already exists" check when I create a tag to cover the whole library,
  not just the loaded pills.

**Edge cases**
- A library smaller than one page shows no Load more.
- A letter with no names stays dimmed and can't be clicked.
- Merging, deleting or creating an entry while several pages are loaded doesn't skip or repeat
  entries on the next Load more.
- Search with no match shows today's "No … match" message.

## Requirements

### P0: Must-have

**EL1. One page at a time.** People, Studios and Films show their first **50** entries. Tags shows its
first **200** pills (pills are small). Load more appends the next page in place, without moving the
scroll position. Load more disappears once everything is loaded.

**EL2. Counts are library totals.** "N people", "N studios" and "N films" count the whole library (or
the whole match set while searching), not what's loaded. The Tags search line "N results for “q”" does
the same.

**EL3. Sorts are unchanged and whole-library.** Every sort on offer today stays, with the same labels,
owner-only gating, defaults and persistence ([`list-toolbar.md`](list-toolbar.md)). Each sort orders
the whole library, so page 2 continues page 1's order.

**EL4. Random tiles across pages.** Random on all four pages uses the session's shuffle seed, as it
already does ([`sort-persistence.md`](sort-persistence.md) SP2/SP3). Loading more continues the same
shuffle with no repeats and no gaps. Re-roll redraws from page 1. The seed stays out of the URL.

**EL5. Search covers the library.** On People, Studios and Tags, typing in the nav box (on the
matching tab, per [`nav-search-live-filter.md`](nav-search-live-filter.md) NS2) shows the first page
of matches from the whole library. Matching is unchanged: a case-insensitive substring of name or
display name. Results update after a short pause in typing, not on every keystroke. Typing doesn't
reshuffle Random. Clearing the box returns to the unfiltered first page. Films has no search, as
today.

**EL6. A–Z restarts the list at a letter.** On People and Studios, in name sort, without a search, in
List view (the same conditions as today), clicking a letter reloads the list to start at that
letter's first name and scrolls to the top of the list. Load more continues into the following
letters. To go back toward A, click an earlier letter. "#" covers names that don't start with a
letter, as today. Letters with no names in the whole library are dimmed and inert. The letter start
is shown as the active letter. Changing sort, view or search clears it.

**EL7. Back restores what was loaded.** Returning by Back (or reload) to any of the four pages
restores the same sort, search, letter start and number of loaded entries, then the scroll position.
The rules for when a restore applies are unchanged (HOLODEX-573): a plain link starts fresh at the
top.

**EL8. Tags All tab is one stream.** The All tab pages tags first, in the chosen sort, then
categories once the tags run out, in today's order (categories are never shuffled). One Load more
serves the stream. The Tags tab and the Categories tab each page only their own kind.

**EL9. Owner actions hold across pages.** On People, merge selection carries across Load more. On
Tags, Manage mode and bulk actions apply to every selected pill, loaded across any number of pages.
After a merge, delete, rename or create, the loaded range refreshes: nothing is skipped or repeated,
and a created entry appears without a reload.

**EL10. Tag owner lookups cover the library.** Creating a tag whose name already exists is caught
whether or not that tag is loaded. The per-pill "Set parent…" picker finds any tag in the library.

### P1: Nice-to-have

**EL-P1-a. Load more says how far along you are**, for example "50 of 2,064". The exact wording is
for the design handoff.

### P2: Future considerations (design for, don't build)

- **Alias matching in list search** (the non-goal above) should be possible to add without
  re-paging.
- **A letter start in the URL**, if shareable "people from M" links are ever wanted.

## Amendments to other specs

- **[`sort-persistence.md`](sort-persistence.md):** People, Studios, Tags and Films are paged too, so
  Random on them uses the same seeded, page-stable ordering as Media. The note that they "return
  their full list in one call and are shuffled client-side" is withdrawn.
- **[`nav-search-live-filter.md`](nav-search-live-filter.md) NS3:** in-place filtering on People,
  Studios and Tags now queries the server for matches across the whole library (EL5), instead of
  filtering an already-fetched list. What matches is unchanged. Results now update after a short
  pause in typing, not on every keystroke.
- **[`people-poster-view.md`](people-poster-view.md):** the non-goal "no pagination change" no longer
  holds. Poster view pages exactly like List view (EL1). The A–Z index stays List-only.

## Success Metrics

Measured on the stress fixture at `-big` (2,000 of each kind), against the HOLODEX-354 baseline:

| Page | Today | Target on first load |
|---|---|---|
| `/people` | 2,064 rows · ~12,500 elements · 112,285px | ≤ 50 rows · ≤ 1,500 elements |
| `/tags` | 4,037 pills · ~14,100 elements · 452,541px | ≤ 200 pills · ≤ 2,000 elements |
| `/films` | 2,017 posters · ~14,200 elements | ≤ 50 posters · ≤ 1,500 elements |
| `/studios` | full table (~2,000 rows, not measured in HOLODEX-354; measure first) | ≤ 50 rows · ≤ 1,500 elements |

- Data fetched to draw the first screen: ≤ 15KB per page, down from 120–170KB.
- Settle time on first load: under 0.5s on the fixture, down from about 2s.
- Every entry in the fixture is reachable by Load more, search or A–Z (checked by test).

## Open Questions

- [design] The Load more control's wording and placement on the pill cloud and the poster grid, and
  how the active A–Z letter looks. → design handoff.
- [engineering] Whether a technology fork exists (offset vs. keyset paging; reuse of the Media grid's
  seeded ordering for entities). If one does, it goes in an architecture topic doc; if not, the
  worklog records "no technology fork".

## Resolved Decisions

- **RD1. Paging style:** server paging + explicit Load more. Virtualization and infinite scroll were
  rejected (owner, 2026-10-18).
- **RD2. A–Z:** clicking a letter restarts the list at that letter. "Letter as a filter" and "load
  every page up to the letter" were rejected (owner, 2026-10-18).
- **RD3. Tags All tab:** one stream, tags then categories. Two separately paged sections were
  rejected (owner, 2026-10-18).
- **RD4. Search scope:** parity, matching name and display name only, no aliases (owner, 2026-10-18).
- **RD5. Split:** pickers/datalists (HOLODEX-586) and owner queues (HOLODEX-587) are subtasks with
  their own branches (owner, 2026-10-18).

## Timeline Considerations

- HOLODEX-586 is adding an optional name filter to the categories list for its category picker.
  This work's paging adds on top of that, so whichever lands second merges onto the other.
- People's merge flow (HOLODEX-586's picker change) and this page's selection (EL9) touch neighbouring
  surfaces. Coordinate if both are open at once.
