# Spec: Films as a first-class entity (F56)

**Status**: Draft
**Phase**: New epic (Jira [HOLODEX-279](https://whoiskevinrich.atlassian.net/browse/HOLODEX-279))
**Owner**: Project owner
**Date**: 2026-08-17
**Feature block**: **F56** — a new entity, `films`, whose membership in a video is a standing
**owner assertion** ("this file is scene 6 of film Y") rather than a value derived from resolved
file/provider fields. Films are browsable like Person/Studio, gated by a config flag, support
enrichment and multi-role posters, and participate in the F36 decision model as a **new kind of
resolver source** for a video's `album`/`title` fields — not merely as the fourth entity riding
the existing shape.

**Depends on** (all shipped):
- the F36 decision model ([ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md),
  migration 0016 `field_source_decisions` — keyed by `entity_type`/`entity_id`/`field_key`)
- the entity-agnostic resolver ([ADR-052](../architecture/ADR-052-baseline-source-contract.md),
  `BaselineSource` + `ResolveFields`)
- the studio entity ([ADR-053](../architecture/ADR-053-studio-entity-and-resolved-link-derivation.md),
  [studio-entity.md](studio-entity.md)) — the **derived**-link precedent this spec deliberately
  diverges from (see Resolved Decisions RD1)
- the person-link resolved derivation ([ADR-072](../architecture/ADR-072-person-link-resolved-derivation.md),
  migration 0037 `video_people(video_id, person_id, role)`) — **unchanged** by this spec; films
  read from it, never write to it
- the entity-image pipeline (person/studio images, `internal/personimage`/`internal/studioimage`)
  — reused for film posters, not reinvented
- the owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md), `requireOwner`)
- the `capabilities` payload ([internal/api/auth.go](../../internal/api/auth.go) `capabilities`
  handler) — the existing mechanism for shipping a server-computed flag to the SPA
  (`card_layout`, `person_gallery_max`) that `films_enabled` extends

**ADR**: **ADR-085 (Proposed, pending `/architecture`)** will record the decisions that rise to
ADR level: (1) the `films`/`film_videos`/`film_images` data model, including the
`UNIQUE(film_id, scene_number)`-with-NULL sentinel (RD5) and the `UNIQUE(name, year)` identity
key (RD9); (2) the **asserted-link model** — a link the resolver may never derive or prune
(RD1), the inverse of ADR-053's rule; (3) the film-as-resolver-source mechanism for `album`/
`title` precedence, including how a video attached to multiple films presents multiple
candidate values to one scalar field (flagged as an open technical question below — the spec
locks the *behavior*, the ADR must lock the *mechanism*); (4) `films_enabled` suspend semantics
over `field_source_decisions` (RD6). Touches **access** (new owner-gated endpoints, new
owner-gated bulk-attach mutation) → `/security-review` before merge.

**Design handoff**: pending `/design-handoff` — films list/detail layout, the two attach
pickers (video-side and film-side), the two-region film detail page, the new films row on
person/studio/tag pages, 3-skin QA.

**Related**: [studio-entity.md](studio-entity.md) (the derived-link entity this spec's RD1
explicitly contrasts against), [tag-categories.md](tag-categories.md) (a recent precedent for a
UX-scoped grouping entity with its own picker), [field-source-of-truth.md](field-source-of-truth.md)
(the decision model films must compete inside, not bypass).

---

## Problem Statement

Holodex has no way to record that a file is part of a larger work. A user with a ripped
theatrical release split across scene files — or a single full-length file — has no way to say
"these six files are one film" and see them, their cast, and their poster as one browsable
thing. Every existing entity (person, studio, tag) is either scanned directly off a file or
*derived* from a resolved field value; a film is neither — it is a relationship the user
asserts and the system must remember durably, distinct from anything a rescan can reconstruct.

## Goals

1. **Films become navigable identity** — a `films` table, `/films` list and `/films/{id}`
   detail pages, enrichment, posters, descriptions, release dates, and inherited cast/studio/
   tags from attached videos — the same navigational grade as Person and Studio.
2. **Attachment is a durable, explicit assertion** — a video's film membership and scene number
   survive scans, refreshes, and re-enrichment untouched; nothing in the automatic relink/prune
   machinery (RD1/ADR-053, ADR-072) may create, move, or delete a film attachment.
3. **Writeback stays inside the existing precedence model** — a film's name reaching a video's
   `Album`/`Title` tag is a candidate value competing under `field_source_decisions`
   (ADR-051), never a bypass; a video attached to two films is a resolvable conflict, not
   silent overwrite.
4. **The feature is safely optional** — `films_enabled=false` removes films from every server
   surface (routes, search, MCP) with zero risk of losing data or corrupting field provenance
   when toggled, and zero risk of hiding a video's only cast/studio/tag exposure.

## Non-Goals

- **Provider-known "phantom scenes."** The scene list is exactly the videos the owner has
  attached — no unfilled slots for scenes a provider reports but the owner doesn't own. A
  scalar `scene_count` from enrichment (for a lightweight "3 of 8 owned" indicator) was
  considered and explicitly deferred; if pursued later it is additive and does not require
  revisiting this spec. *(Why: a completeness/matching UI is a materially larger build the
  owner did not ask for in v1.)*
- **Film identity operations (rename / alias / merge).** Exact `(name, year)` resolve-or-create
  only. A duplicate film is a manual-cleanup problem in v1, same cut studios made in
  [studio-entity.md](studio-entity.md) RD4. *(Why: films are user-created via search-attach, not
  derived from noisy file strings, so duplicate-creation pressure is much lower than for
  studio/tag.)*
- **Multi-video scene ranges.** One video may link to one film exactly once
  (`PRIMARY KEY(film_id, video_id)`); a single file spanning multiple scenes (e.g. merged
  scenes 3–4) is out of scope. *(Why: confirmed extreme edge case, not worth the schema and UI
  cost of ranges.)*
- **Automatic full-film detection.** Whether a file "represents the entire film" is always an
  explicit owner choice at attach time, never inferred from duration, filename, or provider
  data. *(Why: getting this wrong is exactly the failure mode that would make the video-list
  hiding behavior (RD7) untrustworthy.)*
- **MCP write surface for films in v1.** MCP read access is gated by `films_enabled` like every
  other surface (RD6), but no new MCP mutation tools ship in this spec. *(Why: keeps the new
  owner-gated mutation surface — attach/detach/bulk-attach — to REST only for the first cut,
  consistent with how enrichment/decision endpoints started REST-only.)*
- **Backfilling historical relationships.** There is no attempt to infer past film membership
  for existing libraries; every attachment starts from a user's explicit search-attach action.

## Resolved Decisions

*(Locked with the owner across a multi-round brainstorm, 2026-08-17, via question cards.)*

- **RD1 — Links are asserted, never derived.** Unlike `video_studios` (ADR-053 RD1) and
  `video_people` (ADR-072), a `film_videos` row is created and destroyed only by explicit owner
  action (attach / detach / renumber). No scan, refresh, enrich-completion, or decision/curation
  change may create, move, or delete a `film_videos` row, in either `films_enabled` state. This
  is the single most load-bearing rule in this spec — it is the opposite of the pattern every
  other entity in the codebase follows, and the ADR must state it as an explicit invariant, not
  an implicit consequence.
- **RD2 — A film's baseline is the union of its scenes.** A film has no file of its own (unless
  it consists of exactly one full-film file). Its `BaselineSource` for inherited fields (cast,
  tags) is the set union over its attached videos' resolved values — the first entity in
  Holodex whose baseline is other entities rather than a file. Film-owned fields (name, poster,
  description, release date, film-level people roles) resolve through enrichment/decisions
  exactly like Person/Studio; only cast and tags are the union-of-scenes special case.
- **RD3 — People stay owned by the video; films aggregate.** `video_people` (ADR-072, with
  `role`) is untouched. A film's displayed cast is the read-only union of its attached videos'
  people (no double-counting a person in two scenes). Film-level *roles* (billing order,
  director, etc. — attributes of the person's relationship to the *film*, not any one scene)
  are new, additive, film-only data, stored separately from `video_people`.
- **RD4 — Scene list = owned videos only, full-film files excluded.** The film detail page has
  two regions: a full-film-file section and a separate scenes list; a full-film file never
  appears in the scenes list even though it still feeds RD2's cast/tag union. A film may hold
  **multiple** full-film files (e.g. a 2160p and a 1080p copy of the same release) — it's a
  list, not a single slot — and may simultaneously hold standalone scene rips overlapping the
  same content as a full-film file; neither the multiplicity nor the overlap is flagged as an
  error.
- **RD5 — Scene numbering.** `UNIQUE(film_id, scene_number)` where `scene_number` is **NULL**
  for an unnumbered scene. Sparse numbering is legal; unnumbered scenes have no defined order.
  This deliberately uses SQLite's NULL-is-distinct-in-a-unique-index behavior as the *wanted*
  mechanism — the inverse of migration 0037, where that same behavior was a bug worked around
  with an empty-string sentinel for `role`. The companion migration's comment must say so
  explicitly, so a future author doesn't "fix" this table by copying the 0037 pattern and
  silently capping a film at one unnumbered scene. Attaching a video at a number that's already
  taken is rejected with an inline error naming the current occupant — no silent swap, no
  auto-renumbering.
- **RD6 — `films_enabled` is a real, server-side, subtractive flag.**
  - **Additive surfaces gated off:** `/films` routes are not registered, films are excluded
    from FTS and the MCP surface, and the film resolver source (RD8) is suspended — all when
    the flag is false.
  - **Subtractive while on:** a video file marked as representing an entire film is hidden from
    *every* video list — browse, `RelatedShelf`, landing-page recently-added, global search,
    and the shared `EntityVideos.svelte` used by the person/studio/tag detail pages — while
    `films_enabled` is true, reappearing the moment the flag is false or the file is detached.
    This is self-limiting: only a file *attached* as full-film is affected, so nothing changes
    for a library with the flag off, and no full-movie file is ever hidden by inference.
    Attaching a file as full-film is a visible side effect (the file will vanish from browse)
    and the attach UI must say so, not let it be discovered.
  - **Consequence — new scope on three existing pages:** because full-film files are hidden
    from `EntityVideos.svelte`, the person/studio/tag detail pages must each gain a **films
    row**. Without it, a full-film-only release (no separate scene rips) would make a person's
    video count go to zero with no replacement surface — the explicit anti-pattern this project
    avoids (an informational dead end with no affordance).
  - **The full-film video's own detail page is always reachable by direct URL**, flag state and
    hiding notwithstanding — it holds the technical file metadata and is where the film page
    links out to. Hidden from lists ≠ inaccessible.
  - **Never write-destructive.** Toggling the flag off never deletes a `films`/`film_videos`
    row and never reverts an `Album`/`Title` value already written to a file — both are true
    even though the latter is counterintuitive for a "disable this feature" toggle, and the
    design handoff / release notes must say so plainly.
  - **Migrations run regardless of flag state** — the flag gates behavior, not schema.
  - **Default: `false`** (opt-in), matching `mcp_enabled`'s precedent — an unused, empty `/films`
    page and a newly-appearing nav entry is a worse first-run experience on an AMV-style library
    than films simply not existing until turned on. `holodex.yaml.example` documents
    `films_enabled: true` alongside `card_layout: poster` as the recommended pairing for a
    film-library operator.
- **RD7 — Writeback is a resolver source, not a bypass.** A film is a new *candidate source*
  competing for a video's `album` field (and additionally `title`, only for the video marked as
  that film's full-film file) under the existing `field_source_decisions` precedence model
  (ADR-051) — never a direct write from the film page that skips the resolver. A video attached
  to two films therefore presents two candidate values for one scalar field, which is a
  conflict the existing decision UI is built to show, not a last-writer-wins race. **The exact
  mechanism for naming/ordering multiple film candidates on one field is an open technical
  question for ADR-085**, not resolved here (see Open Questions). A scene file writes `Album`
  only; a full-film file writes `Title` **and** `Album`.
  - **Flag-off behavior**: disabling `films_enabled` must **suspend** the film source from
    resolution (it stops contributing candidate values and stops appearing in the decision UI)
    **without deleting any existing `field_source_decisions` row** that names it. Deleting on
    disable would re-resolve `album`/`title` to a different candidate while the file on disk
    still carries the value the film wrote — orphaning that value with nothing in the system able
    to explain it on the next scan. Re-enabling the flag must restore the decision exactly as it
    was, not re-derive it. The precise suspend mechanism (a status column on the decision row, a
    resolver-level source-availability check, or something else) is deferred to ADR-085.
- **RD8 — Film identity: `(name, year)`, not bare-`name` unique.** Unlike `studios.name UNIQUE`
  (migration 0017), film-name collisions across different years/releases are the common case,
  not the exception — `UNIQUE(name, year)` (or a synthetic identity key if `year` proves
  nullable-and-ambiguous at implementation) prevents the studio-style mistake of a single
  bare-name uniqueness constraint that would force artificial disambiguation on every remake or
  re-release.
- **RD9 — Two distinct attach pickers, not one shared modal.** Video→film and film→video attach
  are both in scope but are **not** the same interaction, because the search spaces differ by
  orders of magnitude:
  - **Video → film** (video detail page): small-scale name search over the film library (at
    most low hundreds of films), results by name/poster/year — the `EntityPickerDialog.svelte`
    shape (roving-tabindex list; see [keyboard-list convention](../../.claude/CLAUDE.md)), one
    film selected per action, with an optional scene-number field and the full-film toggle.
  - **Film → video** (film detail page): must search the entire video library (tens of
    thousands of files, frequently meaningless filenames). Requires a **default scope of
    unattached-only videos**, filters on the film's own studio/people to narrow candidates,
    free-text filename search, **multi-select**, and **sequential auto-numbering from a
    starting scene number** for bulk attach — a one-file-at-a-time modal does not scale to this
    picker's search space and would make the film page a secondary, rarely-used attach surface
    rather than the primary one. The film-side picker also flags when a candidate video is
    already attached to another film (legal, but usually accidental from this direction).

## User Stories

**Visitor — navigate by film**
- As a visitor, I want to click into a film and see its poster, description, cast, and scenes,
  so browsing by film works like browsing by person or studio.
- As a visitor, I want global search to match film names (when films are enabled), so typing a
  film's title finds the film, not just videos with that title in their filename.

**Owner — build a film from existing files**
- As the owner, I want to search my film library from a video's page and attach that video as a
  numbered (or unnumbered) scene, so I can build up a film's scene list one file at a time as I
  encounter it.
- As the owner, I want to search my entire video library from a film's page, filtered to files
  I haven't already attached anywhere and narrowed by that film's studio or cast, and attach
  several files at once with sequential scene numbers, so populating an 8-scene film doesn't
  take eight separate round trips.
- As the owner, I want to mark a specific file as representing the entire film (not a scene),
  so the film page knows which file(s) are eligible for a full Title+Album writeback and which
  are scene-only Album writes.
- As the owner, when I try to attach a video at a scene number another video already holds, I
  want a clear error naming the occupant, so I don't silently displace it.

**Owner — trust writeback and decisions**
- As the owner, when a video is attached to two films, I want the conflicting Album candidates
  to show up in the existing decision UI, so I resolve it explicitly instead of getting an
  unpredictable value.
- As the owner, when I turn `films_enabled` off and later back on, I want any Album/Title
  decision I made in favor of a film to come back exactly as I left it, not disappear or need
  to be re-made.
- As the owner, I want turning `films_enabled` off to never touch a file I've already written
  to, so disabling the feature is a safe, reversible UI change, not a data operation.

**Owner — curate a film like a person or studio**
- As the owner, I want the film's enriched fields (description, release date, poster) as the
  same source chips I use on videos/people/studios, so there is one editing model everywhere.
- As the owner, I want a person or studio's page to still show a film even when its only video
  is the full-film file (hidden from that page's regular video list), so I don't lose visibility
  into who's in a movie I own as a single file.

## Requirements

### Must-have (P0)

- **P0-1 — Schema.** `films(id, name, year, …enrichable columns)` with `UNIQUE(name, year)`
  (RD8); `film_videos(film_id, video_id, scene_number, is_full_film, PRIMARY KEY(film_id,
  video_id), UNIQUE(film_id, scene_number))` with `scene_number` nullable (RD5) and
  `is_full_film` a plain boolean flag set at attach time (never inferred); `films_fts` mirroring
  the studios/people FTS pattern (excluded from query results when `films_enabled=false`, RD6);
  a film poster/image table reusing the existing entity-image ingest pattern
  (person_images/studio_images shape), not a new pipeline.
- **P0-2 — No relink/prune participation (RD1).** Neither the general `RelinkVideoEntity`
  reconciliation nor any scan/enrich/decision/curation trigger touches `film_videos`. This must
  be explicit in the implementation (a missing negative test here is the single highest-risk
  gap in this feature) — a regression that accidentally wires films into that machinery would
  silently delete user-asserted attachments.
- **P0-3 — `films_enabled` config flag.** `internal/config`: `FilmsEnabled bool` (env
  `FILMS_ENABLED`), default `false`, following the `MCPEnabled`/`ThumbnailEnabled` pattern.
  Exposed to the SPA via the existing `capabilities` payload
  ([internal/api/auth.go](../../internal/api/auth.go)). When false: `/films*` routes are not
  registered (not merely hidden client-side), films excluded from FTS search results and from
  the MCP tool surface, and the film resolver source (P0-7) is suspended.
- **P0-4 — Video-list hiding while the flag is on (RD6).** A `film_videos` row with
  `is_full_film=true` removes that video from: browse, `RelatedShelf`, landing-page
  recently-added, global search results, and `EntityVideos.svelte` (person/studio/tag pages) —
  while `films_enabled` is true. The video's own detail page (`/media/{id}`) remains reachable
  by direct URL unconditionally.
- **P0-5 — Films row on Person/Studio/Tag detail pages (RD6 consequence).** Each of the three
  existing entity detail pages gains a films section (film name + poster thumb, linking to
  `/films/{id}`) surfacing every film that has at least one attached video carrying that
  person/studio/tag (via the resolved values of the film's attached videos) — shown whenever
  `films_enabled` is true and at least one film matches; absent otherwise.
- **P0-6 — Film API.** `GET /api/v1/films` (name+year list, poster thumb, scene counts),
  `GET /api/v1/films/{id}` (film + `resolved[]` + full-film file(s) + scenes list, each scene
  video's own basic fields + its scene number). Public reads (404 when `films_enabled=false`),
  mirroring people/studios.
- **P0-7 — Film resolver source for Album/Title (RD7).** A film's name becomes a candidate
  value for a linked video's `album` field (and `title`, only for that film's full-film-flagged
  video) inside the existing per-field decision precedence (`field_source_decisions`,
  ADR-051) — never a direct writeback bypass. Suspended (not deleted) when
  `films_enabled=false` (see RD7 for the orphaned-claim rationale). Exact multi-film-candidate
  mechanics: ADR-085 (Open Question).
- **P0-8 — Video → film attach (video detail page).** New affordance on `/media/{id}`: search
  films by name (results show name/poster/year, `EntityPickerDialog.svelte`-shaped), select
  one, optionally set a scene number, optionally mark "represents the entire film." Owner-gated.
  Number-collision rejection per RD5. A video's existing film attachments are visible and
  detachable from this same surface.
- **P0-9 — Film → video attach (film detail page), including bulk (RD9).** New affordance on
  `/films/{id}`: video search defaulting to unattached-only, filterable by the film's own
  studio/people and by filename, multi-select, bulk-attach with sequential auto-numbering from
  a supplied starting number, and a visible flag when a candidate is already attached elsewhere.
  Owner-gated.
- **P0-10 — Full-film file section on the film detail page (RD4).** A distinct list of the
  film's full-film-flagged videos, separate from the scenes list, each showing whether
  writeback is available (P0-11).
- **P0-11 — Writeback restricted to full-film files.** The writeback affordance appears on the
  film page for a video **only if** that video's `film_videos.is_full_film=true`; scene-only
  videos never get a film-page writeback control (Album still resolves/writes through the
  normal video-page writeback flow per P0-7).

### Should-have (P1)

- **P1-1 — Film enrichment.** A provider-facing film entity (candidate provider TBD at
  implementation, likely the existing TMDB movie endpoints already used elsewhere in the
  codebase) supplying description, release date, and poster candidates through the standard
  `entity_enrichment` shadow-store + decision-chip flow — the same pattern as Person/Studio
  enrichment, no new mechanism.
- **P1-2 — Multiple poster sizes/roles.** Following [studio-images.md](studio-images.md)'s
  icon/logo/poster role pattern: at minimum a portrait "poster" role (default aspect for the
  film list/detail header) plus a smaller list-thumbnail size — exact role set decided at
  implementation, reusing the existing image-role machinery rather than a new one.
- **P1-3 — `scene_count` completeness hint.** A scalar field from enrichment (e.g. provider
  reports 8 scenes) rendered as a light "3 of 8 owned" badge — no phantom-scene rows, no match
  flow, additive to P0-6. *(Explicitly deferred from Non-Goals scope; listed here only as the
  cheap version worth reconsidering as a fast-follow, not committed in this spec.)*

### Future considerations (P2)

- **P2-1 — Film identity operations (alias/merge).** Deferred per Non-Goals; revisit if
  duplicate-film pain materializes, mirroring the F23 pattern already used for person/tag.
- **P2-2 — MCP film mutation tools.** Read-only MCP exposure ships in P0-3; write tools (attach/
  detach via MCP) are a separate future slice.
- **P2-3 — Provider-known scene completeness view.** The full "phantom slot" scene list
  considered and rejected in the brainstorm (see Non-Goals) — revisit only if P1-3's cheap
  badge proves insufficient in practice.

## Behavior detail

### Asserted-link invariant (RD1/P0-2)
`film_videos` is written only by: the video-side attach endpoint, the film-side attach/bulk-
attach endpoint, and their corresponding detach endpoints. It is **never** touched by
`UpsertVideo` (scan), enrich completion, or any `field_source_decisions`/curation change — the
exact set of triggers that *do* drive `RelinkVideoStudios`/`RelinkVideoEntity` for studios and
people (ADR-053, ADR-072). This asymmetry is the spec's central architectural claim and must be
covered by an explicit regression test asserting that a full scan/enrich/decision cycle leaves
an existing film attachment byte-for-byte unchanged.

### Resolver source suspension (RD7)
When `films_enabled` flips false, the film source must stop being consulted as a resolver
candidate for `album`/`title` on every affected video, and must stop appearing as a selectable
option in the decision UI — but any `field_source_decisions` row already naming it as the
chosen source is left in place, untouched, so that flipping the flag back on restores the exact
prior resolution with no owner action required. The concrete implementation of "stop being
consulted without deleting the decision" is an ADR-085 design question, not resolved here.

### Video-list hiding (RD6/P0-4)
Hiding keys off `film_videos.is_full_film`, joined against `films_enabled`. A file is affected
if and only if it has at least one `film_videos` row with `is_full_film=true` (a file cannot be
hidden by inference from duration, filename, or provider data — RD1/Non-Goals). This makes the
hiding behavior fully reversible and auditable: flipping `films_enabled` off, or detaching the
film-videos row, immediately restores the video to every list with no other state change.

## API

```
GET    /api/v1/films                                          list + counts (public; 404-family when films_enabled=false)
GET    /api/v1/films/{id}                                     film + resolved[] + full-film file(s) + scenes (public)
POST   /api/v1/films/{id}/fields/{canonical}/decision          { source, manual_value? }   (requireOwner)
DELETE /api/v1/films/{id}/fields/{canonical}/decision                                       (requireOwner)
POST   /api/v1/films/{id}/curation[/clear]                     { field, value, action }     (requireOwner)
POST   /api/v1/films/{id}/enrich/search|apply|…                (P1-1, mirrors person/studio enrich; requireOwner)
POST   /api/v1/media/{id}/films                                { film_id, scene_number?, is_full_film? } attach (requireOwner)
DELETE /api/v1/media/{id}/films/{film_id}                                                    detach (requireOwner)
GET    /api/v1/films/{id}/video-candidates?…                   scoped/filtered search for the film-side picker (requireOwner)
POST   /api/v1/films/{id}/videos/bulk-attach                   { video_ids[], starting_scene_number? } (requireOwner)
POST   /api/v1/media/{id}/writeback                            unchanged; film-sourced Album/Title flow through existing resolution
```

`source ∈ { "file", "provider:<name>", "provider:film" | <film-candidate naming — ADR-085>,
"manual" }`. Errors mirror the person/studio endpoints.

## UI (grounded in real components)

- **`/films`**: the people/studios-list pattern — poster thumb (portrait) + name + year + scene
  count, following [studio-images.md](studio-images.md)'s role pattern for the thumb.
- **`/films/{id}`**: poster header (name, year, description, studio/people/tag chips inherited
  per RD2/RD3) · full-film file section (RD4, writeback control per P0-11) · scenes list (owned
  videos only, RD4) · film→video attach affordance (P0-9, `EntityPickerDialog.svelte`-derived
  but scoped/filtered/multi-select per RD9) · Details section on `SourceSelect`/
  `CurationFieldRow` chips for enrichable fields, matching the Person/Studio Details pattern.
- **`/media/{id}`**: new "Films" section listing current attachments (detachable) + video→film
  attach affordance (P0-8, `EntityPickerDialog.svelte` shape per RD9).
- **`/people/{id}`, `/studios/{id}`, `/tags/{id}`**: new films row (P0-5), shown only when
  `films_enabled` is true and at least one film matches.
- **Global search** and **browse**: films appear as their own result group when
  `films_enabled` is true (mirroring the people/tags/studios FTS group pattern); full-film
  files never appear as browse/search video hits while the flag is on (RD6).
- Tokens only; QA Cinémathèque / Broadcast / Brutalist, per every entity page in this codebase.

## Success Metrics

Single-owner feature (no adoption funnel — success is architectural correctness):
- **Leading:** a full scan/enrich cycle run against a library with existing film attachments
  leaves every `film_videos` row unchanged (RD1 regression test, P0-2) — the derived-link
  machinery never touches films.
- **Leading:** toggling `films_enabled` off and back on restores every prior Album/Title
  decision exactly, with zero re-decisions required and zero files rewritten (RD6/RD7).
- **Leading:** a full-film-only person/studio (all their videos are full-film files) still
  shows a non-empty page via the new films row when the flag is on (RD6 anti-dead-end check).
- **Leading:** the film-side attach picker completes a multi-file bulk attach with sequential
  numbering in one action, verified against a library with 10k+ videos (RD9 — the picker this
  spec is built around).
- **Lagging:** P2-1 (alias/merge) stays unneeded until real duplicate-film pain appears — i.e.
  RD8's `(name, year)` cut was sufficient.

## Open Questions

- **Q1 (engineering, blocking for ADR-085):** the exact mechanism by which a video attached to
  **multiple** films presents multiple candidate values to one scalar `album`/`title` field
  under `field_source_decisions`. Candidates include: a per-film-id source string
  (`provider:film:<id>`, mirroring how the source vocabulary already parameterizes
  `provider:<name>`), or a single `provider:film` source whose value the resolver picks via a
  secondary tie-break (e.g. most-recently-attached) with the decision UI surfacing all
  attached-film candidates individually. This governs both the `field_source_decisions.source`
  string format and the decision-UI candidate list — must be locked in ADR-085 before P0-7
  implementation starts.
- **Q2 (engineering, blocking for ADR-085):** the concrete "suspend, don't delete" mechanism for
  `field_source_decisions` rows referencing a film source when `films_enabled=false` (RD7) — a
  status/availability column on the decision row, a resolver-level source-availability check
  against the live flag, or something else. Must produce identical before/after state across a
  flag-off/flag-on cycle (Success Metrics, leading #2).
- **Q3 (engineering, non-blocking):** provider selection for P1-1 film enrichment — reuse the
  existing TMDB movie-metadata surface already integrated elsewhere in the codebase, or treat
  film enrichment as provider-agnostic from day one. Either satisfies P1-1; pick at
  implementation.
- **Q4 (design, non-blocking):** exact visual treatment of the films row on person/studio/tag
  pages (P0-5) — inline chip row vs. a dedicated section with its own heading. Resolve in
  `/design-handoff`.

## Timeline / routing

No hard deadline. Per the change-routing rules, before/with implementation:
1. ⬜ **`/architecture`** — ADR-085: asserted-link data model (RD1/RD5/RD8), the film
   resolver-source mechanism (RD7, Q1/Q2), and the `films_enabled` suspend semantics (RD6).
2. ⬜ **`/design-handoff`** — films list/detail layout, the two attach pickers (RD9), the
   two-region film detail page (RD4), the new films row on three existing pages (RD6/P0-5),
   3-skin QA.
3. ⬜ **`/testing-strategy`** — the RD1 non-participation regression test (P0-2, highest
   priority), flag-toggle round-trip idempotency (RD6/RD7), scene-number collision handling
   (RD5), multi-film-candidate resolution (Q1), video-list hiding correctness across all five
   surfaces (P0-4), bulk-attach at scale (RD9), endpoint auth, FTS/MCP exclusion when disabled.
4. ⬜ **`/security-review`** — new owner-gated mutation surface (attach/detach/bulk-attach,
   decisions, curation, enrich), the film-side video-candidate search endpoint (ensure it
   respects the same access model as browse, no data leakage of soft-deleted/hidden videos),
   provider data through the existing sanitize/asset perimeter for enrichment.

Slices: **S1** backend (schema, asserted-link guarantee, config flag, resolver source + Q1/Q2
mechanism, film API) ∥ **S2** frontend (pages, both attach pickers, films rows, video-list
hiding) → **S3** enrichment (P1-1/P1-2, cuttable) → **S4** QA + security.
Effort: **XL** (new entity + new resolver-precedence mechanism + cross-cutting UI changes on
three existing pages — larger than F38/studio's L).
