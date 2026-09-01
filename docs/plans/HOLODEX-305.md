---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-305                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fixed the person page bio being unreadable behind the banner photo, and stopped the banner from lifting on hover.
---

# HOLODEX-305 · Person hero: bio hidden behind banner; remove banner hover-raise

Two regressions from the HOLODEX-303 hero-bio work: (1) the bio column added to the hero row
(GitHub PR #284) has no stacking context of its own, so the banner image (a positioned
z-index:0 element via `.person-hero-media`, GitHub PR #283) paints over it wherever the row's
negative-margin overhang crosses the banner, making the bio's top lines unreadable; (2) PR #283
gave all three hero images (banner/poster/headshot) a hover-raise (scale + z-index bump) — the
banner should not raise on hover, only the poster/headshot should.

**Design package:** no spec/ADR/design-handoff — pure regression fix restoring the intended
behavior of two already-shipped, already-specified features (HOLODEX-303 hero-bio row,
HOLODEX-283's own hover-raise), no new UX surface or decision · no testing-strategy update — a
CSS stacking-order fix with no existing visual-regression harness to extend; verified manually
via a repro harness exercising the exact `app.css` rules (real `:hover` simulation +
`elementFromPoint` paint-order checks) since neither local test dataset has an enriched person
with a banner set.

## Gates — definition of done

- [~] spec `write-spec` — not applicable, regression fix only, no behavior change beyond
      restoring the already-specified hero layout
- [~] architecture `architecture` — not applicable, no data model/infra change
- [~] design `design-handoff` — not applicable, no new UX surface (removes an unintended
      interaction, doesn't add one)
- [x] frontend — `web/src/routes/people/[id]/+page.svelte` hero row gets `relative` so bio (and
      any future row child) paints above the banner; `web/src/app.css` +
      `PersonBanner.svelte` add a `.person-hero-media--static` modifier that cancels the
      hover-raise for the banner only
- [~] testing `testing-strategy` — not applicable, no visual-regression test infra exists to
      extend; verified via manual repro harness (see design package note above) and
      `npm run check` (0 errors)
- [~] security `security-review` — not applicable, no auth/access/infra surface touched

## Up next — ordered (position = priority)

1. [ ] [—] none — this bug fix is complete

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-01 · session
- skills: simplify
- handoff: HOLODEX-305 implemented and verified (row-level `position:relative` for the bio
  stacking fix, `.person-hero-media--static` modifier for the banner hover opt-out); `/simplify`
  caught two real issues — moved the stacking fix from two per-child `relative` classes to one
  on the row container (simpler, and fixes the general case for any future row child), and
  replaced 4 threaded `:not()` selectors with a single override block after the base rules — both
  re-verified against the exact `app.css` rules before applying. `npm run check` clean (0
  errors). Ready to push and open the PR.
