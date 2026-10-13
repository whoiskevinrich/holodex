---
key: HOLODEX-570
status: in-progress
profile: backend
depends-on: []
release_note: "Fix: flipping a video horizontally no longer fails verification on MP4 files that carry the same cover art several times."
---

# HOLODEX-570 · Flip drops cover art on MP4s carrying duplicate covr items

Some MP4s carry several identical `covr` items (seen with 7 on testbed AMVs), which ffprobe shows
as one `attached_pic` stream each. ffmpeg's flip pass carried every one. The exiftool tag restore
that follows (`-TagsFromFile … -ItemList:all`) copies only one `CoverArt` and overwrote them, so
the candidate had 1 cover and rule 2 (`streams`) refused the flip. The original was never touched.
Holodex's own poster writeback did not create the duplicates: `-CoverArt<=file` always leaves one.

## Gates — definition of done

- [~] spec — no product rule changed; the spec already requires every stream copied bit-for-bit
- [~] architecture — no technology fork; `file-maintenance.md` step 2 brought to current truth
- [x] backend → `{cmd,internal,providers}/**` — `pipeline.go` restore skips `CoverArt` for the mp4 muxer only (the mov muxer writes no covers, so `.mov` keeps exiftool's restore)
- [x] testing `testing-strategy` — `TestFlip_RealFiles_MP4_DuplicateCovers` was red with the production error (`streams`, 3 → 1), then green with the whole maintenance integration suite locally and in the production image (ffmpeg 7.1.5, exiftool 13.25)

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge on green CI with subject `fix(media): keep every cover when flipping an MP4 that carries duplicates`

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-12 · CoverArt restore skipped for mp4
- skills: code-review high --fix (caught the .mov regression), handoff
- handoff: Fix is built and green in the production toolchain, with a real-file regression test; next is the PR and a squash-merge on green CI.
