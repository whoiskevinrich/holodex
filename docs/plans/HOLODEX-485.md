---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-485
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: backend             # behavior with no UI surface — the mkvpropedit twin of HOLODEX-484's bug fix
depends-on: []
release_note: On hosts with MKVToolNix, writing a poster to an MKV that already has a cover in another format (such as cover.webp) now replaces it rather than leaving two covers.
---

# HOLODEX-485 · mkvpropedit cover path leaves a second cover.* attachment behind

This is done when a poster writeback on the mkvpropedit path deletes every same-role `cover.*` attachment,
whatever its extension, keeps fonts and other roles, and adds the new cover, all in one mkvpropedit
invocation. Spun out of HOLODEX-484 (Relates).

## Gates — definition of done

- [~] spec `write-spec` → `docs/specs/**`. Skipped deliberately: a bug fix bringing the mkvpropedit path in line with HOLODEX-484. No requirement changes.
- [x] backend → `{cmd,internal,providers}/**`. `listMKVAttachments` (`mkvmerge -J`) feeds `coverDeleteArgs`, which deletes by UID (by exact name if no UID is reported). Dispatch now also requires `mkvmerge`.
- [x] testing `testing-strategy`. `TestCoverDeleteArgs` (unit) plus the shared cover integration test, now run on both write paths. A row is in `docs/testing-strategy.md`.

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR after CI; HOLODEX-485 moves to Done via jira-sync

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-28 · session
- skills: code-review, handoff
- handoff: Fixed and tested. Both cover paths pass integration in a Debian container (ffmpeg 5.1.9, mkvpropedit v74), and the mkvpropedit test fails without the fix. origin/main merged, PR opened ready; next is merge after CI.

## Dropped — newest first (the reason is the point)

- [~] [backend] Delete covers by `mime-type:` selector — dropped 2026-09-28: it would also delete `small_cover.*` and other image roles
