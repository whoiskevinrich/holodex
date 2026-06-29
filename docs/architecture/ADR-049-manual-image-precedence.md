# ADR-049: Owner-set person images take precedence over enrichment

**Status**: Proposed
**Date**: 2026-06-28
**Deciders**: Project owner
**Relates to**: [ADR-038](ADR-038-person-images.md) (person-image store + `source` provenance column), [ADR-039](ADR-039-provider-asset-urls.md) (provider asset fetch perimeter), [ADR-043](ADR-043-gallery-cap-and-enrichment-suppression.md) (delete-suppression on re-enrich — the sibling precedence rule), [ADR-048](ADR-048-metadata-curation-and-write-queue.md) (manual-vs-provider precedence for scalar metadata fields), spec [People Images / F25](../specs/people-images.md) (F25.31, F33).

---

## Context

A person has three single-slot core image roles — headshot, banner, poster (ADR-038).
Each `person_images` row already records `source ∈ {upload, promoted, enrichment}`:
`upload` and `promoted` are deliberate owner actions, `enrichment` is provider-fetched.

But the enrichment asset path overwrites them blind. For a core role,
`repo.InsertPersonImage` is delete-then-insert (single-slot invariant), and
`enrich.downloadAssets` calls `ImageSink.StoreAsset` for the first provider asset of
each core role with no check on what currently fills the slot. So an owner who uploads
a hand-picked headshot, then runs (or re-runs) enrichment, **silently loses it** to the
provider's photo. The poster auto-seed (F25.29) has the same exposure for a manually-set
poster, though it already guards the *empty*-slot case via `StoreAssetIfAbsent`.

This is the image-shaped twin of two precedence rules the repo already made:

- **ADR-043** — a *deleted* enrichment gallery image stays deleted across a re-enrich
  (suppression set, consulted before fetch).
- **ADR-048** — for scalar metadata fields, a `manual` curation decision beats a
  provider value in the resolver.

Images had neither: there was no notion of "this slot is the owner's, leave it."

## Decision

**Enrichment never overwrites a core image whose `source` is `upload` or `promoted`.**
It still fills empty slots and refreshes its own (`source = enrichment`) images on
re-enrich. The lock is *implicit in provenance* — no new column, no migration, no owner
toggle. The existing `source` column already carries everything needed.

Mechanically this mirrors the ADR-043 suppression seam:

- New read `repo.LockedCoreRoles(personID) → set<role>` returns the core roles whose
  current row has `source IN ('upload','promoted')`. One indexed query
  (`idx_person_images_person`).
- New `ImageSink.LockedCoreRoles` (passthrough on the `personimage.Sink`) exposes it to
  the enrich package without it importing repo.
- `enrich.downloadAssets` loads the locked set once per run (alongside the suppressed
  set) and, for a core role in that set, **skips the asset before fetching its bytes** —
  so a locked role costs no download and never touches `done[role]`/`headshotRaw`
  bookkeeping. The poster auto-seed is likewise skipped when poster is locked.
- The lookup **fails open**: a query error logs and locks nothing, rather than blocking
  enrichment (same posture as `SuppressedAssetURLs`).

To let a provider image replace a manual one, the owner deletes their image first — the
slot becomes empty and the next enrich fills it. This matches the gallery-delete mental
model (ADR-043) and needs no extra UI.

## Alternatives considered

- **Explicit per-slot lock** (a `locked` column, or generalize `metadata_curation` to
  `entity_type='person'`, `field_key='image:headshot'`, `action='nowrite'` per ADR-048).
  Lets the owner pin even a *provider* image and decouples "who set it" from "may
  enrichment touch it." Rejected for now: a migration + owner-gated endpoint + UI toggle
  is more surface than the need ("don't clobber what I set") warrants. This ADR doesn't
  block it — an explicit lock would just union into the same `locked` set. Recorded as
  the deferred follow-up.
- **In-flow conflict prompt** ("provider has a headshot — keep yours / use theirs"). The
  F30/ADR-048 curation-picker direction applied to binary assets; large surface, defers
  the safe default. Not now.
- **Guard in `repo.InsertPersonImage` instead of the enrich orchestration.** Rejected:
  the repo insert is shared by the upload/promote handlers, which *must* replace a core
  slot. The precedence rule is specifically "enrichment yields to manual," so it belongs
  in the enrichment path, leaving the owner write paths unconditional.

## Consequences

- **Positive**: an owner's uploaded/promoted headshot, banner, or poster survives every
  enrichment and re-enrich. Provider-set images still refresh in place; empty slots still
  fill. No schema change, no migration, no new config.
- **Behavioral contract** (becomes the test matrix): manual core slot → kept; provider
  (`enrichment`) core slot → refreshed; empty → filled; manual poster → not seeded over;
  owner deletes their image → next enrich refills; gallery `extra` → append-only,
  unaffected.
- **Security**: no new surface. The change only *removes* a write; it adds no fetch
  target (SSRF perimeter, ADR-039, unchanged) and reaches no new owner-gated path. The
  locked set is derived from existing per-person rows. No `/security-review` gate beyond
  confirming the above.
- **Testing**: a repo test asserts `LockedCoreRoles` reports only upload/promoted core
  rows (not enrichment, not gallery uploads); enrich tests assert a locked headshot+banner
  are kept while gallery still flows, and a locked poster blocks the headshot seed.
- **Follow-up (deferred)**: the explicit per-slot pin above, plus an owner-view
  provenance badge ("Yours" / "from {provider}") on core-image controls so the protection
  is legible — reuses the existing `ProvenanceBadge` vocabulary.
