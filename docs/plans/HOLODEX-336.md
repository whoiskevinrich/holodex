---
key: HOLODEX-336
status: in-progress
profile: backend
depends-on: []
release_note: "Fix: writing a video's original language back to an MP4 file now works, and both the original language and the tagline can report whether the file is in sync. Add `Language` to `original_language` and `Subtitle` + `Description` to `tagline` in your metadata-mappings.yaml (see the example)."
---

# HOLODEX-336 · MP4 writeback silently drops original_language and mis-targets tagline

Two MP4 write targets never came back. `original_language` wrote `QuickTime:MediaLanguage`, which
exiftool does not define, so the write was dropped (current exiftool fails it outright). `tagline`
had already moved off `Keywords` to `QuickTime:Description` (HOLODEX-466), but no mapping declared
a source to read it back. Both sat in `UnreadableWriteTargets`, which forfeited `in_sync` on
Matroska too.

## Gates — definition of done

- [~] spec — no product rule changed; one stale tense fixed in `tag-writeback-exclusion.md`
- [~] architecture — no technology fork; `writeback.md` formatMap section brought to current truth
- [x] backend → `{cmd,internal,providers}/**` — MP4 `original_language` → `XMP-dc:Language` (the only writable language tag; reads back as `Language`); both exemptions removed; example mapping gains `Subtitle`/`Description`/`Language` file sources
- [x] testing `testing-strategy` — `TestLanguageAndTaglineRoundTrip_BothContainers` was red on MP4 `original_language` with the production error, then green locally and in the production image (ffmpeg 7.1.5, exiftool 13.25)

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge on green CI with subject `fix(writeback): write an MP4's original language where it reads back, and read the tagline back`
2. [ ] [—] Owner: add the three file sources to the live `metadata-mappings.yaml` on deploy (the startup readback-gap warning will name them)

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-12 · XMP-dc:Language target, exemptions dropped
- skills: code-review high --fix, handoff
- handoff: Fix is built and green in the production toolchain with a real-file round-trip test; next is the PR, a squash-merge on green CI, and the live mapping edit on deploy.
