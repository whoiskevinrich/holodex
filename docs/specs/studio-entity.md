# Spec: Studio as a first-class entity (F38)

**Status**: Draft
**Phase**: F36 fast-follow ③ (Jira [HOLODEX-11](https://whoiskevinrich.atlassian.net/browse/HOLODEX-11), epic HOLODEX-4)
**Owner**: Project owner
**Date**: 2026-07-01
**Feature block**: **F38** — promote `studio` from a resolved video field to a first-class
entity: `studios` table, links derived from the resolved field, FTS, list + detail pages, an
entity-backed browse facet — and inherit the F36 decision model as the **third** entity (after
`video` and `person`), exactly per the shape F37 proved.

**Depends on** (all shipped):
- the F36 decision model ([ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md), migration 0016 `field_source_decisions` — keyed by `entity_type`)
- the entity-agnostic resolver ([ADR-052](../architecture/ADR-052-baseline-source-contract.md), `BaselineSource` + `ResolveFields`) — fast-follow ①
- the F37 people refactor ([people-source-of-truth.md](people-source-of-truth.md)) — `personBaseline` precedent (`internal/resolver/person_baseline.go`), the person decision/curation endpoints (`internal/api/person_fields.go`), the `·record` baseline label, the no-writeback shape
- the source-chip control ([F36 handoff](../design/field-source-of-truth-handoff.md), `SourceSelect`/`CurationChip`, `web/src/lib/f36.ts`)
- video enrichment (F26/F27; TMDB emits multi-valued `studio` from `production_companies` — `providers/tmdb/tmdb.go` ~line 478)
- the owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md), `requireOwner`)

**ADR**: [ADR-053](../architecture/ADR-053-studio-entity-and-resolved-link-derivation.md)
(Proposed) records the two decisions that rise to ADR level: (1) the `studios` /
`video_studios` data model, and (2) the **link-derivation rule** — entity links follow the
*resolved* field value (RD1), a new pattern vs. person's scan-time-only links. Extends
ADR-051 §9 / ADR-052; relates ADR-013 (mapping), ADR-017 (FTS), ADR-036 (the alias routing
deliberately *not* adopted in v1). Touches **access** (new owner-gated endpoints) →
`/security-review` before merge.

**Implementation design**: [studio-entity-implementation.md](../plans/studio-entity-implementation.md)
(component map, the `RelinkVideoStudios` reconcile algorithm, per-trigger sequences, API↔handler
mapping). **Design handoff**: [studio-entity-handoff.md](../design/studio-entity-handoff.md)
(pages, `·record` baseline, media-detail link, facet switch, 3-skin QA).

**Related**: [F32 video credits](video-credits-people.md) (parallel; both add entity links to
videos — keep the join-table idioms consistent), HOLODEX-118 (provider trust order —
independent).

---

## Problem Statement

`studio` is a string that never becomes a *thing*. The media page resolves it beautifully
(file Publisher/Label baseline, TMDB candidates, F36 decisions) — and then it dead-ends:
there is no studio page, no "everything from this studio" navigation, and the browse facet
is a string-match over raw `video_metadata` file values (`?studio=Acme`,
`internal/api/handlers.go` ~line 283) — so a video whose studio decision adopts the TMDB
value **displays** one studio but **filters** under another (or none). People and tags got
identity, pages, and search years ago; studio is the last browse-grade concept still living
as a loose string, and the F36 "model must not foreclose Studio" promise stays half-kept
until it rides the decision model as a real entity.

## Goals

1. **Studio becomes navigable identity** — a `studios` table, `/studios` list and
   `/studios/{id}` detail pages, global-search hits via FTS, and click-through from every
   place a studio name renders (media detail, browse facet).
2. **Links agree with what the owner sees (fixes the facet split)** — the video → studio
   link derives from the **resolved** `studio` field, so an adopted provider/custom value
   re-links the video and the facet groups match the displayed values, decisions included.
3. **Third entity on the decision model, zero core diffs** — `studio` flows through
   `ResolveFields` via a `studioBaseline` with no changes to the resolver core, the decision
   store, or the chip components (the F37 proof, repeated).
4. **A studio page worth curating** — TMDB company enrichment (description, country,
   homepage, logo) gives the chips something to adopt/suppress/correct (cuttable slice, RD3).

## Non-Goals

- **Studio identity operations (rename / aliases / merge)** — deferred wholesale (RD4). A
  studio's name is *derived* from field values, so the correction path is fixing the field on
  the videos (decision / custom value) and letting the links follow. F23-style aliases+merge
  for studios is a tracked P2. *(Why: alias-less rename resurrects the old name at the next
  derivation; doing routing properly is F23-sized and shouldn't grow this story.)*
- **Multi-studio as a product concept** — the canonical `studio` field keeps its current
  mapping semantics (single-valued unless an operator maps it multi). The join table (RD2) is
  schema headroom, not a UI for multiple studios per video.
- **Studio writeback / sync state** — a studio has no file; `in_sync` is **omitted** and
  there is no write button (same rationale as F37).
- **Studio photos / logo galleries / owner-uploaded logos** — ~~the *storage + serving* of the
  single logo IS now hardened (self-hosted, normalized; [ADR-057](../architecture/ADR-057-self-hosted-studio-logo.md),
  HOLODEX-130), but the rest of the F25 person-image pipeline (owner upload, delete-suppression,
  galleries, content-hash dedup, promote/reorder) is **not** generalized to studios — a studio
  has one logo, curated only via the existing provider/blank-pin decision.~~ *(Why: a studio has
  no upload UI and one image slot; cloning the multi-role subsystem would be mostly-dead surface.)*
  **Reversed by [F51](studio-images.md) / [ADR-079](../architecture/ADR-079-studio-image-roles.md)
  (2026-08-03):** studios gain three owner-editable image roles (icon/logo/poster) with upload +
  an ADR-049-style provenance lock, superseding ADR-057. Galleries and content-hash dedup remain
  out of scope — every role stays single-slot.
- **MCP parity** — `search_videos` keeps the mapped `{"studio": "Acme"}` filter; exposing
  studio entities over MCP rides the deferred MCP-parity item (F22.5f).
- **Backfilling historical scan data** — links derive from current resolved values (see
  Behavior); no attempt to reconstruct what a studio "was" before promotion.

## Resolved Decisions

*(Locked with the owner 2026-07-01 via question cards.)*

- **RD1 — Links follow the resolved value.** The video → studio link is derived from the
  video's **resolved** `studio` field (decision short-circuit included) and re-derived on
  scan upsert, enrich completion, studio-field decision set/clear, and studio-field curation
  change. Display and grouping can never disagree. Empty resolved value ⇒ no link.
- **RD2 — Join table.** `video_studios(video_id, studio_id)` mirroring `video_people`
  (composite PK, cascade deletes, secondary index on `studio_id`). V1 writes one row per
  distinct value of the resolved field — one for today's single-valued mapping, *n* if an
  operator maps `studio` as multi. No re-migration if multi-studio becomes real.
- **RD3 — TMDB company enrichment in scope, as a cuttable slice.** The TMDB provider gains a
  `studio` entity (match via `/search/company`, enrich via `/company/{id}`: description,
  origin country, homepage, logo asset). `entity_enrichment`, the resolver, and the chips are
  already entity-generic, so this is provider-side + registry work. If it drags, it splits to
  a follow-up issue without changing anything else in this spec.
- **RD4 — Identity ops deferred.** Exact-name resolve-or-create (`studios.name` UNIQUE,
  trimmed; no case-folding beyond what people do today). No rename, no aliases, no merge in
  v1; P2 mirrors F23 if duplicate-name pain materializes.
- **RD5 — Decision-model inheritance is the F37 shape verbatim.** `studioBaseline` with
  `name` as the only baseline-backed field; `·record` label; `—` record chip when empty;
  decision/curation endpoints mirror the person ones; **`name` decisions rejected (400)** —
  and unlike persons there is no rename materialization either (RD4): the name row renders
  as a plain read-only value in v1, not chips.
- **RD6 — External-id de-dup converges same-company spellings** (HOLODEX-122, [ADR-054](../architecture/ADR-054-studio-external-id-dedup.md)).
  The TMDB `production_companies[].id` (today discarded) is captured, stored in a
  `studio_external_ids(external_id PK, studio_id)` join table, and consulted **before** exact
  name in resolve-or-create — so "Warner Bros." and "Warner Bros. Pictures" that share TMDB id
  `174` converge to **one** studio entity. This **refines RD1**: when a provider id proves two
  spellings are one company, both videos link to a single entity carrying **one** canonical name
  (the first spelling to create it — deterministic, never renamed on later re-derivation). The
  common single-spelling case is unchanged (entity name == field value, RD1 verbatim); the
  divergence appears only in the dedup scenario, where converging is the goal. Full-fidelity
  (surfacing both spellings) is P2-1 aliases. The id also makes studio re-enrich (P1-1/S3)
  deterministic and lets a video hint its studio's provider identity.

## User Stories

**Visitor — navigate by studio**
- As a visitor, I want to click the studio on a video and see everything else from that
  studio, so browsing by studio works like browsing by person or tag.
- As a visitor, I want global search to match studio names, so typing "Ghibli" finds the
  studio (and its videos), not just videos with "Ghibli" in the title.

**Owner — trust the facet**
- As the owner, when I adopt TMDB's studio spelling on a video, I want the browse facet and
  the studio page to group that video under the adopted name, so display and navigation never
  tell different stories.
- As the owner, I want a misspelled file-tag studio fixed by one custom-value decision on the
  video, and the bogus studio entity to disappear once nothing links to it.

**Owner — curate a studio like a person**
- As the owner, I want the studio page's enriched fields (description, country, homepage) as
  the same source chips I use on videos and people, so there is one editing model everywhere.
- As the owner, I want to pin a studio's description blank even though the provider has one,
  so a re-enrich never resurrects it.

## Requirements

### Must-have (P0)

- **P0-1 — Schema (migration 0017).** `studios(id, name UNIQUE)` + `video_studios(video_id,
  studio_id, PK(video_id, studio_id), idx on studio_id)` + `studios_fts` (unicode61,
  remove_diacritics 2) with insert/update/delete triggers — the 0001 people pattern verbatim.
- **P0-2 — Link derivation (RD1/RD2).** A single repo/service helper (`RelinkVideoStudios`)
  that resolves the video's `studio` field and reconciles `video_studios` rows
  (resolve-or-create by exact trimmed name; drop stale links), invoked from: scan upsert,
  enrich completion, `studio` decision PUT/DELETE, `studio` curation change.
  - Given a video whose file Publisher is "Acme" and no decision, When scanned, Then it links
    to studio "Acme".
  - Given the owner adopts the TMDB value "Acme Films", Then the link moves to "Acme Films"
    within the same request cycle (no rescan needed).
  - Given the resolved value becomes empty (blank pin on an empty baseline), Then the video
    has no studio link.
  - Given a soft-deleted video, Then it stops counting toward (and listing under) its studio,
    consistent with people/tags behavior on soft delete.
- **P0-3 — One-time backfill.** After migration 0017, existing videos get links without a
  manual rescan: a startup backfill pass derives links for all active videos, recorded as a
  job run in System Activity (F21) so it's observable and idempotent (no-op when nothing to
  do).
- **P0-4 — Studio API.**
  `GET /api/v1/studios` (name-sorted list + active-video counts; empty studios — zero active
  links — are pruned at relink time, so the list never shows them),
  `GET /api/v1/studios/{id}` (studio + `resolved[]` (no `in_sync`) + its videos, paged like
  the person page). Public reads, mirroring people/tags.
- **P0-5 — Studio decision/curation endpoints (RD5).**
  `PUT/DELETE /api/v1/studios/{id}/fields/{canonical}/decision` and
  `POST /api/v1/studios/{id}/curation[/clear]`, `requireOwner`, exact person-endpoint
  semantics (`internal/api/person_fields.go` parity): replace fields only for decisions,
  provider-must-be-matched, sanitized `manual_value`, 404 unknown studio/field, 400 bad
  source, **`name` → 400**.
- **P0-6 — Pages.** `/studios` (list, counts) and `/studios/{id}` (name header, video grid,
  Details section on `SourceSelect`/`CurationFieldRow` chips with `·record` baseline per RD5
  — rendered only when enrichment/decisions exist, so the pre-enrichment page is just name +
  videos). Owner-gated mutations; visitors read-only. Tokens only; QA all three skins.
- **P0-7 — Entity-backed facet + links.** The browse studio facet lists studio entities with
  counts and filters via `?studio_id={id}`; facet entries and the media-detail studio value
  link to `/studios/{id}`. The legacy mapped string filter (`?studio=Acme`, MCP `fields`)
  keeps working unchanged (it filters raw file values; documented as such).
- **P0-8 — Global search.** Studio FTS hits appear in global search results as a studio
  entity group (like people/tags), linking to the studio page.

### Should-have (P1)

- **P1-1 — TMDB company enrichment (RD3).** Provider `studio` entity: `/describe` advertises
  it; match = `/search/company` candidates (name + logo thumb + country as disambiguation);
  enrich = `/company/{id}` → `description`, `country`, `homepage` (canonical `website`),
  `logo` asset (`image.tmdb.org`, existing `asset_hosts` perimeter). Registry gains the
  studio field defs it lacks (reuse `website`; add `description`/`country` or reuse
  `bio`/`nationality` — engineering picks, registry stays flat). Owner-gated
  `/studios/{id}/enrich/*` mirroring the person enrich endpoints, including enrich-runs in
  System Activity.
- **P1-2 — Recently-added / landing parity.** Studio names on video cards (where the card
  layout shows them) become links.
- **P1-3 — Studio external-id de-dup (RD6, HOLODEX-122, [ADR-054](../architecture/ADR-054-studio-external-id-dedup.md)).**
  Capture the TMDB `production_companies[].id`, persist it, and de-dup studios by it.
  - **Capture.** The TMDB movie mapping decodes `production_companies[].id` and emits a
    self-describing **internal sidecar** field `_studio_external_ids` = `"<ns>:<id> <name>"` per
    named, positive-id company (`tmdb:174 Warner Bros. Pictures`). `_`-prefixed field-keys are
    provider→core plumbing: persisted in `entity_enrichment` unchanged but **never displayed**
    (`FieldsFromRows` skips them) and **never resolved** (not a mapped canonical field).
  - **Model.** `studio_external_ids(external_id PK, studio_id FK ON DELETE CASCADE)` — `external_id`
    globally unique (one company id → one studio, the dedup key); a studio may carry *n* ids
    (multi-provider headroom); pruned with its studio (prune-on-empty cascades).
  - **Thread.** `RelinkVideoStudios` parses the video's `_studio_external_ids` rows into a
    name→external_id side-map (keyed by name, robust to resolver reordering/curation) and passes it
    to `ReconcileVideoStudios`; a custom/decided name with no company match falls back to name-only.
  - **Resolve.** `resolveOrCreateStudio` matches `external_id` first, then exact name;
    `INSERT OR IGNORE` attaches/back-fills the id (including onto a name-created studio).
  - Given two videos whose resolved studio is "Warner Bros." and "Warner Bros. Pictures" and both
    TMDB responses carry company id `174`, When both are derived, Then `GET /studios` shows **one**
    studio (not two) and both videos link to it.
  - Given a custom-value decision "My Home Movies" with no company id, When derived, Then it
    resolves by name exactly as today (no id row).
  - Given a studio's last video is removed, Then the studio **and** its `studio_external_ids` rows
    are pruned; a later video carrying the same id re-creates and re-attaches deterministically.
  - **No new provider host/asset/SSRF surface** (a field already in the `/movie/{id}` response) and
    no media-file write (studios have no file).

### Future considerations (P2)

- **P2-1 — Studio aliases + merge** (F23 shape: routing at derivation time, never auto-merge
  same-name). Becomes worthwhile the first time two spellings of one real studio both carry
  real libraries. *(The **deterministic** counterpart — dedup by provider id rather than by
  name — is P2-5.)*
- **P2-2 — Multi-studio UI** if the `studio` field is ever promoted to a merge field
  (schema already permits it, RD2).
- **P2-3 — Studio logo in the image store** (F25 generalization) — realized in
  [ADR-057](../architecture/ADR-057-self-hosted-studio-logo.md)
  ([HOLODEX-130](https://whoiskevinrich.atlassian.net/browse/HOLODEX-130)) as a self-hosted,
  normalized cache **derived from the resolved `logo` field** (`studio_logos`, migration 0020).
  **Superseded** by [F51](studio-images.md) / [ADR-079](../architecture/ADR-079-studio-image-roles.md)
  (2026-08-03): the logo (plus new icon/poster roles) moves onto the Person-style asset-slot
  model with owner upload, and `studio_logos` is replaced by `studio_images`.
- **P2-4 — MCP studio entities** (rides F22.5f).
- **P2-5 — Studio external-id dedup** — **promoted to P1-3 above** and decided in
  [ADR-054](../architecture/ADR-054-studio-external-id-dedup.md) ([HOLODEX-122](https://whoiskevinrich.atlassian.net/browse/HOLODEX-122)).
  Companion to the S3 enrichment slice ([HOLODEX-121](https://whoiskevinrich.atlassian.net/browse/HOLODEX-121)).

## Behavior detail

### Link derivation (RD1)
`RelinkVideoStudios(videoID)`: resolve the video's fields (existing media-detail path),
take the `studio` resolved value(s) — for a multi-mapped field, every value; else the single
value — trim, drop empties, resolve-or-create each name in `studios`, replace the video's
`video_studios` rows with exactly that set, and delete any studio left with zero links
(prune-on-empty keeps RD4 honest: bogus names die when the last video is fixed). All inside
one transaction. Call sites: `UpsertVideo` (scan), enrich completion, decision PUT/DELETE for
`studio`, curation add/suppress/clear for `studio`. Derivation never runs at read time.

### Resolution (studio entity fields)
Identical to F37 with the baseline swapped: `studioBaseline` serves `name` only; every other
canonical studio field resolves an empty baseline, so undecided enrichment fields keep
resolving to the provider value (the F37 RD6 additivity property — tests assert it). The
decision `source` vocabulary is the person one: API/UI say `record`, storage stays the
entity-generic column (whichever of the two F37 Q1 options engineering picked — follow it).

### Facet (P0-7)
The browse facet's studio block switches from `FacetValues` over mapped source keys to
`studios` join counts (active, non-deleted videos). `?studio_id=` is a plain equality join
filter combinable with existing filters/sort. The mapped `?studio=` string filter is
untouched for back-compat (REST + MCP) and may drift from entity grouping by design (raw
file values; the facet UI no longer generates it).

## API

```
GET    /api/v1/studios                                       list + counts (public)
GET    /api/v1/studios/{id}                                  studio + resolved[] + videos (public; no in_sync)
PUT    /api/v1/studios/{id}/fields/{canonical}/decision      { source, manual_value? }   (requireOwner; name → 400)
DELETE /api/v1/studios/{id}/fields/{canonical}/decision                                  (requireOwner)
POST   /api/v1/studios/{id}/curation                         { field, value, action }    (requireOwner)
POST   /api/v1/studios/{id}/curation/clear                   { field, value, action }    (requireOwner)
POST   /api/v1/studios/{id}/enrich/search|apply|…            (P1-1, mirrors person enrich; requireOwner)
GET    /api/v1/media?studio_id={id}                          entity filter (public)
```

`source ∈ { "record", "provider:<name>", "manual" }`. Errors mirror the person endpoints.

## UI (grounded in real components)

- **`/studios`**: the people-list pattern (name + count cards/rows — match whatever
  `/people/+page.svelte` does today, minus headshots; no logo until P1-1 enrichment).
- **`/studios/{id}`**: name header · video grid (existing grid component + paging) · Details
  section on chips (`SourceSelect` replace rows, `·record`, `—` empty-record chip; hidden
  until there's anything beyond `name` to show). Enrich picker (P1-1) reuses `EnrichPicker`
  with company candidates.
- **Media detail**: the resolved studio value renders as a link to its studio entity (the
  link target comes from `video_studios`, so it always matches the displayed value per RD1).
- **Browse facet**: studio entries = entity name + count, click → filtered browse; a
  secondary affordance links to the studio page.
- Tokens only; QA Cinémathèque / Broadcast / Brutalist.

## Success Metrics

Single-owner consistency feature:
- **Leading:** adopting a provider/custom studio value moves the video between studio pages
  and facet groups with no rescan (tests + manual QA) — the facet split is dead.
- **Leading:** zero resolver-core diffs in the implementing PR (entity-genericity proven a
  third time).
- **Leading:** fixing the last misspelled video removes the misspelled studio from
  `/studios` and the facet (prune-on-empty verified).
- **Lagging:** P2-1 (aliases/merge) stays unneeded until real duplicate-name pain appears —
  i.e. RD4 was the right cut.

## Open Questions

- **Q1 (engineering, non-blocking):** registry naming for the enrichment fields (P1-1) —
  reuse `bio`/`nationality`/`website` for description/country/homepage vs. add
  studio-specific keys. Pick at implementation; the registry is flat either way.
- **Q2 (engineering, non-blocking):** backfill trigger mechanics (P0-3) — dedicated startup
  pass vs. piggybacking the first scan's upsert path with a migration flag. Either satisfies
  the acceptance criteria; pick during implementation and record it in the job run.

## Timeline / routing

No hard deadline. Per the change-routing rules, before/with implementation:
1. ✅ **`/architecture`** — [ADR-053](../architecture/ADR-053-studio-entity-and-resolved-link-derivation.md)
   (data model + resolved-value link derivation, RD1/RD2) + the
   [implementation design](../plans/studio-entity-implementation.md).
2. ✅ **`/design-handoff`** — [studio-entity-handoff.md](../design/studio-entity-handoff.md):
   studios list/detail layout, facet block, media-detail link treatment, empty-Details rule,
   3-skin QA.
3. ✅ **`/testing-strategy`** — derivation matrix (scan/enrich/decision/curation ×
   link outcomes), prune-on-empty, backfill idempotency, `studioBaseline` additivity,
   endpoint auth, facet counts vs. soft delete, FTS triggers, chips a11y/3-skin (S1), plus the
   **S3 TMDB company enrichment** coverage (provider/service/endpoint/registry/frontend).
4. ✅ **`/security-review`** — new owner-gated surface (decisions/curation/enrich parity;
   untrusted provider company data through the existing sanitize + asset perimeter; no file
   writes anywhere in F38). **S3 sign-off (2026-07-03): clean** — enrich routes owner-gated,
   provider data through the existing `sanitizeFields`/`SanitizeValue` perimeter, `logo` a
   client-rendered `image_url` field (no server-side fetch; asset download stays person-gated),
   `website`/`logo` render via the existing scheme-gated `UrlValueList`/`<img>` patterns.

Slices: **S1** backend (migration 0017, derivation + backfill, studio API, `studioBaseline`,
decision/curation endpoints) ∥ **S2** frontend (pages, facet, links — against the frozen
payload) → **S3** TMDB company enrichment (P1-1, cuttable) → **S4** QA + security.
Effort: L (per HOLODEX-11).
