# Spec: Studio image roles — icon / logo / poster (F51)

**Status**: Draft
**Phase**: F51 (Jira [HOLODEX-247](https://whoiskevinrich.atlassian.net/browse/HOLODEX-247), epic [HOLODEX-246](https://whoiskevinrich.atlassian.net/browse/HOLODEX-246))
**Owner**: Project owner
**Date**: 2026-08-03
**Feature block**: **F51** — generalize Studio from one self-hosted `logo` cache to three
named image roles (`icon`, `logo`, `poster`), each independently sourced from enrichment
**or** owner upload, with add/edit/remove controls on the studio detail page. Realizes the
Option D deferred by [ADR-057](../architecture/ADR-057-self-hosted-studio-logo.md) — moving
the studio logo off the field/decision model and onto the Person-style image-asset model.

**Depends on** (all shipped):
- the on-disk image store + normalize spine ([ADR-038](../architecture/ADR-038-person-images.md), `internal/personimage`, migration 0009)
- the provenance-lock precedent ([ADR-049](../architecture/ADR-049-manual-image-precedence.md)) — enrichment never overwrites an owner-set core slot
- the provider asset perimeter ([ADR-039](../architecture/ADR-039-provider-asset-urls.md), `AssetClient`, `asset_hosts` allowlist)
- the studio entity + its current single logo cache ([ADR-053](../architecture/ADR-053-studio-entity-and-resolved-link-derivation.md), [ADR-057](../architecture/ADR-057-self-hosted-studio-logo.md), migrations 0017/0020) — **this spec replaces migration 0020's `studio_logos`**
- TMDB company enrichment ([docs/specs/studio-entity.md](studio-entity.md) P1-1; `providers/tmdb/tmdb.go`, currently emits `fields["logo"]`)

**ADR**: A new ADR (next available number) records: (1) the `studio_images` table
(single-slot-per-role, no gallery), (2) generalizing `enrich.ImageSink`/`downloadAssets`
to be entity-generic (`entityType` + `entityID`) rather than person-only, and (3)
retiring the studio `logo` **field** (registry + ADR-051 decision) in favor of the
asset-slot model. **Supersedes [ADR-057](../architecture/ADR-057-self-hosted-studio-logo.md)** (the derived-cache-over-a-field approach) and realizes its own deferred
"Option D". Touches **access** (new owner-gated upload/delete endpoints) and a new
untrusted-bytes ingestion path (owner file upload) → `/security-review` before merge.

**Related**: [Person images / F25](people-images.md) (the pattern this generalizes),
[Studio as a first-class entity / F38](studio-entity.md) (Non-Goal "Studio photos / logo
galleries / owner-uploaded logos" — **reversed by this spec**).

---

## Problem Statement

A studio has exactly one image today: a single, enrichment-only `logo`, hotlink-hardened
by ADR-057 but with no owner override and no second image for a different purpose. The
`/studios` list, the `/studios/{id}` detail header, and any future studio-forward surface
(e.g. a poster-led browse card, the way Person's hero already works) all have to share one
image sized/cropped for whichever context asked for it first. Meanwhile Person already
proved a three-role image model (headshot/banner/poster) with owner upload and an
enrichment-precedence rule — Studio has none of that, and its owner has no way to fix a
wrong or missing provider logo short of pinning the field blank and losing the image
entirely.

## Goals

1. **Three named, independently-managed image roles** — `icon` (studios list), `logo`
   (studio detail header, today's usage), `poster` (no consumer yet, but a real, fully
   editable slot so a future feature needs no backend work).
2. **Owner control, not just provider display.** The detail page can add, replace, and
   remove each role's image directly — the same operations Person's core roles already
   support, without cloning Person's gallery/promote/dedup machinery Studio doesn't need.
3. **Manual images survive enrichment**, exactly like Person (ADR-049): once the owner
   sets a role by hand, a re-enrich never silently replaces it.
4. **Reuse, not a parallel subsystem.** The on-disk store, normalize/bomb-guard spine, and
   SSRF asset perimeter are shared with Person unchanged; only the orchestration layer
   (`enrich.ImageSink`/`downloadAssets`) generalizes from person-only to entity-generic,
   proven at its second real use (the codebase's established bar for generalizing a
   subsystem, e.g. `BaselineSource`/ADR-052 at Person's second use, name-identity/ADR-061
   at its third).

## Non-Goals

- **Gallery, promote, reorder, content-hash dedup for studios.** All three studio roles
  are core/single-slot; there is no `extra` role and nothing to promote from. *(Why: no
  studio use case has ever asked for more than one image per role — cloning that machinery
  would be dead surface, the same call ADR-057 already made.)*
- **TMDB actually supplying `icon` or `poster` images.** The company enrichment response
  has one image today (the logo). This spec makes the pipeline provider-kind-generic —
  any future provider asset kind mapped to `icon`/`poster` flows in with no further schema
  change — but does not add a new TMDB field or a second provider. *(Why: no data source
  exists yet; shipping the generic pipeline now avoids a second migration later.)*
- **A studio image gallery / multiple photos per role.** Same rationale as Person's core
  roles: one icon, one logo, one poster.
- **Rewriting Person's `ImageSink`/`downloadAssets` behavior.** Generalizing the
  orchestration to take an entity type must be behavior-preserving for Person — this is a
  parameter-widening, not a rewrite (mirrors the ADR-052 `BaselineSource` cut: `Resolve`
  became a thin wrapper over the generic core with zero behavior change).
- **Migrating existing `logo` field decisions (ADR-051) into upload/lock state.** A studio
  whose `logo` field previously had a provider-pin or blank-pin decision loses that
  decision's effect (the field disappears from `resolved[]`); the owner re-curates via the
  new upload/remove controls if the default enrichment pick is wrong. *(Why: translating "I
  pinned provider X's logo" into an equivalent upload/lock has no clean 1:1 mapping — the
  bytes were never captured under an ADR-051 field decision, only a URL choice.)*

## Resolved Decisions

*(Locked with the owner 2026-08-03 via question cards.)*

- **RD1 — All three roles ship with full add/edit/remove UI now**, not just icon/logo with
  poster deferred. A single role-generic control renders for `icon`/`logo`/`poster` on the
  detail page; poster simply has no other consumer yet.
- **RD2 — Provenance-lock precedence (ADR-049 pattern), not a per-field decision chip.**
  Once a role's slot is `source='upload'`, enrichment skips it (fills empty slots, refreshes
  its own `source='enrichment'` rows, never touches a locked one). No F36/ADR-051 decision
  UI for images — this mirrors Person's core-role behavior exactly, not the text-field
  chip model.
- **RD3 — `studio_images` replaces `studio_logos` outright**, not a coexisting second
  table. Migration 0036 carries forward existing `studio_logos` rows into
  `studio_images(role='logo', source='enrichment')` and drops `studio_logos`. Serving moves
  from `GET /studios/{id}/logo` to `GET /studios/{id}/images/{role}`.
- **RD4 — The `logo` **field** (registry, ADR-051 decision) is retired**, not kept
  alongside the new asset slot. TMDB's company enrichment switches from
  `Fields["logo"] = url` to `Assets: [{Kind: "logo", URL: url}]` — logo becomes an asset
  like Person's headshot, not a resolved text/URL field. Existing `logo` field decisions
  (if any) become inert; the migration also deletes any `field_source_decisions` rows for
  `entity_type='studio', field_key='logo'` as dead weight.

## User Stories

**Visitor — recognize a studio at a glance**
- As a visitor, I want the studios list to show a small icon per studio, so I can scan the
  list visually instead of by name alone.
- As a visitor, I want the studio detail page to show its logo prominently, exactly as it
  does today.

**Owner — correct or replace a studio's image**
- As the owner, I want to upload my own icon/logo/poster for a studio when the provider's
  version is wrong, missing, or just not what I want, so the studio page reflects my
  library the way I curate it.
- As the owner, when I've uploaded my own image for a role, I want re-enriching that
  studio to leave it alone, so I don't lose a manual fix by refreshing metadata.
- As the owner, I want to delete an image I set and have the next enrich fill it back in
  from the provider, so undoing a manual override doesn't require re-uploading the
  original.

## Requirements

### Must-have (P0)

- **P0-1 — Schema (migration 0036).** `studio_images(id, studio_id, role, source,
  provider, external_id, width, height, byte_size, created_at)`, `UNIQUE(studio_id, role)`
  (every role is core — no partial index needed, unlike Person), `ON DELETE CASCADE`.
  The migration carries forward `studio_logos` rows (`INSERT INTO studio_images (...)
  SELECT ..., 'logo', 'enrichment', ... FROM studio_logos`) before `DROP TABLE
  studio_logos`; the on-disk files move from `studio-logos/{studio_id}/{id}.jpg` to
  `studio-images/{studio_id}/{id}.jpg` (a filesystem move alongside the DB copy, best
  effort — a missing file after move is not fatal, the slot just re-fetches on next
  enrich). Also deletes stale `field_source_decisions` rows for
  `(entity_type='studio', field_key='logo')`.
- **P0-2 — Model + registry.** `model.StudioImageIcon/Logo/Poster` role constants
  (mirroring `PersonImageHeadshot` etc., all "core"); `model.StudioImageSourceUpload` /
  `...SourceEnrichment` (no `promoted` — no gallery to promote from). `Studio` gains
  `IconURL`, `LogoURL` (kept name), `PosterURL` — always populated on both list and detail
  reads, built the same way `LogoURL` is today (`/api/v1/studios/{id}/images/{role}?v={id}`),
  empty when the slot has no image. The registry's `logo` **field** entry (RD4) is removed.
- **P0-3 — Entity-generic asset orchestration.** `enrich.ImageSink` and
  `Service.downloadAssets` gain an `entityType` parameter (mirrors `EnrichmentForEntity`'s
  existing `(entityType, entityID)` shape) instead of being person-only; `assetRoleFor`
  branches on `entityType` (`"person"` → headshot/banner/poster/extra as today; `"studio"`
  → `"logo"` → `StudioImageLogo`, `"icon"`/`"poster"` kinds reserved for a future provider).
  A studio-backed `ImageSink` implementation is added: `StoreAsset` always replaces (every
  role is core, no cap), `SuppressedAssetURLs`/`ExistingAssetURLs` return empty (no
  suppression/dedup store for studios — nothing to suppress without a gallery),
  `LockedCoreRoles` queries `studio_images` for `source='upload'` rows. No poster-seed-
  from-logo (Non-Goals) — unlike Person's headshot→poster seed, nothing about a studio logo
  implies a poster. **Person behavior is unchanged** (same interface shape, `entityType =
  "person"` at its call sites) — this is the acceptance bar for the generalization.
- **P0-4 — TMDB company logo becomes an asset.** `providers/tmdb/tmdb.go`'s company
  enrichment emits `Assets: [{Kind: "logo", URL: ...}]` instead of `Fields["logo"]`. No
  other company field or the `/describe` contract changes.
- **P0-5 — Owner upload/delete API.**
  `POST /api/v1/studios/{id}/images/{role}` (multipart upload; `role ∈ {icon,logo,poster}`,
  400 on an unknown role, mirrors `POST /people/{id}/image` validation/size caps) and
  `DELETE /api/v1/studios/{id}/images/{role}`, both `requireOwner`. Upload runs the bytes
  through `personimage.Normalize` (metadata strip, bomb guard, decode-and-re-encode) before
  storing — the same untrusted-bytes gate every other image upload passes.
- **P0-6 — Public serve route.** `GET /api/v1/studios/{id}/images/{role}` streams the
  on-disk JPEG (`Cache-Control: public, max-age=31536000, immutable`,
  `X-Content-Type-Options: nosniff`), 404 when the slot is empty (the SPA renders its
  existing per-role fallback — monogram for logo/icon, no poster placeholder needed since
  poster has no consumer yet). Replaces `GET /studios/{id}/logo`.
- **P0-7 — Detail-page controls (RD1).** A role-generic image control (upload / replace /
  remove) rendered three times on `/studios/{id}` — icon, logo, poster — owner-gated
  (visitors see the images read-only, same posture as every other studio mutation).
- **P0-8 — List-page icon.** `/studios` list well switches from `s.logo_url` to
  `s.icon_url`; falls back to the existing monogram when empty.

### Should-have (P1)

- **P1-1 — Owner-view provenance indicator** on each role control ("yours" vs. "from
  {provider}"), reusing the existing `ProvenanceBadge` vocabulary — the same deferred item
  ADR-049 flagged for Person, picked up here for both entities if it lands.

### Future considerations (P2)

- **P2-1 — TMDB (or another provider) actually supplying `icon`/`poster` assets** for
  companies, once a data source exists. No schema change required — `assetRoleFor` already
  accepts the kinds.
- **P2-2 — Studio poster surfaced somewhere** (browse card, hero) — purely a frontend
  consumer of the already-shipped `PosterURL`.

## Behavior detail

### Provenance lock (RD2, mirrors ADR-049)
`downloadAssets(ctx, "studio", studioID, ...)` loads `LockedCoreRoles` before fetching;
a role whose current row is `source='upload'` is skipped entirely (no download, no store
call) — identical posture to Person's core-role lock. Deleting the manual image empties the
slot, and the next enrich (or re-enrich) fills it, same as today's Person behavior.

### Serving & cache-busting
Identical to today's studio-logo mechanism (ADR-057 §4): server-assigned row `id` doubles
as the `?v=` cache-buster (delete+insert on replace), no separate version column, 404 on an
empty slot (no placeholder route — the SPA already owns the empty-state rendering).

## API

```
GET    /api/v1/studios/{id}/images/{role}        serve (public; role ∈ icon|logo|poster; 404 if empty)
POST   /api/v1/studios/{id}/images/{role}         upload/replace (requireOwner; multipart)
DELETE /api/v1/studios/{id}/images/{role}         remove (requireOwner)
```

Removed: `GET /api/v1/studios/{id}/logo`, and the `logo` canonical field from
`PUT/DELETE /api/v1/studios/{id}/fields/{canonical}/decision`.

## UI (grounded in real components)

- **`/studios` list**: the existing logo well (`web/src/routes/studios/+page.svelte`
  ~122-136) swaps its source from `s.logo_url` to `s.icon_url`; unchanged monogram
  fallback.
- **`/studios/{id}` detail**: a new role-generic image control (upload / replace / remove),
  rendered for icon, logo, poster — visually similar to Person's `PersonAvatar`/
  `PersonBanner`/`PersonPoster` wrappers over `PersonImageFrame`
  (`web/src/lib/components/person/`), but without the gallery/viewer-modal surface Person
  needs and Studio doesn't. Owner-gated controls; visitors see read-only images.
- Tokens only; QA Cinémathèque / Broadcast / Brutalist.

## Success Metrics

Single-owner curation feature:
- **Leading:** an owner can replace a wrong provider logo/icon/poster without touching a
  file tag or losing the change on the next re-enrich (manual tests + the ADR-049-style
  test matrix).
- **Leading:** zero Person-behavior regressions from the `ImageSink`/`downloadAssets`
  generalization (existing Person image test suite passes unchanged).
- **Lagging:** the poster slot gets a real consumer (P2-2) without any backend follow-up
  work — validates that shipping poster's plumbing now (RD1) was the right call.

## Open Questions

- **Q1 (engineering, non-blocking):** exact upload size/dimension caps for studio images —
  reuse `personimage`'s existing constants verbatim, or a studio-specific config knob. Default
  to reuse; only split if a real need appears (ADR-060 runtime-settings precedent exists if so).

## Timeline / routing

No hard deadline. Per the change-routing rules, before/with implementation:
1. ✅ **`/architecture`** — [ADR-079](../architecture/ADR-079-studio-image-roles.md)
   (table + entity-generic `ImageSink` + retiring the `logo` field; supersedes ADR-057).
2. ✅ **`/design-handoff`** — [studio-images-handoff.md](../design/studio-images-handoff.md):
   studio detail-page image controls, list icon well, empty states, 3-skin QA.
3. ✅ **`/testing-strategy`** — migration data-carry-forward, provenance-lock matrix,
   entity-generic `downloadAssets` (Person regression + Studio new coverage), endpoint auth,
   asset-perimeter reuse, serve-route cache headers, 3-skin a11y (`docs/testing-strategy.md`
   §10).
4. ✅ **`/security-review`** — **sign-off (2026-08-03): clean.** New owner-gated upload/
   delete endpoints correctly mounted inside `requireOwner`; the public serve route only
   ever builds a filesystem path from server-assigned ids (`{role}` is validated against
   the enum but never touches a path); uploaded bytes pass through the same
   `personimage.Normalize` untrusted-bytes gate as every other image upload; the TMDB
   logo-as-asset switch reuses the unchanged SSRF-guarded `AssetClient` (no new host,
   redirect, or scheme latitude); `internal/repo/studio_images.go` is fully parameterized.
   No findings.

Slices: **S1** backend (migration 0036, model/registry, entity-generic `ImageSink`, TMDB
asset switch, API) → **S2** frontend (detail controls, list icon) → **S3** QA + security.
Effort: M.
