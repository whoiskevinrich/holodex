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
- Per-file failures increment an error count and never abort the pass, but they do record the
  pass's job row as `error`, as do an unresolvable or unreadable media root (only a *subdirectory*
  is skipped) and an empty walk while active videos exist.

**Rejected:** content hashing — a full read of every file per pass is too costly at library scale.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## Moves recognised by a sampled content hash, matched after the walk

A row's identity is its id, not its path. The product rules are in
[video-identity-on-move.md](../specs/video-identity-on-move.md) (F79). This section covers the
evidence, where the match runs, and the fold.

- **The fingerprint.** `videos.content_fp` is a SHA-256 over the byte size followed by three 64 KiB
  samples: the start, the middle (offset `size/2`) and the end. A file shorter than 192 KiB is
  hashed whole. The worker computes it on every insert or re-extract, so a writeback, a flip swap or
  an outside edit refreshes it in the same pass that sees the size or mtime change. An unchanged file
  whose row has no fingerprint is fingerprinted without re-extraction. That is the lazy backfill for
  active rows: about 750 MB of seeks-and-reads across a ~4,000-file library, once.
- **Hold, then match.** A file with no row for its path is extracted and fingerprinted as usual. If
  any non-deleted row already has the same `file_size`, the result is **held** instead of inserted.
  Files with no size twin insert at once, so a first scan holds nothing. After the walk, the seen set
  defines the missing set: rows that are active but unseen, plus rows with `active = 0`, never
  soft-deleted ones. Each held file is paired with missing rows whose fingerprint matches. A
  one-to-one pair is repointed: `file_path` and the extracted facts are updated, `active = 1`, and
  `indexed_at` is untouched. Anything else inserts as new and counts as ambiguous. Repointing runs
  before `DeactivateExcept`, so a repointed row is never hidden.
- **Legacy rows.** A hidden row from before fingerprints existed has `content_fp` NULL, and its file
  is gone, so it can never get one. Such a row matches only when every recorded fact is equal: byte
  size, second-truncated mtime, duration, the stream facts (codecs, width, height, bitrate,
  container) and the captured file tags. Because only those rows have a NULL fingerprint, this rule
  retires itself as they heal or are purged.
- **The fold.** After deactivation, a reconcile query pairs each hidden row with active rows of a
  *higher* id carrying the same evidence. Pairs that are one-to-one on both sides, with no
  `entity_keep_separate('video', …)` row, are folded into the older id in one transaction under
  `writeMu` (`FoldMovedVideo`), and the newer row is then hard-deleted. A pair is held back
  while either side has a pending or running `writeback_queue` entry or a `held_originals` row;
  those jobs carry the id and the path, and a later pass folds it once they settle. The survivor
  keeps its own `indexed_at`; the newer row's insert time is the move, which is not an update.
  Everything is re-keyed *before* the delete, because the cascades and the `videos_ad_*`
  triggers remove whatever is still keyed to the newer id. The carry reuses `applyCarry` from
  duplicate videos ([entity-identity.md](entity-identity.md#duplicate-videos-are-computed-on-demand-not-queued))
  with these differences:
  - every field the newer row has an edit on goes to the newer row: the older row's edits for that
    field are cleared first. The older row was hidden from the move until now, so every edit on the
    newer row is the more recent one. Part and edition carry too, since it is the same file;
  - enrichment rows move too, and for each provider the side with the newer `fetched_at` wins
    whole, so one provider's fields never mix two fetches;
  - dismissals, writeback history and snapshots, extraction reviews and job attribution move to
    the older id.

  The survivor keeps its own thumbnail. The newer id's thumbnail files are left on disk, the same
  as purge leaves them.
- **Reporting.** The pass's `job_runs` row carries the counts in its `detail` text (`moveDetail`),
  which the activity history already renders under a scan's numeric columns, so no new column
  or migration is needed ([observability-and-jobs.md](observability-and-jobs.md)). An
  ambiguous file declined by the scan-time match is not counted again when it reappears as an
  ambiguous fold pair in the same pass.

**Rejected:**

- **Full-file hash.** It reads every byte. The backfill would read the whole library, and so would
  every writeback refresh. This is the same cost that rules content hashing out of change detection
  above.
- **Stream-only (packet) hash via ffmpeg.** It survives metadata writeback, but it demuxes the whole
  file. Recomputing the sampled hash on each in-place change covers writeback more cheaply.
- **Size plus stream facts alone for every row.** The spec rules this out as evidence for rows that
  can carry a fingerprint. It survives only as the legacy rule, where it is the most that exists.
- **Insert, then fold.** One code path, but it creates a transient row for every move. Thumbnail
  and enrichment work would be queued for an id about to vanish, and it contradicts the spec's "no
  new video".
- **Stat the old path at match time.** No holding is needed, but a move plus a copy in one pass would
  be decided by worker order, which breaks the spec's ambiguity rule.

Decided in: pending squash (HOLODEX-568).

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

**A small Go reader enforces it, because neither tool can.** exiftool's Matroska module ignores
`TargetTypeValue`: only a `TagTrackUID` changes a tag's group, so one occurrence of a key wins
whatever its level. Which one has changed between versions: the first in older ones, the last in
13.25 (what the runtime image ships) and 13.59. ffprobe's `format_tags` is no better; it also ignores the level, and
there the *last* occurrence wins. Holodex's selection must not depend on either order. So for Matroska files
`internal/metadata/matroska.go` walks the Segment's level-1 elements, skipping each by size so
Clusters are never read, and decodes `Info` and `Tags` with each SimpleTag's level and binding.
`selectMatroskaTags` then rewrites exiftool's flat map by the level rule the spec states
([phase-1-mvp.md](../specs/phase-1-mvp.md), open question 2) before `mapExiftool` sees it:

- "Track/chapter tags ignored" is read as any non-zero `Tag*UID`: track, edition, chapter or
  attachment.
- A multi-value key gets every level-50 value joined, so several `ACTOR` SimpleTags all survive,
  which exiftool's flattening dropped. A substituted `DATE_*` value is given exiftool's
  `YYYY:MM:DD` form, so it parses the same.
- An `Info` element exiftool gives the same key (`Title`; `DateUTC` as `DateTimeOriginal`) keeps
  exiftool's value. Info outranks standard tags there too.
- Keys that match no SimpleTag pass through untouched. If the reader can't walk the file (an
  unknown-size Cluster, truncation), exiftool's values are used as before.

Track-bound statistics tags (`BPS`, `NUMBER_OF_FRAMES`, ffmpeg's per-track `ENCODER`) therefore no
longer reach `extra_metadata`.

**Rejected:** `exiftool -G1` and dropping `Track#` groups. It fixes track-level leaks but leaves
collection, season and chapter tags indistinguishable from the episode's own. ffprobe tags: same
blindness to level, opposite winner. A third-party EBML library: the reader needs about a dozen
element IDs, and the parse is bounded (16 MB per element) and read-only.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf); target-level
selection in [`7a59cde6`](https://github.com/whoiskevinrich/holodex/commit/7a59cde6) (HOLODEX-536).

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
