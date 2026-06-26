# ADR-043: Person gallery — configurable cap, owner over-cap override & enrichment URL suppression

**Status**: Accepted
**Date**: 2026-06-25
**Deciders**: Project owner
**Relates to**: [ADR-038](ADR-038-person-images.md) (person-image store + the gallery cap), [ADR-039](ADR-039-provider-asset-urls.md) (provider asset URLs + SSRF perimeter), [ADR-014](ADR-014-configuration-and-data-layout.md) (config precedence), [ADR-030](ADR-030-access-control-gating-seam.md) (owner gate), spec [People Images / F25](../specs/people-images.md) (F25.8, F25.23–25).

---

## Context

ADR-038 gave each person three single-slot core roles (headshot/banner/poster) and a
free-form gallery of `extra` images, with a hard-coded 20-image gallery cap enforced
in `repo.InsertPersonImage`. Three needs surfaced after that shipped:

1. **A bug report**: "setting the headshot/poster at a full gallery errors with
   *gallery full*." On investigation the backend was already correct — core roles take
   a separate delete-then-insert path that never counts against the cap, and a repo
   test asserted it. The real defect was a UI one: the persistent "Gallery is full"
   banner used the **error** token (`text-warn`/`border-warn`), so a full gallery read
   as a *failure of the action just taken* rather than a neutral status.
2. **The cap should be operator-configurable** — 20 is right for most libraries but
   not a universal constant.
3. **The owner should be able to exceed the cap deliberately** (curate a rich gallery)
   while **enrichment stays bounded** (a chatty provider must not dump 100 images).
4. **Deleting an enrichment-sourced gallery image should stick** — a later re-enrich
   currently re-adds it, so the owner's curation is silently undone.

The single-owner model (ADR-030) means there is no separate "admin" tier: every
gallery write is already owner-gated. "Admins override the cap" therefore means *the
owner can opt past the cap on a manual upload*, not a second identity class.

## Decision

### 1. Configurable cap (`PERSON_GALLERY_MAX`, default 20)

A new config field `PersonGalleryMax` (yaml `person_gallery_max`, env
`PERSON_GALLERY_MAX`, default 20; a non-positive value falls back to 20) flows through
`config` → `repo.SetGalleryCap`. `GalleryCap` stays as the built-in default constant;
`Repo.GalleryCapValue()` returns the override or that default, so a bare `repo.New(db)`
(tests, MCP stdio) keeps 20. The effective value is advertised to the SPA via
`/capabilities` (`person_gallery_max`) so the UI warns at the right number rather than
hard-coding it.

### 2. Owner over-cap override (explicit flag, not a tier)

`InsertPersonImage` is reshaped to take a `PersonImageInsert` struct (the positional
signature had grown unwieldy and needed two more fields anyway). The struct carries an
`OverCap bool`. The cap check becomes `if !in.OverCap && count >= cap`. The owner-gated
upload handler reads an `allow_over_cap` form field and threads it through; the
enrichment `Sink` never sets it. The default SPA flow still warns at the cap and offers
an explicit **"Add anyway"** action that sets the flag — deliberate, not silent.

### 3. Enrichment URL suppression on delete

Migration `0012` adds:

- `person_images.source_url TEXT NOT NULL DEFAULT ''` — the upstream asset URL for an
  enrichment-sourced row (empty for owner uploads/promotes). Threaded in via the new
  `PersonImageInsert.SourceURL`.
- `person_image_suppressions(person_id, source_url, created_at)` — a per-person list of
  deleted asset URLs, composite PK (`INSERT OR IGNORE` idempotent), `ON DELETE CASCADE`.

`DeletePersonImage` reads the row's `role` + `source_url` and, **only for a gallery
`extra` with a non-empty `source_url`**, records the URL into the suppression table —
all in the same write transaction as the delete. Core-role deletions do *not* suppress
(a re-enrich may legitimately refill an empty headshot/banner/poster). The enrich
`downloadAssets` orchestration consults the per-person suppressed set (via a new
`ImageSink.SuppressedAssetURLs`) and **skips** any asset whose URL is suppressed before
fetching it. The lookup **fails open** — a suppression-store error logs and treats
nothing as suppressed rather than blocking enrichment.

## Alternatives considered

- **Suppress by `provider`+`external_id`** rather than URL — coarser; it would block a
  whole provider record's imagery, not the one image the owner removed. Rejected: the
  owner deletes a *specific image*, so the URL is the right key.
- **Also suppress core-role deletions** — rejected for v1: an empty core slot is a
  prompt to refill, and a re-enrich filling it is the desired behavior, not a
  regression. (Revisitable if owners report otherwise.)
- **Keep the positional `InsertPersonImage` signature** and add two trailing params —
  same call-site churn, worse readability. The struct is the cleaner long-term shape.

## Consequences

- **Positive**: the cap is operator-tunable; the owner is never hard-blocked from
  curating; enrichment growth stays bounded; deleted provider images stay deleted; the
  full-gallery UI no longer reads as an error.
- **Migration**: `0012` is additive (one nullable-with-default column + one table); the
  `down` drops both. No backfill — pre-existing enrichment rows simply have an empty
  `source_url` and so are never suppressed (correct: we never recorded where they came
  from, so we can't safely suppress them).
- **Security**: the override is reachable only through the existing `requireOwner`
  gate — no new exposure; enrichment can never set it. The suppression list is
  per-person, non-PII operator data; the SSRF perimeter (ADR-039) is unchanged (we only
  *skip* URLs, never add new fetch targets). Verified by `/security-review`.
- **Testing**: repo tests cover configurable cap + `OverCap` bypass + suppress-on-delete
  (extra suppresses, core does not, upload-with-no-URL does not); API tests cover
  core-role upload at a full gallery → 201 (the bug-fix regression guard), over-cap → 409,
  and `allow_over_cap` → 201; an enrich test covers skipping a suppressed asset URL.
