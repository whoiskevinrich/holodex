---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-95
status: in-progress
profile: full                # film vocabulary widens (arch fork vs per-entity scope), new owner-gated film curation write, UI
depends-on: []
release_note: Film pages now show their Details to every visitor, and films gain runtime, tagline and genres from their metadata provider, editable by the owner like any other field.
---

# HOLODEX-95 · Film detail page adopts the shared entity-field renderer

Child of HOLODEX-20 (Unified metadata presentation). Rescoped 2026-10-13: media, people and studio
pages already share the entity-field renderer; the film page's Details section is the remaining
bespoke one, and it is owner-only. Owner picked option (a) on 2026-10-16: the film Details section
becomes the studio page's, visitors see it, and runtime / tagline / genres join the film field set.
The rejected option (b) was to scope the registry's canonical keys per entity type.

**Design package:** [film-details-parity-handoff.md](../design/film-details-parity-handoff.md) ·
[film-details-parity-mockup.svg](../design/film-details-parity-mockup.svg)

Spun out: HOLODEX-579 (film homepage / status / original language), HOLODEX-580 (AutoFieldRows
pill target size).

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — F59 P0-13..P0-16, P2-5/6; provider contract §3 + §4.2c (no wire change, no version bump); promote / claims / render-hints / two-tier specs widened to four entity types; `docs/reference/canonical-fields.md` Film fields
- [x] architecture `architecture` → `docs/architecture/**` — `entity-relationships.md` film Vocabulary: one global registry, film field set widened; per-entity canonical scope rejected
- [x] design `design-handoff` → `docs/design/**` — `film-details-parity-handoff.md` + SVG; film-enrichment, render-hints, promote, claimed-keys handoffs and QA checklists point at film
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [/] testing `testing-strategy` — strategy updated (F59 invariants, F39/F44/F49 to four entities, films adversarial case); tests land with the build
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. [ ] [design] Owner signs off `docs/design/film-details-parity-handoff.md` + SVG, then `/implement`
2. [ ] [backend] Widen `filmScalarFields` (runtime, tagline) + genres multi; film curation route; `resolveFilm` gains promotions/claims/auto-register; `'film'` in `promotableEntityTypes` + `entityTypeFields` — `internal/api/film_fields.go`
3. [ ] [frontend] Film Details → studio pattern (visitor branch, soleProvider note, CurationFieldRow, AutoFieldRows); `'film'` in `PromotionEntityType` — `web/src/routes/films/[id]/+page.svelte`
4. [ ] [testing] Tests per the HOLODEX-95 invariants in `docs/testing-strategy.md`
5. [ ] [security] `/security-review` the new owner-gated film curation write
6. [ ] [—] After the squash: pin "Vocabulary widened: decided in" in `docs/architecture/entity-relationships.md` to the squash SHA
7. [ ] [—] After merge: provider repos refresh contract-sync (`contract-watch`) for contract §3 / §4.2c

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-16 · session
- skills: design-critique, design-handoff, code-review high --fix (docs), handoff
- handoff: Design package is written and pushed (option (a): runtime/tagline/genres become film fields; visitors see Details) and HOLODEX-95 carries fp:ready-to-build — next, the owner signs off the handoff + SVG and runs `/implement`.
