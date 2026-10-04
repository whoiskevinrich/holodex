# Writeback

This doc owns how a resolved value reaches a media file and how Holodex knows whether the file agrees:
the copy → write → rename model and its write backends, the `formatMap` write targets, the durable
write queue and its status, the post-write read-back, snapshots and revert, the tag-key file contract,
tag deletion for a cleared field, and the sync-state model. Which value is decided is
[field-resolution.md](field-resolution.md); which media tools ship, and how their presence selects the
Matroska backend, is [deployment.md](deployment.md); the forced re-extract reused here is
[media-ingest.md](media-ingest.md). Every writeback route sits behind `requireOwner`
([security-perimeter.md](security-perimeter.md)).

## Copy → write → rename on a same-directory temp file, one tool invocation per job

A write never touches the original until the end. `writeback.WriteBatch` copies the file to a sibling
`<file>.holodex-tmp`, writes every field of the job onto that copy in **one** tool invocation, and
renames the temp over the original. On any error the temp is deleted and the original is untouched.

- The temp sits in the same directory so the rename is one same-volume, atomic `rename`.
- A write needs free space equal to the file's size and costs a full-file read and write, whatever
  the backend. A job lands whole or not at all; per-field success is not a reportable state.
- Tools get argv (no shell); the path comes from the `videos` row, never from a request.

**Rejected:** editing the original in place, or relying on exiftool's `-overwrite_original` alone —
either can leave a half-written original after a crash or OOM.

Decided in [`66d2a04a`](https://github.com/whoiskevinrich/holodex/commit/66d2a04a).

## Matroska: mkvpropedit edits the temp copy in place; ffmpeg remux is the fallback

`writeMKVBatch` uses MKVToolNix when it is installed (selection rule: [deployment.md](deployment.md)),
else `writeMKVWithFFmpeg` (`-map 0 -c copy -map_metadata 0` into a new file). WebM takes the same path.

- mkvpropedit edits the temp's Segment Info title, global `TAGS` and attachments, so streams,
  chapters, editions, track-level tags and other attachments survive byte-for-byte.
- `TAGS` is merged via `mkvextract` + `mergeTagsXML`: existing Simple tags stay verbatim; a rewritten
  field replaces its old Simple case-insensitively. A Title `Delete` drops the Segment Info title.
- The ffmpeg path stays maintained and tested: it is the path wherever MKVToolNix is absent, and the
  rollback target.
- mkvpropedit may relocate `TAGS` past the Clusters, so the reader must follow the SeekHead.

**Rejected:** ffmpeg remux only — it rebuilds the container through ffmpeg's model of Matroska and
fails on attachments it cannot decode.

Decided in [`c7d7bd1e`](https://github.com/whoiskevinrich/holodex/commit/c7d7bd1e).

## A fragmented MP4 is remuxed in the copy step, with its tags restored

When `checkNotFragmented` finds a top-level `moof` in a `.mp4`/`.m4v`/`.mov`, `stageTemp` replaces the
copy with an ffmpeg stream-copy remux (`-map 0 -c copy -map_metadata 0 -movflags +faststart -f
<mp4|mov>`, container matching the extension). The one exiftool call then carries `-TagsFromFile <orig>
-all:all` **before** the batch's assignments, because ffmpeg drops XMP (where MP4 `edition` lives).
The remuxed temp is re-checked for `moof` before exiftool runs; rename is unchanged.
`ErrFragmentedMP4` (`writeback_unsupported_container`, wrapping ffmpeg's output) is only the fallback
when ffmpeg is missing or fails or the re-check fails. The file comes out progressive for good; revert
restores tag values, not byte layout.

**Rejected:** a background sweep remuxing every fragmented file — rewrites files nobody asked to
touch. **Rejected:** remux without `-TagsFromFile` — silently deletes XMP-held fields.

Decided in [`ea57b67c`](https://github.com/whoiskevinrich/holodex/commit/ea57b67c).

## `formatMap` is the single canonical → write-tag table

`formatMap` (`internal/writeback/tags.go`; `TagForField`, `ResolveForContainer`) maps each canonical
field to one tag per container family. A field with no row for the file's container is reported
unmapped, never silently skipped; multi-value fields are written as repeated assignments.
`edition` → `Edition` (Matroska/WebM) and `XMP-prism:Edition` (MP4/MOV), both read back as `Edition`.
MP4 `tagline` → `QuickTime:Description`, outside the scanner's tag keys. A new row's key is chosen by
writing and reading back generated clips on every container, and must read back under a key a
`file:` source can name.

Decided in [`99f61954`](https://github.com/whoiskevinrich/holodex/commit/99f61954).

## A durable `writeback_queue`, one job per file, enqueued only by explicit owner actions

Every production write goes through `internal/writequeue`. An enqueue inserts a `writeback_queue` row
(`status ∈ pending|running|failed`, `attempts`, JSON `payload`, `batch_id`) and returns `202` with the
job id. Workers run on the application-lifetime context, so no client action cancels a write.

- **One job per file** carries every field; the worker calls `WriteBatch` once.
- **Concurrency** is `WRITEBACK_CONCURRENCY` (default 1, fully serialized).
- **Batches.** `EnqueueMany` inserts one job per video under a shared `batch_id`, the key revert and
  batch status use.
- **Crash recovery.** At boot `RecoverRunningWritebacks` resets `running` to `pending` (the original
  is intact) and orphan `.holodex-tmp`/`.holodex-new` files are swept.
- **Success deletes the row; failure keeps it.** Nothing auto-retries; `RetryFailedWriteback` /
  `DismissFailedWriteback` act on one video's failed rows. `id` is `AUTOINCREMENT`, because a reissued
  id would collide with an earlier job's snapshot batch.
- Each job records a `kind=writeback` job run ([observability-and-jobs.md](observability-and-jobs.md)).
- The payload is data: the worker re-validates every tag name it acts on, whatever the API checked.
- **Entry points.** A decision PUT/DELETE (`/media/{id}/fields/{canonical}/decision`) changes what
  resolves, never the file. Jobs come only from `POST /media/{id}/writeback`, revert, tag sync
  (`/tags/{id}/writeback/sync`, `/tags/writeback/sync`; W recomputed per video, videos deduplicated
  across tags), merge propagation, extraction resolve, and the film-studio cascade (decide, then
  enqueue in the same request). Setting a tag's writeback flag never enqueues.

**Rejected:** an in-memory queue — loses writes on restart. **Rejected:** synchronous per-field
writes — no atomic batch, no backpressure (that branch survives only where no queue is wired).
**Rejected:** rule-based writeback on enrich — silent overwrites at a blast radius nobody chose.

Decided in [`dab68517`](https://github.com/whoiskevinrich/holodex/commit/dab68517).

## Writes are fire-and-forget; job status is a property of the video

The client never waits for a write. The media payload carries the video's writeback state
(`repo.GetVideoWritebackStatus` → `{pending, failed, error}`) from `writeback_queue` by `video_id`:
a `pending`/`running` row is pending, a `failed` row is failed, and **no row means nothing to report**
(succeeded, swept or never existed render the same). It survives reload and restart because it is
read from the rows the worker consumes. This holds only while `FinishWriteback` deletes on success and
failures persist until acted on. `GET /writeback/jobs/{id}` (absent reads `done`, safe only for a
poll seeded by a fresh enqueue) and `GET /writeback/batches/{batchID}/status` (`pending`/`running`
from the queue, `done`/`failed` from `job_runs.batch_id`) remain for callers holding an id.

**Rejected:** polling a job id held by the page — dies on reload, cannot show an earlier failure.
**Rejected:** server-sent events — a new transport for a single-owner server.

Decided in [`4dc2e51d`](https://github.com/whoiskevinrich/holodex/commit/4dc2e51d).

## Every successful write is read back through the scan path

The queue's `PostWriteFunc` (wired in `cmd/holodex/main.go`) runs after every successful write, with
no field or source gate: it re-extracts embedded cover art, then calls `refresh.Service.ReExtract`
([media-ingest.md](media-ingest.md)). The stored file layer is therefore post-write state and
`in_sync` is correct without a rescan. The file is re-read rather than patched from the values sent,
because the tools normalize (multi-value joining, tag aliasing). The read-back is also how a written
Person or Studio becomes an entity: `UpsertVideo` creates and links it through the scan path, and only
once the write landed.

**Rejected:** gating the read-back to entity fields — every other written field, and every
merge-propagated video, keeps asserting its pre-write value. **Rejected:** inserting a new entity
inline at resolve — a second create path that can disagree with the file.

Decided in [`339559c7`](https://github.com/whoiskevinrich/holodex/commit/339559c7).

## Prior values are snapshotted per batch; revert is a forward write

Before writing, the worker reads each mapped field's on-disk value (`writeback.ReadCurrentValues`)
into `file_writeback_snapshots` (`batch_id`, `video_id`, `field_key`, `prior_value`, `""` if absent)
under the job's `batch_id` or one derived from its id. Capture is idempotent per `(batch, video)`, so
a crash-recovered re-run cannot snapshot its own first attempt; a snapshot failure never blocks the
write. On success each mapped field gets a `file_writebacks` audit row (`field_key`, `tag_name`,
`value`, `source`, `written_at`), which is also a sync witness (below).

`POST /writeback/batches/{batchID}/revert` (`Queue.Revert`) enqueues one ordinary job per video with
the snapshotted values, so revert inherits atomicity and its own snapshot (undo of undo). A prior
`""` is skipped: reverting a write that added a tag is still a gap (HOLODEX-495).

**Rejected:** full per-field history — storage and diffing for a rare case. **Rejected:** `.bak`
files beside the media — disk sprawl with no history surface.

Decided in [`078626a2`](https://github.com/whoiskevinrich/holodex/commit/078626a2).

## The tag file contract: a genres write makes every tag key match the written set

Tags reach the file only through `genres`. The worker computes the written set W: `TagNamesForVideo`
(ancestor-expanded; the final projection drops a tag with `tags.writeback_enabled = 0` by its own flag
while the walk still climbs through it) unioned with the deny-filtered raw genres, minus ignored tag
names. Client-sent values are ignored.

- **Extra tag keys.** The writer reads `writeback.ExtraTagKeys` (`Genres`, `Keywords`, `Category`,
  `Categories`) with `exiftool -G1 -a`, keeps only values matching W by tag identity (`tagKeeper`),
  and rewrites each changed key under the name exiftool reported (bare on Matroska/WebM); an emptied
  key is deleted. It never adds values or creates keys, and filters at job time.
- Derived writes travel as `tagkey:<name>` fields, accepted by the worker only via `ValidTagKeyName`
  and refused outright from HTTP clients.
- Each changed key snapshots, audits and reverts **by tag name**; a revert job skips the filter.
- After a successful genres write and before the read-back, `PromoteIgnoredFileTags` turns the
  video's `source='file'` links to ignored tags into `manual`, so they stay on the video.
- An empty W deletes Genre; a delete with nothing to remove writes nothing.

**Rejected:** canonical fields per key — fake fields for keys the owner never edits.
**Rejected:** clearing or mirroring every key — destroys or duplicates other tools' data.

Decided in [`99f61954`](https://github.com/whoiskevinrich/holodex/commit/99f61954).

## A cleared field deletes every file tag it reads from, by bare name

A cleared decision ([field-resolution.md](field-resolution.md)) reaches the file only through an
explicit flag: a writeback entry `{field, clear: true}`, accepted only when the video's standing
decision is cleared, or a `writequeue.JobField{Clear: true}` from the film-studio cascade. An empty
value list still means "nothing to write".

- `buildBatch` expands a `Clear` (`writeback.ClearTagNames`) to one `Delete` for the write target plus
  one per un-namespaced source in the live mapping, so no fallback tag resurrects the value.
- **Every delete name is bare**: exiftool then deletes it from every group, matching the bare-name
  reader; a group-qualified delete misses XMP.
- **Every name passes `writeback.ValidClearTagName`**, fail-closed and re-run in the worker: shape
  `^[A-Za-z][A-Za-z0-9]*$`, never `all`, no group prefix on Matroska/WebM, and membership in the
  field's own write target or mapping sources. A bad name could become `-all=` and strip the file;
  a rejected name is dropped and named in the job-run detail. It is not `ValidTagKeyName`, and
  neither widens to serve the other.
- `markWriteTargets` stamps a cleared row with its write target so it is writable.

**Rejected:** deleting whenever values sanitize to empty — turns a safe skip into a destructive write.

Decided in [`4e2e782b`](https://github.com/whoiskevinrich/holodex/commit/4e2e782b).

## Sync state: tri-state `in_sync`, from file read-back or the write ledger

`in_sync` on a replace field is `true`, `false` or absent (unknown), computed in `replaceMarkers` from
pre-loaded inputs.

- **File read-back** is the default witness: `decided == fileVal` through a declared `file:` source
  (`baselineValue` keeps `declared` apart from `ok`). No declared source means unknown, never `false`.
- **The write ledger** witnesses what the file cannot answer: image fields (`Display: image_url`) and
  text fields in the read-back-gap set. `true` iff the newest successful `file_writebacks` row holds
  the decided value, `false` if another, unknown if none (`repo.LastWrittenValues` →
  `resolver.Options.LastWritten`). It cannot see edits made outside Holodex.
- **One gap definition.** `writeback.ReadbackGaps` (a field with no `file:` source matching its write
  tag, minus `UnreadableWriteTargets`) is computed by the API and passed in `Options`, so the resolver
  never knows containers. `LogReadbackGaps` warns at start and on `POST /admin/reload-config`;
  `GET /owner/readback-gaps` and a field's `readback_gap` expose it;
  `TestExampleMappingCoversWriteTargets` holds the shipped example mapping to it by key match.

**Rejected:** an implicit read-back source for the written tag — changes precedence (a bare `YEAR`
outranks a full provider date). **Rejected:** comparing against the write target in the resolver —
puts the container table in the pure resolver. **Rejected:** a `written_value` on decisions —
duplicates the ledger.

Decided in [`b72f35ea`](https://github.com/whoiskevinrich/holodex/commit/b72f35ea).

## The file's tag set is recorded at every extract; tags get a set-valued `in_sync`

`videos.file_tags` (nullable JSON array) holds every raw tag name the extractor read, including names
the link step skips. `UpsertVideo` writes it with the associations, so scan, refresh and read-back keep
it current. `NULL` is unknown, `[]` is read-and-empty; the scanner's unchanged-file fast path
(`StatByPath`) applies only once it is non-null, so old rows fill on the next scan.

On the media read the `genres` row diffs W against the file set F by the writer's own `tagKeeper`
identity rule: per-item `on_file`, `file_only` (F − W), and `in_sync`, absent when F is unknown. The
row is kept when W is empty but F is not. Each attached tag carries `written` and tri-state
`on_file`, computed at read time.

**Rejected:** ledger-only — no answer for never-written files. **Rejected:** `on_file` on
`video_tags` or a `video_file_tags` table — the former cannot hold denied or detached names, the
latter adds a join and a second rewrite per scan.

Decided in [`4e86e31f`](https://github.com/whoiskevinrich/holodex/commit/4e86e31f).
