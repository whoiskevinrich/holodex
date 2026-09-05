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
- [/] backend — 311/312/310 shipped; nothing outstanding
- [/] frontend — 309/311/312/317/310 shipped and verified live
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

### 2026-09-04 · HOLODEX-310 cast coverage — ADR-089 D1's storage amended by one lookup

- skills: (none — implementation)
- **The design changed because of one fact found before writing any code:** the provider's billed
  cast is *already persisted* in the enrichment shadow as `actors` (film 1 held all 20 Dune names).
  D1 had it being copied into `film_people_roles` — which is keyed by `person_id`, so it would have
  forced **creating a Person row for every billed performer**, including ones with no footage, who
  would then sit in `/people` with empty pages. Owner chose the read-time alternative via a card.
  So the billed-but-absent group writes **nothing at all**: it is the shadow minus the scene union,
  computed on read. Clearing the provider empties it by construction. `film_people_roles` stays a
  purely owner-asserted table with no provider writer.
- Needed one new repo primitive: **`LookupEntityIDByName`** — the canonical-nameKey-then-alias prefix
  of `resolveOrCreateByName` with the create step deliberately absent. `PersonIDByName` was not
  enough; it is a bare `COLLATE NOCASE` match that never consults `entity_aliases`, so an aliased or
  merged-away name would have read as missing. That is D2's whole point.
- Rendering: chips, **not** a second `PeopleGrid` as the handoff first said — most of these names
  have no Person row, which is precisely their meaning, so there is no portrait or id to tile. Names
  that do resolve get a link.
- Tests: 4 new, **non-vacuity verified** by disabling the coverage check — three of the four fail with
  their intended diagnostics. One of my own tests was wrong and got fixed rather than the code: it
  expected a case variant and an alias of the same person to count as two billed credits, which
  contradicts the identity dedupe another test in the same file pins.
- Verified live with a genuinely partial case rather than a happy path: enriched a scene video, then
  suppressed three actors from it, giving **"Your scenes cover 17 of 20 billed cast"** and exactly
  Josh Brolin / Javier Bardem / Zendaya as absent chips, all linked, none duplicated into Cast.
  Contrast across the three skins — coverage line 6.31/4.90/5.73, chips 9.16/12.09/17.19.
- handoff: **all seven epic stories are shipped.** What remains before this PR can leave Draft is
  `/security-review` (the one gate never run) and a decision on the open item carried from 317 —
  where `release_date` lives now that the year is directly editable.

### 2026-09-04 · owner review round 2 — banner shipped (312), index poster bug fixed (318), owner-only gating

- skills: (none — implementation)
- Five owner requests, all done and verified live on the films testbed:
  1. **Poster border thinner.** The "border" was never a border — `object-contain` + `p-1` let the
     light `bg-logo-plate` show through as an inset. The hero frame now uses `p-0.5`; the row
     variant keeps `p-1`, where 4px on a small thumbnail is right.
  2. **Studio row gated** `{#if isOwner || studios.length}`, copied verbatim from `media/[id]`.
     Films were the last page showing a visitor "No studio set". Description already complied —
     it only ever rendered when set — so it needed no change rather than a no-op condition.
  3. **Details section is owner-only.** It is provenance/curation machinery, not reader content.
     That retired the visitor's "Enriched from X" note, so the `soleProvider` derived went with it
     rather than lingering as unreachable code.
  4. **HOLODEX-312 banner shipped** end to end: `FilmImageBanner` replaces `FilmImageThumb`
     (no migration — `film_images.role` is a comment, not a CHECK), `assetRoleFor` maps
     `banner`/`backdrop`, TMDB emits `backdrop_path` **for films only** (a video has no image sink,
     so emitting one there would muddy ADR-086's fields-vs-assets split), and the header renders
     an 8:3 band with a scrim under the overlapping poster.
  5. **HOLODEX-318 fixed** — the films index rendered a monogram unconditionally and never read
     `poster_url`, so every film browsed as a lettered plate while its detail page showed art.
- **`EntityImageSlot` needed one prop, `fit`.** It only did `object-contain`, which pillarboxed a
  16:9 backdrop inside the 8:3 band against the light plate — bright bars down both sides, caught
  only by looking at the render. `fit="cover"` crops per the handoff and drops the inset.
- Verified against the real provider, not a fixture: re-enriching film 4 pulled TMDB movie 329865's
  actual Arrival backdrop into `film_images` and it renders as the hero.
- **A measurement error worth remembering:** the first three-skin contrast run reported 17.83/NaN/20.12
  because the helper only parsed `rgb()` and the skin `--bg` tokens are hex — it silently scored
  `#0c0a09` as rgb(0,0,9). Re-run with a hex parser: title 16.84/16.38/18.97, year 6.31/4.90/5.73.
  Those are a **floor** (text vs the scrim's end colour), not a guarantee over bright artwork.
- One existing test failed correctly and was rewritten rather than patched: `TestFilmImage_InvalidRole`
  used `"banner"` as its example invalid role. It now uses `"thumb"` (pinning the retirement) plus a
  never-valid value so it does not depend on thumb staying dead.
- handoff: only HOLODEX-310 (cast layer + difference render) is left in the epic.

### 2026-09-04 · HOLODEX-317 — the year became a field instead of a message; backlink removed

- skills: (none — implementation)
- Built the owner's direction: the film year is now an **editable field in the header** via
  `NameEditControl` — the Media page's Title/Studio affordance — and a `(name, year)` clash renders
  as that control's inline **verdict**, in the control the owner just used. The Details-section
  advisory and its state are deleted outright.
- **The drop-in did not work as-is, and the reason was worth the detour.** `NameEditControl`
  hardcoded `<h1>`; mounting it for the year would have put a second `h1` on the page and declared
  "1999" a page heading. It also had no empty-state text, and prefilled the edit input from the
  displayed string — so a "No year set" placeholder would have become a value to delete. Widened it
  with three optional, defaulted props (`as`, `editLabel`, `placeholder`); every existing call site
  is untouched. Recorded in `entity/CLAUDE.md`, including a pointer for the next person tempted to
  copy `name-edit-row` by hand instead of mounting the component.
- Backend: `repo.SetFilmYear` beside `FillFilmYear`, differing on exactly one axis — **an owner may
  overwrite, a provider may not** — with a test pinning both halves so a later consolidation cannot
  quietly grant providers that right. `PUT /films/{id}/year` returns **`200 {conflict}`, not a 4xx**:
  `NameEditControl` routes a resolved conflict to the verdict card and a rejected promise to a red
  error string, so a 409 would have silently restored the exact presentation this story deletes.
  That status code is asserted directly.
- Removed the "← All films" backlink in the same change (owner request) — the nav's Films link
  already goes there, matching the person page's #286.
- Verified live end to end: empty state → pencil → collision verdict (linked occupant + rationale +
  View/Cancel) → cancel → clean save renders `1984`. **Exactly one `h1`** confirmed in the DOM, and
  the "All films" anchor confirmed gone while the nav link remains. Contrast: `No year set`
  6.31/4.90/5.73, verdict claim 16.00/15.59/18.50, rationale 6.00/4.67/5.59 across the three skins.
- Tests: 3 new Go (7 in the film-year files now), 3 new client. Full Go suite green, `npm run check`
  0 errors, 163 frontend tests pass. The committed mockup was corrected too — its Details panel still
  showed a Year row, which is now precisely the arrangement the story removed.
- **Known limitation, called out rather than left to be discovered:** `NameEditControl` rejects an
  empty commit (right for a name), so a year cannot be *un*-set from the UI once given. Setting a
  wrong year is recoverable; returning to "no year" is not.
- handoff: 310 (cast) and 312 (banner) remain. 312 touches the same header, so rebase on this.
  Studio still wants the same treatment via `StudioPicker` — commented on HOLODEX-285, not done here.

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
