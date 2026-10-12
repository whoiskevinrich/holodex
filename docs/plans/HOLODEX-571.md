---
key: HOLODEX-571
status: in-progress
profile: backend
depends-on: []
release_note: "Fix: flipping a video horizontally no longer fails with \"the new file's tags didn't match the original\" on MP4 files whose video track names no encoder."
---

# HOLODEX-571 · hflip still fails verification on MP4s with no compressor name

HOLODEX-569's fix set `-metadata:s:v:N encoder=` (empty) to stop ffmpeg stamping its name.
ffmpeg 7.1's CLI treats an empty value as unset and stamps "Lavc libx264" anyway. The mp4 muxer
writes that into the sample description's CompressorName, which ffprobe reads as the stream
`encoder`. So every flip of an MP4 whose original field was empty failed rule 4 on edge @ 43ffb13d.
Cover art (`attached_pic`) was checked in the same run and survives.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — rule 4 gains its third exception, the re-encoded MP4 stream's `encoder` (owner's call, 2026-10-12)
- [x] backend → `{cmd,internal,providers}/**` — `verify.go` exemption (MP4, re-encoded stream only); `args.go` passes the original's encoder only when it has one
- [x] testing `testing-strategy` — `verify_test` pins the scope; `TestFlip_RealFiles_MP4_NoCompressorName` was red with the production error, then green with the whole maintenance integration suite in the ffmpeg 7.1 image

## Up next — ordered (position = priority)

1. [ ] [—] Open the PR, then squash-merge on green CI with subject `fix(media): let a flip pass verification on MP4s whose video names no encoder`

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-12 · CompressorName exemption
- skills: code-review high --fix, handoff
- handoff: Fix is built and green in the ffmpeg 7.1 image, real-file regression test included. Next: open the PR and merge it on green CI.
