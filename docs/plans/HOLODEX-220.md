---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-220                 # the tracker key; must match the branch key regex
status: in-review
depends-on: [HOLODEX-114]    # [KEY-…] cross-epic deps that must land first
release_note:                # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
---

# HOLODEX-220 · Media page restructure — one sync verb, render once

Done means `/media/[id]` carries a single sync verb instead of four "refresh" variants, and renders
each enriched value once instead of two or three times — with no capability removed, no new endpoint,
and no data-model change. Both halves are subtractive.

**Design package:** spec n/a · ADR n/a · [handoff](../design/media-page-restructure-handoff.md) · testing-strategy § TBD

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [~] spec `write-spec` → `docs/specs/**` — n/a: no new capability, both parts subtractive
- [~] architecture `architecture` → `docs/architecture/ADR-*` — n/a: no new seam or data-model change
- [x] design `design-handoff` → `docs/design/media-page-restructure-handoff.md`
- [~] backend — n/a: frontend-only; `runEnrichRefreshAll` stays for person/studio
- [ ] frontend
- [ ] testing `testing-strategy`
- [~] security `security-review` — until: this touches an owner-gated mutation path (today it only
      removes call sites of existing gated endpoints)

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [ ] [frontend] Part A — collapse the four refresh controls to one sync verb; move the solid-accent
   slot off `Refresh all` — `web/src/routes/media/[id]/+page.svelte`
2. [ ] [frontend] Part B — chip radiogroup becomes the expanded state of a field row —
   `web/src/routes/media/[id]/+page.svelte`
3. [ ] [frontend] Part B — merge the duplicate cast renders — ⛔ blocked on HOLODEX-114 (F40/ADR-059)
   making `RelinkVideoEntity` the sole writer of `video_people` from resolved values; until then label
   the grid file-only rather than merging
4. [ ] [testing] Three-skin QA + the contrast pairs marked "still to measure" in the handoff

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-07-26 · design critique → handoff → review
- skills: design-critique, design-handoff
- handoff: The handoff is written and PR #179 is up for design review ahead of implementation. Start at
  Part A (item 1) — it is unblocked and self-contained. Do not start item 3 until HOLODEX-114 lands.
