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
4. [x] [—] Correct the stale provider docs; flip ADR-086 to Accepted — `docs/specs/metadata-provider-contract.md`, `docs/specs/tmdb-provider.md` → HOLODEX-313
5. [x] [backend] `(name, year)`-gated year fill — `internal/repo/films.go`, `internal/api/film_year.go` → HOLODEX-311
6. [ ] [backend] Provider-written `film_people_roles` + the union-minus-credits difference in the film payload — `internal/repo/film_people_roles.go`, `internal/api/film_videos.go` → HOLODEX-310
7. [ ] [frontend] Cast difference group + coverage counts line — `web/src/routes/films/[id]/+page.svelte`  ⛔ blocked on #6 → HOLODEX-310
8. [ ] [backend] `banner` role in, `thumb` out; TMDB emits `backdrop_path` as a banner asset — `internal/enrich/assets.go`, `internal/model/model.go`, `providers/tmdb/tmdb.go` → HOLODEX-312
9. [ ] [frontend] Two-image header — banner band, scrim, overlap row, both `EntityImageSlot` instances — `web/src/routes/films/[id]/+page.svelte`  ⛔ blocked on #8 → HOLODEX-312

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-04 · owner review of the 311 message → interim fix + a better direction (HOLODEX-317)

- skills: (none — review response)
- Owner, looking at the live page: *"The red text feels out of place. I see the released date below,
  so it's unclear what the error is trying to convey."* Both halves correct.
  1. **Red said failure for something that succeeded.** The spec and handoff had already argued this
     should read as an advisory — then it shipped with `text-warn`, identical to real failures. The
     distinction existed in the code and nowhere on screen. Now `text-muted`.
  2. **The message had no referent.** A film with no year renders *nothing* in the header where the
     year goes, so the sentence explained an invisible absence — directly above a visible
     `Released: 2021-09-15` that read as a contradiction. Reworded to name both fields explicitly.
- **The owner's counter-proposal is better than either option I mocked up**: use the Media detail
  page's Studio approach on the Films page for Studio *and* release date — reusable components, one
  affordance across pages, the docked-pencil edit already expected elsewhere. Assessed as highly
  viable: `NameEditControl` is already generic over its conflict payload (`TConflict`) and renders a
  `verdict` snippet inline *in place of the edit form*, which is exactly where a year collision
  belongs. Filed as HOLODEX-317.
- **The strongest evidence came from the page itself:** `films/[id]/+page.svelte` already hand-rolls
  `name-edit-row` / `name-edit-pencil` — CSS hooks belonging to `NameEditControl` — without importing
  it. The page was imitating the pattern rather than reusing it, which is the exact drift PR #257's
  ADR was written about. Adopting the component is a simplification, not an addition.
- Studio's half of that belongs to HOLODEX-285 (In Progress, currently building a bespoke
  `FilmStudioCascadeDialog`) — commented there rather than changed unilaterally.
- **Lesson worth carrying:** I wrote "this is an advisory, not a failure" in three artifacts and then
  styled it as a failure. Prose intent in a handoff does not survive unless the token choice encodes
  it — check the built pixels against the sentence that specified them.
- handoff: §4 of the design handoff is now marked interim in full. 310 (cast) and 312 (banner) still
  open; 317 should land before or with 312, since both touch the film header.

### 2026-09-04 · HOLODEX-311 year fill shipped; ADR-089 D3 amended twice by building it

- skills: (none — implementation)
- **D3 was wrong in two ways that only writing it exposed, and both are now amended in the ADR with
  the original reasoning preserved in Options Considered.**
  1. *"Overwrite the year"* → **fill-only**. `films.year` is owner-asserted and no prior value is
     stored anywhere, so an overwrite is a one-way door: clearing the provider afterwards could not
     put the owner's value back. Fill-only makes the spec's "clearing restores the prior year" true
     by construction instead of by a column that would have to be invented to support it.
  2. *"A collision rejects the entire apply, including the enrichment rows"* → **the identity write
     is gated, not the enrich**. That reading fought ADR-033, which makes the shadow store
     deliberately additive and ungated — by the time `release_date` is readable those rows exist,
     and rolling them back would mean threading a pre-commit guard through `Service.Enrich`, shared
     by all four entity kinds. Owner chose the narrow gate via a question card.
- Shipped: `repo.FillFilmYear` (fill-only, collision-reporting, guarded in SQL as well as in Go),
  `api.syncFilmYear` called from enrich apply/clear and the release_date decision set/clear, a
  `year_collision` on the apply response, and the SPA advisory line.
- Tests: 6 new, and **non-vacuity was verified rather than assumed** — deleting the collision guard
  makes `TestFilmEnrichApply_YearCollisionWithheldAndNamed` and `TestFillFilmYear` both fail. Full Go
  suite green; `npm run check` 0 errors; 160 frontend tests pass.
- Verified live on the films testbed against the real TMDB sidecar, using the best fixture available:
  the testbed already had `Dune (2021)`, so a second yearless `Dune` collides for real. Result:
  HTTP 200, 13 fields applied, `year_collision {film_id: 1, "Dune", 2021}`, both films' years
  untouched, and the advisory renders at 5.35/6.96/6.36 contrast across the three skins.
- **A verification-method lesson worth keeping:** my first collision check reported "no collision"
  when the real answer was a 502 — the sidecar was down and the curl only parsed the body without
  checking the status. Always assert the HTTP status before reading the payload.
- Left in the testbed DB deliberately: film 2 (`Dune`, no year) is now a standing collision fixture,
  and film 3 (`Blade Runner 2049`, 2017) a clean-fill one.
- handoff: 310 (cast layer + difference render) and 312 (banner) remain. 312's contract prerequisite
  already landed under 313, so it only has to make the documented `banner` kind true.

### 2026-09-04 · HOLODEX-313 provider docs corrected — and one of my own claims retracted

- skills: (none — docs)
- **Retraction, and the most important thing in this entry.** The spec, ADR-089, PR #293 and the
  Jira issue all claimed the contract's `film:<film-id>` contradicted ADR-085's
  `provider:film:<id>`. **It does not.** `film:<id>` is the resolver *namespace* (the `Enrichment`
  map key, `handlers.go:1401`, and the prefix a provider must not collide with — which is exactly
  what a provider contract should document); `provider:film:<id>` is the *decision-source* string
  in `field_source_decisions.source`, built by `fieldsource.ForProvider` from the standard
  `provider:<name>` grammar. Both correct, different layers. All four artifacts corrected, and
  ADR-089 Action Item 9 now carries an explicit "do not reconcile these" note so the next reader
  does not 'fix' a correct document.
- Genuinely stale and now fixed: contract §3 and §4.3 said film enrichment was not live (false
  since ADR-086); the film asset table called `poster` the only *planned* kind; `tmdb-provider.md`
  listed three entity types, and — a defect I had not flagged — still described a studio's `logo`
  as an `image_url` **field**, which stopped being true at F51/ADR-079. That doc also uses "film"
  to mean *movie file* throughout, so it now opens with an explicit disambiguation note rather
  than being rewritten wholesale.
- ADR-086 flipped to Accepted, in the file and the index row.
- **Two new issues filed rather than fixed inline:** HOLODEX-315 (TMDB's `/describe` declares
  `asset_kinds: [photo, logo, poster]` but the person builder emits `banner` too — latent only
  because core dispatches on the response kind, not the manifest) and HOLODEX-316 (17 of 18
  ADRs in the 070–089 range still say "Proposed" despite shipping, so flipping 086 alone is
  correct-but-lonely; the status field currently carries no signal).
- handoff: the epic's remaining work is all implementation — HOLODEX-311 (year + collision),
  310 (cast layer + difference), 312 (banner). 312 now has its contract prerequisite in place:
  §4.3 documents the film `banner` kind as decided-but-not-built, so that story only has to make
  it true.

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
