---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-604                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below; only `done`/`released` are read from here
profile: feature             # the gate posture (see flightplan.yaml `postures:`)
depends-on: [HOLODEX-601]    # stacked on HOLODEX-601's branch until PR #567 merges
release_note:                # the ONE user-facing sentence; authored once by /handoff
approved:
  design:
    on: 2026-10-29
    at: f1b8b221
---

# HOLODEX-604 · Whole-list scroll on every library list

The Media grid, Films, playlists (smart and snapshot), the Tags Posters and Table views, and the
videos on person, studio and tag pages scroll the whole list the way People and Studios do since
HOLODEX-601. The Tags Chips view and the owner queues keep Load more. Architecture: **no technology
fork** — it reuses 601's virtualizer and sparse page cache; `stack.md`'s long-lists section was kept
current in place (`[~] architecture — no technology fork`; the `feature` posture carries no
architecture row).

**Design package:** [spec](../specs/entity-list-paging.md) (EL11, EL14, EL15, RD10–RD13) ·
[stack.md, Long lists](../architecture/stack.md#long-lists-tanstack-virtuals-core-over-a-sparse-page-cache) ·
[handoff](../design/entity-list-virtual-scroll-handoff.md#the-other-library-lists-holodex-604) ·
[testing-strategy §32.2](../testing-strategy.md)

## Gates — definition of done

<!-- States: [ ] not started · [/] in progress · [~] deliberately skipped · [x] done. [~] and [x]
     both count as SETTLED. PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**`
- [x] design `design-handoff` → `docs/design/**`
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [/] testing `testing-strategy`

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority. Each item: [gate] one-liner — file path.
     ⛔ marks blocked. The CHECKBOX settles an item: [x] done, [~] dropped. Done items are deleted;
     dropped ones move to ## Dropped. -->

1. [ ] [backend] Snapshot playlist read pages with `limit`/`offset` + `total`; no `limit` still returns all — `internal/api/playlists.go`
3. [ ] [frontend] Two-line title box on `VideoCard`; whole-list `VideoGrid` mode on VirtualRows — `web/src/lib/components/video/`
4. [ ] [frontend] Media grid onto SparseList + wholeListPage (Back, scan refresh, Recently added) — `web/src/routes/+page.svelte`
5. [ ] [frontend] Films, playlist detail, Tags Posters/Table (All-tab composite fetcher), EntityVideos — `web/src/routes/`
6. [ ] [testing] §32.2 tests, harness twins, live QA on `-big` — `docs/testing-strategy.md`
7. [ ] [—] Before each push: `git fetch`; merge `origin/HOLODEX-601-virtual-entity-lists` (or `origin/main` once #567 merges)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-29 · session
- skills: testing-strategy, handoff, implement
- handoff: Crossed into build — design signed off at f1b8b221; Draft PR open on base HOLODEX-601-virtual-entity-lists. Start at the snapshot-paging backend.

### 2026-10-28 · session
- skills: write-spec, design-handoff

## Dropped — newest first (the reason is the point)
