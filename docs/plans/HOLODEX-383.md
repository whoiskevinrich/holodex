---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-383
status: in-progress
profile: ui
release_note: The Films row on person, studio and tag pages now shows each film's poster when it has one, instead of a lettered plate for every film.
---

# HOLODEX-383 · Person/Studio/Tag Films row always draws the monogram

`FilmsRow.svelte` rendered `monogram(f.name)` unconditionally, so a film with a poster showed
its art on `/films` and `/films/{id}` but a letter on the person page. The list read it consumes
(`GET /films?person_id=…`) already carried `poster_url` — the third surface with the HOLODEX-318
(films index) / HOLODEX-329 (media-detail chips) defect.

## Gates — definition of done

- [~] design `design-handoff` — n/a: same tile, same fallback; the monogram becomes the empty state
- [x] frontend — `FilmsRow.svelte`: `{#if f.poster_url}<img>{:else}monogram{/if}`, mirroring the
  HOLODEX-318 fix on `routes/films/+page.svelte`
- [x] testing `testing-strategy` — no render harness for this component (none for the 318 fix
  either); `npm run check` 0 errors; live-verified on `backend-films` with a seeded film + poster
  + actor: `<img>` loaded 400×600 filling the 80×120 plate, poster-less sibling still draws "N"

## Up next — ordered (position = priority)

1. [ ] [—] nothing open — merge closes the ticket; third occurrence strengthens HOLODEX-296
   (shared poster-tile component)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-14 · diagnosed, fixed, live-verified
- skills: code-review (no findings)
- handoff: PR open; nothing open on the branch.
