---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-414
status: in-progress
release_note: The Enrich picker's candidate pictures now match what you're matching — a widescreen still for a media file, the poster for a film, a headshot for a person, and the logo for a studio — instead of one portrait box for everything.
---

# HOLODEX-414 · Candidate slot shape per entity kind (media backdrop, studio logo)

Follow-up to HOLODEX-406 / F64 (PR #346), which gave every picker row one fixed 40 × 60 slot
with deliberately no entity-kind branching. Kevin's ask (2026-09-18): people keep profile
posters, films keep film posters, **media uses a backdrop-compatible (16:9) thumbnail, studios
use the studio logo**. Same slot, same plate, same monogram, same error handling — only the box
shape follows the kind, and the sidecar sends `backdrop_path` (w300) on the video path.

**Decisions locked from a two-option mockup** (A: slot height locked at 60, width follows the
kind · B: wide slots locked at 80 wide, height follows): **A** — 40 × 60 / 108 × 60 / 120 × 60,
every row stays 76 px, because 406 §4.6 already ruled against handing row height back to the
text stack and thinning a logo's letterbox. **Poster fallback** for a media candidate with no
backdrop (letterboxed in the 16:9 box), not monogram-only. Explicit `w-* h-15` pairs replace
`aspect-[2/3]` so the img can never widen the box (the 406 fixture-size trap). New required
`entityType` prop on `EnrichPicker`; five mounts (four detail pages + `EnrichQueueRow` via
`row.entity_type`). No `asset_hosts` change — backdrops are on the same TMDB host — so no
security-review gate.

## Gates — definition of done

- [x] design `design-handoff` — `candidates-image-per-kind-handoff.md` +
  `candidates-image-per-kind-mockup.svg` (person/film unchanged · media backdrop / poster
  fallback / monogram · studio wordmark / symbol · slot anatomy with px) + verifier-tagged
  `candidates-image-per-kind-qa-checklist.md`; supersession pointer added atop the 406 handoff.
  Mockup reviewed inline by Kevin before any of it was written (A + poster fallback chosen).
- [x] spec `write-spec` — amended `docs/specs/candidates-image.md`: header amendment note, FR3
  (kind-shaped box table, required `entityType` prop, explicit `w-* h-15`), FR4 (rendition per
  `entity_type` table + video/film Given/When/Then), AC4/7/9, sidecar + geometry test notes,
  Resolved Decisions 7–9 (kind-shaped · height locked · poster fallback); contract §2.3
  `image_url` row now carries shape guidance per `entity_type` and target widths (185 / 300)
- [~] architecture `architecture` — n/a: presentation rule + one sidecar field; no seam
- [x] sidecar — `resolveMovie` / `searchMovie` / `findMovieByIMDB` take `entityType`;
  `movieSearchEntry.BackdropPath` (the find result reuses that struct; `movieDetails` already had
  it); `movieThumbURL(entityType, backdrop, poster)` — video → w300 backdrop else w185 poster,
  film → poster. `TestTMDBResolveImageURL`: search (backdrop · neither · poster-only fallback),
  film search, by-tmdb-id and by-imdb-id for both kinds; fixtures carry `backdrop_path` on all
  three movie responses. **Mutation-tested**: dropping the `entityType == "video"` guard fails
  all three film paths. `go test ./providers/tmdb` green; `/code-review high` clean
- [ ] frontend — `slotShape` / `SLOT_CLASS` in `candidateImage.ts` + vitest; `EnrichPicker`
  `entityType` prop + slot classes; five mounts; stub `wide-N` rendition + video `twins`;
  `enrichment/CLAUDE.md` rule
- [ ] testing `testing-strategy` — per-kind slot width assertion (exact `offsetWidth`), row floor
  unchanged; `docs/testing-strategy.md` entries beside F64's
- [~] security `security-review` — n/a unless `asset_hosts` widens (it does not)
- [ ] `/code-review high --fix` before each commit; three-skin QA per the checklist §3
- [ ] PR: Draft now (design gate landed), ready when the gates above are green; Jira
  `needs-design` cleared on this push, `needs-spec` cleared when the spec edit lands

## Up next

1. Frontend (`candidateImage.ts` + vitest, `EnrichPicker` prop + classes, five mounts,
   `enrichment/CLAUDE.md` rule), stub `wide-N` rendition + video `twins` persona.
2. Geometry harness per-kind exact width + `docs/testing-strategy.md` entries; three-skin QA §3.
3. Human QA §4.4 (375 px) decides whether a `sm:` step on the two wide classes is needed.

## Session log

- **2026-09-18** — `/design-handoff`. Filed HOLODEX-414, renamed the worktree branch to
  `HOLODEX-414-picker-thumb-per-kind`, fired In Progress. Rendered the A/B mockup inline, Kevin
  picked A + poster fallback via cards. Wrote the handoff, SVG, QA checklist, 406 supersession
  note. Handoff: design gate is green; next session starts at the spec/contract edit, then
  the sidecar `resolveMovie` signature change.

- **2026-09-18 (2)** — spec + sidecar. Skills: `/write-spec` (edit), `/code-review high --fix`
  (clean). Handoff: spec and sidecar gates green, `needs-spec` cleared; next session starts at
  the frontend — `slotShape`/`SLOT_CLASS` in `candidateImage.ts`, the `entityType` prop, five
  mounts, stub `wide-N` 300 × 169 rendition + video `twins`, then the geometry assertion.
