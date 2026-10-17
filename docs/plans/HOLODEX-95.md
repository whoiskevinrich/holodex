---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-95
status: in-progress
profile: feature             # visitor-visible Details is a behaviour change (F59 P0-13); frontend only
depends-on: []
approved:
  design:
    on: 2026-10-16
    at: efc6d081
release_note: Visitors now see a film's Details, starting with its release date, the same way they already do on studio pages.
---

# HOLODEX-95 · Film detail page adopts the shared entity-field renderer

Child of HOLODEX-20 (Unified metadata presentation). Rescoped 2026-10-13: media, people and studio
pages already share the entity-field renderer; the film page's Details section is the remaining
bespoke one, and it is owner-only. Owner picked **option A** on 2026-10-16: the film Details
section becomes the studio page's and visitors see it. The film field set is unchanged.

An earlier cut the same day widened the film field set (runtime, tagline, genres) and wired
promotion, claims and additional-detail rows for films. It was drawn from TMDB's movie payload. The
owner's production probe showed the production provider sends only `release_date`, `description`,
`actors`, `external_provider_id` and `_source_url` for films (95 of 103 films enriched), so none of
it would render. Reverted before any code; deferred to HOLODEX-579.

**Design package:** [film-details-parity-handoff.md](../design/film-details-parity-handoff.md) ·
[film-details-parity-mockup.svg](../design/film-details-parity-mockup.svg)

Spun out: HOLODEX-579 (film fields and additional-detail rows once a production provider sends
them), HOLODEX-580 (AutoFieldRows pill target size, found in the design critique), HOLODEX-582
(one shared Details component for film and studio, from the build's code review).

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — F59 P0-13 (visitors see Details); films-entity cross-link
- [x] design `design-handoff` → `docs/design/**` — `film-details-parity-handoff.md` + SVG (production-shaped); film-enrichment handoff §1/§4f point at it
- [~] backend → `{cmd,internal,providers}/**` — none: `GET /films/{id}` already returns resolved fields to visitors
- [/] frontend → `web/src/**` — film + studio visitor Details unboxed with `soleProvider`/`rowProvenance` (`lib/format.ts`); live-verified on the stress fixture (visitor: border 0, transparent, padding 0, 44px, one note; owner: boxed); awaiting the owner's build-vs-mockup comparison
- [x] testing `testing-strategy` — `films/[id]/page.svelte.test.ts` (visitor note, custom pill, empty, owner box) + `format.test.ts`; full suite 59 files / 667+ tests green

## Up next — ordered (position = priority)

1. [ ] [frontend] Owner compares the built film + studio Details with `film-details-parity-mockup.svg`, then `/handoff` marks #525 ready
2. [ ] [—] Spun out of review: shared Details component, SourceBadge-on-visitor-block rule, custom url pill → HOLODEX-582

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-16 · session
- skills: design-critique, design-handoff, code-review high --fix (docs + code), handoff, implement
- handoff: Built and live-verified (film + studio visitor Details unboxed; tests green) on draft PR #525 — next, the owner compares the build with the approved mockup, then `/handoff` marks it ready.
