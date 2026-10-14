---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-576
status: in-progress
profile: ui                  # bug fix in two existing dialogs; behaviour already specced
depends-on: []
release_note: Clearing a film scene's number and saving now leaves the scene unnumbered instead of showing a validation error, and the bulk attach picker attaches unnumbered when its starting number is erased.
---

# HOLODEX-576 · Edit scene number dialog can't clear a scene number

Spun out of HOLODEX-330's live QA. Svelte 5's `bind:value` on `<input type="number">` binds `null`
once the field is cleared, but `parseSceneNumberInput` only treated `''` as blank, so `Number(null)`
(0) failed the positive-integer rule. The bulk-attach picker's inline copy of the rule had the same
hole for its starting scene number.

**Design package:** none (bug fix; the specced "blank = unnumbered" rule is unchanged)

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — bug fix, no UX change; the dialogs already promise "leave blank for unnumbered"
- [x] frontend → `web/src/**` — `sceneNumber.ts` treats null/undefined as blank; bulk dialog uses it; live-verified on `/films/9000`
- [x] testing `testing-strategy` — `sceneNumber.test.ts` pins null/undefined; HOLODEX-576 entry in `docs/testing-strategy.md`

## Up next — ordered (position = priority)

1. [ ] [—] Open the PR, merge, and confirm CI moves HOLODEX-576 to Done

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-13 · session
- skills: code-review (high --fix), handoff
- handoff: Fixed and live-verified: both scene-number fields clear to unnumbered (edit dialog PATCH null → "—", bulk attach unnumbered). Next: merge the PR and confirm CI moves 576 to Done.

## Dropped — newest first (the reason is the point)
