---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-411
status: in-progress
release_note: A studio's logo on the Film and Media detail pages now sits directly on the page background instead of a light plate.
---

# HOLODEX-411 · Studio logo sits bare on the page background

HOLODEX-397 put the studio logo on the light `--logo-plate` inside a `border-rule` frame. The
owner's testbed look (11.11) rejected it: a transparent light mark on a cream box on a near-black
page reads as a jarring badge. Drop the plate on the logo state only — icon and monogram keep it.

## Gates — definition of done

- [~] spec `write-spec` — n/a: presentation-only
- [~] architecture `architecture` — n/a
- [x] design `design-handoff` — options mockup (A plate / B none / C frame) reviewed 2026-09-18,
  **B chosen**; `docs/design/studio-logo-link-card-handoff.md` revised in place (§1 row, code,
  tokens, states, QA 11.2/11.10/11.11) + `studio-logo-no-plate-mockup.svg`
- [x] frontend — `StudioLinkCard.svelte`: `bare = Boolean(logo_url)` gates `border border-rule
  bg-logo-plate`; box size, clamp and inset unchanged
- [x] testing `testing-strategy` — HOLODEX-397 row carries the revision; live three-skin QA on
  `/media/{id}` 2026-09-18: logo box transparent + `border-width: 0px` in all three skins,
  monogram plate `#e9e0d0` dashed intact, dark-on-transparent logo confirmed faint (accepted cost)
- [~] security `security-review` — n/a

## Up next — ordered (position = priority)

1. [ ] [—] Owner look on the prod skin with a real TMDB logo (this testbed's logos are synthetic)
2. [ ] [—] On merge, HOLODEX-411 → Done via CI (branch-keyed)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-18 · filed, decided, implemented, QA'd
- skills: code-review high --fix (no findings)
- Filed HOLODEX-411 from the owner's bug report, renamed the worktree branch, In Progress fired.
  Mockup of three treatments → B (no plate). One-file component change + handoff revision +
  testing-strategy note. Testbed gotcha hit: the immutable `?v=N` logo URL served a stale
  fixture from a previous DB at the same version number — bump the version (re-upload) to bust it.
- handoff: implemented and verified; PR open, awaiting the owner's prod-skin look.
