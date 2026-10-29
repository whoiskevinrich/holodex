---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-604                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below; only `done`/`released` are read from here
profile: feature             # the gate posture (see flightplan.yaml `postures:`)
depends-on: [HOLODEX-601]    # #567 merged to main as 8c235dac; this branch now targets main
release_note: The Media grid, Films, playlists, the Tags Posters and Table views, and the videos on person, studio and tag pages now scroll the whole list, so you can drag anywhere and only what's in view loads.
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
- [x] backend → `{cmd,internal,providers}/**`
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy` — §32.2 automated, live-QA'd on `-big`; owner screen-reader pass and build-vs-mockup check passed 2026-10-29

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority. Each item: [gate] one-liner — file path.
     ⛔ marks blocked. The CHECKBOX settles an item: [x] done, [~] dropped. Done items are deleted;
     dropped ones move to ## Dropped. -->

1. [ ] [testing] Nav-harness Back scenario for the Media grid and a person's videos; card contract at other densities and the poster layout — `web/geometry/nav.mjs`
2. [ ] [—] Keyboard focus no longer arms a card's hover montage → HOLODEX-605
3. [ ] [—] Position chip shows above an entity page's list → HOLODEX-606
4. [ ] [—] Sweep HOLODEX-604 with the PR: In Review when #568 is marked ready, Done on merge (CI moves only the branch's own key)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-29 · session
- skills: testing-strategy, handoff, implement, code-review, handoff
- handoff: Every gate settled: the owner passed the screen-reader pass and the build-vs-mockup check; PR #568 marked ready on base main. Next: the owner merges, then sweep Jira.

### 2026-10-28 · session
- skills: write-spec, design-handoff

## Dropped — newest first (the reason is the point)
