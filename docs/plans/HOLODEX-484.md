---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-484
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: backend             # behavior with no UI surface — a bug fix restoring the intended cover replace
depends-on: []
release_note: Writing a poster to an MKV that already has a cover in another format (such as cover.webp) now replaces it instead of failing with "Attachment stream has no mimetype tag". Subtitle fonts are kept.
---

# HOLODEX-484 · MKV cover writeback fails when the existing cover has a different filename

This is done when a poster writeback on the ffmpeg path replaces any existing `cover.*` attachment, whatever its
extension, keeps every other attachment (subtitle fonts) untouched, and labels the new cover at its
real output attachment index.

## Gates — definition of done

- [~] spec `write-spec` → `docs/specs/**`. Skipped deliberately: this is a bug fix restoring HOLODEX-413's intended "a cover writeback replaces the existing cover". No requirement changes.
- [x] backend → `{cmd,internal,providers}/**`. `probeStreams` (ffprobe) feeds `buildFFmpegArgs`, which drops every same-role `cover.*` by exact probed name and indexes the new attachment at kept + i.
- [x] testing `testing-strategy`. Two unit tests plus a real-ffmpeg integration test (`cover.webp` + font), with a row in `docs/testing-strategy.md`.

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR after CI; HOLODEX-484 moves to Done via jira-sync
2. [ ] [backend] Same-role cover replace on the mkvpropedit path → HOLODEX-485

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-28 · session
- skills: code-review, handoff
- handoff: Fixed and tested (unit + integration reproduce the field error without the fix); origin/main merged in, PR opened ready — next is merge after CI, then HOLODEX-485 for the mkvpropedit path.

## Dropped — newest first (the reason is the point)

- [~] [backend] Remove every attachment before attaching — dropped 2026-09-28: fonts for subtitled AMVs must survive the remux
