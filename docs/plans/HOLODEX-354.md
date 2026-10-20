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
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] [—] Backend: limit/offset/q/seed plus a letter start (`from`) on the four list endpoints and the `/categories` filter struct, in the `/media` envelope `{items,total,limit,offset}`. Add per-letter presence for the A–Z dimming. Reuse ADR-045's seeded ordering. No technology fork is expected (same offset paging as `/media`); record it here if that changes
2. [ ] [—] Frontend: the four routes plus `AlphaIndex` active state, the count-line suffix, the Load more live region, and Back restoring the loaded count (the `listScroll` `loaded` field, as `EntityVideos` does)
3. [ ] [—] `/testing-strategy`: endpoint contract tests, a test that every entry is reachable, and the -big fixture metrics from the spec
4. [ ] [—] Build on HOLODEX-586 (PR #539, `ed5928ca`):
   - `GET /categories?q=` is in, via `repo.ListCategories(ctx, repo.CategoryFilter{Query})`. With `q` it is capped at `repo.CategoryQueryCap` = 25 and ranked exact, then prefix, then word start, then anywhere. Add the paging fields to `CategoryFilter`.
   - The TS signature is now `api.listCategories(opts = {}, fetchFn?)`.
   - `/tags/+page.svelte:175` needs categories only for remove mode.
   - Merge whichever of the two lands second onto the other.
5. [ ] [—] Sweep subtasks HOLODEX-586/587 by hand with this story: In Review when the PR is ready, Done on merge

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-18 · session (design)
- skills: design-handoff, implement
- handoff: Crossed into build. The design was signed off at `542851ec`, `origin/main` is merged in (586 #539 and 587 #540 are both on main), and the Draft PR is open. Start at Up next item 1, the backend.

### 2026-10-18 · session
- skills: write-spec
- handoff: HOLODEX-354 is split. Subtasks 586 (pickers) and 587 (owner queues) are running in their own sessions. The F80 spec is written with the owner's A–Z, Tags-stream and search-scope decisions. Next: the design handoff.

## Dropped — newest first (the reason is the point)
