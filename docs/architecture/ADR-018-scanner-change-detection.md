# ADR-018: Scanner Change Detection — Incremental Scan by (path, size, mtime)

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Re-extracting metadata from every file on every scan cycle is infeasible at 50k+ files — each file costs two subprocess calls (exiftool + ffprobe, ADR-004). Scans must be incremental: only files that are new or changed should be re-extracted. The scanner must also handle removals and files that are mid-copy.

## Decision

Detect changes by comparing **(canonical path, file size, modification time)** against the stored record; re-extract only on a mismatch.

### Scan algorithm
1. Walk `MEDIA_PATH` (following symlinks, dedup by canonical path — ADR-011).
2. For each discovered file, compare `size` + `mtime` against the existing `videos` row for that canonical path:
   - **No row** → new file → extract + insert.
   - **Row exists, size/mtime unchanged** → skip (no extraction).
   - **Row exists, size or mtime differs** → re-extract + update.
3. Any active row whose canonical path was **not** seen during the walk → mark `active = false` (removed/unavailable).

### Mid-copy protection
- A file whose `mtime` is within a short quiet-period threshold (`SCAN_MIN_AGE_SECONDS`, default 5) is **skipped this cycle** and picked up next cycle, avoiding indexing partially-written files.

### Stored fields
`videos` already carries `file_size` and `file_mtime` (Phase 1 data model) for exactly this comparison.

## Rationale

- **(size, mtime)** is the standard, cheap change signal — a `stat` per file, no content hashing. It catches re-encodes, tag edits, and replacements.
- **Content hashing is deferred.** It is more robust against mtime-preserving edits but costs a full file read per file — unacceptable at scale. Available later as an optional `SCAN_VERIFY=hash` mode if needed.
- **Quiet period** prevents the classic "indexed a half-copied file" failure on network shares.
- **Soft-delete via `active`** keeps history and makes re-appearing files (remounted drive) cheap to reactivate.

## Consequences

- Steady-state scans are dominated by `stat` calls (fast); extraction only runs for genuine changes.
- The filesystem watcher (F1.5) feeds the same comparison logic for individual change events; the periodic scan is the reconciliation backstop.
- Orphaned people/tags (no remaining active videos) are left in place by default; optional cleanup is a minor future task.
- `holodex_scan_duration_seconds` (Phase 2 metrics) will reflect the cheap steady-state cost, not the initial full extraction.
