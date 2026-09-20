---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-432
status: in-progress
release_note: The Studios list now shows each studio's logo in its row — the image TMDB actually supplies — instead of a monogram, with icon-only and logo-less studios looking exactly as before.
---

# HOLODEX-432 · Studios list rows draw the logo role

`/studios` reads `icon_url` for its row well, but TMDB only ever fills the `logo` role, so every
enriched studio shows a monogram in the list while its logo sits on the payload unused
(`logo_url` is already on `GET /studios`). Frontend-only: swap the well for a fixed 96×32 slot
that draws the logo bare (HOLODEX-411 rules) and keeps today's 40×26 plate for icon / monogram
rows, centred in the same slot so the name column stays on one line. Child of F51
(HOLODEX-246). No spec/ADR — presentation-only change to an existing page.

## Gates — definition of done

- [ ] spec — n/a (no new capability, field, or endpoint; recorded in the handoff's "Why no spec/ADR")
- [ ] architecture — n/a (ADR-079 image roles untouched)
- [/] design `design-handoff` — [studio-list-logo-handoff.md](../design/studio-list-logo-handoff.md) +
  [mockup SVG](../design/studio-list-logo-mockup.svg); **option C recommended** (fixed `w-24 h-8`
  slot, logo bare + centred, name always shown; A free-width and B wordmark-replaces-name rejected)
  — waits on Kevin's pick
- [ ] backend — n/a beyond two stale comment lines on `model.Studio.IconURL/LogoURL`
- [ ] frontend — `web/src/routes/studios/+page.svelte` well → slot (handoff §2), `alt=""`,
  `loading="lazy"`; stale comments in `types.ts`
- [ ] testing `testing-strategy` — render test for the three slot states + agent rows 11.3–11.12
- [ ] security — n/a (no auth/access/infra)

## Up next — ordered (position = priority)

1. [ ] [HOLODEX-432] Kevin picks A / B / C from the inline cards (C recommended) → flip design to `[x]`
2. [ ] [HOLODEX-432] frontend per handoff §2, then `/testing-strategy`, `/code-review high --fix`,
   three-skin QA (handoff §11), mark PR ready
3. [ ] [HOLODEX-432] on merge: CI moves 432 to Done (story-keyed branch); nothing to sweep by hand

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-19 · design-handoff
- skills: design-handoff
- handoff: filed HOLODEX-432 under F51, renamed branch `HOLODEX-432-studio-list-logos`, In Progress.
  Handoff + SVG mockup committed with three options rendered on the real tokens at the `lg` column
  width; C (fixed 96×32 slot) recommended because A jags the name column by up to 56px and B leaves a
  wordmark row unnamed for the A–Z nav. No code yet — next session implements §2 once Kevin picks.
