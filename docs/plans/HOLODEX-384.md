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
- [ ] frontend — `FilmsRow.svelte` (sizing via container query on `--cols`/`--card-w`, frame
  chrome, caption block, lift hook, `py-2 -my-2` on the `<ul>`), `app.css` (`.person-hero-media`
  → `.media-lift`, `--static` variant kept), `PersonBanner.svelte` + `people/[id]/+page.svelte`
  (4 rename sites), `EntityVideos.svelte` (hand `cols` / stage-aligned width to the footer)
- [ ] testing `testing-strategy` — handoff §9: geometry equality at density 4 and 8 in both
  layouts (9.2–9.4), hover computed-style + reduced-motion (9.5), banner still static (9.6),
  400px overflow rung (9.7); three-skin QA
- [~] security `security-review` — n/a: no auth/access/infra surface

## Up next — ordered (position = priority)

1. [ ] [frontend] `EntityVideos` → expose `cols` (and the stage-aligned measured track width)
   to the footer as CSS vars — `web/src/lib/components/entity/EntityVideos.svelte`
2. [ ] [frontend] `FilmsRow` sizing + chrome + caption + lift hook — `FilmsRow.svelte`
3. [ ] [frontend] rename `.person-hero-media` → `.media-lift` — `app.css` + 4 sites
4. [ ] [testing] handoff §9.2–9.7 as a geometry check; three-skin QA on `backend-films`
5. [ ] [handoff] mark PR ready → In Review

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-15 · design gate
- skills: design-handoff
- decisions: D1 grid-derived sizing, D2 separate from 296 — both put to Kevin with side-by-side renders, both taken as recommended; recorded in handoff §10, 296 linked in Jira
- handoff: HOLODEX-384 created + In Progress; branch renamed; handoff doc + SVG committed;
  Draft PR open. Next session starts at Up-next 1 — nothing in code has changed yet.
