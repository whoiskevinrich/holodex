---
key: HOLODEX-564
status: in-progress
profile: feature
depends-on: []
release_note: ""
approved:
  design:
    on: 2026-10-11
    at: 166ac812
---

# HOLODEX-564 · Jump to any position in a run from the run strip

Owner ask (2026-10-11): clicking `1 of 389` in the run strip lets you pick which number to go to,
and you can see each item's title while you choose. Of three mockups, the owner chose **option A**,
a searchable list popover, and asked for the list to scroll with the mouse wheel. Relates to
HOLODEX-438 (video playlists, Done).

## Gates — definition of done

- [x] spec → `docs/specs/smart-playlists.md` RD14 + P0-13 (title search covers the whole run, for every run source)
- [~] architecture — no technology fork (one more visibility-gated read beside the existing run-ids read)
- [x] design → `docs/design/run-position-jump-handoff.md` + `run-position-jump-mockup.svg` (owner sign-off is recorded at `/implement`)
- [x] backend → `internal/api/media_brief.go` (`POST /media/brief {ids}`, ≤1000 ids per request, request order, soft-deleted or unknown ids left out)
- [ ] frontend → `NextUpStrip.svelte` trigger + new `video/RunJumpPopover.svelte`
- [ ] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] Frontend: an `api.mediaBrief` client that asks in chunks of 1000 ids, then the trigger and a virtualised wheel-scrolling list with number and title query and a roving tabindex
2. [ ] Testing: add a section to `docs/testing-strategy.md` (`/testing-strategy`)
3. [ ] Consider `/security-review` for `POST /media/brief` (a public read by arbitrary id). It isn't in the `feature` posture; the owner decides

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-11 · backend whole-run details read
- skills: code-review high --fix
- handoff: `POST /media/brief` is in, with handler tests for request order, dropping soft-deleted and unknown ids, and the 1000-id cap. The Draft PR is #493. Start at Up next item 1, the frontend.

### 2026-10-11 · crossed into build
- skills: implement
- handoff: Crossed into build. Design signed off at 166ac812, fresh main merged in, and the Draft PR is open. Start at Up next item 1, the backend whole-run details read.

### 2026-10-11 · spec edit, RD14 + P0-13
- skills: write-spec
- handoff: The owner chose title search over the whole run for every run source, so the spec and the handoff's loading section now say that. Next is `/implement`, then the backend details read before the popover.

### 2026-10-11 · design handoff for option A
- skills: design-handoff
- handoff: The owner picked option A (searchable list, wheel-scrollable). The handoff and SVG mockup are committed. Start with the spec edit, which has to settle where a grid run's titles come from, then `/implement`.
