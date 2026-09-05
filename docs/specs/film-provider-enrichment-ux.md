# Spec: Film provider enrichment on the film detail page (F59)

**Status**: Draft
**Phase**: New epic (Jira [HOLODEX-308](https://whoiskevinrich.atlassian.net/browse/HOLODEX-308))
**Owner**: Project owner
**Date**: 2026-09-04
**Feature block**: **F59** — a film's canonical details (title, studio, cast, release year, posters,
description) can be filled from a metadata provider from the film detail page, the same way a person
or a studio already can. The enrichment *backend* for films shipped with F56/ADR-086; this block
builds the owner-facing surface for it and closes the gap between what a provider sends for a film
and what the core is willing to store.

**Depends on** (all shipped):
- [ADR-086](../architecture/ADR-086-film-provider-enrichment.md) / [films-entity.md](films-entity.md)
  (F56) — `entity_type: "film"`, `filmEnrichResolve/Apply/Clear`, the `film_images` poster sink, and
  the `films_enabled` route gate. **The resolve/apply/clear trio and the F47 review-queue routes are
  already mounted**; nothing in this spec adds a new enrichment endpoint.
- [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md) /
  [field-source-of-truth.md](field-source-of-truth.md) (F36) — the per-field decision grammar film
  scalars already resolve through, and the `SourceBadge` control that renders it.
- [ADR-079](../architecture/ADR-079-studio-image-roles.md) (F51) — entity-generic
  `ImageSink`/`downloadAssets`/`assetRoleFor` role plumbing, widened Person → Studio → Film.
- [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md) (F47) — confidence
  routing, dismissal, and the owner enrich queue that already ranks film rows.
- [ADR-030](../architecture/ADR-030-access-control-gating-seam.md) — the owner gate (`requireOwner`).
- `EntityImageSlot` (HOLODEX-286) — the entity-generic image-role control, generic over role.

**Related**: [films-entity.md](films-entity.md) (F56) — this block resolves its deferred **P1-1**
(film enrichment) and **P1-2** (multiple poster roles), and supplies the real signal behind its
deferred **P1-3** scene-coverage badge · [film-studio-cascade-writeback.md](film-studio-cascade-writeback.md)
(F57) — the film→video Studio cascade this spec deliberately does *not* copy for cast ·
[metadata-provider-contract.md](metadata-provider-contract.md) — the film vocabulary and asset-kind
tables this block corrects and extends.

**ADR**: **[ADR-089](../architecture/ADR-089-film-enrichment-field-vocabulary.md) (Proposed)** records
the field-vocabulary decisions as D1–D6: the cast landing zone and its deliberate asymmetry with
ADR-087's Studio cascade, the non-overlap merge rule, the `year` identity write, the `banner` role
replacing the consumer-less `thumb`, and the SPA entity-kind widening. Touches the **enrichment write
path** and adds a second **asset download** per film → `/security-review` before merge.

**Design handoff**: [film-enrichment-handoff.md](../design/film-enrichment-handoff.md) — the Details
section's provider chips, the two-image header, and the dual-cast merge rule at stressed scale.
Mockup: [film-enrichment-mockup.svg](../design/film-enrichment-mockup.svg).

**Test plan**: [testing-strategy.md](../testing-strategy.md) §4 (four backend rows), §5 (two frontend
rows), and three new entries under *Critical invariants*.

---

## Problem Statement

Film enrichment is built and unreachable. `internal/api/film_enrich.go` exposes
`POST /films/{id}/enrich/resolve`, `POST /films/{id}/enrich` and
`DELETE /films/{id}/enrich/{provider}`; `internal/api/enrich.go` adds films to the F47
dismiss/refresh route table; `internal/resolver/film_baseline.go` resolves film scalars; and
`providers/tmdb` advertises `entity_types: [person, video, studio, film]` and has film branches in
`resolve()` and `buildMovieEnrichResponse`. But `web/src/routes/films/[id]/+page.svelte` imports no
enrichment component at all, and says so in its own header comment: *"films have no rename/aliases/
enrichment providers wired yet."* An owner looking at a film has no way to reach any of it.

Wiring the controls is small — the enrichment components are already entity-agnostic. The larger
problem is what happens once they work: **the core's film field vocabulary is narrower than what a
provider already sends.** `filmScalarFields` is `{description, release_date}`. TMDB's film response
also carries `title`, `studio` (plus the `_studio_external_ids` sidecar), `actors`, `director`,
`runtime`, `genres`, `tagline`, `original_title` and `external_provider_id`. All of it is dropped or
parked as an inert auto-registered field. Of the six details an owner considers canonical for a film —
title, studio, cast, release year, posters, description — only **description** and the **portrait
poster** land end-to-end.

Two of the four gaps are not oversights. A film has no studio column and no cast column: both are
read-only set unions over the film's attached videos (films-entity RD2/RD3), which is a deliberate
model choice. And `name`/`year` are the `UNIQUE(name, year)` identity pair, which is why `name` is
baseline-only with "no rename in v1" — enriching either half is an entity rename that can collide,
not a field write. So this block cannot be a list of new field keys; each gap needs its own answer.

## Goals

1. Give the owner the same enrichment affordance on `/films/{id}` that `/people/{id}` and
   `/studios/{id}` already have — provider chips, candidate picker, per-field provenance, clear and
   refresh — reusing the existing components with no changes to them.
2. Close the film field-vocabulary gap for the details an owner considers canonical, choosing a
   landing zone per field rather than widening `filmScalarFields` indiscriminately.
3. Give a film a landscape image with a real consumer on screen, and retire the `thumb` role that has
   none.
4. Make the film's cast section say something true that the scene union alone cannot: which billed
   performers are *not* represented in the footage on disk.
5. Bring the provider-facing documentation back in line with the code it describes.

## Non-Goals

1. **Film rename.** `name` stays baseline-only. If title enrichment is ever wanted it routes through
   the [ADR-061](../architecture/ADR-061-unified-entity-name-identity.md) unified name-edit and
   collision-detect machinery Person and Studio already use — a separate block, not a field addition.
2. **Changing how a film's cast, tags or studios are derived.** The scene union stays the primary
   answer (films-entity RD2/RD3). This block adds a second, clearly-labelled film-level layer beside
   it; it does not replace, merge into, or reorder the union.
3. **Cascading cast to attached videos.** Considered and rejected — see RD1.
4. **A film-level Studio field.** Studio on a film is already owner-editable via F57's cascade
   (`POST /films/{id}/studio/cascade`). Provider studio values are out of scope here; F57 owns that
   surface and a provider-seeded cascade would be a change to F57, not to this block.
5. **Phantom scenes.** Unchanged from films-entity: provider scene lists are still not modelled.
6. **New enrichment endpoints.** Every route this block needs is already mounted.
7. **Multi-provider film UI.** Films inherit whatever HOLODEX-168/85 eventually ships for every
   entity; no film-specific provider picker.

## Resolved Decisions

*(Locked with the owner 2026-09-04 via question cards, after a codebase survey established that the
backend already shipped.)*

- **RD1 — Provider cast is film-level, never written onto the attached videos.** A film's billed
  cast is film-owned, additive data; applying it mutates no video. *(Amended during implementation —
  it is read from the enrichment shadow rather than copied into `film_people_roles`, which would have
  forced creating a Person row per billed performer. See ADR-089 D1; the principle below is
  unchanged.)* This is **deliberately asymmetric
  with Studio**, which cascades a decision to every attached video under
  [ADR-087](../architecture/ADR-087-film-studio-cascade-decide-and-writeback.md). The asymmetry is the
  point, not drift: a studio is effectively single-valued and genuinely is a property every scene of
  the film shares, whereas cast is many-valued and "billed on the theatrical release" is a different
  claim from "appears in this scene." Cascading cast would write performers onto scenes they are not
  in, and would make the union circular — it would report back what the film had just written into it.
  Considered and rejected: cascade-like-Studio; coverage-signal-only with no stored field; omitting
  cast from film enrichment entirely.

- **RD2 — The cast section renders the scene union in full, then only the non-overlapping billed
  names.** At realistic scale (a 10-person union against TMDB's 20-name billing cap) a naive two-list
  render is roughly half duplicates and reads as noise. Showing only the difference below the union
  turns the second list into information: *these performers are billed on the release and appear in no
  scene you own.* This is films-entity's deferred **P1-3** coverage signal, delivered by data this
  block already has to store. The merge rule is the design; a merged single list or two full lists
  both fail.

- **RD3 — Year is enrichable behind a `(name, year)` uniqueness check; title is not.** `films.year` is
  half the identity key, so writing it can collide with an existing film. On collision the apply is
  **rejected with an inline error naming the occupant**, matching the scene-number collision posture
  in films-entity ("no silent swap, no auto-bump"). Title stays read-only (Non-Goal 1).

- **RD4 — The horizontal poster is the existing `banner` asset kind, and it replaces `thumb`.** The
  provider contract already defines `banner` (~16:9, synonym `backdrop`) and Person already consumes
  it. `film_images.role='thumb'` survives in `assetRoleFor` and `model` but lost its last consumer in
  HOLODEX-307 and no provider emits it. Adding `banner` beside a dead `thumb` would leave two
  unused-or-barely-used roles; `banner` takes its place.

- **RD5 — The banner's consumer is named before the role exists.** `thumb` was deleted precisely for
  having no consumer. `banner` ships with the film detail header rendering it, or it does not ship.

- **RD6 — Ship the SPA wiring first, on its own.** It is independent of every decision above, needs no
  gate artifact of its own, and makes the vocabulary gap visible on the page rather than in a document.

## User Stories

1. As an owner viewing a film, I want to pick a provider match and have the film's description,
   release year and poster filled in, so I do not retype what a provider already knows.
2. As an owner, I want to see which source each film field came from and override it, exactly as I can
   on a person or a studio.
3. As an owner, I want the film page to look finished — a landscape backdrop behind the title, not a
   bare band.
4. As an owner, I want to know when my scenes do not cover a film's billed cast, so I can tell an
   incomplete rip from a complete one.
5. As an owner, I want a provider match on a film whose year collides with another film to fail loudly
   and tell me which film it collided with.
6. As a visitor, I want film provenance shown the same restrained way it is on a studio — one
   section-level note, no controls.

## Requirements

### Must-have (P0)

- **P0-1 — Widen the SPA's enrichment entity kind.** Add `'film'` to the `EnrichEntityKind` union
  (`web/src/lib/types.ts`) and to `ENRICH_ENTITY_BASE` (`web/src/lib/api.ts`), and add
  `enrichFilmResolve` / `enrichFilmApply` / `enrichFilmClear` against the mounted routes. This is the
  single hardcoded chokepoint; `runEnrichRefresh`/`runEnrichRefreshAll` (`web/src/lib/enrichRefresh.ts`)
  are already generic and start working once the union widens. Acceptance: a film's refresh and
  refresh-all calls hit `/films/{id}/enrich/...` with no film-specific branch in `enrichRefresh.ts`.

- **P0-2 — Mount the existing controls on the film Details section.** `EnrichProviderChips` in the
  Details section's header row and `EnrichPicker` at page level, mirroring
  `web/src/routes/studios/[id]/+page.svelte`. **No enrichment component may be modified**; they take
  no entity id and no entity kind, and the caller injects resolve/apply/dismiss closures. Acceptance:
  the diff touches `films/[id]/+page.svelte`, `api.ts`, `types.ts` and the owner queue page only —
  `web/src/lib/components/enrichment/**` is unchanged.

- **P0-3 — Respect the films gate in the UI.** Film enrich routes are registered only when
  `films_enabled`; the chips must not render when films are off. Acceptance: with the flag off the
  film Details section renders exactly as it does today, with no dead controls and no failing request.

- **P0-4 — Film rows in the owner enrich queue are actionable.** Add a `film` entry to the kind→
  `{resolve, apply, href}` map in `web/src/routes/owner/enrichment/+page.svelte`.
  `internal/repo/enrich_queue.go` already ranks `EnrichEntityFilm` rows, so they are being produced
  and currently cannot be acted on. Acceptance: a queued film row resolves, applies and dismisses from
  the queue without navigating to the film.

- **P0-5 — `films.year` is filled from the provider's `release_date`, behind a uniqueness check.**
  *(Amended during implementation — see ADR-089 D3.)* Derive the year component from the **resolved**
  `release_date` on every path that can change it; before writing, check `(name, year)`.

  Two narrowings, both deliberate: the fill **only fills a blank year, never overwrites** one (an
  overwrite silently rewrites owner-asserted identity and, with no stored prior value, cannot be
  undone on clear); and a collision **withholds the identity write, not the enrich** (the shadow
  store is additive and ungated by ADR-033, so the rows already exist by the time `release_date`
  is readable). The response carries a `year_collision` naming and linking the occupying film, and
  the page renders it as an advisory, not a failure.

  Acceptance: a clean apply fills the year and the header shows it; an apply against a film that
  already has a year leaves it untouched; a colliding apply leaves **both** films' identity columns
  unchanged, returns the occupant, **and** still resolves the provider's other fields.

- **P0-6 — Provider cast is film-level and writes nothing.** *(Amended during implementation — see
  ADR-089 D1.)* The billed list is already persisted in the enrichment shadow, so the page reads it
  there rather than copying it into `film_people_roles` — which, being keyed by `person_id`, would
  force creating a Person row for every billed performer, including ones with no footage. **No
  `video_people` row, no video decision, no writeback, and no Person row is produced.** Acceptance:
  an adversarial test asserts rendering a film's billed cast leaves the people count unchanged, and
  that every attached video's `video_people`/`field_source_decisions` are untouched.

- **P0-7 — The cast section shows the union, then only the difference.** Billed names that already
  appear in the scene union are not rendered a second time; the remainder render in a visually
  distinct, labelled group. Acceptance: with a union of 10 and a billed list of 20 sharing 14 names,
  the section renders 10 union chips and 6 difference chips — never 30.

- **P0-8 — `banner` becomes a film image role with a consumer.** `assetRoleFor` maps film
  `banner`/`backdrop` → `model.FilmImageBanner`; `providers/tmdb` emits the movie `backdrop_path` as a
  `{kind: "banner"}` asset for `entity_type == "film"`; the film detail header renders it. **No
  migration is required** — `film_images.role` is `TEXT NOT NULL` with a descriptive comment, not a
  `CHECK` constraint, and its `UNIQUE (film_id, role, source)` already lets an uploaded and a
  provider-sourced banner coexist. Acceptance: enriching a film pulls a banner and the header renders
  it; the film's poster is untouched.

- **P0-9 — Retire `thumb`.** Remove `model.FilmImageThumb` and its `assetRoleFor` case (RD4).
  Acceptance: no `film_images` role other than `poster`/`banner` is reachable, and the model's role
  validator rejects `thumb`.

- **P0-10 — The header carries both images without either becoming unreadable.** Per the handoff:
  banner as a background band, the existing poster + title row overlapping it, a legibility scrim, and
  `EntityImageSlot` owning upload/replace/remove for **both** roles. Acceptance: three-skin QA with
  banner-only, poster-only, both, and neither.

- **P0-11 — Correct the provider-facing docs.** `metadata-provider-contract.md` §3 asserts film
  enrichment is not live (false since ADR-086); §4.3's film table calls `poster` the only planned
  film asset kind (to be revisited under P0-8). `tmdb-provider.md` lists three entity types, says a
  studio's logo is an `image_url` field (it became an asset in F51/ADR-079), and states there is no
  film poster sink — true of `entity_type: "video"`, false of `"film"`, and the document uses "film"
  for both. ADR-086 is still marked Proposed although its work merged. Acceptance: a provider author
  can implement film enrichment from the contract alone, without reading Go.

  > **Not a defect, recorded so it is not "fixed" later:** the contract's `film:<film-id>` and
  > ADR-085's `provider:film:<id>` are *both correct and describe different layers*. `film:<id>` is
  > the resolver **namespace** (the `Enrichment` map key, and the prefix a provider must not
  > collide with — the contract's concern); `provider:film:<id>` is the **decision-source** string
  > persisted in `field_source_decisions.source`, built by `fieldsource.ForProvider` from the
  > standard `provider:<name>` grammar (ADR-051). Changing either to match the other would break
  > the layer it belongs to.

- **P0-12 — The year is an owner-editable field, and a collision is its inline verdict.**
  *(Added 2026-09-04 after owner review of P0-5's first cut — see ADR-089 D3.)* The year renders in
  the header via `NameEditControl` (the Media page's Title/Studio affordance), with `No year set` as
  its resting empty state, mirroring the `No studio set` line below it. A `(name, year)` clash renders
  as that control's `verdict`, naming and linking the occupant.

  Two defects this fixes, both worth stating because both were shipped first: a collision message
  styled `text-warn` reads as a *failure* for a request that succeeded; and a message about "this
  film's year" has **no referent** when the header renders nothing where the year belongs — while
  sitting beside a visible `Released` value it appears to contradict. Acceptance: exactly one `h1` on
  the page (the year is not a heading); an owner set may overwrite where the provider fill may not;
  a collision returns `200 {conflict}` so the control shows a verdict, not an error.

### Should-have (P1)

- **P1-1**: **Scene-coverage summary line.** Render the counts the difference list implies — *"your
  scenes cover 8 of 12 billed cast"* — as a single muted line above the cast section, closing
  films-entity P1-3 properly rather than leaving it implicit in chip counts.
- **P1-2**: **Provider link badge on films.** `ProviderLinkBadge` is already entity-agnostic;
  [ADR-083](../architecture/ADR-083-provider-link-badge-person-studio.md)'s `LinkTemplates` map is
  keyed by entity kind and needs a `film` key. Deferred only because it is orthogonal to the field
  vocabulary.
- **P1-3**: **Success toast after a film enrich**, inheriting whatever HOLODEX-86 ships.

### Future considerations (P2)

- **P2-1**: Provider-seeded Studio cascade — offer the provider's `studio` value as the pre-filled
  input to F57's existing cascade dialog, rather than as a film field. Belongs to F57.
- **P2-2**: Film title enrichment via the ADR-061 name-edit machinery (Non-Goal 1).
- **P2-3**: Attach-suggestion from the difference list — "3 billed performers are missing; find scenes
  featuring them." Depends on P1-1 landing first.
- **P2-4**: Multiple poster roles per film (films-entity P1-2's broader reading — alternate
  regional/edition posters), beyond the single `poster`/`banner` pair this block ships.

## Behavior detail

### The cast merge (RD2/P0-7)

The film page already assembles its cast as a set union over `film_videos` (see
`internal/api/film_videos.go`). Film-level credits come from `film_people_roles`, which predates this
block and already carries `(film_id, person_id, role)` with the `''` role sentinel from migration
0043. The page renders:

1. **Cast** — the scene union, unchanged, in its existing `PeopleGrid`.
2. **Billed, not in your scenes** — `film_people_roles` minus the union, by person identity.

Set difference is by resolved person identity, not display string, so an alias or a case variant does
not produce a phantom "missing" entry. A billed name with no matching Person row is a genuine miss and
belongs in the difference list.

Clearing the provider removes the `film_people_roles` rows that provider wrote and leaves any the
owner added by hand — the same provenance split every other cleared field already honours.

### The year write (RD3/P0-5)

`release_date` continues to resolve as an ordinary film scalar through the ADR-051 decision grammar.
`films.year` is a separate identity column and is set as a *consequence* of the resolved
`release_date`, inside the same commit, gated on `(name, year)`. Keeping the check inside the decision
path means a rejected apply never half-writes: the decision, the year, and the enrichment row move
together or not at all.

### The header's two images (RD5/P0-10)

`EntityImageSlot` is already generic over role (`TRole extends string`) and already has the
`variant="frame"` hero mode HOLODEX-307 added for the poster. The banner is the same component with a
wide `frameClass`; **no new component is introduced.** One behavioural difference must be decided in
the handoff: `EntityImageSlot` falls back to the entity monogram for every non-poster role, but
Person's banner (F25.30) deliberately shows *no* placeholder to visitors and an owner-only "+ Add
banner" instead. A monogram stretched across an 8:3 band is not the right empty state.

## API

No new endpoints. Existing, already mounted:

| Method | Path | Handler |
|---|---|---|
| `POST` | `/films/{id}/enrich/resolve` | `filmEnrichResolve` |
| `POST` | `/films/{id}/enrich` | `filmEnrichApply` |
| `DELETE` | `/films/{id}/enrich/{provider}` | `filmEnrichClear` |
| `POST`/`DELETE` | `/films/{id}/enrich/{provider}/dismiss` | `enrichDismiss`/`enrichUndismiss` |
| `POST` | `/films/{id}/enrich/{provider}/refresh` · `/films/{id}/enrich/refresh-all` | `enrichRefresh`/`enrichRefreshAll` |
| `GET` | `/films/{id}/images/{role}` | film image serving — gains `banner` |

All owner-gated inside `requireOwner`, all registered only when `films_enabled`.

Changed response shapes: `Film` gains `banner_url` beside `poster_url` (and drops `thumb_url` per
P0-9); the film detail payload gains the film-level credits list and, with P1-1, the coverage counts.

## UI (grounded in real components)

| Surface | Component | Change |
|---|---|---|
| Film Details section header | `EnrichProviderChips` | **Reused unchanged.** Added to the existing `Details` heading row, mirroring `studios/[id]` |
| Candidate picker | `EnrichPicker` | **Reused unchanged.** Page-level, three injected closures |
| Per-field provenance | `SourceBadge` (`baselineKey="record"`) | Already present on the film Details section |
| Header images | `EntityImageSlot` `variant="frame"` | **Reused.** Second instance for `role="banner"`; empty-state behaviour decided in the handoff |
| Cast | `PeopleGrid` | **Reused** for the union; the difference group is a labelled second instance |
| Owner enrich queue | `EnrichQueueRow` | **Reused unchanged**; the page's kind map gains `film` |
| Provider link | `ProviderLinkBadge` | P1-2 only; needs an ADR-083 `LinkTemplates` film key |

## Success Metrics

Single-owner instance — adoption metrics do not apply. Verification is behavioural:

1. Enriching a film from TMDB fills description, release year, poster and banner in one apply.
2. No attached video's `video_people`, `field_source_decisions` or `file_writebacks` changes as a
   result of any film enrichment (the RD1 invariant).
3. A colliding year apply leaves both films unchanged and names the occupant.
4. The cast section never renders a name twice.
5. Toggling `films_enabled` off and on leaves film enrichment state intact (inherited ADR-085
   suspension behaviour; asserted here because this block adds new film-owned rows).

## Open Questions

**Q0 (resolved 2026-09-04, ADR-089 D6):** now that the year is directly editable (P0-12), it can
disagree with the resolved `release_date`. Resolved as: they are different claims — identity vs
provider metadata — that diverge legitimately, so the page states the divergence in a muted,
owner-only note on the year row and reconciles nothing.

**Q1 (design, blocking the handoff):** does the banner become the header's hero with the poster
demoted to an overlapping slot (Person-style), or sit behind an otherwise-unchanged header? PR #292
made the poster the hero seven days before this spec, so this is a re-open, not a blank canvas.

**Q2 (design, non-blocking):** the visitor empty state for a film with a banner but no poster, and the
reverse. Person answers the banner half (F25.30: no placeholder); the poster half has no precedent in
`variant="frame"`.

**Q3 (behaviour):** should a billed performer with no existing Person row create one on apply, or stay
inert text until a scene attaches them? Creating rows makes the difference list clickable and feeds
completeness; staying inert avoids populating the people index with performers the library has no
footage of. Leaning inert-until-attached; to be settled in the ADR.

**Q4 (scope):** `maxCastCredits` is 20 in `providers/tmdb`. For films specifically, is 20 the right
billing window, or should film enrichment request a deeper cast than a video does? Deferred — 20 until
the difference list proves it truncates something real.

## Timeline / routing

| Step | Skill | Artifact |
|---|---|---|
| 1 | `/write-spec` | this file |
| 2 | `/architecture` | ADR-089 — cast landing + asymmetry, merge rule, year identity write, `banner`-replaces-`thumb`, SPA widening |
| 3 | `/design-handoff` | `film-enrichment-handoff.md` + committed `film-enrichment-mockup.svg` |
| 4 | `/testing-strategy` | §4/§5 rows + Critical invariants |
| 5 | — | **P0-1..P0-4 ship first** (HOLODEX-309), independent of 2–4 |
| 6 | — | P0-5 (HOLODEX-311), P0-6/7 (HOLODEX-310), P0-8..P0-10 (HOLODEX-312), P0-11 (HOLODEX-313) |
| 7 | `/security-review` | before marking the PR ready — enrichment write path + a second per-film asset download |
