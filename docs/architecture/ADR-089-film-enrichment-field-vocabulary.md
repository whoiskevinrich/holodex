# ADR-089: Film enrichment field vocabulary — where each provider value lands on a film

**Status:** Proposed
**Date:** 2026-09-04
**Deciders:** Project owner

**Relates to:** [ADR-086](ADR-086-film-provider-enrichment.md) (film provider enrichment — established
`entity_type: "film"`, the poster-as-asset rule, and the TMDB entity-aware remap; this ADR is the
follow-on that decides what the *rest* of a provider's film payload does, and is the first to widen
the film field vocabulary beyond `{description, release_date}`) · [ADR-085](ADR-085-films-entity.md)
(films entity — its RD2/RD3 read-only-union model and its `UNIQUE(name, year)` identity pair are the
two constraints this ADR has to work around, and it does not relax either) ·
[ADR-087](ADR-087-film-studio-cascade-decide-and-writeback.md) (film-studio cascade — **this ADR
deliberately does not follow it for cast**, and D1 records why the asymmetry is intentional) ·
[ADR-079](ADR-079-studio-image-roles.md) (studio image roles — the `assetRoleFor` per-entity role map
D4 edits) · [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field decisions — the
grammar `release_date` already resolves through, and inside whose commit D3 places the identity write)
· [ADR-039](ADR-039-provider-asset-urls.md) (asset download perimeter — the film banner download
passes through it unchanged) · [ADR-061](ADR-061-unified-entity-name-identity.md) (unified entity
name-identity — named as the *only* correct future home for film title enrichment, explicitly not
built here) · [ADR-066](ADR-066-enrichment-auto-apply-and-dismissal.md) (the review queue already
ranks film rows that nothing can currently action).
**Spec:** [film-provider-enrichment-ux.md](../specs/film-provider-enrichment-ux.md) (F59) — resolves
its RD1–RD5. Epic [HOLODEX-308](https://whoiskevinrich.atlassian.net/browse/HOLODEX-308).

---

## Context

ADR-086 made films enrichable and shipped the plumbing: `entity_type: "film"`, the
`filmEnrichResolve/Apply/Clear` trio, the `film_images` poster sink, and a TMDB entity-aware remap. It
did not decide what the core should *do* with the rest of a film payload, because at the time nothing
consumed it.

Something does now. The film detail page is being wired to the enrichment controls (F59 P0-1..P0-4),
and the moment it is, the gap becomes visible on screen: TMDB sends a film's `title`, `studio`,
`actors`, `director`, `runtime`, `genres`, `tagline`, `original_title` and `external_provider_id`
alongside the `description`, `release_date` and poster the core accepts. `filmScalarFields` is
`{description, release_date}`. Everything else is dropped or auto-registered inert under
[ADR-056](ADR-056-provider-field-render-hints.md).

### Current state (survey, 2026-09-04)

- `internal/api/film_fields.go` — `filmScalarFields = []string{"description", "release_date"}`.
  `filmFields` synthesizes `name` with **only** a baseline source (`{Namespace: "file", Key: "name"}`)
  and the file comment states "no rename in v1". No provider source can reach `name`.
- `internal/model/model.go:352` — `Film.Year int`, a column distinct from any resolved field.
  Migration 0043 makes `(name, year)` the film's uniqueness constraint.
- `internal/api/film_videos.go` — a film's cast, tags and studios are set unions over `film_videos`,
  assembled outside the resolver entirely. There is no film cast column and no film studio column.
- `film_people_roles` (migration 0043) already exists — `PRIMARY KEY (film_id, person_id, role)` with
  the `''` role sentinel — and was built for film-level billing data. It has no provider writer.
- `internal/enrich/assets.go` — `assetRoleFor` maps film `poster`/`""` → `FilmImagePoster` and
  `thumb` → `FilmImageThumb`. `FilmImageThumb` lost its last consumer in HOLODEX-307 (PR #292) and no
  provider emits `thumb`. Person already maps `banner`/`backdrop` → `PersonImageBanner`.
- `film_images.role` is `TEXT NOT NULL` with a `-- 'poster' | 'thumb'` **comment**, not a `CHECK`
  constraint, and is unique on `(film_id, role, source)`.
- `web/src/lib/types.ts` / `web/src/lib/api.ts` — `EnrichEntityKind` and `ENRICH_ENTITY_BASE` list
  `person | studio | video`. Every enrichment *component* is already entity-agnostic; this pair is the
  only hardcoded chokepoint.

### Why this isn't just ADR-087 again

ADR-087 answered a structurally identical-looking question — *"a film has no studio of its own; what
happens when the owner sets one?"* — with a cascade: decide the value on every attached video, and let
the existing union report it back. The obvious move is to reuse that answer for cast.

It is the wrong answer for cast, for reasons that do not apply to studio:

1. **Cardinality.** A studio is effectively single-valued per film and is genuinely a property every
   scene shares — a scene of *The Matrix* really was produced by the same company as the film. Cast is
   many-valued and per-scene: a performer billed on the release appears in *some* scenes, not all.
2. **Truth of the derived value.** Cascading studio makes the union say something true. Cascading cast
   makes it say something false — that every billed performer appears in every scene.
3. **Circularity.** After a cast cascade the union is no longer independent evidence; it reports back
   what the film wrote into it, and the "is my footage complete?" question becomes unanswerable
   forever.
4. **Blast radius.** ADR-087's cascade is best-effort across N videos and produces N file writebacks.
   For a many-valued field that is N × M writes on data the owner did not assert.

So this ADR consciously diverges from its nearest sibling, and says so, because an unexplained
divergence between two adjacent film features reads as drift six months from now.

### Forces

- **The union model is load-bearing and must not be relaxed.** ADR-085 RD2/RD3 make the film's cast
  the union of its scenes; F59 Non-Goal 2 keeps it that way. Any cast landing zone has to sit *beside*
  the union, not inside it.
- **The identity pair is why `name` is read-only.** `UNIQUE(name, year)` means enriching either half
  is a rename that can collide. Half the pair (`year`) is cheap to make safe; the other half (`name`)
  is a whole identity-machinery integration.
- **A dead image role already exists.** Shipping a second one would be the same mistake twice in a
  fortnight.
- **The components are already generic.** Nothing about this ADR should require touching
  `web/src/lib/components/enrichment/**`.
- **Owner-asserted data must never be silently destroyed.** Everything here is additive or gated.

## Decision

### D1 — Provider film cast lands in `film_people_roles`; it never cascades to attached videos

An applied provider cast writes film-level credit rows keyed to the **film**. No `video_people` row,
no `field_source_decisions` row on any attached video, and no `file_writebacks` entry is produced by a
film enrichment.

This is a deliberate, documented asymmetry with ADR-087's Studio cascade, for the four reasons in
*Why this isn't just ADR-087 again*. The invariant is testable and is specified adversarially in F59
P0-6: applying cast to a film with N attached videos leaves all N videos byte-identical.

Provenance follows the existing shape — rows carry the writing provider, so clearing a provider
removes that provider's credits and leaves owner-entered ones intact.

### D2 — The film page renders the scene union, then only the set difference

The cast surface is **not** two lists and **not** one merged list. It is:

1. the scene union, unchanged, as the primary Cast section; then
2. `film_people_roles` **minus** the union, in a distinct, labelled group meaning *billed on the
   release, present in no scene you own.*

Difference is computed by resolved person identity, not display string, so an alias or case variant
never manufactures a phantom missing entry.

This is a decision about *meaning*, not layout. A merged list destroys the distinction between "in my
footage" and "on the release." Two full lists are ~50% duplicates at realistic scale (a 10-person
union against TMDB's 20-name `maxCastCredits` window). The difference is the only rendering where the
second list carries information the first does not — and that information is exactly ADR-085's
deferred P1-3 scene-coverage signal, obtained without modelling phantom scenes.

### D3 — `films.year` is written as a consequence of the resolved `release_date`, inside the same commit, gated on `(name, year)`; `name` stays baseline-only

`release_date` remains an ordinary film scalar resolving through ADR-051. `films.year` is not a
resolved field — it is an identity column updated as a **consequence** of that resolution, in the same
transaction, after a `(name, year)` uniqueness check.

On collision the entire apply is rejected: no decision row, no year change, no enrichment row. The
error names and links the occupying film. This mirrors films-entity's scene-number collision posture
("no silent swap, no auto-bump") rather than inventing a second convention for the same class of
problem.

`name` is explicitly **not** made enrichable. Film title enrichment, if ever wanted, is an ADR-061
unified name-edit integration — collision detection, merge offer, alias handling — and is a separate
decision, not a field-list edit. Recorded here so a future session does not "fix" the read-only name
by appending a provider source to `filmFields`.

### D4 — `banner` becomes the film's second image role, replacing `thumb`; no migration

`assetRoleFor` gains film `banner`/`backdrop` → `model.FilmImageBanner`, and **loses** the
`thumb` → `FilmImageThumb` case. `model.FilmImageThumb` is removed.

Reusing the contract's existing `banner` kind (~16:9, `backdrop` accepted as a synonym) means no new
asset kind, no provider-contract enum change, and Person's proven download path. `providers/tmdb`
emits the movie `backdrop_path` as a `banner` asset when `entity_type == "film"` — the data was always
in `movieDetails` and simply never read.

**No migration is needed.** `film_images.role` carries a descriptive comment, not a `CHECK`, and
`UNIQUE (film_id, role, source)` already permits an uploaded and a provider-sourced banner to coexist
exactly as it does for posters. Retiring `thumb` is a code change; any stray `thumb` row is inert.

**The role ships with its consumer or it does not ship.** `thumb` was deleted for having none; adding
`banner` speculatively would repeat that within a fortnight. The consumer is the film detail header
(F59 P0-10, design handoff).

### D5 — The SPA's `EnrichEntityKind` widens to include `film`; no enrichment component changes

`'film'` is added to the `EnrichEntityKind` union and to `ENRICH_ENTITY_BASE`, and three thin
`api.ts` methods are added against the already-mounted routes. `EnrichPicker`, `EnrichProviderChips`,
`ProvenanceBadge`, `ProviderIcon`, `ProviderStatusChip`, `EnrichQueueRow` and `enrichRefresh.ts` are
**unmodified** — they take no entity id and no entity kind, and callers inject the three closures.

Recorded as a decision rather than left implicit because "add enrichment to entity kind N" has been
mistaken for a component-generalization task before; it is not one, and a PR that touches
`components/enrichment/**` has gone wrong.

## Options Considered

### D1 — where a provider's film cast lands

| Option | Verdict |
|---|---|
| **`film_people_roles`, no cascade** | **Chosen.** Table exists for exactly this; additive; reversible; no video mutated; keeps the union as independent evidence |
| Cascade to every attached video (ADR-087 shape) | Rejected — writes performers onto scenes they are not in, makes the union circular, N × M writebacks on unasserted data |
| Coverage signal only, nothing stored | Rejected — cannot render a stable difference list across reloads without storing the billed set, and gives the owner no cast field at all |
| Omit cast from film enrichment | Rejected — the union answers "who is in my footage" but never "who is missing", which is the question a film-level entity exists to answer |

### D2 — how the two cast sources render

| Option | Verdict |
|---|---|
| **Union, then set difference** | **Chosen.** The only rendering where the second group carries new information; delivers ADR-085 P1-3 free |
| Two full lists side by side | Rejected — ~14 of 30 chips duplicated at realistic scale; reads as noise |
| One merged list with per-chip provenance | Rejected — destroys the "in my footage" vs "on the release" distinction, which is the entire value |
| Union only, billed set stored but hidden | Rejected — stores data with no consumer; the `thumb` mistake in a different costume |

### D3 — the identity pair

| Option | Verdict |
|---|---|
| **Year only, collision-gated; name read-only** | **Chosen.** Makes the useful half safe for a fraction of the cost; leaves the rename problem where it belongs |
| Both, via ADR-061 name-edit machinery | Rejected *for now* — correct destination, but a separate block; would hold the whole epic behind an identity integration |
| Neither; show provider title/year as an inert hint | Rejected — the owner would retype data the provider already supplied, for no safety gain over a collision check |
| Year with silent rename-on-collision | Rejected — contradicts films-entity's explicit no-silent-swap posture for the same class of conflict |

### D4 — the landscape image

| Option | Verdict |
|---|---|
| **Reuse `banner`, retire `thumb`** | **Chosen.** No contract change, proven path, removes a dead role instead of adding a second |
| New `landscape`/`backdrop` film-only kind | Rejected — the contract already defines `banner` with `backdrop` as an accepted synonym |
| Repurpose `thumb` to mean 16:9 | Rejected — silently changes the meaning of an existing (if unused) role string; a stray row would be reinterpreted |
| Defer the horizontal image | Rejected by the owner — the header is the named consumer and the reason to build it |

## Trade-off Analysis

**What this buys.** Five of the six canonical film details land, each in a place that matches what it
actually is: description and release date as resolved scalars; the poster and banner as assets; cast
as film-owned additive credits. The sixth (title) is consciously left to the machinery built for it.
The cast difference converts an enrichment feature into a library-completeness tool at no extra
storage cost.

**What it costs.** A second cast group on the film page is more surface than a single list, and the
difference computation adds a per-render set operation over two collections that are both small but
both grow with the film. The Studio/cast asymmetry is a real inconsistency an unfamiliar reader will
trip on — mitigated only by writing the rationale down, which is why D1 carries it explicitly.

**What could go wrong.** If a future block *does* make film cast cascade (say, for single-scene films
where the union and the billing genuinely coincide), D1's invariant test will fail loudly rather than
letting the change land quietly — which is the intent. If ADR-061 later absorbs films, D3's year
handling must move inside that machinery rather than persisting as a parallel identity write; the
comment in the year path should say so.

**What is deliberately not solved.** Provider `studio` for films is left to ADR-087's surface (F59
P2-1); `genres`, `runtime` and `tagline` continue to auto-register inert under ADR-056 rather than
being promoted speculatively.

## Consequences

- `film_people_roles` gains its first provider writer; its provenance column becomes load-bearing for
  clear-provider behaviour.
- The film detail payload grows a film-level credits list and a `banner_url`, and loses `thumb_url`.
- `assetRoleFor` becomes the fourth entity-role map to change shape; the Person → Studio → Film
  widening ADR-086 performed is now Person → Studio → Film-with-two-roles.
- `model.FilmImageThumb` is removed — a breaking change to nothing, since no consumer or producer
  exists.
- The provider contract's film asset table and §3 liveness statement, and `tmdb-provider.md`'s entity
  list, become wrong on merge if not corrected in the same PR (F59 P0-11).
- ADR-086 should move Proposed → Accepted; its work merged and this ADR builds on it.
- The film page acquires an opinion about library completeness, which is new for an entity detail page
  and may want generalizing to Studio/Person later — noted, not built.

## Action Items

1. `web/src/lib/types.ts`, `web/src/lib/api.ts` — widen `EnrichEntityKind` / `ENRICH_ENTITY_BASE`, add
   the three film enrich methods (D5, F59 P0-1).
2. `web/src/routes/films/[id]/+page.svelte` — mount `EnrichProviderChips` + `EnrichPicker`; update the
   stale header comment (D5, F59 P0-2).
3. `web/src/routes/owner/enrichment/+page.svelte` — add the `film` kind-map entry (F59 P0-4).
4. `internal/repo` + `internal/api` — provider-written `film_people_roles` rows, and the union-minus-
   credits difference in the film detail payload (D1/D2, F59 P0-6/P0-7).
5. `internal/api/film_fields.go` (or the film apply path) — the `(name, year)`-gated year write inside
   the decision commit, with a comment naming ADR-061 as its future home (D3, F59 P0-5).
6. `internal/enrich/assets.go`, `internal/model/model.go` — `banner` in, `thumb` out (D4, F59 P0-8/9).
7. `providers/tmdb/tmdb.go` — emit `backdrop_path` as a `banner` asset for `entity_type == "film"` (D4).
8. `internal/filmimage` + `internal/repo/film_images.go` — banner role sink and serving URL.
9. `docs/specs/metadata-provider-contract.md`, `docs/specs/tmdb-provider.md` — correct §3, the film
   asset table, and the `film:` vs `provider:film:` naming split; flip ADR-086 to Accepted (F59 P0-11).
10. `/testing-strategy` — the D1 no-video-mutation invariant, the D2 no-duplicate-chip invariant, and
    the D3 collision-rejects-atomically invariant.
11. `/security-review` before the PR is marked ready — enrichment write path plus a second per-film
    asset download through the ADR-039 perimeter.
