# File maintenance

This doc owns how Holodex rewrites a media file's **picture**, or **trims its length**, and leaves
everything else intact: the re-encode pipeline per container, the encoder table, the verification gate,
the two-rename swap and its crash recovery, held originals and their expiry, the trim's cut and frame
checks, how a trimmed edition becomes a new library item, and how maintenance jobs share the write
queue with tag writes. The behaviour the owner sees is in the
[video file maintenance spec](../specs/video-file-maintenance.md) (F77) and the
[video trimming spec](../specs/video-trimming.md) (F88). The tag-write model reused
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

Decided in [`04dd6cad`](https://github.com/whoiskevinrich/holodex/commit/04dd6cad) (HOLODEX-556).

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
- **Renames** are a third kind, `kind='rename'`, claimed by the tag-write lane under the same
  one-running-job rule. A rename is refused while a `held_originals` row exists, because `held_path`
  is derived from `file_path` ([file-rename.md](file-rename.md)).
- **Shutdown** cancels the ffmpeg process through the application-lifetime context. The job returns
  to `pending` at boot and starts again from the original. A re-encode cannot resume midway.

**Rejected:** a separate `maintenance_queue` table. It would duplicate recovery, retry, dismiss and
`job_runs` wiring, and the page would need two reads for "is anything happening to this file".
**Rejected:** running maintenance in the tag-write pool. With the default concurrency of 1, one flip
would stall every tag write in the library for its whole duration.

Decided in [`04dd6cad`](https://github.com/whoiskevinrich/holodex/commit/04dd6cad) (HOLODEX-556).

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
     Its track name, language, default and forced flags are copied from the original track, and
     the original track's own tags are carried onto it with `--tags 0:<file>`.
     - `--no-date`: mkvmerge would stamp its own mux date (Segment Info `DateUTC`, ffprobe's
       `creation_time`). When the original had one, `mkvpropedit --edit info --set date=` puts it
       back, in the `YYYY-mm-ddTHH:MM:SSZ` form mkvpropedit accepts.
     - **Statistics follow the original.** If any track carries mkvmerge's `_STATISTICS_TAGS`
       (the file was written by mkvmerge), mkvmerge regenerates them. The re-encoded track's
       change, and the copied tracks' come out identical apart from the mux stamps. If none
       does, `--disable-track-statistics-tags` keeps mkvmerge from adding new tags to tracks
       that were only copied.

  Every other track, attachment, chapter, edition and global or track tag comes from the original
  byte for byte. **MKVToolNix (mkvmerge, mkvextract, mkvpropedit) is therefore required for
  Matroska maintenance.** Without it, the preflight refuses. There is no ffmpeg-remux fallback
  here, because that fallback is exactly what fails on attachments.
- **MP4 (`.mp4`, `.m4v`, `.mov`).**
  1. ffmpeg writes `<file>.holodex-new` with every stream mapped explicitly (`-map 0:<i>` each),
     `-c copy -c:v:<n> <encoder> -filter:v:<n> hflip -map_metadata 0 -map_chapters 0 -fflags
     +bitexact -movflags +faststart`, matching the extension's container.
     - Cover art is an `attached_pic` video stream, so it is excluded from the filter by index
       and copied.
     - The QuickTime chapter track (a `data` stream tagged `text`) is **not** mapped when the
       file has chapters. `-map_chapters` regenerates it in the same position, and mapping the
       source copy as well writes it twice (seen with two or more chapters).
  2. exiftool runs `-TagsFromFile <original> -XMP:all -ItemList:all -UserData:all` on the
     output: ffmpeg drops XMP, where MP4 `edition` lives, and with `+bitexact` writes no `©too`,
     so the item list and udta are restored too. Not `-all:all`, which also turns the container's
     brand fields into new `Keys` tags the original never had. For `.mp4`/`.m4v` it also passes
     `--ItemList:CoverArt`: the mp4 muxer already carried each cover as its own stream, and
     exiftool copies only one of several duplicate `covr` items, which some taggers leave behind.
     The mov muxer writes no covers, so a `.mov` keeps exiftool's restore.
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

Decided in [`04dd6cad`](https://github.com/whoiskevinrich/holodex/commit/04dd6cad) (HOLODEX-556).

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

Decided in [`04dd6cad`](https://github.com/whoiskevinrich/holodex/commit/04dd6cad) (HOLODEX-556).

## The verification gate compares probes, not exit codes

Before any rename, `maintenance.Verify(original, candidate, op)` compares:
- `ffprobe -show_format -show_streams -show_chapters -of json`;
- `exiftool -j -a -G1 -struct`, normalised.

One rule maps to each output-contract clause in the spec:
- stream count, order, type, language, title and disposition are equal;
- copied streams are byte-equal by packet hash (`ffprobe -show_packets` digest);
- every tag key and value is equal, except `_STATISTICS`-listed tags on the re-encoded track,
  Matroska's container writing-application field, MP4's `ftyp` brand fields (ffmpeg writes its
  own `ftyp`, and `-brand` sets only the major brand), and the re-encoded MP4 track's `encoder`
  (the `stsd` CompressorName: ffmpeg 7.1 fills it on every re-encode, and an empty `-metadata`
  value counts as unset, so it can't be held empty);
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

Decided in [`04dd6cad`](https://github.com/whoiskevinrich/holodex/commit/04dd6cad) (HOLODEX-556).

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

Decided in [`04dd6cad`](https://github.com/whoiskevinrich/holodex/commit/04dd6cad) (HOLODEX-556).

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

Decided in [`04dd6cad`](https://github.com/whoiskevinrich/holodex/commit/04dd6cad) (HOLODEX-556).

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

Decided in [`04dd6cad`](https://github.com/whoiskevinrich/holodex/commit/04dd6cad) (HOLODEX-556).

## A trim is a maintenance operation with its own relaxed rules, not a new pipeline

A trim is a `kind='maintenance'` queue row with `op='trim'`. It inherits the lane, the one-job-per-file
claim, recovery, Retry/Dismiss, `job_runs` and visitor redaction unchanged. The payload carries:
- `in_time` and `out_time`: the `mediaTime` the picker read from `requestVideoFrameCallback` for the
  in and out frames;
- `mode` (`replace` | `edition`) and, for an edition, the edition name;
- the folder holding the two screen captures, a name the server generates (next sections).

The route takes all of this and both captures in one multipart request, so a job can't exist without
its captures or the captures without a job. The worker reads the file's path and works out an
edition's target when the job runs, because a rename can land while it waits.

The operation's verifier rules are the F77 gate with three relaxations: duration, chapters and the
copied streams' extent (below). **Replace** lands through the existing two-rename swap and
`held_originals`, with no change. **Edition** lands through a new path, and nothing is held
(section after next).

**Rejected:** a separate trim worker or queue. It would need its own one-job-per-file rule against
both lanes. A trim and a tag write on the same file must serialise exactly as a flip and a tag write
already do.

Decided in [`59d55a75`](https://github.com/whoiskevinrich/holodex/commit/59d55a75) (HOLODEX-613).

## The trim cut: the kept range re-encoded whole, every other stream copied and cut on packets

- **Times are the file's own presentation times.** They are what a browser reports and what ffprobe
  lists for the video. They are never shifted by the container's start time: AAC priming makes that
  negative (−0.023 s in a test file) while the video still starts at zero. ffmpeg's `-ss` adds the
  start time by default, which landed cuts a frame early on Matroska files with audio, so every seek
  passes `-seek_timestamp 1`.
- **Cut points.** The encode seeks to the midpoint between the in frame and the frame before it, and
  stops at the midpoint between the out frame and the frame after it. Both neighbours' times come from
  the source's own frames (ffprobe around each cut), never from the browser. A cut at the file's first
  or last frame has no neighbour, so that side gets no seek or stop. Seeking to the frame's own start
  time landed one frame late on 4 of 8 MP4 test points, because a timestamp parsed fractionally below
  the frame's pts discards the frame. A half-frame margin landed exactly on 16 of 16 at a constant
  rate. It also absorbs Matroska's millisecond timestamps, which a browser reports rounded (1.502 for
  1.5015). The margin is the real gap, not half of `r_frame_rate`'s interval, because frames can sit
  closer together than that: on a variable-frame-rate MKV with 10 ms gaps at a nominal 25 fps, half
  the nominal interval landed correctly on 1 of 5 cuts and midpoints on 5 of 5.
- **Video.** The kept range is re-encoded **whole**, through the F77 encoder table, colour pinning and
  `-fps_mode passthrough`, with no new rows. Accurate input seeking (`-ss` before `-i`) decodes from
  the preceding keyframe and drops frames before the cut. The encoder clock is `-enc_time_base 1:90000`.
  Seeking to a midpoint puts every kept frame half a frame off the source's frame-rate clock, and
  rounding to that clock gave neighbouring frames the same timestamp: the right pictures, with a
  judder no frame check can see.
- **Every other stream is copied, never re-encoded.** Audio and subtitle packets inside the range are
  kept bit for bit. The packet nearest each cut point is the boundary. An audio packet is about
  32 ms (AC3) or less, so sync is off by at most half a packet. Lossless and object audio (TrueHD,
  DTS-HD MA, Atmos) survive because nothing decodes them. Attachments and cover art are copied whole.
- **Chapters are computed, not inherited.** Holodex builds the expected chapter list from the original:
  shift chapters inside the range, drop those outside, clamp those that straddle a cut. It writes the
  list explicitly (`mkvpropedit --chapters` XML for Matroska; an ffmetadata chapter block for MP4),
  rather than trusting each tool's split behaviour. The gate compares against the same list.
- **Matroska takes three steps.** ffmpeg encodes the video range into the intermediate, as for a flip.
  mkvmerge then cuts every other track on its own, with `--split parts:<in>-<out>`; with no video
  track left it cuts on any packet, not on keyframes. It must be its own pass, because `--split`
  applies to a whole output and the intermediate is already the kept range. A second mkvmerge pass
  joins the two with the intermediate's track in the original position and the computed chapters as
  `--chapters` XML. The F77 mkvmerge flags (`--no-date`, statistics, tag carry) apply unchanged. A
  file with nothing left to cut skips the cut: its tags and attachments come from the original with
  `-D -A`. Chapter languages aren't kept: ffprobe doesn't report them, and rule 5 asks for titles.
- **FLAC in Matroska is re-encoded, losslessly, with the video.** mkvmerge won't split a FLAC track
  at all. So step 1 maps each FLAC track beside the video and writes it with ffmpeg's `flac` encoder,
  and step 2 leaves it out with `-a !<ids>`. Step 3 puts it back in its original position with its
  name, language, flags and track tags, as it does the video. FLAC is lossless, so the decoded samples
  are the original's bit for bit, and ffmpeg keeps the bit depth (a 24-bit track stays 24-bit; a
  32-bit one needs `-strict experimental`, without which the encoder silently writes 24 bits). The
  same accurate seek cuts it on the sample, not the packet, so it is closer to the video than a copied
  track. Its packets can't match the original's, so the gate decodes both instead: the output track's
  samples must hash equal to the original's decoded over the same seek range. Rejected: refusing the file (the owner's call, 2026-11-07); cutting the FLAC with ffmpeg
  `-c copy` (still on packets, and a second ffmpeg pass for nothing); remuxing it to another lossless
  codec (changes the file's codec). Proven in `holodex:test-image` (ffmpeg 7.1, mkvmerge 92): the
  kept range's PCM matched the source's for 16-, 24- and 32-bit tracks.
- **MP4.** One ffmpeg pass does the same seek on every mapped stream: `-c copy`, with
  `-c:v:<n> <encoder>` for the main video track and the computed chapters as `-map_chapters` input.
  `-copypriorss 0` stops ffmpeg copying the packets between the keyframe it seeks to and the cut. Cover
  art's one packet sits at time zero, so each cover stream gets `-copypriorss:<n> 1`. Then the F77
  exiftool restore runs. A file with a second video track is refused, in both containers, because a
  copied video track can only be cut on a keyframe.
- **Verifier relaxations for `op='trim'`:**
  - the output video's first-to-last frame distance equals `out − in`, within one frame. A file's
    overall duration also counts an audio packet running past the last frame, so this is measured on
    the video itself;
  - a variable-frame-rate source (`r_frame_rate` differs from `avg_frame_rate`) needn't keep its
    `r_frame_rate`, which is a ceiling that depends on the frames kept;
  - chapters equal the computed list;
  - copied streams are packet-hash-equal to the original's packets **within the range**. The boundary
    packet at each end is excluded, since which side of the cut it falls on is the tool's choice;
  - duration and statistics tags are recomputed on **every** stream, not only the re-encoded one.
  Every other F77 rule stands.

**Rejected:** a smart cut, which re-encodes only the GOPs at each edge and splices in the copied
middle. The spliced edges must reproduce the source encoder's SPS/PPS, profile, level and reference
structure, or players fail at the join. That can't be guaranteed for arbitrary sources, and the gate
couldn't prove a join plays. It stays a P2 behind the same contract. **Rejected:** re-encoding audio
for sample-exact cuts. Every trim would lose audio quality, each codec would need its own encoder, and
ffmpeg can't re-encode TrueHD or DTS-HD MA at all.

Decided in [`59d55a75`](https://github.com/whoiskevinrich/holodex/commit/59d55a75) (HOLODEX-613).

## Frame checks: exact against Holodex's own decode, shape-only against the screen

Two checks run after the gate's probe rules and before anything lands. They follow from the Q1 spike
(results on HOLODEX-613). Comparing ffmpeg's output with ffmpeg's own decode of the source was exact.
Comparing a browser frame with ffmpeg's was not: on still, black and slowly fading frames the
decoder-to-decoder difference is as large as the frame-to-frame one, and a browser tone-maps HDR, so
its pixels for the right frame were 17–25 grey levels off.

- **Shared comparison.** Each frame is scaled to 240×135 grey. Mean absolute difference (MAD) is used
  between two ffmpeg decodes. Zero-mean normalised correlation (NCC), which ignores brightness and
  contrast, is used whenever a browser image is involved.
- **References.** The worker decodes the original's frames n−2…n+2 around each cut point. It selects
  them by timestamp (`-copyts` with a `select` filter), after a seek that only has to land early. A
  reference decoded the way the cut seeks would shift with any seek error, and the cut check would
  then pass the same wrong frame: this happened in testing. It also decodes the output's first and last
  frames.
- **Cut check (always).** The output's first frame must be nearest by MAD to reference n, and the same
  for the last frame. A neighbour nearer than n by at most 0.5 grey levels passes: the output is a
  lossy re-encode, so a tie that close means the frames barely differ and the two can't differ visibly.
- **Screen check (only where it can tell).** The captures arrive with the enqueue
  (`media/{id}/maintenance/trim`, owner-only, each proved to decode as a JPEG, at most 8 MB). They are
  stored under `DATA_PATH/maintenance/<generated name>/`, deleted when the job succeeds or a failed job
  is dismissed, and kept for Retry otherwise; a boot sweep removes any folder no queue row names. A
  capture is matched by NCC against references n−2…n+2. The check runs only when reference n is
  **distinct**: its NCC to each neighbour is at least 0.02 below 1. In the spike, distinct frames led by
  ≥0.04 and still frames by ≤0.002. A capture whose best match is below 0.7 shows another scene, and
  fails rather than matching the least-bad neighbour. It runs before the encode, so a disagreement
  costs no encoding. A skipped check is recorded with its reason.
- **On a mismatch** the worker corrects once, across both checks. The two move opposite ways. A
  capture matching reference n+k means the owner saw that frame, so the frame to keep moves by +k. An
  output holding reference n+k means the cut landed late, so the cut moves by −k and the frame to keep
  stays. A second mismatch of either kind fails the job, naming the check and the offset. `.holodex-new`
  is deleted, and nothing lands.

**Rejected:** comparing the output with the browser capture by absolute pixels, the spec's first R6.
It fails on HDR files and can't tell still frames apart. **Rejected:** trusting timestamps with no
frame check. The cut check is the only evidence that ffmpeg cut where the decode says it did, and the
screen check is the only evidence that the browser and ffmpeg agree on which frame a time names.

Decided in [`59d55a75`](https://github.com/whoiskevinrich/holodex/commit/59d55a75) (HOLODEX-613).

## Save as edition: a new file beside the original and a cloned row, journaled in the queue row

- **The target name** is the original's stem, then ` {edition-<name>}`, then the original extension,
  in the original's directory. That marker is the one the filename pattern lifts
  (`internal/extract/pattern.go` `markers`), so a rescan from scratch still reads the edition.
  - `<name>` is sanitised: path separators, braces, control characters and a leading dot are removed,
    and its length is capped so the whole name fits the volume's limit.
  - A stem that already carries an `{edition-…}` marker has it replaced, not doubled.
  - The target is refused at preflight and when the job starts if a file (`Lstat`, so a symlink counts)
    or a library row, even a trashed one, holds that name. A trashed row's state must not merge into
    the new edition.
- **The build** is the trim cut above, written to `<target>.holodex-new`. It also writes the new
  edition into the file: Matroska's `Edition` tag through mkvpropedit, MP4's `XMP-prism:Edition`
  through exiftool. The verifier expects exactly that one tag to differ from the original.
- **Landing.**
  1. Record `stage='landed'` and the target path in the queue row's payload.
  2. Move `.holodex-new` to the target through F86's no-overwrite primitive (`rename.Folder.Move`), so
     a file that took the name during the encode stays and the trim fails. No original is held,
     because the original is never touched.
  3. In one transaction under `writeMu`, `repo.CloneVideoAsEdition(srcID, targetPath, edition)` runs.
     It upserts the `videos` row by `file_path`, so a scanner that saw the file first is filled in
     rather than duplicated. Then it copies the source's current state:
     - **Copied:** `video_people`, `video_tags`, `video_studios`, `film_videos`, `playlist_videos`,
       `metadata_curation`, `field_source_decisions`, `entity_enrichment`, `facet_not_applicable`,
       `enrichment_dismissals`, and `video_poster_sources` with their `frame` and `upload` slot files
       (copied to the new id, then `Present`).
     - **A film link is kept, without its scene number.** A film's numbers are unique and the source
       keeps its own, so the owner numbers the edition if it needs one.
     - **`part` is copied.** **`edition`** becomes a `manual` decision with the new name, so neither the
       source's edition decision nor a stale filename read can win.
     - **Not copied:** history (`file_writebacks`, snapshots, `writeback_queue`, `held_originals`,
       extraction reviews, other `job_runs`, F86's `file_renames` and `video_file_names`, so an edition
       starts with no rename history and no name lock, and `video_plays` and `video_watch`, so it starts
       unwatched) and derived state (montage, completeness, `resolved_values`, all rebuilt once the row
       is marked dirty). `repo.CloneTables` classifies every table holding per-video state, and a
       test fails on one it doesn't list. `video_metadata`, file facts and the `file`/`auto` slots come from `ReExtract` and
       thumbnail generation on the new file.
     - **Links:** an `entity_keep_separate('video', lo, hi)` row is written, which is what "label as
       editions" stores (`LabelVideoPair`), so the pair never enters the duplicates queue. A
       `video_trimmed_from` row is the edition's first history entry, "trimmed from". A `job_runs` row
       (`kind=maintenance`, detail `trim-edition`) is attributed to the new video for the audit trail.
       It can't carry the link, or the outcome notice, because `job_runs` is pruned after 30 days and
       the page reads neither from it. The outcome is `video_trim_results`, on the source, until
       dismissed.
  4. Copy the poster slot files (`thumbnail.CopyOwnerSlots`) and present them: `repo` can't import
     `thumbnail`. Then run `ReExtract` and queue a frame thumbnail for the new id, since nothing else
     does at runtime. Then mark the queue row done.
- **The SQL shape** follows `applyCarry` (`internal/repo/video_pairs.go`), INSERT…SELECT per table,
  but copies instead of moving. It has its own field list, because `carryExcludedFields` drops `part`
  too.
- **Recovery.** Boot returns a `running` row to `pending`, and the worker claims it again. An edition
  row with `stage='landed'` then skips the cut:
  - target present → run steps 3 and 4 (idempotent on `file_path`);
  - target absent → drop the stage, so the job restarts from the original.

  The orphan sweep removes a leftover `<target>.holodex-new` as it removes any `.holodex-new`.
- **Preflight additions:** the target name is free; the directory takes an exclusive probe, as for a
  flip; free space allows `1.5 × source size × kept fraction`, for either mode: an edition adds a file,
  and a replace writes the new file before its held original becomes a rename.

**Rejected:** letting the scanner discover the new file and copying state onto it in a hook. The
edition would exist as a bare item until the hook ran, and the hook would have to match a scanned
row back to its trim. **Rejected:** writing the edition into a sibling folder. A folder per edition
would break the film folder convention, and a different volume would make the landing a copy, not a
rename.

Decided in [`59d55a75`](https://github.com/whoiskevinrich/holodex/commit/59d55a75) (HOLODEX-613).

## After a trimmed replace: the F77 read-back, with duration-derived state reset

A replace lands through the F77 swap, then the same `PostWriteFunc` read-back, so duration, size and
chapters are refreshed on the same row. The maintenance wrapper's montage invalidation already
regenerates the moving thumbnail. A generated thumbnail is reset and queued for a new frame. An
owner's slots aren't touched. The `frame` and `upload` slots are owner images and are kept even when their frame time
now lies outside the file. `video_poster_sources.frame_at` stays as a record of where the image came
from. Restore puts the full-length original back and runs the same read-back.

Decided in [`59d55a75`](https://github.com/whoiskevinrich/holodex/commit/59d55a75) (HOLODEX-613).
