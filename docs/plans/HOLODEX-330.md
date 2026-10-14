---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-330
status: in-review
profile: ui                  # frontend a11y bug fix; one fill token on an existing badge, no UX change
depends-on: []
release_note: Fix — an unnumbered scene's "—" badge on a Films grid card stays readable over a bright thumbnail.
---

# HOLODEX-330 · VideoCard's scene badge can fail AA contrast over a bright thumbnail

`VideoCard`'s unnumbered scene badge used a translucent `bg-black/70`, so over a bright thumbnail
`text-muted` fell to ~2.7:1 (white thumbnail), below AA's 4.5:1 for 10px text. It now uses the
opaque `bg-bg` that HOLODEX-328 gave the media page's Films chip: 6.31:1 whatever the thumbnail.

**Design package:** none (applies HOLODEX-328's existing dim-chip treatment; the handoff doc's
"VideoCard still uses `bg-black/70`" note brought current)

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — no new design: reuses HOLODEX-328's approved `bg-bg` dim fill; only the stale note in `media-detail-films-people-handoff.md` was corrected
- [x] frontend → `web/src/**`
- [x] testing — class-only change with no unit seam (no component-test harness); verified live on `backend-stress-open-9600`: dim badge computes `rgb(12,10,9)` / `rgb(155,144,130)` = 6.31:1, and reads cleanly over a thumbnail forced to near-white; `npm run check` 0 errors

## Up next — ordered (position = priority)


## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-13 · session
- skills: code-review (high --fix), handoff
- spun out: HOLODEX-576 (Edit scene number dialog can't clear a number — found during QA; Relates link)
- handoff: Fix committed (`f9a45050`) and verified live at 6.31:1; all gates settled — next: push and open the PR ready for review.

## Dropped — newest first (the reason is the point)
