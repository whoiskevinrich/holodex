---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-432
status: in-progress
release_note: The Studios list now shows each studio's logo in its row — the image TMDB actually supplies — the same way the film and media pages do; a wordmark logo stands in for the name, and a light halo keeps dark marks readable on the dark skins. The cream plate behind studio icons, detail-page images and film posters is gone everywhere — images sit bare with the same halo; only a studio with no image still shows its lettered placeholder.
---

# HOLODEX-432 · Studios list rows draw the logo role

`/studios` read `icon_url` for its row well, but TMDB only ever fills the `logo` role, so every
enriched studio showed a monogram in the list while its logo sat on the payload unused
(`logo_url` is already on `GET /studios`). Frontend-only: the `StudioLinkCard` image box is
extracted into a shared `StudioLogoBox` (+ pure `studioLogo.ts` caption rule) and a new
`StudioListRow` mounts it in front of the name / ring / count — **option B**, the card's rule
verbatim (owner's pick 2026-09-20 over the recommended C). Child of F51 (HOLODEX-246). No
spec/ADR — presentation-only change to an existing page. **Scope widened 2026-09-20 by owner
request to carry [HOLODEX-437](https://whoiskevinrich.atlassian.net/browse/HOLODEX-437)** (no
plate under any image, app-wide — option B) in the same PR.

## Gates — definition of done

- [ ] spec — n/a (no new capability, field, or endpoint; recorded in the handoff's "Why no spec/ADR")
- [ ] architecture — n/a (ADR-079 image roles untouched)
- [x] design `design-handoff` — [studio-list-logo-handoff.md](../design/studio-list-logo-handoff.md) +
  [mockup SVG](../design/studio-list-logo-mockup.svg); **B chosen** (StudioLinkCard rule: h-12
  bare box, aspect-following, wordmark replaces the name; A free-width and C fixed-slot rejected);
  name announced once (`alt=""` beside text, `alt={name}` + `title` when hidden); **halo** added same day on
  the owner's first look (`.logo-halo` app.css hook, both roles, card inherits), strength 1+3+6 from a
  live six-step comparison; **HOLODEX-437** [image-plate-handoff.md](../design/image-plate-handoff.md) +
  [mockup SVG](../design/image-plate-mockup.svg), option B (no plate under an image, halo; plate only
  under the monogram) — inventory of the four plate sites recorded there
- [ ] backend — n/a (no `internal/` edit; the stale role comment lives in `types.ts` only)
- [x] frontend — `entity/studioLogo.ts` (+test), `entity/StudioLogoBox.svelte` (extracted),
  `entity/StudioLinkCard.svelte` (mounts the box), `entity/StudioListRow.svelte` (new),
  `routes/studios/+page.svelte` (row → component); entity `CLAUDE.md` table + `types.ts` comment;
  `.logo-halo` hook in `app.css`; **437**: `StudioLogoBox` icon branch, `EntityImageSlot` both variants,
  `CompletenessQueueRow` — plate only when no image, contained images bare + halo + transparent inset
- [x] testing `testing-strategy` — row added under the HOLODEX-411 entry: 6 unit cases +
  driven-browser 11.3–11.11 verified 2026-09-20 (3 skins, a11y tree, 375px); 11.12–11.13 +
  human rows open; 437 appended to the same row (§7.3/7.4/7.6 verified live, 7.5/7.7 by shared
  class expression — no film on the worktree DB)
- [ ] security — n/a (no auth/access/infra)

## Up next — ordered (position = priority)

1. [ ] [HOLODEX-432] Kevin's human look (studio-list handoff §11.14–11.16 + plate handoff §7.9) on the
   dev testbed → merge main → `gh pr ready`
2. [ ] [HOLODEX-437] **hand-sweep with 432** — the branch carries only the 432 key, so CI never moves
   437: In Review when the PR is marked ready, Done on merge
3. [ ] [HOLODEX-432] on merge: CI moves 432 to Done (story-keyed branch)
4. [ ] [HOLODEX-436] `/studios` (and likely `/people`) owner toolbar overflows at 375px —
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
  (HOLODEX-411 had rejected a halo — superseded by the owner), verified in 3 skins. Kevin asked for a
  strength comparison → live six-step lab on both dark skins × four backgrounds → **1+3+6**. Then
  "the plate is bulky across the app" → inventoried every image frame (four plate sites; posters /
  headshots / video already edge-to-edge), showed A/B/C, **B chosen and pulled into this PR as
  HOLODEX-437**: plate only under the monogram, contained images bare + halo + transparent inset,
  `cover` untouched. Verified live on the list, the studio Images section (logo + uploaded poster),
  the queue (monogram rows). PR #370 stays Draft for the rest of his look; 437 needs the hand-sweep.

### 2026-09-19 · design-handoff
- skills: design-handoff
- handoff: filed HOLODEX-432 under F51, renamed branch `HOLODEX-432-studio-list-logos`, In Progress.
  Handoff + SVG mockup committed with three options rendered on the real tokens at the `lg` column
  width; C (fixed 96×32 slot) recommended because A jags the name column by up to 56px and B leaves a
  wordmark row unnamed for the A–Z nav. No code yet — next session implements §2 once Kevin picks.
