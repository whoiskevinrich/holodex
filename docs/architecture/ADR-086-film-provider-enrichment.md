# ADR-086: Film provider enrichment — own `entity_type`, poster as an asset

**Status:** Proposed
**Date:** 2026-08-25
**Deciders:** Project owner

**Relates to:** [ADR-085](ADR-085-films-entity.md) (films entity — resolves that ADR's deferred
"Film provider enrichment" open item and its "What we'll need to revisit" namespace-collision note;
does **not** touch the `"film:"` synthetic resolver-source mechanism ADR-085 §4 introduced, which is
a distinct concern from real provider enrichment) · [ADR-033](ADR-033-metadata-source-plugins.md)
(metadata source plugins — `entity_type` is already a generic, unvalidated string threaded through
`Resolve`/`Enrich`; this ADR is the first to declare `"film"` as a supported value) ·
[ADR-079](ADR-079-studio-image-roles.md) (studio image roles — the `ImageSink`/`downloadAssets`/
`assetRoleFor` entity-generic widening this ADR extends a third time, Person → Studio → Film, same
`entityType` leading-parameter shape) · [ADR-039](ADR-039-provider-asset-urls.md) (asset
download perimeter — film poster downloads pass through unchanged) · [ADR-049](ADR-049-manual-image-precedence.md)
(image provenance lock — film poster provenance reuses the same `source`/`provider` shape) ·
[ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field decisions — film `description`/
`release_date` compete through the standard decision grammar, unlike `album`/`title`'s ADR-085
namespace special case).
**Spec:** [Films entity / F56](../specs/films-entity.md) — resolves **Q3** (provider selection for
P1-1); [Metadata provider contract](../specs/metadata-provider-contract.md) — resolves the "Film
provider enrichment" **Open items** entry. Epic
[HOLODEX-279](https://whoiskevinrich.atlassian.net/browse/HOLODEX-279).

---

## Context

ADR-085 shipped the Films entity (F56) as a purely **owner-asserted** structure — `films`/
`film_videos`/`film_people_roles`/`film_images` (migration 0043) and the `"film:<id>"` synthetic
resolver namespace that lets a film's name compete for an attached video's `album`/`title`. It
deliberately did not build real **provider-facing** film enrichment (spec P1-1) and left two things
open:

1. Whether film enrichment gets its own `entity_type: "film"` or piggybacks on `entity_type:
   "video"` — deferred because, in the owner's words, "the decision was deferred in the past because
   I wasn't ready to implement it."
2. What a film poster image is — `film_images` already has a `role` column shaped after ADR-079
   (`'poster' | 'thumb'`, migration 0043) but no `ImageSink` adapter, repo layer, or enrichment
   consumer exists for it.

Both are now decided by the owner (this session):

- **A video may itself be a whole film, or may be one scene of a film — the film is its own
  entity**, distinct from any single attached video, and must resolve/enrich independently of
  which (if any) video is currently attached.
- **Film enrichment definitely needs a portrait poster image**, mapping to the existing `poster`
  asset kind (~2:3 aspect) already defined in `metadata-provider-contract.md` §4.3 — not a new
  kind.

**Constraints / forces**

- `internal/enrich`'s `entity_type` plumbing (`Resolve`/`Enrich`, `Source.Supports`/
  `entityTypesSupport`) is already a generic, unvalidated string — declaring `"film"` needs no core
  validation-layer change. `model.EnrichEntityFilm = "film"` already exists
  (`internal/model/model.go:360`) as an unconsumed constant.
- `entity_enrichment` (migration 0005) has no `entity_type` `CHECK` constraint — it can already hold
  film rows.
- `internal/api/film_fields.go` (F56 scaffolding) already has `filmScalarFields` (`description`,
  `release_date`), `filmProviders()`, and `resolveFilm()` reading `EnrichmentForEntity`/curation/
  decisions — built ahead of provider wiring, on the assumption enrichment would land later.
- `imageBackedEntityType()` in `internal/enrich/service.go` (~line 704) hardcodes `person`/`studio`
  only — a film poster asset from a provider is currently silently discarded, not stored.
- The `"film:"` synthetic resolver namespace (ADR-085 §4) is a **different mechanism** — video↔film
  attachment competing for a video's own fields — and must not collide with a real film-entity
  provider's own declared namespace.
- `providers/tmdb/tmdb.go` already has full movie support (`resolveMovie`/`fetchMovieDetails`/
  `fetchMovieCredits`, lines ~245–536) built for `entity_type: "video"` — an obvious first film
  provider, but its field keys (`overview`) and poster handling (`poster_url` field) are
  video-shaped, not film-shaped.

## Decision

### 1 — Film enrichment gets its own `entity_type: "film"`

Not a reuse of `"video"`. A film's identity and enrichment lifecycle are independent of which (if
any) scenes are attached to it — a film can exist with zero attached videos, one full-length video,
or many scene videos, and none of those states should gate whether the film itself resolves a
description/release date/poster from a provider. Reusing `"video"` would conflate a video file's own
resolved metadata with the film's and breaks down entirely for a film with no full-length video
attached to key off of.

This activates the existing `model.EnrichEntityFilm` constant. Surface work is enumeration, not new
mechanism: `mountEnrich`'s hardcoded `{people, studios, media}` entity loop
(`internal/api/enrich.go` ~90–116, ~325–416) gains `film`; `enrich_review.go`'s two hardcoded
entity-type switches (~30, ~290–341) gain a film case; `metadata-sources.yaml`'s `entity_types`
documentation lists `film` as a fourth declarable value alongside `person`/`video`/`studio`.

### 2 — Film poster is an asset (`film_images.role = 'poster'`), never a canonical field

Mirrors the ADR-079 Person/Studio pattern, not video's `poster_url` field. `film_images` already has
`role TEXT NOT NULL — 'poster' | 'thumb'` (migration 0043, shaped after ADR-079) with `UNIQUE
(film_id, role, source)` — deliberately allowing an uploaded poster and a provider-sourced poster to
coexist as distinct rows, unlike Studio's single-slot-per-role `UNIQUE(studio_id, role)`, because a
film's provider poster and an owner's manual upload are both meaningful at once. No schema work is
needed — this decision activates a table that has sat unused since F56.

`enrich.ImageSink` / `Service.downloadAssets` / `assetRoleFor` widen a third time (Person → Studio
via ADR-079, now Studio → Film) using the same `entityType` leading-parameter shape ADR-079
established — mechanical, not novel. A new repo-layer film-images adapter
(`GetFilmImage`/`UpsertFilmImage`/`DeleteFilmImage`/`LockedFilmImageRoles`) mirrors ADR-079's studio
adapter, dispatched by `entityType` inside the shared `ImageSink` implementation.

Portrait aspect (~2:3) matches the existing `poster` kind exactly — no new asset kind, no new aspect
contract in `metadata-provider-contract.md` §4.3, just a new consumer of a kind that already exists
there.

### 3 — TMDB is the first provider; it needs an entity-type-aware remap, not new endpoints

`resolveMovie`/`fetchMovieDetails`/`fetchMovieCredits` already exist for `entity_type: "video"`
(`providers/tmdb/tmdb.go` 245–536) — no new TMDB API calls are needed. Two changes: (a) `resolve()`'s
switch (~245–253) gains a `"film"` case; (b) `buildMovieEnrichResponse`'s `fields["overview"]`
(video's canonical key) must remap to `fields["description"]` (film's canonical key — already reused
by `film_fields.go`'s `filmScalarFields` from `registry.go`'s studio-entity `description` canonical),
and `PosterPath` (currently emitted as `fields["poster_url"]`, ~543–544) must route to an asset
(`kind: poster`) via the widened `ImageSink` instead, when `entity_type == "film"` — video's own
behavior is unchanged.

This resolves spec `films-entity.md` **Q3** ("non-blocking" — reuse TMDB vs. provider-agnostic from
day one): reuse is strictly less work and the field shapes already line up, so this ADR does not
pursue a provider-agnostic abstraction that has no second consumer yet. Wiring the remap itself is
implementation follow-up, not part of this ADR's decision surface.

### 4 — Lock the `"film:"` namespace-collision boundary ADR-085 flagged

ADR-085's "What we'll need to revisit" named this: once a real provider enriches under
`entity_type: "film"` (this ADR), `resolveDecided`'s `"film:"`-prefix check (the ADR-085 §4
video↔film-attachment mechanism) must stay an **exact-prefix match** against the reserved
`provider:film:<id>` token shape, not a pattern a real provider's own declared namespace could spoof.
TMDB's provider name is `"tmdb"`, not `"film"` — no actual collision exists today — but P1-1 shipping
is the trigger condition ADR-085 named for turning this from a note into a test, so this ADR makes it
an explicit Action Item.

## Options Considered

**`entity_type`**

| Option | Assessment |
|---|---|
| **A (chosen) — own `entity_type: "film"`** | Complexity: Low (generic plumbing already supports arbitrary values). Cost: a repo/adapter layer + a few switch-case additions. Familiarity: High — identical pattern to ADR-079's Studio addition. |
| **B (rejected) — reuse `entity_type: "video"`, enrich only fully-attached full-film videos** | Complexity: Low upfront (no new entity-type wiring). Cost: looks cheap short-term, breaks down for a film with zero or partial attached videos — no video to key enrichment against. Familiarity: none — video enrichment is per-file; a film is not. |

**Option A pros:** film resolves independently of any attached video, matching the product reality
that a video may be a whole film or one scene of one; matches the Person/Studio precedent exactly;
`model.EnrichEntityFilm` was already named for this. **Cons:** touches three hardcoded entity-type
lists (`mountEnrich`, `enrich_review.go` ×2, `metadata-sources.yaml` docs).

**Option B pros:** zero new entity-type plumbing. **Cons:** rejected directly by the owner — conflates
a video file's own metadata with the film's; a film's identity and lifecycle are independent of
which scenes happen to be attached yet, which is exactly why this decision was deferred previously.

**Poster**

| Option | Assessment |
|---|---|
| **A (chosen) — asset via `film_images` (`role='poster'`)** | Schema already exists and is already shaped for this (migration 0043); matches Person/Studio precedent exactly (ADR-079); portrait aspect is the `poster` kind's existing default. |
| **B (rejected) — resolved field (`film.poster_url`), video-style** | No `ImageSink` widening needed. Breaks the established Person/Studio precedent for exactly the reason ADR-079 retired Studio's own poster-as-field; would need its own upload/provenance-lock mechanism duplicated from ADR-079 rather than reused. |

## Trade-off Analysis

Both decisions extend an established precedent rather than inventing a new one: the `entity_type`
choice repeats the Person → Studio widening `internal/enrich` was already designed for (a generic
string with no enum), and the poster choice repeats the Person → Studio asset-model widening ADR-079
performed, onto a table that was already shaped for it in F56 but left unwired. The entity_type
decision is validated by a real product fact (films and their attached videos have decoupled
lifecycles) rather than an implementation convenience; the poster decision is validated by schema
that has sat ready and unused since migration 0043. Neither decision required weighing genuinely
competing designs — the "video-shaped" alternative for each was considered and rejected on
consistency and correctness grounds, not on cost.

## Consequences

**What becomes easier**
- Film enrichment reuses the entire existing decision-chip / curation / shadow-store UI and
  mechanism unmodified — no new frontend model, same as Person and Studio.
- `film_images`, unused since F56, gets its first real writer.
- Closes both of ADR-085's forward references — the "Film provider enrichment" open item and the
  namespace-collision revisit note — with concrete resolutions instead of leaving them open.

**What becomes harder**
- `ImageSink`'s signature changes for every implementer/call site a third time (ADR-079 already
  paid down this cost once) — mechanical, but recompiles the Person/Studio call sites too (behavior
  unchanged).
- Three more hardcoded entity-type switch/list sites need a fourth case (`mountEnrich`,
  `enrich_review.go` ×2) — `internal/enrich`'s entity-type genericity doesn't extend to
  `internal/api`'s routing layer, which stays a manual enumeration by design (explicit route
  registration, not reflection).

**What we'll need to revisit**
- P1-2 (multiple poster sizes/roles, e.g. a smaller list-thumbnail) is out of scope here —
  `role='thumb'` already exists in the schema but has no consumer yet; this ADR wires `'poster'`
  only.
- Provider-agnostic film enrichment (spec Q3's alternative) is not pursued. If a second film
  provider is ever added, the TMDB-specific field-key remap (§3) is the precedent it would need to
  follow, not a generalized contract change made preemptively here.

**Security review touch-points** (for the `/security-review` gate)
- Film poster download reuses the existing ADR-039 asset perimeter (SSRF allowlist via `base_url`,
  `asset_hosts`) unchanged — no new perimeter surface, same posture as ADR-079's Studio addition.
- New owner-gated film enrich endpoints (search/apply/dismiss/refresh) must confirm `requireOwner`
  parity with the existing Person/Studio enrich routes.

## Action Items

1. [ ] `internal/api/enrich.go`: `mountEnrich`'s `{people, studios, media}` entity loop gains
   `film` (dismiss/refresh/refresh-all routes); new `film_enrich.go` handler
   (`filmEnrichResolve`/`Apply`/`Clear`) mirroring the existing studio handlers.
2. [ ] `internal/api/enrich_review.go`: both hardcoded entity-type switches (~30, ~290–341) gain a
   `film` case.
3. [ ] `internal/enrich`: widen `ImageSink` + `downloadAssets` + `assetRoleFor` with `entityType` a
   third time (Person/Studio call sites pass their existing values unchanged);
   `imageBackedEntityType()` (~`service.go:704`) gains `film`.
4. [ ] `internal/repo`: `film_images` CRUD (`GetFilmImage`/`UpsertFilmImage`/`DeleteFilmImage`/
   `LockedFilmImageRoles`) + film `ImageSink` adapter dispatch, mirroring ADR-079's studio adapter.
5. [ ] `providers/tmdb/tmdb.go`: `resolve()`'s switch (~245–253) gains `"film"`;
   `buildMovieEnrichResponse` remaps `fields["overview"]` → `fields["description"]` and routes
   `PosterPath` to an asset (`kind: poster`) instead of `fields["poster_url"]` when
   `entity_type == "film"` (video behavior unchanged).
6. [ ] `metadata-sources.yaml`(`.example`): document `film` as a fourth declarable `entity_type`.
7. [ ] `docs/specs/metadata-provider-contract.md`: replace the "Film provider enrichment... not yet
   decided" **Open items** bullet with a resolved reference to this ADR; add §4.2c (film canonical
   fields: `description`, `release_date`) mirroring §4.2a/§4.2b; note film's use of the existing
   `poster` asset kind in §4.3.
8. [ ] `docs/specs/films-entity.md`: resolve **Q3** with a reference to this ADR §3 (TMDB reuse, not
   provider-agnostic).
9. [ ] Add this ADR's row to `docs/architecture/README.md`. ADR-085 itself is not edited (ADRs are
   immutable) — the back-reference lives in this ADR's **Relates to** line and in the specs updated
   above.
10. [ ] Regression test: `resolveDecided`'s `"film:"`-prefix branch (ADR-085 §4) stays an
    exact-prefix match against the reserved `provider:film:<id>` token and cannot be spoofed by a
    real provider's own namespace — the defensive test ADR-085 flagged as a revisit item once P1-1
    ships.
11. [ ] `/testing-strategy`: film enrich search/apply/dismiss/refresh round-trip; film poster asset
    storage + suppression + provenance lock (ADR-049 parity); `entity_type="film"` resolve/curation
    decision precedence.
12. [ ] `/security-review` before merge: new owner-gated film enrich mutation surface, film poster
    download through the ADR-039 perimeter, `requireOwner` parity check against Person/Studio.
