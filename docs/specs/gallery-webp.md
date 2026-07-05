# Spec: WebP support for provider-sourced images (F42)

**Status**: Draft
**Phase**: Phase 3 enrichment follow-up
**Owner**: Project owner
**Date**: 2026-07-05
**Feature block**: **F42** — stop silently dropping **WebP** images returned by metadata providers.
Register a WebP decoder so provider WebP assets pass the person-image normalization gauntlet and are
stored (re-encoded to JPEG) like any other accepted format.

**Issue**: [HOLODEX-141](https://whoiskevinrich.atlassian.net/browse/HOLODEX-141)
**ADR**: none required — no architectural decision or contract change (core-side decoder registration
only; the SSRF/asset perimeter, the provider protocol, and the on-disk format are all unchanged). This
spec is the artifact of record.

**Depends on** (all shipped):
- the person-image ingest normalizer (`internal/personimage`, `Normalize` — decode → bomb-guard →
  optional downscale → **re-encode to JPEG q85, metadata stripped**)
- the enrichment asset perimeter (`internal/enrich/assets.go`, SSRF allowlist + 16 MiB cap, [ADR-039](../architecture/ADR-039-provider-asset-urls.md)) — **unchanged**
- the person-image store + gallery (`internal/repo/person_images.go`, [ADR-038](../architecture/ADR-038-person-images.md)/[ADR-043](../architecture/ADR-043-gallery-cap-and-enrichment-suppression.md))

**Touches the untrusted-provider asset perimeter (a new accepted input format) → a `/security-review`
sign-off is required before merge** (label `needs-security-review`). Expected to come back clean: WebP
rides the **existing** decode/validate/re-encode gauntlet; the change adds a decoder, not a new code
path.

---

## Problem Statement

Holodex accepts provider images by **decoding the bytes** — the accepted formats are exactly the image
decoders registered in `internal/personimage`, today **JPEG / PNG / GIF**. WebP is *intentionally*
excluded (`golang.org/x/image` is not yet a dependency). A provider that returns a WebP asset URL
therefore has its image **downloaded** (through the SSRF gate, consuming the fetch) and then **fail to
decode** in `Normalize`, so the asset is **skipped with a warn** and never stored. The result: people
enriched from a WebP-serving provider show **missing gallery images** for no visible reason, and the
wasted download cost is paid anyway. As providers increasingly serve WebP, this silently drops a
growing share of otherwise-valid imagery.

## Goals

1. **Accepted, not dropped** — a provider-returned WebP image is decoded, normalized, and stored the
   same as a JPEG/PNG/GIF, for every image role (headshot / banner / poster / gallery `extra`).
2. **No perimeter weakening** — WebP goes through the identical `Normalize` gauntlet (full decode
   rejecting polyglots/SVG, decompression-bomb guards, downscale, re-encode to JPEG, metadata strip);
   the SSRF/asset-host allowlist and byte caps are untouched.
3. **Fail-safe on the unsupported subset** — inputs the decoder cannot handle (e.g. animated WebP)
   **skip cleanly** (as today), never crash or store garbage.

## Non-Goals

- **Serving WebP to the browser.** All ingested images are already re-encoded to **JPEG** on the way
  in; the frontend never receives WebP and needs **no change**. *(Why: output format is unchanged; this
  is purely an accepted-input change.)*
- **Animated WebP.** `golang.org/x/image/webp` is decode-only and does not support animation; animated
  WebP continues to be skipped. *(Why: out of scope; fail-safe covers it.)*
- **WebP for local media / thumbnails.** F42 is scoped to the **provider person-image** ingest path.
  The video thumbnail pipeline (ffmpeg/exiftool, ADR-009) is separate and unchanged. *(Why: different
  pipeline, different perimeter; not the reported gap.)*
- **A new dependency beyond `golang.org/x/image`.** Exactly one well-maintained Go sub-repo module is
  added for the decoder; nothing else.

---

## Users & Value

- **Owner**: people enriched from WebP-serving providers now show their full gallery/headshot imagery
  instead of unexplained gaps.
- **Operator**: no wasted downloads that end in a decode-skip; enrichment yields what it fetched.

---

## Functional Requirements

### Must-Have (P0)

#### FR1 — Register the WebP decoder

Import `golang.org/x/image/webp` (blank import, alongside the existing `image/gif` / `image/png` /
`image/jpeg` registrations) in `internal/personimage` so `image.DecodeConfig` / `image.Decode` sniff
and accept WebP. Update the in-file comment that currently documents WebP's *exclusion*.

- **Given** a provider returns a valid (still) WebP asset within the byte cap, **when** enrichment
  downloads and normalizes it, **then** it is decoded, re-encoded to JPEG, and stored — appearing in
  the person's images for its role.

#### FR2 — Re-encode + normalization unchanged

WebP inputs are subject to the **same** `Normalize` steps and bounds as other formats: dimension /
pixel caps, optional downscale to the configured max dimension, JPEG re-encode at q85, full metadata
strip. No WebP-specific branch, sink, or storage format.

- **Given** a WebP exceeding the dimension/pixel guards, **then** it is rejected exactly as an
  oversize JPEG/PNG would be.
- **Given** a stored WebP-sourced image, **then** on disk it is a JPEG with stripped metadata,
  indistinguishable from a JPEG-sourced one.

#### FR3 — Fail-safe on unsupported/invalid WebP

An input the decoder cannot handle (animated WebP, truncated/corrupt bytes, a non-image mislabeled as
WebP) **fails decode → asset skipped with a warn**, the same neutral path as any undecodable asset
today. No crash, no partial write.

- **Given** an animated or corrupt WebP, **then** the asset is skipped, enrichment continues, and no
  image row is written for it.

---

## Acceptance Criteria

1. A provider asset that is a valid still WebP (within the 16 MiB cap) is stored as a JPEG person-image
   for its role (headshot/banner/poster/gallery), where before F42 it was dropped.
2. The stored file is JPEG with metadata stripped — byte-format identical to a JPEG-sourced image.
3. An animated or corrupt WebP is skipped cleanly (warn logged, enrichment proceeds, no row written).
4. The SSRF/asset-host allowlist, cross-host-redirect refusal, and byte caps behave identically to
   pre-F42 (WebP adds no new fetch target or relaxed limit).
5. Non-WebP formats (JPEG/PNG/GIF) are unaffected — identical behavior to today.
6. `/security-review` signs off on the diff (new accepted input format through the unchanged gauntlet).

---

## Test Notes (for `/testing-strategy`)

- **Decode/normalize** — a still WebP fixture normalizes to JPEG (assert output format + stripped
  metadata + downscale); dimension/pixel guards reject an oversize WebP.
- **Fail-safe** — animated WebP fixture and a corrupt/truncated WebP → decode error → skipped, no
  panic, no row.
- **Perimeter unchanged** — existing enrich/asset tests (SSRF allowlist, redirect refusal, byte cap)
  stay green with the decoder registered.
- **Regression** — JPEG/PNG/GIF ingest unchanged; add a WebP case to the person-image ingest table
  test and, if present, the enrich-stub fixtures (`testdata/enrich-stub/`).

---

## Rollout

Single small change: one blank import + a decoder dependency (`golang.org/x/image`), no migration, no
contract change, no flag. Absence of WebP inputs is a no-op; presence recovers previously-dropped
images. Ship after `/security-review` on the diff.
