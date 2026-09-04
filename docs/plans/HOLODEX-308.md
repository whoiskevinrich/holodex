---
key: HOLODEX-308
status: in-progress
depends-on: []
release_note: Films can now be enriched from a metadata provider, and the film page shows which billed cast members appear in none of your scenes.
---

# HOLODEX-308 · F59 — Film provider enrichment on the film detail page

Done means an owner can enrich a film from the film page and get its description, release year,
poster and banner in one apply; the cast section tells them which billed performers their scenes do
**not** cover; and the provider-facing docs describe the code that actually shipped. The enrichment
backend already exists (ADR-086/HOLODEX-284) — this epic is the surface for it plus the field
vocabulary decisions that surface forces.

**Design package:** [film-provider-enrichment-ux.md](../specs/film-provider-enrichment-ux.md) (F59) ·
[ADR-089](../architecture/ADR-089-film-enrichment-field-vocabulary.md) ·
[film-enrichment-handoff.md](../design/film-enrichment-handoff.md) +
[mockup](../design/film-enrichment-mockup.svg) ·
[testing-strategy §4/§5 + Critical invariants](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/film-provider-enrichment-ux.md`
- [x] architecture `architecture` → `docs/architecture/ADR-089-film-enrichment-field-vocabulary.md`
- [x] design `design-handoff` → `docs/design/film-enrichment-handoff.md` + committed SVG mockup
- [ ] backend
- [/] frontend — HOLODEX-309 (wiring) done and verified live; 310/312's UI still to come
- [/] testing `testing-strategy` → §4 (4 rows), §5 (2 rows), 3 Critical invariants written; §5's wiring row is now covered by real tests (`api.test.ts`, 8 cases)
- [ ] security `security-review` — until: the enrichment write path and the second per-film asset download are implemented

## Up next — ordered (position = priority)

1. [x] [frontend] Widen `EnrichEntityKind` + `ENRICH_ENTITY_BASE`, add the three film enrich methods — `web/src/lib/types.ts`, `web/src/lib/api.ts` → HOLODEX-309
2. [x] [frontend] Mount `EnrichProviderChips` + `EnrichPicker` on the film Details section; update the stale header comment — `web/src/routes/films/[id]/+page.svelte` → HOLODEX-309
3. [x] [frontend] Add the `film` entry to the owner enrich-queue kind map — `web/src/routes/owner/enrichment/+page.svelte` → HOLODEX-309
4. [ ] [—] Correct the stale provider docs; flip ADR-086 to Accepted — `docs/specs/metadata-provider-contract.md`, `docs/specs/tmdb-provider.md` → HOLODEX-313
5. [ ] [backend] `(name, year)`-gated year write inside the decision commit — `internal/api/film_fields.go` → HOLODEX-311
6. [ ] [backend] Provider-written `film_people_roles` + the union-minus-credits difference in the film payload — `internal/repo/film_people_roles.go`, `internal/api/film_videos.go` → HOLODEX-310
7. [ ] [frontend] Cast difference group + coverage counts line — `web/src/routes/films/[id]/+page.svelte`  ⛔ blocked on #6 → HOLODEX-310
8. [ ] [backend] `banner` role in, `thumb` out; TMDB emits `backdrop_path` as a banner asset — `internal/enrich/assets.go`, `internal/model/model.go`, `providers/tmdb/tmdb.go` → HOLODEX-312
9. [ ] [frontend] Two-image header — banner band, scrim, overlap row, both `EntityImageSlot` instances — `web/src/routes/films/[id]/+page.svelte`  ⛔ blocked on #8 → HOLODEX-312

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-04 · HOLODEX-309 wiring built and verified live end to end
- skills: (none — implementation)
- Shipped F59 P0-1..P0-4 in one commit. `web/src/lib/components/enrichment/**` is untouched,
  which is the spec's own acceptance guard: those components take no entity id and no entity
  kind, so widening `EnrichEntityKind` + `ENRICH_ENTITY_BASE` was the entire integration.
  `enrichRefresh.ts` needed no logic change at all; `api.test.ts` gained 8 cases pinning that
  films ride the generic path, so a future film-specific branch there reads as a regression.
- Verified against the films testbed, not just unit tests: resolve returned 10 ranked TMDB
  candidates, apply filled description/release date/poster, the chip flipped to Refresh with a
  Clear overflow, clear reverted cleanly, the owner queue showed "Films · 1" linking to
  /films/1, and the visitor view showed the section-level "Enriched from tmdb" note with zero
  controls. Chip contrast clears AA in all three skins (low 4.67).
- **Two local-config gaps found and fixed (both gitignored, neither is repo code):** the
  `backend-films` launch config had no `FILMS_ENABLED`, and
  `metadata-sources.local.films.yaml` declared `entity_types: [person, video, studio]`. The
  core *intersects* that operator list with the sidecar's `/describe`, so even though the
  sidecar has advertised `film` since ADR-086, the films testbed could never have shown film
  enrichment. Worth knowing before anyone debugs a "missing" film provider chip.
- handoff: HOLODEX-309 is complete and pushed to the epic's Draft PR #293. Next is Up-next #4
  (HOLODEX-313, the stale provider docs) — it is pure docs, needs no design input, and its
  §4.3 edit is a prerequisite for HOLODEX-312's banner work.

### 2026-09-04 · brainstorm corrected the premise, then all three pre-implementation gates written
- skills: product-brainstorming, write-spec, architecture, design-handoff, testing-strategy
- The ask was "add provider enrichment to the films page". A codebase survey found the backend
  already shipped: `internal/api/film_enrich.go`, the F47 review-queue routes, and TMDB's
  `entity_types: [..., film]` are all live behind `films_enabled`. The film page just never got
  wired, and says so in its own header comment. The reuse answer is that no enrichment component
  needs changing at all — `EnrichPicker`/`EnrichProviderChips` take no entity id and no entity kind.
- The real finding: only 2 of the 6 canonical film details land today. `filmScalarFields` is
  `{description, release_date}` while TMDB already sends title, studio, actors, director and a poster.
  Studio and cast are blocked by a *deliberate* decision (read-only unions over attached scenes), and
  title/year are the `UNIQUE(name, year)` identity pair — which is why `name` is read-only.
- Owner decisions via question cards: cast → `film_people_roles` (not the ADR-087 cascade), render
  union-then-difference, year-only with a collision check, banner in the film header, ship the SPA
  wiring first.
- Two of those re-open week-old decisions: the banner touches PR #292's poster-as-hero header, and
  the cast asymmetry diverges from ADR-087 on purpose — both are written down as such so they don't
  read as drift later.
- Also found and filed: PR #257's ADR number collides with `main` for the second time (HOLODEX-314).
- handoff: all three pre-implementation gates are on disk and the epic is a Draft PR; start at Up-next
  #1 — HOLODEX-309 is a self-contained frontend PR that needs no further design input, and landing it
  makes the remaining field gaps visible on the page instead of only in the spec.
