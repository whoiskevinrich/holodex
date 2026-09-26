---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-411
status: in-progress
profile: ui
release_note: A studio's logo on the Film and Media detail pages now sits directly on the page background instead of a light plate, and a wordmark logo no longer repeats the studio's name beside it.
---

# HOLODEX-411 · Studio logo sits bare on the page background

HOLODEX-397 put the studio logo on the light `--logo-plate` inside a `border-rule` frame. The
owner's testbed look (11.11) rejected it: a transparent light mark on a cream box on a near-black
page reads as a jarring badge. Drop the plate on the logo state only — icon and monogram keep it.

## Gates — definition of done

- [x] design `design-handoff` — options mockup (A plate / B none / C frame) reviewed 2026-09-18,
  **B chosen**; then a caption critique (A always / B logo only / C aspect rule) → **C chosen**;
  `docs/design/studio-logo-link-card-handoff.md` revised in place (§1 rows, code, tokens,
  states, QA 11.2/11.2b/11.10/11.11) + `studio-logo-no-plate-mockup.svg` +
  `studio-logo-caption-mockup.svg`
- [x] frontend — `StudioLinkCard.svelte`: `bare = Boolean(logo_url)` gates `border border-rule
  bg-logo-plate`; aspect rule `naturalWidth ≥ 2 × naturalHeight` → no caption, name to `alt` +
  link `title`; caption withheld until the logo loads; `onerror` restores it; entity `CLAUDE.md`
  row updated
- [x] testing `testing-strategy` — HOLODEX-397 row carries the revision; live three-skin QA on
  `/media/{id}` 2026-09-18: logo box transparent + `border-width: 0px` in all three skins,
  monogram plate `#e9e0d0` dashed intact, dark-on-transparent logo confirmed faint (accepted cost);
  900×220 wordmark → logo alone with `alt`/`title`, no caption in 8 × 150ms samples from
  navigation; 300×300 symbol → caption kept, `alt=""`; monogram unchanged

## Up next — ordered (position = priority)

1. [ ] [—] Owner look on the prod skin with a real TMDB logo (this testbed's logos are synthetic)
2. [ ] [—] On merge, HOLODEX-411 → Done via CI (branch-keyed)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-18 · filed, decided, implemented, QA'd
- skills: code-review high --fix (no findings), design-critique, code-review
- Filed HOLODEX-411 from the owner's bug report, renamed the worktree branch, In Progress fired.
  Mockup of three treatments → B (no plate). One-file component change + handoff revision +
  testing-strategy note. Testbed gotcha hit: the immutable `?v=N` logo URL served a stale
  fixture from a previous DB at the same version number — bump the version (re-upload) to bust it.
- Owner's look on the no-plate build: "transparency terrific, the name beside a wordmark is
  redundant" → `/design-critique` → aspect rule (C) over logo-only (B) so a symbol-only studio
  is never unnamed (397's objection). Second push on the same PR.
- handoff: implemented and verified; PR #349 open, awaiting the owner's prod-skin look with a
  real TMDB logo (wordmark and symbol).
