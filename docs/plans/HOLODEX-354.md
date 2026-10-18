---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-354
status: in-progress
profile: feature             # new user-facing paging behaviour over existing endpoints; no auth or data-model change
depends-on: []
release_note: ""
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
- [ ] design `design-handoff` → `docs/design/**`
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] [—] `/design-handoff` for F80: the Load more control on rows, posters and the pill cloud, the active A–Z letter, and the Tags stream. Commit the SVG mockup next to the handoff
2. [ ] [—] Decide whether paging is a technology fork (offset vs. keyset; reusing the `/media` seeded ordering for entities). If it isn't, the worklog notes "no technology fork". The `feature` posture carries no architecture gate
3. [ ] [—] `/implement`: owner sign-off on the handoff, merge `origin/main`, push, open the Draft PR
4. [ ] [—] Coordinate with HOLODEX-586: it adds an optional `q` to `GET /categories`, and this branch layers limit/offset on top. `/tags/+page.svelte:175` then needs categories only for remove mode
5. [ ] [—] Sweep subtasks HOLODEX-586/587 by hand with this story: In Review when the PR is ready, Done on merge

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-18 · session
- skills: write-spec
- handoff: HOLODEX-354 is split. Subtasks 586 (pickers) and 587 (owner queues) are running in their own sessions. The F80 spec is written with the owner's A–Z, Tags-stream and search-scope decisions. Next: the design handoff.

## Dropped — newest first (the reason is the point)
