# ADR-009: Cover Art & Thumbnail Strategy — Tiered Extraction with Throttled, Priority-Aware Backfill

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Holodex must show a representative image on every video card. Images come from two sources with very different cost profiles, and the generation work must not overload the host (which may be modest hardware — a NAS, a Pi, a low-power home server) while still ensuring the user quickly sees images for whatever they are currently browsing.

The naive framing of "extract at index time vs. extract at load time" is misleading: deferring to load time does not reduce host CPU load, it relocates the spike to the moment the user opens a 50-card grid (50 simultaneous ffmpeg spawns) — incurring both the CPU spike and user-facing latency. The real levers are **bounded concurrency**, **persisting results so each image is generated once**, and **prioritizing what the user is actually viewing**.

## Decision

A **tiered strategy** distinguishing the two image sources, with a **throttled, priority-aware background queue** for the expensive case, and **user-configurable aggressiveness**.

### Tier 1 — Embedded cover art (index time, synchronous)
- During indexing the scanner already has the file open for exiftool/ffprobe. If the container has embedded cover art (poster/attachment), extract it then. This is a near-free byte copy.
- Files with embedded art display an image immediately, with zero extra CPU cost.

### Tier 2 — Generated frame thumbnails (background, throttled)
- Files **without** embedded art are enqueued for ffmpeg frame extraction (frame at ~10% of duration, scaled to ~400px wide, encoded JPEG).
- The queue is drained by a **bounded worker pool** (default 2 workers) running at **low process priority** (`nice`/`ionice` where available) so ffmpeg yields to other host activity.
- Generation is non-blocking: cards show a placeholder until the thumbnail is ready, then it appears.

### Tier 3 — Priority bump for visible items
- When the user opens a view or applies a filter, the **currently visible** items lacking a thumbnail are pushed to the **front** of the queue.
- This guarantees the user sees images for what they are looking at quickly, while the bulk backfill proceeds slowly in the background.

### Backfill posture: Eager by default, user-configurable
- **Default: eager** — the whole library is backfilled in the background (throttled, with visible items prioritized) so the browser feels complete and every file eventually has an image.
- Fully tunable via env vars so users on constrained hardware can reduce or disable backfill.

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `THUMBNAIL_ENABLED` | `true` | Master switch for all generation (Tier 2/3). Tier 1 embedded-art extraction is unaffected. |
| `THUMBNAIL_BACKFILL` | `eager` | `eager` (backfill whole library) or `lazy` (only generate for viewed items; never proactively backfill) |
| `THUMBNAIL_WORKERS` | `2` | Number of concurrent ffmpeg generation workers — the primary host-load ceiling |
| `THUMBNAIL_NICE` | `true` | Run ffmpeg generation at low CPU/IO priority |
| `THUMBNAIL_SEEK_PERCENT` | `10` | Position in the video (% of duration) to extract the frame from |
| `THUMBNAIL_WIDTH` | `400` | Output width in px (aspect-preserved) |

In `lazy` mode, Tier 3 (priority bump for visible items) still applies — viewed items are generated on demand — but the library is never proactively swept.

## Storage

- Thumbnails are stored on disk at `DATABASE_PATH/thumbnails/:video_id.jpg`, **not** as DB BLOBs.
- Rationale: on-disk files are served with HTTP cache headers and OS sendfile, are trivially inspectable, and keep the SQLite database small. Each thumbnail is generated exactly once.
- Estimated footprint: ~15–40 KB per generated thumbnail; ~150–400 MB for a 10,000-file library with no embedded art.

## Cost Reference (for capacity reasoning)

| Operation | Approx cost |
|-----------|-------------|
| Embedded art extraction | Negligible (byte copy during existing file open) |
| Frame thumbnail, H.264 1080p | ~50–150 ms CPU |
| Frame thumbnail, HEVC 4K | ~200–500 ms CPU |
| At 2 workers, 10k files w/o art | ~10–20 min of throttled background work, spread out |

## Consequences

- ffmpeg is required in the image (already true per ADR-004/ADR-007).
- The generation queue is an in-process component (single-process deployment per ADR-008); queue depth is exposed as `holodex_thumbnail_queue_depth` in Phase 2 Prometheus metrics.
- The priority bump (Tier 3) requires the frontend to report visible item IDs — implemented via the existing list query (the API knows which IDs a page returned and can enqueue-with-priority server-side, so no extra client call is needed).
- This ADR supersedes the placeholder reference to "ADR-006 (Thumbnail strategy)" in the Phase 2 spec; the correct reference is ADR-009.
- Phase 1 implements Tier 1 (embedded art) only. Tiers 2 and 3 are Phase 2 (consistent with thumbnail generation being a Phase 2 feature, F11).

---

## Implementation status (2026-06-11)

Correcting the consequence above: **Tier 1 was *not* built in Phase 1.** The
Phase-1 codebase shipped with no thumbnail column, no serving endpoint, and
cover-art keys explicitly excluded from extraction (`internal/metadata/extractor.go`).
All three tiers were therefore implemented together in Phase 2 (F11):

- **Storage path** is `DATA_PATH/thumbnails/{id}.jpg` — the ADR body's
  `DATABASE_PATH/thumbnails` reference is superseded by [ADR-014](ADR-014-configuration-and-data-layout.md).
- **State** is tracked in a single `videos.thumbnail_state` column
  (`NULL`→`embedded`/`generated`/`failed`). A transient `failed` is retried by the
  one-shot startup backfill sweep; the list hot-path only enqueues never-attempted
  (`NULL`) items, so a broken file is not re-attempted on every browse.
- **Queue depth** (F11.8) is exposed as `thumbnail_queue_depth` on
  `GET /api/v1/admin/status`; the full Prometheus `/metrics` endpoint is deferred
  with the rest of Phase 2 observability (F13).
- **`nice`/`ionice`** are applied only on Unix (skipped on Windows, where they
  don't exist); generation runs unthrottled when they're absent.
- Lives in `internal/thumbnail/`; the scanner calls it for Tier 1 + new-file
  enqueue, the API for Tier 3 priority bump + serving.
