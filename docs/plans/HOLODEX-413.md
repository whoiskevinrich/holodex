---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-413
status: in-progress
release_note: Writing a poster into an MKV now labels PNG covers correctly and replaces the existing cover instead of stacking a second one — files whose earlier cover ffmpeg could not decode ("dimensions not set") are repaired by re-running the poster writeback.
---

# HOLODEX-413 · Writeback (ffmpeg path) attaches PNG covers as image/jpeg and never replaces cover.jpg

Owner's prod log: `writeback ffmpeg: exit status 1 … [mjpeg] bits 42 is invalid … Could not find
codec parameters for stream 2 … [matroska] dimensions not set … Could not write header`. The failing
stream is the file's *existing* `cover.jpg` — attached by an earlier Holodex writeback as a PNG
labelled `image/jpeg`. ffmpeg trusts the label, decodes garbage, gets no dimensions, and refuses to
`-c copy` the stream — which `-map 0` demands on every writeback, text-only included.

## Gates — definition of done

- [~] spec `write-spec` — n/a: bug fix inside F28's existing contract (ADR-041)
- [~] architecture `architecture` — n/a
- [~] design `design-handoff` — n/a: no UI
- [x] backend — `internal/writeback/writeback.go`: `coverMIME(tempPath)` derives `image/png` /
  `image/jpeg` from the extension `downloadImageToTemp` already sniffed; both the mkvpropedit
  `--attachment-mime-type` and the ffmpeg `-metadata:s:t:N mimetype=` use it. `buildFFmpegArgs`
  adds `-map -0:m:filename:<tagName>` per image entry so a cover writeback replaces the existing
  attachment (mirrors mkvpropedit's `--delete-attachment name:`); text-only batches keep bare
  `-map 0` so existing posters are still preserved
- [~] frontend — n/a
- [x] testing `testing-strategy` — `TestBuildFFmpegArgs_ReplacesExistingCover` + `TestCoverMIME`;
  live repro 2026-09-18 with a fixture MKV carrying a PNG labelled image/jpeg: old args fail with
  `dimensions not set`, new args repair the file (one `png` attachment, `mimetype=image/png`), a
  following text-only writeback succeeds, a third cover writeback still yields exactly one cover
  (metadata-key match is case-insensitive, `FILENAME` after remux), and the negative map is a
  no-op on a file with no cover
- [~] security `security-review` — n/a: no new input surface; the fetch stays behind `imageFetch`

## Up next — ordered (position = priority)

1. [ ] [—] Owner re-runs the poster writeback on the broken file after deploy — that is the repair
   path; a text-only writeback alone leaves the undecodable attachment in place (by design)
2. [ ] [—] On merge, HOLODEX-413 → Done via CI (branch-keyed)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-18 · diagnosed, filed, fixed, verified live
- skills: code-review high --fix (no findings)
- Read the ffmpeg log against `buildFFmpegArgs`: two bugs, one root cause. Filed HOLODEX-413,
  renamed the worktree branch, In Progress fired. Kept `downloadImageToTemp`'s signature (four
  callers + tests) — the mime rides on the sniffed extension instead.
- handoff: fix implemented, unit + live-ffmpeg verified; PR open, nothing left but the merge sweep.
