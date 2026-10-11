---
key: HOLODEX-564
status: in-progress
profile: feature
depends-on: []
release_note: ""
---

# HOLODEX-564 · Jump to any position in a run from the run strip

Owner ask (2026-10-11): clicking `1 of 389` in the run strip lets you pick which number to go to,
and you can see each item's title while you choose. Of three mockups, the owner chose **option A**,
a searchable list popover, and asked for the list to scroll with the mouse wheel. Relates to
HOLODEX-438 (video playlists, Done).

## Gates — definition of done

- [ ] spec — add jump-to-position to `docs/specs/smart-playlists.md` (run strip). Settle where a grid run's titles come from, since the run holds only ids
- [x] design → `docs/design/run-position-jump-handoff.md` + `run-position-jump-mockup.svg` (owner sign-off is recorded at `/implement`)
- [ ] backend — only if the spec needs a batch title read for a run's ids
- [ ] frontend → `NextUpStrip.svelte` trigger + new `video/RunJumpPopover.svelte`
- [ ] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] `/write-spec` edit: jump-to-position in the run strip, plus the title source for grid runs (the handoff's "Loading and missing data")
2. [ ] `/implement` — owner signs off on the handoff, then the Draft PR opens
3. [ ] Build: trigger, a virtualised wheel-scrolling list, number and title query, roving tabindex
4. [ ] Clear the `needs-design` label in Jira once the design is signed off

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-11 · design handoff for option A
- skills: design-handoff
- handoff: The owner picked option A (searchable list, wheel-scrollable). The handoff and SVG mockup are committed. Start with the spec edit, which has to settle where a grid run's titles come from, then `/implement`.
