# File maintenance

This doc owns how Holodex rewrites a media file's **picture** and leaves everything else intact: the
re-encode pipeline per container, the encoder table, the verification gate, the two-rename swap and
its crash recovery, held originals and their expiry, and how maintenance jobs share the write queue
with tag writes. The behaviour the owner sees is in the
[video file maintenance spec](../specs/video-file-maintenance.md) (F77). The tag-write model reused
here (same-directory temp, the durable queue, read-back after write) is [writeback.md](writeback.md).
Which media tools ship is [deployment.md](deployment.md). Every maintenance route sits behind
`requireOwner` ([security-perimeter.md](security-perimeter.md)).

## A separate topic from writeback, sharing its queue

Writeback moves a *resolved value* into a file, and its contracts (`formatMap`, sync state,
snapshots) are about tags. Maintenance moves *no* values. It replaces one stream and has to prove that
nothing else changed. The two share the durable queue (next section), the same-directory temp file
and the post-write read-back. They share none of the tag machinery, so they are separate docs.

**Rejected:** folding maintenance into `writeback.WriteBatch` as an "operation" field. Every tag-write
invariant (one exiftool invocation per job, snapshot-before-write, revert as a forward write) would
need a maintenance exception, and a re-encode has no snapshot it could revert to.

Decided in _(pending: HOLODEX-556 squash)_.

## One `writeback_queue` with a `kind`, two worker lanes, and one running job per file

A maintenance job is a `writeback_queue` row with `kind = 'maintenance'`. Existing rows default to
`kind = 'write'`. The payload names the operation (`hflip`, later aspect-ratio ops) and, for held
originals, `restore` or `discard`. Because the job lives in the same table, it inherits what the queue
already gives a tag write:
- durability and crash recovery (`running` → `pending` at boot);
- `job_runs` history (`kind=maintenance`);
- Retry and Dismiss on failed rows;
- the rule that job status is a property of the video. The media payload's writeback state gains the
  row's `kind` so the page can tell "flipping" from "writing to file".
- **Visitors see none of it.** The payload's writeback state is visible to visitors today, with only
  `Error` blanked by `redactWritebackStatusForVisitor`. That helper is extended so a non-owner gets
  no `kind`, no pending or failed state from a maintenance row, and no held-original block. A
  maintenance row is invisible to them, and `held_path` (an absolute filesystem path) never leaves
  the server for a visitor, which is the same rule as `redactFileMetadataForVisitor`.

- **Two lanes.** Tag-write workers claim only `kind='write'`. One dedicated maintenance worker claims
  only `kind='maintenance'`. That worker's concurrency is fixed at 1, because a re-encode saturates
  the CPU. A re-encode can take an hour, and tag writes for *other* files must not queue behind it.
- **One running job per file.** In both lanes, a claim skips any pending row whose `video_id` already
  has a `running` row of either kind:
  `… AND video_id NOT IN (SELECT video_id FROM writeback_queue WHERE status='running')`.
  The claim already runs under `writeMu`, so the check and the `running` update are atomic. A tag
  write enqueued during a flip waits; it is never refused. The reverse also holds.
- **Refused at enqueue:** a second `maintenance` row for a video that already has a pending or running
  one.
- **Shutdown** cancels the ffmpeg process through the application-lifetime context. The job returns
  to `pending` at boot and starts again from the original. A re-encode cannot resume midway.

**Rejected:** a separate `maintenance_queue` table. It would duplicate recovery, retry, dismiss and
`job_runs` wiring, and the page would need two reads for "is anything happening to this file".
**Rejected:** running maintenance in the tag-write pool. With the default concurrency of 1, one flip
would stall every tag write in the library for its whole duration.

Decided in _(pending: HOLODEX-556 squash)_.

## The re-encode: ffmpeg makes the picture, the container's own tool rebuilds the file

ffmpeg re-encodes the video, but it is never trusted to carry the rest of the file. Its model of each
container drops or rewrites things the output contract requires: MP4 XMP, Matroska attachments it
cannot decode, and its own `encoder` tags. So each container goes through two steps.

- **Matroska (`.mkv`).**
  1. ffmpeg encodes **only the main video track** (`-map 0:v:<n> -vf hflip -c:v <encoder>
     -map_metadata -1 -map_chapters -1`) into an intermediate file in the same directory.
  2. `mkvmerge` builds `<file>.holodex-new` from the **original**, with that track removed (`-d !<n>`),
     plus the intermediate's video track. The intermediate is added with `--no-global-tags
     --no-track-tags --no-chapters`. `--track-order` puts the new track in the original's position.
     Its track name, language, default and forced flags are copied from the original track.

  Every other track, attachment, chapter, edition and global or track tag comes from the original
  byte for byte. mkvmerge also regenerates the `_STATISTICS` tags, which is the recompute the spec
  asks for. **MKVToolNix is therefore required for Matroska maintenance.** Without it, the preflight
  refuses. There is no ffmpeg-remux fallback here, because that fallback is exactly what fails on
  attachments.
- **MP4 (`.mp4`, `.m4v`, `.mov`).**
  1. ffmpeg writes `<file>.holodex-new` with every stream mapped explicitly (`-map 0:<i>` each),
     `-c copy -c:v:<n> <encoder> -filter:v:<n> hflip -map_metadata 0 -map_chapters 0 -fflags
     +bitexact -movflags +faststart`, matching the extension's container.
     - Cover art is an `attached_pic` video stream, so it is excluded from the filter by index
       and copied.
     - The QuickTime chapter track (a `data` stream tagged `text`) is **not** mapped when the
       file has chapters. `-map_chapters` regenerates it in the same position, and mapping the
       source copy as well writes it twice (seen with two or more chapters).
  2. exiftool runs `-TagsFromFile <original> -XMP:all` on the output: ffmpeg's `-map_metadata`
     carries the iTunes/udta tags but drops XMP, where MP4 `edition` lives. Not `-all:all`,
     which also turns the container's brand fields into new `Keys` tags the original never had.
- `<n>` is the first video stream that is not `attached_pic`. A file with no such stream is refused at
  preflight.
- The scanner admits only `.mp4` and `.mkv` (`scanner.mediaExts`), so those two families are the
  whole surface. WebM and others never reach the page.
- ffmpeg runs under `nice`/`ionice -c 3` on Unix (`thumbnail.wrapNice`), with argv only. Every path
  comes from the `videos` row, never from a request, and every path argument to ffmpeg, mkvmerge and
  exiftool goes through `ffmpegArg()`, so a name can never be read as an option or an `@` option
  file.
- **The input is opened as the file it claims to be**, the way `fragments.go` does it:
  `-nostdin -protocol_whitelist file` and a forced demuxer (`-f matroska` / `-f mov`). A playlist
  disguised as `.mkv` can't make ffmpeg fetch URLs or pull in other local files.
- **The sibling files are never followed.** Before writing `.holodex-new`, renaming to
  `.holodex-orig`, or restoring from it, the worker `Lstat`s the name and refuses a symlink. The
  preflight probe is created with `O_CREATE|O_EXCL`, never `os.Create`, which truncates.

**Rejected:** ffmpeg alone with `-map_metadata 0`. It drops XMP-held fields and fails on Matroska
attachments, which breaks output-contract rules 3 and 4.

Decided in _(pending: HOLODEX-556 squash)_.

## A fixed encoder table: same codec, constant quality, source colour carried through

`internal/maintenance/encoders.go` maps a source codec to one encoder invocation. The table is code,
not config, because each row is validated against the verifier.

| Source codec | Encoder | Rate control |
|---|---|---|
| H.264 | `libx264 -preset slow` | `-crf 16` |
| HEVC | `libx265 -preset slow` | `-crf 18` |
| VP9 | `libvpx-vp9 -row-mt 1` | `-crf 20 -b:v 0` |
| AV1 | `libsvtav1 -preset 6` | `-crf 22` |

- Every invocation pins `-pix_fmt`, `-color_primaries`, `-color_trc`, `-colorspace` and `-color_range`
  to the ffprobe values of the source stream.
- Frame timing passes through with `-fps_mode passthrough`, never `-r`. Forcing a rate would drop
  or duplicate frames on a variable-frame-rate source and break the length rule.
- HEVC HDR10 static metadata (mastering display, MaxCLL) is passed through `-x265-params` from the
  source's side data.
- **Refused at preflight:**
  - a codec outside the table (VC-1, MPEG-2, MPEG-4 Part 2 and so on);
  - an encoder the installed ffmpeg lacks (checked with `ffmpeg -encoders` once at boot);
  - a Dolby Vision stream, whose RPU cannot survive a re-encode.
- **No bitrate band** (spec Q2). Constant quality is the rule, and the verifier does not compare
  bitrates. A flip of an over-compressed source may produce a larger file. The free-space preflight
  allows `1.5 × source size`, which is the only size assumption.

**Rejected:** matching the source bitrate with two-pass encoding. It is twice as slow and gives no
quality guarantee: a low-bitrate source would come out visibly worse. **Rejected:** lossless
(`-crf 0` / `-qp 0`). Files would grow 5–20×, which defeats the free-space preflight on any real
library.

Decided in _(pending: HOLODEX-556 squash)_.

## The verification gate compares probes, not exit codes

Before any rename, `maintenance.Verify(original, candidate, op)` compares:
- `ffprobe -show_format -show_streams -show_chapters -of json`;
- `exiftool -j -a -G1 -struct`, normalised.

One rule maps to each output-contract clause in the spec:
- stream count, order, type, language, title and disposition are equal;
- copied streams are byte-equal by packet hash (`ffprobe -show_packets` digest);
- every tag key and value is equal, except `_STATISTICS`-listed tags on the re-encoded track and
  container writing-application fields;
- chapters are equal;
- on the re-encoded stream, codec, width, height, pixel format, colour fields and frame rate are
  equal;
- durations agree within one frame.

A failure names its rule, becomes the job's error, and deletes `.holodex-new`. The original is
untouched, because no rename has happened yet. Each operation declares which rules it relaxes. Flip
relaxes none. A future aspect-ratio operation relaxes width and height, or sample aspect ratio, and
nothing else.

**Rejected:** trusting the tool's exit status. Both ffmpeg and mkvmerge exit 0 while dropping
metadata they don't model, and that silent loss is the one outcome the spec forbids.

Decided in _(pending: HOLODEX-556 squash)_.

## The swap is two same-directory renames, journaled in `held_originals`

The original is **renamed**, never copied, to a sibling `<file>.holodex-orig`. The verified output is
then renamed into place. Both renames happen inside one directory on one volume, so each is atomic
and instant whatever the file size. A held original therefore costs no extra disk beyond the file
that already existed.

`held_originals` (`video_id` PK, `held_path`, `op`, `state ∈ swapping|held|restoring|discarding`,
`created_at`, `expires_at`) is the journal:

1. Insert or update the row with `state='swapping'`.
2. Rename the original to `.holodex-orig`.
3. Rename `.holodex-new` to the original path.
4. Set `state='held'`.

- **Boot recovery runs before the orphan sweep.** It replays each `swapping` row:
  - original path present and `.holodex-new` present → the swap never started; drop the row;
  - original path missing, `.holodex-orig` present and `.holodex-new` present → finish step 3;
  - `.holodex-new` gone and the original path missing → move `.holodex-orig` back.

  The sweep then removes the remaining `.holodex-new`/`.holodex-tmp` files as it does today. It never
  touches `.holodex-orig`.
- **Second operation while one is held.** The swap skips step 2. The new output is renamed straight
  over the current file and the existing `.holodex-orig` stays, so restore always returns the file to
  its state before maintenance began. `expires_at` is unchanged.
- **Restore** is a `maintenance` job (so it serialises with tag writes): set `restoring`, rename
  `.holodex-orig` over the current file, delete the row. **Discard** ("Keep change") is a
  `maintenance` job that deletes `.holodex-orig` and then the row.
- **Expiry** is a periodic pass (`kind=held-expiry` in `job_runs`, on the purge interval). It
  **queues** a `discard` job for every `held` row past `expires_at`, rather than deleting files
  itself, so a discard serialises with any other job on the same file exactly like Keep change.
  A video whose lane is busy is picked up next pass. A pass that queued or failed nothing records
  no run. A file that cannot be deleted leaves its row `held` and is retried on the next pass. The window is `maintenance_hold_days`
  (default 7) in `holodex.yaml`.
- The scanner admits only `.mp4`/`.mkv`, so `.holodex-orig` is never ingested. This is a test, not an
  assumption: adding the suffix to `mediaExts` must fail it.
- **Purge honours `delete_remove_files`.** The purge job reads `held_originals` before it removes
  the video row:
  - **On (the default):** both the current file and `.holodex-orig` are unlinked.
  - **Off:** purge deletes no media file, so `.holodex-orig` is renamed back over the current file
    and the file left on disk is the unflipped original. Either way the row goes with the video.
- `held_path` is never stored from a request. It is derived as `file_path + ".holodex-orig"`, and
  restore, discard, expiry and purge each re-derive it and refuse if the stored value disagrees.

**Rejected:** a central `.holodex/held/` directory at the library root. The library can span mounts,
so a hold could become a cross-volume copy (slow, and twice the space) and a restore could fail
halfway. **Rejected:** holding originals under `/data`. That is always a different volume from
`/media`, so every hold is a full copy.

Decided in _(pending: HOLODEX-556 squash)_.

## Preflight is computed on request, not cached

`GET /media/{id}/maintenance/preflight?op=hflip` returns `{ok, reason, need_bytes, free_bytes}`. The
dialog calls it on open, and enqueue re-runs it so a stale dialog can't bypass it. It checks:
- **Writable directory:** create (exclusively) and remove a zero-byte `.holodex-probe` beside the
  file. That catches
  `:ro` mounts, permissions and EROFS on every OS without parsing mount tables.
- **Free space:** `statfs` (or `GetDiskFreeSpaceEx` on Windows, via `golang.org/x/sys`).
- **Tools:** the boot-time encoder and MKVToolNix probe.
- **Stream support:** the ffprobe read of the file against the encoder table.
- **No conflicting job:** no pending or running maintenance job for the video.

**Rejected:** a global "media is writable" flag detected at boot. Mounts can differ per directory and
can be remounted without restarting Holodex.

Decided in _(pending: HOLODEX-556 squash)_.

## After a swap or restore: read-back, then a thumbnail reset

The maintenance worker calls the queue's existing `PostWriteFunc` (cover-art re-extract, then
`refresh.Service.ReExtract`), so duration, resolution, size and mtime are refreshed on the same row.
The video's identity is its path, which hasn't changed. It then calls `repo.ResetThumbnailState`
**only when the thumbnail was generated from a video frame** (`ThumbnailGenerated`), so the
thumbnail worker regenerates it from the new picture. An embedded-cover-art or owner-uploaded
thumbnail shows no frame of the video, so it is left alone. A discard changes no file and skips
both steps. The worker then wakes the tag-write lane, whose same-file jobs were skipped while this
one ran. The thumbnail URL already carries `?v=<image mtime>`
(`imageVersion`, `internal/api/handlers.go`), so it changes when the thumbnail is regenerated. The
stream URL gets the same treatment from the SPA, `?v=<file_mtime>` (already in the media payload,
and ignored by the stream handler), so the player doesn't keep showing a cached mirrored picture
after the swap.

Decided in _(pending: HOLODEX-556 squash)_.
