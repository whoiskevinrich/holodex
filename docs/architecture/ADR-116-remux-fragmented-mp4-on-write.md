# ADR-116: Remux a fragmented MP4 inside the writeback job, restoring its tags

**Status:** Proposed
**Date:** 2026-09-28
**Deciders:** Project owner

**Extends:** [ADR-041](ADR-041-metadata-writeback.md) §file-safety (copy → write → rename). For one
kind of file, the copy step becomes a remux; the write and rename steps are unchanged.
**Spec:** [fire-and-forget-writeback](../specs/fire-and-forget-writeback.md) R3.7, amended.
**Relates to:** HOLODEX-479, which added the refusal this ADR keeps as its fallback. HOLODEX-481
(scan-time flag and warning) was closed as Won't Do in favour of this.
[ADR-073](ADR-073-post-write-baseline-resync.md): the post-write re-probe picks up the new file
unchanged. [ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md): snapshots and revert
cover tag values, not byte layout.

---

## Context

A **fragmented MP4** has `moov` followed by `moof`/`mdat` fragments. HLS/DASH downloads and some
recorders produce them. ExifTool's QuickTime writer refuses these files ("Can't yet handle movie
fragments when writing"), and every retry fails the same way. HOLODEX-479 detects the top-level
`moof` before exiftool runs and refuses with `ErrFragmentedMP4`. The refusal message includes an
ffmpeg command that the owner has to run by hand.

HOLODEX-479's report assumed the container image lacks ffmpeg, but it doesn't: the `Dockerfile`
installs `ffmpeg`, the extractor already runs `ffprobe`, and the MKV writeback fallback already
remuxes with ffmpeg (`writeMKVWithFFmpeg`). Remuxing therefore adds no new dependency and no new kind
of operation.

The owner's direction for the fix: it **should be invisible**. Writing metadata to such a file
should just work, with no warning and no manual step.

Two facts, confirmed by experiment on 2026-09-28 (ffmpeg 7 / ExifTool 13), constrain the design:

1. `ffmpeg -map 0 -c copy -map_metadata 0` carries the common QuickTime keys: title, comment and
   genre.
2. It **drops XMP**. Writeback stores `edition` as `XMP-prism:Edition` (`tags.go`), so a bare remux
   would silently delete a previously written field. That breaks ADR-041's merge contract: "every
   other tag … on the file is preserved." `exiftool -TagsFromFile <original> -all:all` restored it,
   and the original can be the fragmented file, because ExifTool *reads* fragmented files fine.

## Decision

**D1 — Remux replaces the copy step, only for a fragmented BMFF file.** In `writeExiftoolBatch`,
when `checkNotFragmented` finds a top-level `moof` in a `.mp4`/`.m4v`/`.mov` file, step 1 of
ADR-041 changes:

```
1. remux  ffmpeg -nostdin -y -i <orig> -map 0 -c copy -map_metadata 0
                 -movflags +faststart -f <mp4|mov> <orig>.holodex-tmp
          (a plain copy for every other file, as today)
2. write  exiftool -m -overwrite_original
                 -TagsFromFile <orig> -all:all   ← remuxed files only
                 -TAG=VALUE …                    ← the batch, as today
                 <orig>.holodex-tmp
3. rename .holodex-tmp → <orig>   (atomic, unchanged)
```

- The same temp path and the same single rename are used. On any failure the temp file is deleted
  and the original stays untouched, so ADR-041's safety model holds unchanged.
- The output format is passed explicitly with `-f`, because the temp extension tells ffmpeg nothing.
  `.mov` stays `mov` and `.mp4`/`.m4v` become `mp4`, so the file's container matches its extension.
- `+faststart` puts `moov` at the front. The result is a normal progressive file.
- It costs no more disk than today: ADR-041 already makes a full-size temp copy, and the remux
  simply writes that copy.

**D2 — Restore every tag from the original in the same exiftool call.** `-TagsFromFile <orig>
-all:all` comes before the batch's `-TAG=VALUE` assignments, and ExifTool applies them in order.
The file keeps everything it had (XMP included), and the batch's fields override it. Together with
D1 this keeps ADR-041's merge contract. It is still **one exiftool invocation per batch** (ADR-041,
[atomic + batched](../specs/fire-and-forget-writeback.md)).

**D3 — The remuxed temp file is re-checked before exiftool runs.** If the temp file still has a
top-level `moof`, the job fails. That would mean an unusual ffmpeg build or flags that didn't take.
Exiftool is never handed a file it will refuse.

**D4 — HOLODEX-479's refusal is the fallback, not the path.** `ErrFragmentedMP4` (code
`writeback_unsupported_container`, with the remux hint) is returned only when the remux can't
happen or fails:

- ffmpeg is not on `PATH`, as in a bare-metal install without it;
- ffmpeg exits non-zero, for example on a stream `-c copy` can't carry;
- the D3 re-check fails.

In those cases the error wraps ffmpeg's trimmed output, so `errors.Is` and the UI's prefix match
keep working, and the operator can see why the automatic path didn't apply.

**D5 — No new state and no new UI.** There is no scan-time flag, no migration, and no warning. The
remux happens inside the job the owner already started, so the owner sees the same status flow as
any write: pending, then done. ADR-073's post-write re-probe records the new size and mtime.

## Options considered

### A: Remux on write, with tag restore *(chosen)*
| Dimension | Assessment |
|---|---|
| Complexity | Low: one branch in the copy step, and one extra exiftool argument pair |
| Cost | A stream copy of the file, which today is already a byte copy. `+faststart` adds a second pass |
| Owner experience | Invisible: the write just succeeds |
| Risk | ffmpeg can refuse exotic streams. The fallback is today's refusal, so nothing regresses |

### B: Background sweep that remuxes every fragmented file (flag at scan, then queue)
Rejected. It rewrites whole files the owner never asked to touch, and it needs up to twice the file
size in free disk space across the library. It also changes bytes that seeding and checksum tools
rely on, and nobody benefits until the owner writes metadata anyway. The scan-time flag would also
need a migration and a backfill.

### C: Flag at scan time and warn in the UI (HOLODEX-481 as filed)
Rejected by the owner. It tells the owner about a problem Holodex can fix itself, and the warning
can go stale between scans.

### D: Refuse only (status quo after HOLODEX-479)
Superseded. The error is actionable, but it still asks the owner to do by hand something the image
can already do.

### E: Remux without `-TagsFromFile`
Rejected. Experiment showed ffmpeg drops XMP, so a written `edition` would silently disappear.

## Consequences

- Writeback to a fragmented MP4/M4V/MOV succeeds with no owner action, and the file comes out
  progressive (`+faststart`). The file's byte layout is changed permanently: the ADR-067 revert
  restores tag *values*, not fragmentation. We accept that, because a fragmented layout is not
  something anyone needs to keep.
- A remux-on-write takes longer than a byte copy (two passes for `+faststart`). It runs inside the
  queue worker and never blocks the request (ADR-091).
- There is a new `exec` of ffmpeg over a user file on the exiftool path. It has the same shape as
  the MKV fallback: no shell, argv only, the path taken from the DB row, and output confined to the
  sibling temp path. The security review covers it.
- HOLODEX-479's refusal code and UI copy stay. They become rare instead of routine.

## Action items

1. [ ] `internal/writeback`: add a `remuxToTemp` helper and branch in `writeExiftoolBatch`, add
       `-TagsFromFile` for remuxed files, and re-check the temp file (D1–D3).
2. [ ] Fall back to `ErrFragmentedMP4` wrapping ffmpeg's output (D4).
3. [ ] Tests:
       - integration: ffmpeg-made fMP4 with comment/genre → `WriteBatch` title → title written,
         comment/genre kept, no `moof`, container still mp4;
       - XMP survival: ffmpeg can't fragment a file *and* keep its XMP, so the fixture grafts the
         XMP `uuid` box from an exiftool-tagged progressive file onto the fMP4. If that proves
         brittle, fall back to asserting that the argv carries `-TagsFromFile` ahead of the batch,
         and keep the 2026-09-28 experiment as the evidence;
       - `.mov` keeps `mov`;
       - ffmpeg failure → `ErrFragmentedMP4`, original untouched, no temp file.
4. [ ] Amend spec R3.7; `/security-review` before merge.
