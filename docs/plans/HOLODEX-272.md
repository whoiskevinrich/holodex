---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-272
status: in-progress
depends-on: [HOLODEX-270]
release_note: Adding or removing a person on a video now goes through a picker on the video detail page — known candidates, full-library search, or an inline create-fallback — protected by the same duplicate-video safeguard as a title or studio change.
---

# HOLODEX-272 · People attach/detach + relationship picker (F56.5)

Done means the video detail page's People grid gets add/remove affordance via a new
`PersonPicker` popover (known-candidate chips, full-library search, inline create-fallback,
required Actor/Director role choice), committing through the existing field-decision model
(constructing an `actors`/`director` manual decision, not writing `video_people` directly — see
spec § Resolved Decisions), and every attach/detach runs through the HOLODEX-270 composite-key
collision gate exactly as Title and Studio already do. This is the last open story under epic
HOLODEX-267.

**Design package:** [spec](../specs/people-relationship-picker.md) ·
[design handoff](../design/people-relationship-picker-handoff.md) · testing-strategy §5: not yet
written

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/people-relationship-picker.md`
- [~] architecture — not needed (resolved in spec § Resolved Decisions: attach/detach extends
      ADR-051/ADR-072 through the existing field-decision model rather than writing `video_people`
      directly, so no new persistence/API shape is introduced)
- [x] design `design-handoff` → `docs/design/people-relationship-picker-handoff.md`
- [ ] backend — `internal/repo/video_collision.go` (`FindPeopleCollision`, sibling to
      `FindTitleCollision`/`FindStudioCollision`), attach/detach endpoint constructing the
      `actors`/`director` manual decision, wired into the same 409+override gate
- [ ] frontend — `PersonPicker.svelte` (new, multi-select — `StudioPicker`/`EntityPickerDialog`
      are both single-select and not directly reusable), People-grid empty-state guard fix,
      hover-reveal remove control on `PersonPoster` call sites
- [ ] testing `testing-strategy` → `docs/testing-strategy.md` §5
- [ ] security `security-review` — required before this leaves Draft: attach/detach is a new
      owner-mutable surface (person↔video linking has no dedicated endpoint today)

## Up next — ordered (position = priority)

1. [ ] [backend] `FindPeopleCollision` + attach/detach endpoint
2. [ ] [frontend] `PersonPicker.svelte` + People-grid wiring (empty-state guard, remove control)
3. [ ] [testing] `docs/testing-strategy.md` §5 row
4. [ ] [security] `/security-review` — new mutation surface, required before Draft → ready

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-11 · Design handoff written
- skills: design-handoff, visualize
- handoff: Read `StudioPicker.svelte`/`PickerShell.svelte`/`PersonPoster.svelte` and the current
  (broken) People-grid markup in full before designing. Built a grounded mockup (Holodex's actual
  Cinémathèque tokens, `PersonPoster`-shaped cards) comparing the two live design questions side
  by side: role-choice UI (Option A — a global Actor/Director segmented toggle above search — vs
  Option B — a per-result-row role pill defaulting to Actor) and the hover-reveal remove control
  on grid cards. Resolved to Option B: a video's people list routinely mixes both roles in one
  sitting, and B lets an owner add all of them in a single open/close cycle without toggling a
  global mode between searches. Wrote
  `docs/design/people-relationship-picker-handoff.md` (Layout, Design Tokens, Components, States
  and Interactions, Responsive, Edge Cases, Animation, Accessibility) grounded in the real
  component props/state, not invented ones. Next: backend (`FindPeopleCollision` + attach/detach
  endpoint).

### 2026-08-11 · Spec written, epic-status review led here
- skills: write-spec, graphify
- handoff: Investigated "what's left to close HOLODEX-267" and found HOLODEX-272 was the only
  open child story with zero code/branch/PR started despite being marked In Progress. Also
  confirmed HOLODEX-267's own `needs-design`/`needs-spec` labels are stale bookkeeping (every
  other child story shipped its own spec+design; no separate epic-level umbrella doc was ever
  intended) — worth clearing once this story's gates land. Set up a dedicated worktree
  (`HOLODEX-272-people-picker`, branch `worktree-HOLODEX-272-people-picker` — key substring is
  enough for Jira linkage, no rename needed; Jira status was already In Progress, no transition
  needed). Ran an Explore agent to ground the spec in current code before writing it: confirmed
  `video_people` is fully derived (F40/ADR-072, rebuilt from resolved `actors`/`director` field
  decisions on every relink, zero other writers), no `FindPeopleCollision` exists, and neither
  `StudioPicker` nor `EntityPickerDialog` supports multi-select. Surfaced the two load-bearing
  open questions (attach mechanism, role-on-attach) via AskUserQuestion before writing
  requirements — resolved to "extend the field-decision model" (no new ADR needed) and "require
  an Actor/Director role choice at attach time." Wrote
  `docs/specs/people-relationship-picker.md`. Next: design handoff for `PersonPicker.svelte`.
