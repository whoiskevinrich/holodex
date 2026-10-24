---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-357                 # the tracker key; must match the branch key regex
status: in-review                   # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: ui                  # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Tag links, source chips and the Promote and Attach pills are now at least 24px tall, so they're easier to tap.
approved:
  design:
    on: 2026-10-24
    at: 4ec9f848
---

# HOLODEX-357 · Chips adopt the 24px target size

Epic 3 of the frontend backlog plan (Touch-target size), settled ahead of epic 2 (Provenance
badges, HOLODEX-144) as the plan asks. Carries HOLODEX-580 (the Promote / Attach pills).

**Design package:** the owner picked option A from a side-by-side mockup on 2026-10-24 (adopt
24px; move the tag padding onto the link; `min-h-6` on the `text-xs` pills). The rule is recorded
in [theming.md "Target size"](../design/theming.md).

## Gates — definition of done

<!-- Rows match the `ui` posture. -->

- [x] design `design-handoff` → `docs/design/**` — `theming.md` "Target size": WCAG 2.2 AA 2.5.8 adopted; owner chose option A on 2026-10-24
- [x] frontend → `web/src/**` — owner `TagLinkChip` link carries the chip's padding and its glyph (28px); `min-h-6` on `CurationChip` radios, both Custom openers and inputs, `AutoFieldRows` pills, and the `StudioPicker` / `FilmStudioCascadeDialog` candidate chips
- [x] testing `testing-strategy` — `blockedBy: HOLODEX-357` retired on `tag-chips-stay-tappable` (8/8) and `source-chips-stay-tappable` (4/4); harness rows updated

## Up next — ordered (position = priority)

1. [ ] [—] On merge, move HOLODEX-357 and HOLODEX-580 to Done (580 has no branch, so CI won't move it)
2. [ ] [—] → HOLODEX-593: the chips' own small controls (tag `×`, value-chip edit and remove, attached-chip `×`) are still under 24px

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-24 · session
- skills: code-review
- built: option A. Full harness: 468 passed, 0 failed; the 36 ERR and 4 VOID are the known setup
  gaps also seen on HOLODEX-359 (no `enrich-stub` for the flood picker, a stale `/people` selector).
  Type-check clean, 700 unit tests pass.
- then: the owner signed off on the theming.md "Target size" text as committed in 4ec9f848; sign-off re-pinned there.
- handoff: Option A is built and both tap-target checks are green; next, merge and close 357 and 580, then design epic 2's provenance badges at 24px.

## Dropped — newest first (the reason is the point)
