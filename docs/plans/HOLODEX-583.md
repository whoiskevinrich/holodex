---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-583
status: in-progress
profile: ui                  # layout bug fix in an existing dialog; behaviour already specced
depends-on: []
release_note: Setting a person's headshot, poster or banner from the gallery no longer squashes the saved image horizontally when the browser window is short.
---

# HOLODEX-583 · Person image crop squashes the saved image horizontally on short viewports

`CropEditor`'s `.crop-frame` (`overflow:hidden`) flex-shrank below its aspect-ratio height once the
modal hit `max-h-[80vh]`. The canvas then mapped a non-square frame onto the 600×600 output, so the
saved headshot came out squashed horizontally. The frame is now `shrink-0`, and the modal body scrolls.

**Design package:** none (bug fix; the crop editor's WYSIWYG promise is unchanged)

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — bug fix, no UX change; restores the specced WYSIWYG crop
- [x] frontend → `web/src/**` — `CropEditor.svelte` frame `shrink-0`; live-verified on `/people/20301`
- [x] testing `testing-strategy` — HOLODEX-583 entry in `docs/testing-strategy.md` (layout-only, jsdom can't measure it; dev guard is the tripwire)

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR and confirm CI moves HOLODEX-583 to Done

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-17 · session
- skills: handoff
- handoff: Fixed and live-verified: the crop frame holds 1:1 / 2:3 at 720px and 600px viewports (was 413×383). Next: merge the PR and confirm CI moves 583 to Done.

## Dropped — newest first (the reason is the point)
