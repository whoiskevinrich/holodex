# Spec: Two-tier video poster resolution — sharp detail page, small list thumbnails (F53)

**Status**: Draft
**Phase**: F53 (Jira [HOLODEX-253](https://whoiskevinrich.atlassian.net/browse/HOLODEX-253), epic [HOLODEX-19](https://whoiskevinrich.atlassian.net/browse/HOLODEX-19))
**Owner**: Project owner
**Date**: 2026-08-05
**Feature block**: **F53** — the `media/{id}` detail page currently reuses the same small
list-card thumbnail for its larger poster display, so enrichment-sourced artwork that's
genuinely high-resolution looks soft when stretched. Extraction (`internal/thumbnail`)
starts producing a second, larger derivative in the same pass; the detail page switches to
it. List view is unchanged.

**Depends on** (all shipped):
- the thumbnail pipeline itself ([ADR-009](../architecture/ADR-009-thumbnail-strategy.md),
  `internal/thumbnail`) — Tier 1 embedded-art extraction, Tier 2 ffmpeg frame-grab, both
  writing to a single `DATA_PATH/thumbnails/{id}.jpg`
- owner poster upload ([F52](video-owner-mode-editing.md), HOLODEX-252) — uploads land in
  the same pipeline (`internal/api/video_poster.go` calls `ExtractEmbedded`), so this
  feature's larger tier benefits uploaded posters for free, no separate handling
- metadata writeback ([ADR-041](../architecture/ADR-041-metadata-writeback.md),
  `internal/writeback`) — relevant only as the thing that already gets this right; see
  Problem Statement

**ADR**: none. This is an incremental extension of ADR-009's existing tiers (same package,
same on-disk convention, one new sibling output file) — no new table, no cross-cutting
decision, no change to how images enter the system. See "Timeline / routing" for the
explicit routing rationale.

**Related**: [Owner-mode video editing / F52](video-owner-mode-editing.md) (poster upload
lives in the same pipeline this spec extends).

---

## Problem Statement

The owner runs an enrichment provider outside this repo. Its poster looks correct-sized in
the library list but "low resolution" on the `media/{id}` detail page. Tracing the actual
pipeline (not assumed): **writeback already embeds the image at full resolution, straight
from the live provider URL** (`internal/writeback`'s `downloadImageToTemp` does a fresh
HTTPS GET with no resizing) — the file itself is fine regardless of which provider supplied
it. The gap is one step later and entirely local: `internal/thumbnail`'s extraction
(`extractCoverArt` for Tier 1 embedded art, `generateFrame` for Tier 2 ffmpeg frame-grabs)
produces exactly **one** downsampled derivative, written to `DATA_PATH/thumbnails/{id}.jpg`
and capped at `THUMBNAIL_WIDTH` (default 400px). Both the list card
(`VideoCard.svelte:21`) and the detail page's `<video poster>`
(`media/[id]/+page.svelte:535`, rendered `w-full` inside a `max-w-4xl` / 896px container)
bind to that same single small file — the detail page is stretching a thumbnail sized for a
grid card.

This was initially framed as "providers need to return multiple poster sizes," but that's
ruled out by the trace above: the provider is not in the causal path, and depending on
provider cooperation would be fragile anyway for a provider outside this repo that Holodex
doesn't control. The fix belongs entirely in the local extraction step.

## Goals

1. **The detail page renders a genuinely sharper poster** for any video whose source art
   (enrichment-provided or owner-uploaded) exceeds today's 400px cap — visibly, not just in
   theory.
2. **The list view is unaffected** — same small, fast-loading thumbnail, same bandwidth
   profile, zero behavior change to `VideoCard.svelte` or the list API payload.
3. **No dependency on provider cooperation.** The fix works identically for every provider,
   including ones outside this repo, because it never touches `internal/enrich` or
   writeback — it operates purely on bytes Holodex already has on disk.
4. **No new expensive work.** The second derivative is a by-product of extraction that
   already runs (embedded-art byte read, or ffmpeg frame-grab) — not a second network
   fetch, and not a second video seek/decode for Tier 2.

## Non-Goals

- **Any change to `internal/enrich`, ADR-039, `metadata-sources.yaml`, or a provider sidecar
  (including the TMDB one).** Writeback already sources full resolution directly from the
  live provider URL; provider size-affordance was considered and ruled out as the fix for
  this problem. *(Why: adding provider-contract surface for a problem that isn't caused by
  the provider is pure unnecessary risk.)*
- **A new database migration or schema.** The poster URL is computed by convention exactly
  like `thumbnail_url` is today (`/api/v1/media/{id}/poster?v={mtime}`), not persisted.
  *(Why: this is a derived-file-naming convention, not new entity state.)*
- **Responsive/srcset multi-breakpoint image serving.** Two fixed tiers (list, detail) is
  the whole ask. *(Why: scope discipline — a general responsive-image system is a much
  larger, separate investment nothing here currently needs.)*
- **Generalizing to Person headshots or Studio icon/logo/poster.** Those share the identical
  single-resolution-reused-everywhere pattern, but are explicitly out of scope for this
  pass. *(Why: keep this change small and provable; tracked as a known follow-up, not
  silently bundled in.)*
- **A bulk/eager backfill job for the existing library.** *(Why: RD6 below — lazy pickup via
  existing triggers is sufficient, and the existing per-video "Regenerate" button already
  gives an impatient owner a manual override.)*

## Resolved Decisions

*(Locked with the owner 2026-08-05 via a product-brainstorming session + question cards.)*

- **RD1 — Size logic lives entirely in Holodex, not the provider contract.** No changes to
  `internal/enrich`/ADR-039/TMDB sidecar. Confirmed correct by the writeback trace in the
  Problem Statement — the provider isn't the bottleneck.
- **RD2 — Confirmed root cause: local extraction, not source quality.** Writeback already
  embeds full-resolution art; the single downsampled `thumbnail_url` derivative is reused by
  both list and detail views. This is a two-derivative-output problem, not a fetch-more-data
  problem.
- **RD3 — Scoped to video poster only.** Person/Studio images share the pattern; deferred.
- **RD4 — Two pre-generated files, not resize-on-request.** Extraction writes both sizes up
  front (mirrors today's `{id}.jpg` model), served as static files exactly like the existing
  thumbnail — no new caching/width-bucket logic, no per-request image processing.
- **RD5 — A new fixed `POSTER_WIDTH` config, not the untouched original extraction.** Capped
  and configurable (mirrors `THUMBNAIL_WIDTH`), not an unbounded pass-through of whatever a
  provider's "original" tier happens to be (which can be several MB).
- **RD6 — Lazy backfill.** No new bootstrap sweep. Existing videos get the poster-size
  derivative the next time any of the pipeline's existing triggers fire for them (re-scan,
  re-enrich/writeback, poster upload, or the existing manual "Regenerate" button). Until
  then, the new poster route falls back to serving today's thumbnail bytes (P0-6) so nothing
  breaks or regresses in the interim.

## User Stories

- As the owner, I want the `media/{id}` detail page to show a sharp poster instead of an
  upscaled list thumbnail, so a video's artwork looks as good as the source the provider
  actually offered.
- As the owner, I want the library list view to keep loading small, fast thumbnails
  unchanged, so browsing a large library stays snappy.
- As the owner, I want a poster I upload manually (F52) to also render sharp on the detail
  page, not just enrichment-sourced ones, since it goes through the same pipeline.
- As the owner, I want existing library videos to pick up the sharper poster automatically
  the next time their thumbnail is touched by scanning, re-enriching, or clicking
  Regenerate — not require a special one-time migration step.

## Requirements

### Must-have (P0)

- **P0-1 — New config.** `POSTER_WIDTH` (yaml `poster_width`, env `POSTER_WIDTH`),
  `thumbnail.Config.PosterWidth`, default **1200** (3× today's 400px `THUMBNAIL_WIDTH`
  default; comfortably covers the detail page's ~896px CSS container at up to ~1.3× pixel
  density without shipping a multi-MB original). Same `if cfg.PosterWidth <= 0` defaulting
  pattern as `Width` in `thumbnail.New`.
- **P0-2 — New path convention.** `PosterPath(dir, id) = filepath.Join(dir,
  strconv.FormatInt(id,10)+"-poster.jpg")`, exported alongside `ThumbPath` the same way, same
  directory (`DATA_PATH/thumbnails/`).
- **P0-3 — Tier 1 (`extractCoverArt`) produces both derivatives in one exiftool read.** The
  function already holds the full extracted byte buffer (`data`) before any width check; add
  a second scale pass (or straight copy, when `cfg.Width <= cfg.PosterWidth` and source is
  already ≤ `PosterWidth`) writing to `PosterPath`. One `exiftool -b` invocation total, up to
  two `scaleToWidth` (ffmpeg) passes — no second network/disk read of the source.
- **P0-4 — Tier 2 (`generateFrame`) produces both derivatives from a single seek/decode.**
  Grab the video frame once (at up to `PosterWidth`, the larger target), write it to
  `PosterPath`, then derive the thumbnail-tier output by rescaling that already-decoded frame
  locally — **must not re-seek the video a second time.** (Open Question Q2 below covers the
  exact ffmpeg invocation shape.)
- **P0-5 — `Video.PosterURL` field**, computed by convention identically to how
  `ThumbnailURL` is built today (`setThumbnailURL`-equivalent):
  `/api/v1/media/{id}/poster?v={mtime}`. Included in the video JSON response.
- **P0-6 — `GET /api/v1/media/{id}/poster` serve route.** Serves `{id}-poster.jpg` when
  present; **falls back to serving the existing `{id}.jpg` thumbnail bytes** when the poster
  file doesn't exist yet (a video that hasn't been through extraction since this feature
  shipped — see RD6). This is the mechanism that makes lazy backfill safe: the route always
  resolves, quality just improves silently once a video's next natural trigger runs.
- **P0-7 — Frontend binding swap.** `media/[id]/+page.svelte`'s `<video poster={...}>`
  (line ~535) switches from `video.thumbnail_url` to `video.poster_url`, reusing the same
  `api.thumbnailReload`/`thumbVersion` cache-busting call already in place (or an equivalent
  helper — the URL's `?v={mtime}` already changes on file rewrite, so cache-busting is
  inherited for free). `VideoCard.svelte` is untouched — still binds to `thumbnail_url`.

### Should-have (P1)

- **P1-1 — Config docs.** Add `POSTER_WIDTH` to `docs/reference/configuration.md` alongside
  `THUMBNAIL_WIDTH`, same table.

### Future considerations (P2)

- **P2-1 — Apply the same two-tier pattern to Person headshots and Studio
  icon/logo/poster** ([studio-images.md](studio-images.md), F51/F25), which share the
  identical single-resolution-reused-everywhere shape today. No schema change required when
  it happens — this spec's `PosterPath`/`POSTER_WIDTH` pattern generalizes directly.
- **P2-2 — A bulk "regenerate all posters" owner action**, for an owner who wants the
  upgrade applied immediately across their whole library instead of waiting for lazy
  triggers. RD6 explicitly deferred this; the existing per-video Regenerate button remains
  the manual escape hatch until/unless this is requested.
- **P2-3 — Resize-on-request / responsive `srcset` serving.** RD4 explicitly chose two fixed
  pre-generated files for now; a general responsive-image system is a separate, larger
  investment if a real need for more than two tiers ever appears.

## Behavior detail

### Why this doesn't touch enrichment or writeback
`internal/writeback`'s `downloadImageToTemp` downloads straight from the resolved field's
live URL and embeds it untouched (10MiB cap only, no resize) — confirmed by code trace, not
assumption. The media file already carries full-resolution art after writeback, independent
of provider. This spec only changes what happens to that already-embedded (or
already-generated-frame) art *after* it's extracted back out for web serving.

### Tier 1 sizing logic
Given the extracted byte buffer and its decoded width `w`:
- `w <= min(ThumbnailWidth, PosterWidth)`: both outputs are the same bytes, written twice
  (cheap — already in memory, no re-invoking exiftool).
- `ThumbnailWidth < w <= PosterWidth`: poster tier is a straight copy of the source bytes;
  thumbnail tier is scaled down.
- `w > PosterWidth`: both tiers are independently scaled down from the same source buffer.

### Tier 2 sizing logic
Frame-grab is comparatively expensive (video seek + decode) and runs on a low-priority
background queue (ADR-009 Tier 2/3) — the two-output requirement must not double that cost.
Grab the frame once at `PosterWidth`, write it as the poster tier, then produce the
thumbnail tier by rescaling that single decoded frame (a second cheap ffmpeg/image pass over
already-extracted pixel data, not a second seek). See Open Question Q2 for the exact
implementation shape.

### Serving & fallback (RD6 mechanism)
`GET /api/v1/media/{id}/poster` checks for `{id}-poster.jpg`; if absent, serves `{id}.jpg`
(today's thumbnail) instead of 404ing. This means `Video.PosterURL` always resolves to a
valid image from the moment this feature ships, even for a video whose poster tier hasn't
been generated yet — quality improves the next time that video's extraction runs, with zero
special-casing on the frontend.

## API

```
GET /api/v1/media/{id}/poster    serve poster-tier JPEG (falls back to thumbnail-tier if not yet generated)
```

Unchanged: `GET /api/v1/media/{id}/thumbnail` (list tier, same as today).

## UI (grounded in real components)

- **`media/[id]/+page.svelte`** (~line 535): `<video poster={...}>` source switches from
  `video.thumbnail_url` to `video.poster_url`. No new component, layout, state, or
  interaction — purely which URL field an existing binding points at.
- **`VideoCard.svelte`**: unchanged.
- Tokens/theming: not applicable — no new visual element, styling, or skin-dependent surface
  is introduced.

## Success Metrics

Single-owner curation feature:
- **Leading:** after any of the four natural triggers (scan, re-enrich/writeback, poster
  upload, manual Regenerate) runs on a video with source art wider than 400px,
  `GET /media/{id}/poster` serves a derivative distinct from and larger than
  `GET /media/{id}/thumbnail` (verified by comparing dimensions/byte size, not just that the
  route responds).
- **Leading:** manual QA — open a TMDB-enriched video's detail page before and after this
  ships and confirm the poster visibly reads as sharp, not just technically served from a
  different file.
- **Lagging:** zero change in list-view (`/`) payload size or load time — confirms the list
  tier truly wasn't touched.

## Open Questions

- **Q1 (engineering, non-blocking):** is 1200px the right `POSTER_WIDTH` default? Proposed
  based on the detail page's ~896px CSS container width × ~1.3, but worth sanity-checking
  against a real TMDB "original" poster's actual resolution and the resulting JPEG size
  before locking the default.
- **Q2 (engineering, non-blocking):** exact ffmpeg invocation for Tier 2's "one seek, two
  outputs" requirement (P0-4) — e.g. a filter-graph `split` into two `scale` outputs in a
  single ffmpeg call, vs. capturing one frame to a temp buffer and running two independent
  (cheap, no-seek) scale passes over it. Needs a spike against the existing `scaleToWidth`
  helper before implementation.
- **Q3 (engineering, non-blocking):** does `ExtractEmbedded`/the Tier 2 queue currently
  skip-if-`{id}.jpg`-already-exists? If so, confirm that re-adding an id to the queue (e.g.
  via the existing manual Regenerate path) still re-runs full extraction rather than
  short-circuiting — otherwise RD6's "next natural trigger" story needs a small adjustment
  for videos whose thumbnail already exists but whose poster tier doesn't.

## Timeline / routing

No hard deadline. Per the change-routing rules:

1. **`/architecture`** — **not needed.** This is an incremental extension of ADR-009 (same
   package, same on-disk convention, one new sibling output file per video) — no new table,
   no cross-cutting decision, no change to how images enter the system. Explicit routing
   call, not a silent skip.
2. **`/design-handoff`** — **not needed.** The only frontend change is which existing URL
   field a single existing `<video poster>` attribute binds to — no new component, layout,
   state, breakpoint, or interaction to hand off.
3. **`/testing-strategy`** — ✅ **done (2026-08-05).** Added to
   [docs/testing-strategy.md](../testing-strategy.md): extended the existing Thumbnail
   pipeline row (§4) and added a Frontend row (§5) for the binding swap, plus a full
   adversarial Given/When/Then block (§10) covering Tier 1's two-derivative generation across
   the three width bands, Tier 2's single-seek-two-outputs requirement, the poster-route
   fallback/lazy-backfill behavior (P0-6), and the list-view-unaffected regression guard.
4. **`/security-review`** — ☐ not yet run. New public, unauthenticated read route
   (`GET /media/{id}/poster`) mirrors the existing `/thumbnail` route's posture exactly (id-
   keyed static file serve, no new mutation, no owner-gating change) — expected low risk,
   run before merge per the checklist.

## Gate status

- [x] `/testing-strategy`
- [ ] `/security-review`

Slices: **S1** backend (config, `PosterPath`, Tier 1 + Tier 2 dual-output, `PosterURL` field,
serve route + fallback) → **S2** frontend (detail-page binding swap) → **S3** QA + security
review.
Effort: S.
