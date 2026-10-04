# Media ingest

This doc owns how media files become `videos` rows and how their bytes are served: the scanner
(walk, change detection, symlink policy, workers), metadata extraction, thumbnail and poster
generation and storage, HTTP Range streaming, soft-delete with its `active`/`deleted_at` read seam
and purge job, and the per-item forced refresh. Which media tools ship in the image is
[deployment.md](deployment.md); how captured tags map to canonical fields and merge with providers
is [field-resolution.md](field-resolution.md); the config keys named here are resolved as described
in [config-and-settings.md](config-and-settings.md); job-run recording is
[observability-and-jobs.md](observability-and-jobs.md).

## Incremental scan keyed on (canonical path, size, mtime)

`internal/scanner` walks `MEDIA_PATH` and feeds canonical file paths to a bounded worker pool
(`scan_workers`). Each worker `stat`s the file and compares size and second-truncated mtime with the
row from `repo.StatByPath`: no row → extract and insert; unchanged → skip with no subprocess;
differs → re-extract and `UpsertVideo`. At the end of a pass, `DeactivateExcept` sets `active = 0`
on every row not seen.

- **Triggers.** An initial pass, a periodic ticker (`scan_interval_seconds`), an `fsnotify`
  watcher that only nudges a debounced full pass (it carries no event detail), and an owner
  rescan. All are serialized by `scanMu`; the manual trigger uses `TryLock`, so repeats are dropped.
- **Mid-copy guard.** A file modified within `scan_min_age_seconds` is skipped this pass but still
  recorded as seen, so it is not deactivated.
- **Empty-walk guard.** A pass that sees zero media files skips deactivation, so a transiently empty
  mount cannot hide the library. An unchanged file on an `active = 0` row is reactivated without
  re-extraction.
- Per-file failures increment an error count and never abort the pass.

**Rejected:** content hashing — a full read of every file per pass is too costly at library scale.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## Symlinks followed and deduplicated by canonical path

With `follow_symlinks` on (the default), every entry is resolved with `filepath.EvalSymlinks`, and
per-pass visited sets of canonical directories and files make each real file indexed once and
symlink loops terminate. `scan_max_depth` is a secondary recursion backstop. Targets outside
`MEDIA_PATH` are followed and indexed, so several drives can be stitched into one library. The
canonical path is what is stored in `videos.file_path`. With following off, symlinked entries are
skipped. Hardlinks have distinct canonical paths and are indexed as separate rows.

The traversal guarantee does not depend on `MEDIA_PATH` containment: every byte-serving endpoint
looks up the stored path by video id, and no client ever supplies a filesystem path.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## exiftool for tags, ffprobe for stream facts

`metadata.Extractor` runs two subprocesses per file, both with argv (no shell) and an absolute path,
so a filename cannot be parsed as a flag.

- **exiftool** (`-j -api largefilesupport=1`, plus `-ee` for Matroska/WebM so it reads level-1
  elements relocated past the first Cluster) is the only tag source. It produces title, people,
  tags, recording date and the cover-art presence flag. Every other human-meaningful tag goes to
  `Extra`, which lands in `video_metadata` (see [field-resolution.md](field-resolution.md)).
- **ffprobe** (`-show_format -show_streams`) supplies width, height, duration, codecs, bitrate and a
  normalized container label. Its values override exiftool's for those fields. An ffprobe failure
  leaves them empty and does not fail the extract.
- Matroska language suffixes (`Title-und`) are stripped from keys, so MKV SimpleTags and MP4 atoms
  share one key space. An untagged title falls back to the filename stem, a missing date to mtime.

**Rejected:** ffprobe alone — its `format_tags` coverage of encoder-specific tag locations is too
shallow.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## Only container-level Matroska tags describe the video

The intent: per-video fields come from the file's own (MOVIE/EPISODE, target 50, or untargeted)
tags; track-level tags (target 30), such as an audio track's `TITLE`, never become the video's
title, people or tags; and multi-value fields (people, tags) never inherit from collection or season
levels, which would smear a series' cast across every episode.

**What enforces it today is incidental, not a target-level filter.** Stream tags are never read
(ffprobe is consulted only for stream technical facts), and inside exiftool's flattened output the
first value seen for a single-value key wins. No code inspects Matroska target levels, so a file
whose first occurrence of a key is track- or chapter-level would map that value. Making this
explicit is open as HOLODEX-536.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## Tiered thumbnails on disk, generated by a throttled priority queue

`internal/thumbnail.Manager` is one in-process instance shared by the scanner and the API. The
per-video lifecycle is `videos.thumbnail_state`, which is one of: empty (never attempted),
`embedded`, `generated`, `uploaded` or `failed`.

- **Tier 0:** an owner-uploaded poster (`uploaded`). Sweeps never replace it.
- **Tier 1:** embedded cover art, pulled with `exiftool -b` at index time when the extractor flagged
  it.
- **Tier 2:** files without art are enqueued for an ffmpeg frame at `thumbnail_seek_percent` of the
  duration. The source is decoded once and scaled into both tiers.
- **Tier 3:** list responses push visible never-attempted items to the front of the queue
  (`EnqueueHigh`). Previously failed items are left to the startup sweep, so a broken file is not
  retried on every browse.
- Workers are bounded (`thumbnail_workers`). On Unix, ffmpeg runs under `nice` and `ionice -c 3`;
  on Windows it runs unthrottled. `thumbnail_backfill` selects `eager` (sweep the library) or
  `lazy` (only viewed items).
- **Storage:** `DATA_PATH/thumbnails/{id}.jpg` (`thumbnail_width`) and `{id}-poster.jpg`
  (`poster_width`) are written atomically, once. The disk file is the cache.
- **Serving:** by id, after `repo.VideoVisible`. Responses use `http.ServeContent` with
  `Cache-Control: no-cache`, so unchanged images return 304. A missing file is a 404, which the
  client treats as "not ready yet".

**Rejected:** DB BLOBs — they bloat SQLite and lose sendfile and validators.
**Rejected:** generating at view time — it moves the CPU spike to the moment a grid opens, adding
latency.

Decided in [`59798d1b`](https://github.com/whoiskevinrich/holodex/commit/59798d1b).

## Video bytes streamed by id through `http.ServeContent`

`GET /api/v1/media/{id}/stream` resolves the path with `repo.PathByID`, opens it as an `*os.File`
and hands it to `http.ServeContent` with the file's mtime. The standard library handles `Range`,
`If-Range`, `206`/`Content-Range`, `Accept-Ranges` and conditional requests, and `Content-Type`
comes from the extension. Large files are never buffered whole. This governs delivery only: there
is no transcoding, so codec support is the browser's.

**Rejected:** a hand-rolled read-and-write handler — it breaks seeking and gets range math wrong.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## Soft-delete as a `deleted_at` axis orthogonal to `active`

`videos.deleted_at` (nullable ISO-8601 UTC, indexed) records owner delete intent. `active` records
disk presence and belongs to the scanner. A row is library-visible iff
`active = 1 AND deleted_at IS NULL`. `purge_at = deleted_at + delete_grace_period_seconds` is
computed and never stored, so changing the grace period reflows every pending purge.

- **The scanner never touches intent.** `index` short-circuits a soft-deleted row before change
  detection and records it as seen, so it is neither reactivated nor re-extracted.
  `UpsertVideo`'s update set omits `deleted_at`. Only the API sets or clears it; only purge removes
  the row.
- **One read seam.** `VideoFilter.build()` carries the visibility clause, so every list, count,
  search, facet, related or MCP surface built on it inherits the clause. Direct-by-id reads that
  bypass it (`GetVideo`, `PathByID`, `VideoVisible`) apply `deleted_at IS NULL` themselves, so a
  trashed item 404s when fetched, streamed or thumbnailed. Purge-side reads (`ExpiredSoftDeleted`,
  `PurgePath`) are the only code that sees soft-deleted rows. Tests enumerate every surface.

**Rejected:** storing delete in `active` — the next scan of a still-present file would reactivate
it.
**Rejected:** a separate deletion table — it adds a join to every read for no extra expressiveness.

Decided in [`6d3cb986`](https://github.com/whoiskevinrich/holodex/commit/6d3cb986).

## A dedicated `internal/purge` ticker hard-deletes expired items

`purge.Purger` runs its own ticker (`purge_interval_seconds`), independent of the scan clock, and
also runs when scanning is disabled. Each pass records a `kind = purge` job run. For each item it
removes the file first (when `delete_remove_files`), then deletes the row. `ON DELETE CASCADE` and
the FTS delete trigger clean up the junction tables and the search index. A file that is already
missing counts as success. A permission failure leaves the row soft-deleted, counts an error and
retries on the next tick, so the row is never deleted while its file survives. A grace period of 0
disables automatic purge.

**Rejected:** folding purge into the scanner — the two run on different clocks, and purge must run
with scanning off.

Decided in [`6d3cb986`](https://github.com/whoiskevinrich/holodex/commit/6d3cb986).

## Per-item refresh: forced re-extract plus re-enrich as one `internal/refresh` operation

`refresh.Service.Refresh(id)` resolves the target (`repo.RefreshTarget`, distinguishing missing from
soft-deleted). It then re-reads the file through `Scanner.BuildVideoFromFile`, which has no size or
mtime check, so tag edits that preserve mtime are caught. The file layer is committed with
`UpsertVideo` before any provider call. Each provider the item is already matched to is then
re-enriched through its stored external id, never through a picker. The result is one typed
`Report` and one `kind = refresh` job run.

- A file-read failure aborts before any write; a provider failure affects only its own result. Each
  layer is written only to its own store (the invariant is [field-resolution.md](field-resolution.md)'s).
- A refresh does not take `scanMu`, so it never waits behind a library pass; racing a scan is
  redundant work, not corruption, since both write the same derived rows under `writeMu`.
- `ReExtract(id)` is the file-only half with no job run. The post-write read-back uses it (see
  [writeback.md](writeback.md)).

**Rejected:** composing the operation in the handler — one click would scatter 1 + N job rows and
leave no single report.
**Rejected:** a forced full rescan — it re-walks the whole library to sync one file.

Decided in [`7a5c03c0`](https://github.com/whoiskevinrich/holodex/commit/7a5c03c0).
