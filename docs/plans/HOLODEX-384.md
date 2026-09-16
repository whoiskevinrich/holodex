---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-384
status: in-progress
depends-on: []
release_note: The film posters below a person, studio or tag's videos are now the same height as the video cards above them and lift forward on hover, the way the person's headshot does.
---

# HOLODEX-384 · Entity Films row: card-height match + hover lift

`FilmsRow.svelte` (person/studio/tag, via `EntityVideos`' `footer`) is a fixed `w-20`/120px
shelf that ignores the video grid's density and has no hover affordance. Done means: each film
poster is exactly as tall as the `.video-frame` above it at every density (and equal to the
column width in poster layout), carries the frame's border/radius and `VideoCard`'s caption
block, and lifts on hover/focus via the HOLODEX-302 hero hook renamed to a neutral `.media-lift`.
"Extract and reuse" was the first ask — it was already true, no work there.

**Design package:** [entity-films-row-handoff.md](../design/entity-films-row-handoff.md) +
[entity-films-row-mockup.svg](../design/entity-films-row-mockup.svg).

## Gates — definition of done

<!-- States: [ ] not started · [/] in progress · [~] deferred · [x] done. ONLY /handoff sets [x]. -->

- [~] spec `write-spec` — n/a: no behaviour or data-model change; pure presentation of an existing row
- [~] architecture `architecture` — n/a
- [x] design `design-handoff` — `docs/design/entity-films-row-handoff.md` + committed SVG mockup
- [~] backend — n/a
- [x] frontend — `FilmsRow.svelte` (reads `effectiveDensity()` + `activity.cardLayout` itself;
  `.films-shelf` size container, frame chrome, VideoCard caption block, `.media-lift` hook,
  `--lift-slack` padding on the `<ul>` — 6% of tile width, code-review caught the fixed 8px clipping tall tiles), `app.css` (`.person-hero-media` → `.media-lift` + `--static`
  kept; new `.films-shelf` sizing block), `PersonBanner.svelte` + `people/[id]/+page.svelte`
  (4 rename sites). `EntityVideos` untouched — its grid is never stage-aligned, so there was
  nothing to thread down.
- [x] testing `testing-strategy` — `npm run check` 0 errors. Live on `backend-films` (person 1
  "QA Actor", 4 videos curated on, 2 films): **poster h == `.video-frame` h** at cols 2 / 4
  (8 tier-capped to 4 at 1280px) in poster layout (900.575/900.6, 438.375/438.375) and cols 4
  in wide layout (166.5/166.5, tile w 111 = 296·3/8); caption blocks 44/44; tile h == card h;
  375px → cols 1, 184.03/184.05, no horizontal overflow. Hover: `matrix(1.06…)`, z 5, accent
  border + caption, 150ms — all three skins. `.media-lift--static` and the reduced-motion gate
  confirmed present in the served stylesheet (`@layer components`). Harness rung NOT added —
  the invariant is an equality between two elements the geometry harness can't express →
  HOLODEX-395; recorded in `docs/testing-strategy.md` §11. Human items §9.8–9.10 still open.
- [~] security `security-review` — n/a: no auth/access/infra surface

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue. -->

1. [ ] [human] Kevin runs handoff §9.8–9.10 on `backend-films-wide` (local launch profile,
   `CARD_LAYOUT=wide`) → `/people/1`; pass → mark PR #336 ready (fires In Review)
2. [ ] [testing] → HOLODEX-395 relative geometry measure + the §9.2 rung (separate ticket)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 · session
- skills: code-review

### 2026-09-15 · design gate
- skills: design-handoff
- decisions: D1 grid-derived sizing, D2 separate from 296 — both put to Kevin with side-by-side renders, both taken as recommended; recorded in handoff §10, 296 linked in Jira
- handoff: HOLODEX-384 created + In Progress; branch renamed; handoff doc + SVG committed;
  Draft PR open. Next session starts at Up-next 1 — nothing in code has changed yet.

### 2026-09-16 · frontend + live verification
- skills: code-review high --fix (1 fixed: proportional lift slack; 1 documented: size container = stacking context)
- handoff: frontend gate closed in one pass; `EntityVideos` threading dropped after reading the
  source (never stage-aligned). Verified live at 3 densities × 2 layouts × 3 skins + 375px; all
  numbers in the testing gate above. Added a gitignored `backend-films-wide` launch profile
  (same testbed, `CARD_LAYOUT=wide`) — the committed profile pins poster layout. Filed
  HOLODEX-395 for the harness gap. PR stays Draft pending Kevin's §9.8–9.10.
