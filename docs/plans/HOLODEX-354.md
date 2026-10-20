---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-354
status: in-progress
profile: feature             # new user-facing paging behaviour over existing endpoints; no auth or data-model change
depends-on: []
approved:
  design:
    on: 2026-10-18
    at: 542851ec
release_note: "People, Studios, Tags and Films open on one page with a Load more button instead of drawing the whole library, and the A–Z index jumps by restarting the list at a letter."
---

# HOLODEX-354 · Paged entity lists — People, Studios, Tags and Films load a page at a time

The stress fixture (`-big`, 2,000 of each kind) showed every entity list fetching and drawing its whole
table. `/people` draws 2,064 rows on a 112,285px page, and `/tags` draws 4,037 pills on a 452,541px page.
This branch adds server paging to the list endpoints and a Load more control to the four list routes,
following the Media grid's pattern. The owner chose server paging + Load more over virtualization.
The pickers and datalists (HOLODEX-586) and the owner queues (HOLODEX-587) are subtasks on their own
branches. The home page's whole-tag fetch (item 4 in the issue) is already gone.

**Design package:** spec F80 `docs/specs/entity-list-paging.md` · design handoff to come

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`. F80 `docs/specs/entity-list-paging.md`, EL1–EL10. Its decisions:
  - RD1: server paging + Load more
  - RD2: an A–Z letter restarts the list at that letter
  - RD3: the Tags All tab is one stream, tags then categories
  - RD4: search parity, matching name and display name with no aliases
  - RD5: the 586/587 split

  Amends `sort-persistence.md`, NS3 in `nav-search-live-filter.md`, and `people-poster-view.md` in place.
- [x] design `design-handoff` → `docs/design/**`. `entity-list-paging-handoff.md` + `entity-list-paging-mockup.svg` (`542851ec`):
  - the shared "Load more (N left)" control on rows, posters and the pill cloud
  - the active A–Z letter in gold, with "from M · Back to start" on the count line
  - Tags run straight into categories in one cloud

  The owner approved option B and the Load more control on 2026-10-18; pinned to `542851ec`.
- [x] backend → `{cmd,internal,providers}/**`. People, Studios, Tags and Films are paged on the server.
  - The shared `NamedListFilter` gains Query, Seed, Limit and Offset. Rows are ordered by the shown label with an id tie-break, and Random uses `holo_shuffle`.
  - Each endpoint answers in the `/media` envelope `{items,total,limit,offset}`. `letters=1` adds the name-order offsets for the A–Z restart.
  - `/categories` pages only when `?limit=` is sent, so the HOLODEX-586 picker shape is unchanged.
  - Films get `ListFilmsPage`, which replaces the now-unused `ListFilms`.
  - No technology fork: this is the same offset paging and ADR-045 shuffle as `/media`.
  - Covered by `internal/api/entity_list_paging_test.go`.
- [x] frontend → `web/src/**`. People, Studios, Films and Tags run on the shared `lib/pagedList.svelte.ts`.
  - `PagedList` owns the run, the letter restart, Back's snapshot and restore, and the stale-reply guard. `pagedListPage` and `settledQuery` wire a page to it.
  - New: `shared/LoadMoreButton.svelte`, and an `AlphaIndex` `active` state with the count-line "from L · Back to start".
  - Tags pages tags, then categories, 200 a page. Its parent picker uses server search, and remove-from-category reads memberships on demand.
  - `peopleNav.ts` and `shuffle.ts` lost their last callers and were removed.
  - QA'd live on `-big` (`backend-stress-open-9800` + `web-9800`):
    - /people: 50 rows, 728 elements, 4,261px (was 12,477 elements, 112,285px)
    - /tags: 200 pills, 465 elements, 4,608px (was 452,541px)
    - the S/Z restart, Back restoring 100 rows at scroll 6,145, search across the library, tags running into categories, and random films tiling 100 unique

  Vitest 674 green.
- [ ] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] [—] `/testing-strategy`:
   - endpoint contract tests, done for the backend
   - a test that every entry is reachable
   - a search test on a person's Displayed-As spelling (deferred from the code review)
   - the `-big` fixture metrics from the spec
2. [ ] [—] Code-review follow-ups (all deliberately skipped):
   - In Manage mode the last Load more on /tags drops focus, because tag pills there are buttons, not links. Give the pill a focus target.
   - The All tab reads 200 categories up front just to count them. A `limit=1` read gives the total.
   - The tags page is 31KB per 200 pills, over the spec's 15KB target, because each tag carries its aliases.
3. [ ] [—] Move browse, `EntityVideos`, playlists and the five owner queues onto `shared/LoadMoreButton.svelte`
4. [ ] [—] Perf follow-up, if measured: `attachPersonDisplayNames` reads every person's display names on each page. Narrow `DisplayNames` to the page's ids if the `-big` profile shows it
5. [ ] [—] Sweep subtasks HOLODEX-586/587 by hand with this story: In Review when the PR is ready, Done on merge

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-19 · session (frontend)
- skills: code-review (high --fix)
- handoff: The four list pages now page through the server with Load more, the A–Z restart, the Tags stream and Back restore, QA'd live on the -big fixture. The `/testing-strategy` gate is the only one left, then the PR can be marked ready.

### 2026-10-19 · session (backend)
- skills: code-review (high --fix)
- handoff: The server now pages People, Studios, Tags and Films (and `/categories` when `?limit=` is sent), with search, seeded Random and the A–Z letter offsets; the Go suite is green. The frontend hasn't caught up, so the list pages show only their first 50 entries until Up next item 1 lands.

### 2026-10-18 · session (design)
- skills: design-handoff, implement
- handoff: Crossed into build. The design was signed off at `542851ec`, `origin/main` is merged in (586 #539 and 587 #540 are both on main), and the Draft PR is open. Start at Up next item 1, the backend.

### 2026-10-18 · session
- skills: write-spec
- handoff: HOLODEX-354 is split. Subtasks 586 (pickers) and 587 (owner queues) are running in their own sessions. The F80 spec is written with the owner's A–Z, Tags-stream and search-scope decisions. Next: the design handoff.

## Dropped — newest first (the reason is the point)
