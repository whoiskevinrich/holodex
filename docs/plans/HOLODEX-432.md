---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-432
status: in-progress
release_note: The Studios list now shows each studio's logo in its row — the image TMDB actually supplies — the same way the film and media pages do; a wordmark logo stands in for the name, a light halo keeps dark marks readable on the dark skins, and icon-only or logo-less studios keep their plate.
---

# HOLODEX-432 · Studios list rows draw the logo role

`/studios` read `icon_url` for its row well, but TMDB only ever fills the `logo` role, so every
enriched studio showed a monogram in the list while its logo sat on the payload unused
(`logo_url` is already on `GET /studios`). Frontend-only: the `StudioLinkCard` image box is
extracted into a shared `StudioLogoBox` (+ pure `studioLogo.ts` caption rule) and a new
`StudioListRow` mounts it in front of the name / ring / count — **option B**, the card's rule
verbatim (owner's pick 2026-09-20 over the recommended C). Child of F51 (HOLODEX-246). No
spec/ADR — presentation-only change to an existing page.

## Gates — definition of done

- [ ] spec — n/a (no new capability, field, or endpoint; recorded in the handoff's "Why no spec/ADR")
- [ ] architecture — n/a (ADR-079 image roles untouched)
- [x] design `design-handoff` — [studio-list-logo-handoff.md](../design/studio-list-logo-handoff.md) +
  [mockup SVG](../design/studio-list-logo-mockup.svg); **B chosen** (StudioLinkCard rule: h-12
  bare box, aspect-following, wordmark replaces the name; A free-width and C fixed-slot rejected);
  name announced once (`alt=""` beside text, `alt={name}` + `title` when hidden); **halo** added same day on
  the owner's first look (`.logo-halo` app.css hook, both roles, card inherits)
- [ ] backend — n/a (no `internal/` edit; the stale role comment lives in `types.ts` only)
- [x] frontend — `entity/studioLogo.ts` (+test), `entity/StudioLogoBox.svelte` (extracted),
  `entity/StudioLinkCard.svelte` (mounts the box), `entity/StudioListRow.svelte` (new),
  `routes/studios/+page.svelte` (row → component); entity `CLAUDE.md` table + `types.ts` comment;
  `.logo-halo` hook in `app.css`
- [x] testing `testing-strategy` — row added under the HOLODEX-411 entry: 6 unit cases +
  driven-browser 11.3–11.11 verified 2026-09-20 (3 skins, a11y tree, 375px); 11.12–11.13 +
  human rows open
- [ ] security — n/a (no auth/access/infra)

## Up next — ordered (position = priority)

1. [ ] [HOLODEX-432] Kevin's human look (handoff §11.14–11.16) on the dev testbed → `gh pr ready`
   (merge main first — the branch is 1 commit behind nothing yet, but check)
2. [ ] [HOLODEX-432] on merge: CI moves 432 to Done (story-keyed branch); nothing to sweep by hand
3. [ ] [HOLODEX-436] `/studios` (and likely `/people`) owner toolbar overflows at 375px —
   pre-existing, found by §7's check; separate fix

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-20 · pick B → implement → test → review → QA
- skills: code-review (high --fix: 2 findings fixed — `loading` before `src`, `decide()` gated on
  `bare`), testing-strategy (row added by hand)
- handoff: Kevin chose **B** over the recommended C, with "name announced once and only once".
  Extracted `StudioLogoBox` + `studioLogo.ts` out of `StudioLinkCard` so the card and the new
  `StudioListRow` share one rule; page `{#each}` → component. Regenerated the SVG (B across 3
  skins, A/C rejected strip) and rewrote the handoff §1–§11 as built. Live QA on a :7802 backend
  (this worktree's `data/`, `web-432` launch entry with `HOLODEX_API_PORT=7802`) against 1000×83 /
  1000×378 / 1000×974 logos + icon-only + monogram: geometry, alt/title, a11y tree, 3 skins,
  375px all green; `npm run check` clean, 385/385 tests. Found and filed HOLODEX-436 (toolbar
  overflow at 375px, pre-existing). Kevin's first look: the dark SLM wordmark vanished on the row →
  asked for a halo on logos and icons; shipped as the `.logo-halo` drop-shadow in `--logo-plate`
  (HOLODEX-411 had rejected a halo — superseded by the owner), verified in 3 skins. PR #370 stays
  Draft for the rest of his look.

### 2026-09-19 · design-handoff
- skills: design-handoff
- handoff: filed HOLODEX-432 under F51, renamed branch `HOLODEX-432-studio-list-logos`, In Progress.
  Handoff + SVG mockup committed with three options rendered on the real tokens at the `lg` column
  width; C (fixed 96×32 slot) recommended because A jags the name column by up to 56px and B leaves a
  wordmark row unnamed for the A–Z nav. No code yet — next session implements §2 once Kevin picks.
