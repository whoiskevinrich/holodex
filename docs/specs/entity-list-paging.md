# Spec: Paged lists — every library list scrolls the whole library; the Tags pill cloud loads a page at a time (F80)

**Status**: Draft
**Story**: HOLODEX-354 · subtasks HOLODEX-586 (pickers and datalists → bounded search), HOLODEX-587
(owner queues paged) · HOLODEX-601 (People and Studios become one scrollable list) · HOLODEX-604
(the Media grid, Films, playlists, the Tags Posters and Table views, and the videos on person, studio
and tag pages follow)
**Owner**: Project owner
**Date**: 2026-10-18 · amended 2026-10-27 (HOLODEX-601) · amended 2026-10-28 (HOLODEX-604)

**Amends**: [`sort-persistence.md`](sort-persistence.md) (Random on these four pages is no longer a
client-side shuffle), [`nav-search-live-filter.md`](nav-search-live-filter.md) NS3 (in-place filtering
on People, Studios and Tags queries the server), [`people-poster-view.md`](people-poster-view.md)
(its "no pagination change" non-goal no longer holds, and the A–Z index now shows in Poster view),
[`quick-wins.md`](quick-wins.md) QW5 (Back to the Media grid restores the spot, not every loaded
page), [`list-views.md`](list-views.md) P0-3 (Table rows scroll the whole list instead of Load more)
and [`person-hover-card.md`](person-hover-card.md) (the Films link lands below every video). See
[Amendments to other specs](#amendments-to-other-specs).

**Design handoff**: [`entity-list-paging-handoff.md`](../design/entity-list-paging-handoff.md), with the
[mockup](../design/entity-list-paging-mockup.svg). It covers the Load more control and the Tags stream.
The whole-list scroll (EL6, EL11–EL13) is in
[`entity-list-virtual-scroll-handoff.md`](../design/entity-list-virtual-scroll-handoff.md), with its
[mockup](../design/entity-list-virtual-scroll-mockup.svg). The lists added by HOLODEX-604 (EL14,
EL15) are in the same handoff.

---

## Problem Statement

The People, Studios, Tags and Films pages fetch their entire library and draw every entry at once.
The Media grid is the only list that loads a page at a time. At 2,000 of each kind (the stress
fixture), `/people` draws 2,064 rows and 2,064 images, about 12,500 page elements on a 112,000px
page. `/tags` draws 4,037 pills on a 452,000px page. Each page takes about 2 seconds to settle, and
the cost grows with the library. Every visit pays it, even a visitor who only wants the first
screen. The server answers in milliseconds; the cost is in sending everything and drawing everything.

**After the first cut (2026-10-27).** Paging fixed the cost but made far entries hard to reach. On
People (about 1,500 entries, 50 per page) the owner rarely sorts by name, so A–Z doesn't help, and
reaching the middle of the list by count or Random means pressing Load more 15 times. The real need
is to **jump anywhere in the list, in any sort**, without paying to load everything before it.

**After People and Studios (2026-10-28).** The whole-list scroll worked there, and the other library
lists still made you press Load more: the Media grid, Films, a playlist, the Tags Posters and Table
views, and the videos on a person's, studio's or tag's page. The Media grid is the longest list in
the library and the one most often browsed deep. The owner asked for the same scroll everywhere a
list is browsed rather than worked through.

## Goals

1. **First screen is bounded.** Opening any library list draws about one screen of entries, or one
   page of pills on the Tags Chips view, whatever the library size.
2. **Nothing becomes unreachable.** Every entry is still reachable: on a whole-list page (EL11) by
   scrolling to it; in the Tags Chips view by Load more; on People, Studios and Tags by search; on
   People and Studios by A–Z too.
3. **Jump anywhere on every whole-list page.** The scrollbar spans the whole list from the first load,
   under every sort, filter and search, in every view. Dragging to any point shows the entries there,
   and only those are fetched.
4. **Same behaviour, new plumbing.** Sort, Random, search, filters, counts and Back keep the results
   the owner gets today. The visible changes are the whole-list scroll, Load more on the Tags Chips
   view, A–Z scrolling to its letter, and video cards that are all one height (EL15).

## Non-Goals

- **Whole-list scroll in the Tags Chips view.** The pill cloud keeps Load more. *(Why: pills vary in
  width, so a row's contents can't be known before its tags arrive. Posters and Table on the same
  page do scroll the whole list: EL14.)*
- **Lists past the browser's page-height limit.** A browser caps how tall a page can be, so a
  whole-list page holds up to about 60,000 videos at phone width in Firefox, and several hundred
  thousand on a desktop. *(Why: going past it needs a scrollbar that no longer maps one-to-one onto
  the list, and no library here is near it.)*
- **A–Z on lists other than People and Studios.** Films, the Media grid and the Tags views keep
  their sorts as they are. *(Why: not asked for, and the name sort there is one of several.)*
- **Whole-list scroll on the scenes of a film, and on shelves** (Recently added, "More with …").
  *(Why: a film's scenes are a short, fixed row, and a shelf scrolls sideways by design.)*
- **Whole-list scroll in owner work queues** (enrichment, duplicates, extraction, trash). They keep
  the explicit Load more. *(Why: a queue is worked from the top, and HOLODEX-587 already pages them.)*
- **Infinite scroll** (more entries appended as you reach the bottom). *(Why: it still can't jump
  far. The whole-list scroll replaces it, and Load more stays in the Tags Chips view and the queues.)*
- **Browser find (Ctrl+F) across the whole list.** On a whole-list page it finds only entries drawn
  on screen. The nav search box (EL5) is how to find a name anywhere. *(Why: drawing every entry so
  the browser can find it is exactly the cost this spec removes.)*
- **Alias matching in search.** Search matches what it matches today: name and display name. *(Why:
  a parity port. Alias matching is its own change.)*
- **A search, letter or scroll position in the URL.** As today, only the sort goes in the URL; a shared
  link opens at the start. *(Why: unchanged from [`list-toolbar.md`](list-toolbar.md); seeds and
  queries stay out of links.)*
- **Pickers and datalists.** *(Why: HOLODEX-586 covers them.)*
- **Tag hierarchy display.** The Tags page stays one flat pill cloud. *(Why: separate concern.)*

## User Stories

**Visitor**
- As a visitor, I want People to open quickly on a large library, so that I'm not waiting for
  thousands of rows I won't scroll to.
- As a visitor, I want to drag the People scrollbar halfway down and see the people there, in
  whatever sort I'm using, without loading everyone before them.
- As a visitor, I want to click "M" in the A–Z index and land on the M names, so that I can find
  someone without scrolling through A to L.
- As a visitor, I want search on People, Studios and Tags to find any match in the library, not just
  among what's on screen.
- As a visitor, I want Random to keep its order as I scroll or load more, without repeats or gaps.
- As a visitor, I want Back from a detail page to return me to the same spot in the list.
- As a visitor, I want to drag the Media grid's scrollbar to the middle of the library, under any
  filter or sort, and see the videos there without pressing Load more dozens of times.
- As a visitor, I want a long playlist, Films, and a person's, studio's or tag's videos to scroll
  the same way, so that every list in the library behaves alike.

**Owner**
- As the owner, I want merge, manage and bulk actions to keep working across everything I've selected,
  wherever in the list I selected it.
- As the owner, I want the "already exists" check when I create a tag to cover the whole library,
  not just the loaded pills.

**Edge cases**
- A library smaller than one screen scrolls no further than its entries.
- A letter with no names stays dimmed and can't be clicked.
- Merging, deleting or creating an entry while scrolled deep into People doesn't jump the list back
  to the top, and in the Tags Chips view doesn't skip or repeat entries on the next Load more.
- Search with no match shows today's "No … match" message.
- A part of the list that fails to load says so in place, and the rest of the list keeps working.
- Removing a video from a snapshot playlist deep in the list closes the gap without moving the scroll.
- A scan finishing while the Media grid is scrolled deep keeps the same video in view.

## Requirements

### P0: Must-have

**EL1. The Tags Chips view loads one page at a time.** It shows its first **200** pills (pills are
small). Load more appends the next page in place, without moving the scroll position. Load more
disappears once everything is loaded. (Every other library list scrolls the whole list instead:
EL11.)

**EL2. Counts are library totals.** "N people", "N studios", "N films" and every "N videos" count the
whole library, playlist or entity (or the whole match set while searching or filtering), not what's
on screen. The Tags search line "N results for “q”" does the same.

**EL3. Sorts are unchanged and whole-library.** Every sort on offer today stays, with the same labels,
owner-only gating, defaults and persistence ([`list-toolbar.md`](list-toolbar.md)). Each sort orders
the whole library, so any point in the list, or page 2, continues the same order.

**EL4. Random tiles across the list.** Random on every paged list uses the session's shuffle seed, as it
already does ([`sort-persistence.md`](sort-persistence.md) SP2/SP3). Scrolling anywhere, or loading
more, continues the same shuffle with no repeats and no gaps. Re-roll redraws the list from the top.
The seed stays out of the URL.

**EL5. Search covers the library.** On People, Studios and Tags, typing in the nav box (on the
matching tab, per [`nav-search-live-filter.md`](nav-search-live-filter.md) NS2) shows the matches from
the whole library, from the top. On a whole-list page the matches are themselves a whole-list scroll
(EL11). Matching is unchanged: a case-insensitive substring of name or display name. Results update
after a short pause in typing, not on every keystroke. Typing doesn't reshuffle Random. Clearing the
box returns to the unfiltered list at the top. Films has no search, as today.

**EL6. A–Z scrolls to a letter.** On People (every view: List, Table and Poster) and Studios, in name
sort, without a search, the A–Z index shows. Clicking a letter scrolls the list so that letter's first
name sits at the top, and moves focus to it. The list is not restarted: scrolling up from there
reaches the earlier letters. "#" covers names that don't start with a letter, as today. Letters with
no names in the whole library are dimmed and inert. **The active letter follows the scroll**: it is
the letter of the first entry in view, so the index also says where you are. Right after a click it
is the clicked letter, even when that letter's first name shares its row with the end of the
letter before. Clicking a letter never
reorders or filters the list; it only moves through it.

**EL7. Back restores the spot.** Returning by Back (or reload) to any paged list restores the same
sort, filters and search, then the scroll position. On a whole-list page only the entries at that
position are fetched, not everything above them, and the same entry is first in view even if the
window's width changed. In the Tags Chips view the same number of loaded pills is restored, as
before. A playlist page, which kept no position before, now restores it too. The rules for when a
restore applies are unchanged (HOLODEX-573): a plain link starts fresh at the top.

**EL8. Tags All tab is one stream.** The All tab pages tags first, in the chosen sort, then
categories once the tags run out, in today's order (categories are never shuffled). One Load more
serves the stream. The Tags tab and the Categories tab each page only their own kind.

**EL9. Owner actions hold across the list.** On People, merge selection holds wherever the people were
picked: someone selected and then scrolled far out of view stays selected, and "N selected" counts
them all. On Tags, Manage mode and bulk actions apply to every selected pill, loaded across any number
of pages. After a merge, delete, rename, create, playlist removal or finished scan, the list
refreshes where it is: on a whole-list page the scroll position stays put and the entries in view are
re-read; in the Tags Chips view the loaded range refreshes with nothing skipped or repeated. A
created entry appears without a reload.

**EL10. Tag owner lookups cover the library.** Creating a tag whose name already exists is caught
whether or not that tag is loaded. The per-pill "Set parent…" picker finds any tag in the library.

**EL11. Every library list scrolls the whole list.** The **whole-list pages** are People (List, Table
and Poster views), Studios, the Media grid (Grid and Table), Films, a playlist (smart or snapshot),
the Tags Posters and Table views, and the videos on a person's, studio's or tag's page. Each list is
as tall as the whole list from the first load, under **every sort** and filter, and with or without
a search. It is not an A–Z feature: dragging the scrollbar, scrolling, paging with the keyboard, or
Back lands anywhere in any order. Only the entries at and near the current position are fetched;
nothing loads the whole library or the whole playlist. There is no Load more on these lists. A sort,
filter or search change, or a Random re-roll, returns to the top. A change of view, of density or of
the window's width keeps the first entry in view in view. While the list scrolls, a readout beside the scrollbar gives the position of the
first entry in view out of the total ("412 of 1,512"), in every sort and in search results, and it
fades shortly after scrolling stops.

**EL12. Entries not yet loaded hold their place.** Where entries haven't arrived yet, placeholders
the exact size of a real entry stand in, so nothing shifts when they arrive. Dragging fast past
thousands of entries fetches only where the drag stops, not every spot it passed. Entries arrive in
place without moving the scroll position. A part of the list that fails to load shows an inline
message with a retry, in place of those entries; the rest of the list keeps scrolling and loading.

**EL13. The whole list is reachable by keyboard and screen reader.** A screen reader hears each entry's
position out of the total ("412 of 1,512"), and the list announces its total. The keyboard moves
entry by entry through the whole list, past what's on screen, and focus stays on the same entry while
entries around it are drawn and removed. Moving focus to an entry not yet loaded shows its placeholder
until it arrives.

**EL14. What each list keeps.** Each whole-list page keeps its own toolbar, sorts, filters and
actions as they are today. Where a list has more to it than People and Studios:
- **Media grid.** Every filter, scope, search and sort applies to the whole list. The Recently added
  shelf above it is unchanged. After a scan finishes, the list refreshes where it is (EL9). Back from
  a video lands on the same video, and the videos seen just before leaving show at once rather than
  as placeholders. Play all and Save as playlist act on the whole match set, as today.
- **Films.** The name and Random sorts, as today. No search and no A–Z.
- **Playlist.** Smart and snapshot playlists both scroll the whole list, and a snapshot fetches only
  what's in view, like a smart one. Changing the playlist's sort or re-rolling its shuffle returns to
  the top. Freezing a smart playlist keeps the scroll where it is. The owner's remove (×) on a
  snapshot takes the video out and closes the gap without moving the scroll. A playlist with stale
  references shows its notice in place of the list, as today.
- **Tags Posters and Table.** Every tab, sort and search scrolls the whole list. On the All tab it is
  one list, tags then categories (EL8). In Manage mode, each entry's own menu works wherever it is in
  the list. Bulk selection stays in the Chips view, as today.
- **Videos on a person's, studio's or tag's page.** The page's details stay above the videos and
  scroll away with the page. The Films row stays **below** the videos, so it comes after the last
  video; its jump link lands on it at once, without fetching the videos in between. The section's
  toolbar, Play all and Save as playlist are unchanged.

**EL15. Every card in a list is one height.** Every entry in a whole-list view is the same height, so
the scrollbar is right before anything arrives. A video card's title takes up to two lines, and a
one-line title leaves the second line empty. A title too long for two lines is cut short as today,
with the full title on hover. This holds for every grid of video cards, not only the whole-list
ones, so a card looks the same wherever it appears.

### P1: Nice-to-have

**EL-P1-a. Load more says how much is left.** The design handoff settled this as the existing
"Load more (N left)" wording, so it ships with EL1.

### P2: Future considerations (design for, don't build)

- **Alias matching in list search** (the non-goal above) should be possible to add without
  re-paging.
- **A letter start in the URL**, if shareable "people from M" links are ever wanted.
- **Libraries past the page-height limit** (the non-goal above) would need a scrollbar that maps
  onto the list by proportion, not pixel for pixel.

## Amendments to other specs

- **[`sort-persistence.md`](sort-persistence.md):** People, Studios, Tags and Films are paged too, so
  Random on them uses the same seeded, page-stable ordering as Media. The note that they "return
  their full list in one call and are shuffled client-side" is withdrawn.
- **[`nav-search-live-filter.md`](nav-search-live-filter.md) NS3:** in-place filtering on People,
  Studios and Tags now queries the server for matches across the whole library (EL5), instead of
  filtering an already-fetched list. What matches is unchanged. Results now update after a short
  pause in typing, not on every keystroke.
- **[`people-poster-view.md`](people-poster-view.md):** the non-goal "no pagination change" no longer
  holds. Poster view scrolls the whole list exactly like List view (EL11), and since 2026-10-27 shows
  the A–Z index too (EL6).
- **[`quick-wins.md`](quick-wins.md) QW5:** Back to the Media grid still restores the filters and
  the spot, with no flash for the videos seen just before leaving. It no longer restores "every
  loaded page": there are no loaded pages to restore, only the position (EL7, EL14).
- **[`list-views.md`](list-views.md) P0-3:** Table rows on the Media grid, People and Tags scroll the
  whole list (EL11). The acceptance line "Load more adds the next page under the last row" is
  replaced by EL12's: entries arrive in place, with no gap or duplicate row.
- **[`person-hover-card.md`](person-hover-card.md):** the Films link still lands on the Films
  heading, which now sits after every video on the page (EL14). "Load more never moves the page
  again" becomes: nothing arriving in the video list moves the page.

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
- Every entry in the fixture is reachable by scroll, Load more, search or A–Z (checked by test).
- **People and Studios, anywhere in the list (HOLODEX-601):** dragging the scrollbar to any point, in
  any sort, shows real entries within 0.5s of the drag stopping, and the page holds no more than about
  three screens of entries at any position. A drag across the whole list makes no more fetches than a
  stop at its end would.
- **The other whole-list pages (HOLODEX-604):** the same three targets hold on the Media grid, Films,
  a playlist of at least 500 videos, the Tags Posters and Table views, and a person's videos, on the
  `-big` fixture. A snapshot playlist's first load fetches about one screen, not the whole playlist.

## Open Questions

- ~~[design] The Load more control's wording and placement on the pill cloud and the poster grid, and
  how the active A–Z letter looks.~~ → [design handoff](../design/entity-list-paging-handoff.md): the
  shared "Load more (N left)" control; the active letter in gold, with "from M · Back to start" on the
  count line (owner, 2026-10-18). The "from M · Back to start" line is withdrawn with the letter
  restart (RD2).
- ~~[engineering] Whether a technology fork exists (offset vs. keyset paging; reuse of the Media grid's
  seeded ordering for entities).~~ None for F80's paging. HOLODEX-601's whole-list scroll is one: how
  it is built goes in the architecture topic doc.
- ~~[design] How the placeholders, the inline retry and the scroll-tracking A–Z look in each view.~~ →
  [whole-list scroll handoff](../design/entity-list-virtual-scroll-handoff.md): still placeholders in
  the row's own frame, one retry row per failed page, and a position chip (owner, 2026-10-27).

## Resolved Decisions

- **RD1. Paging style:** People and Studios are one whole-list scroll that fetches only what's in view
  (owner, 2026-10-27, HOLODEX-601), and since HOLODEX-604 so is every library list but the Tags
  Chips view, which keeps server paging + explicit Load more (RD6). The
  2026-10-18 decision rejected virtualization because a whole-list virtual scroller sends the whole
  library; fetching only the entries in view doesn't, so that reason no longer holds.
- **RD2. A–Z:** clicking a letter scrolls to that letter's place in the list (owner, 2026-10-27). The
  2026-10-18 letter restart and its "Back to start" link are withdrawn. "Letter as a filter" stays
  rejected.
- **RD3. Tags All tab:** one stream, tags then categories. Two separately paged sections were
  rejected (owner, 2026-10-18).
- **RD4. Search scope:** parity, matching name and display name only, no aliases (owner, 2026-10-18).
- **RD5. Split:** pickers/datalists (HOLODEX-586) and owner queues (HOLODEX-587) are subtasks with
  their own branches (owner, 2026-10-18).
- **RD6. Whole-list scroll scope:** People (all three views: List, Table and Poster) and Studios
  (owner, 2026-10-27). Extended on 2026-10-28 (owner, HOLODEX-604) to the Media grid, Films,
  playlists, the Tags Posters and Table views, and the videos on person, studio and tag pages. The
  Tags Chips view and the owner queues keep Load more.
- **RD10. One card height:** a video card's title always reserves two lines (owner, 2026-10-28).
  "One line, truncated" and "measure each row" were rejected.
- **RD11. Films row on entity pages:** stays below the videos (owner, 2026-10-28). Moving it above
  them, and leaving entity pages on Load more, were rejected.
- **RD12. Snapshot playlists:** fetched a page at a time like smart ones (owner, 2026-10-28).
  "Fetch the whole snapshot, draw only what's in view" was rejected.
- **RD13. Tag pages:** a tag's own page gets the whole-list scroll with person and studio pages
  (owner, 2026-10-28).
- **RD7. Every sort, every search:** the whole-list scroll is how these pages page, under every sort
  and with or without a search, not an A–Z feature (owner, 2026-10-27).
- **RD8. A–Z everywhere it can be:** the index shows in every People view, Poster included, and its
  active letter follows the scroll (owner, 2026-10-27).
- **RD9. Position readout:** a "412 of 1,512" readout beside the scrollbar while scrolling, in every
  sort. "The scrollbar alone" was rejected (owner, 2026-10-27).

## Timeline Considerations

- HOLODEX-586 is adding an optional name filter to the categories list for its category picker.
  This work's paging adds on top of that, so whichever lands second merges onto the other.
- People's merge flow (HOLODEX-586's picker change) and this page's selection (EL9) touch neighbouring
  surfaces. Coordinate if both are open at once.
