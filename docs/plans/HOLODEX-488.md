---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-488
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: backend             # behavior with no UI surface — a writeback bug fix on the mkvpropedit path
depends-on: []
release_note: MKV files that already carry per-track tags (anything muxed by ffmpeg) now actually receive studio, overview, genre, year and other tag writes. Before, only the title and poster landed, while the write reported success.
---

# HOLODEX-488 · MKV writeback silently drops tags beside per-track tags

This is done when an mkvpropedit tag write lands on a file whose only Tags are TrackUID-targeted, the
per-track Tags survive it, and an mkvpropedit refusal (`No changes were made.`, exit 0) fails the write
rather than being recorded as success. Relates to HOLODEX-486 (the image that made mkvpropedit the
production path).

## Gates — definition of done

- [~] spec `write-spec` → `docs/specs/**`. Skipped deliberately: a bug fix restoring the documented writeback contract. No requirement changes.
- [x] backend → `{cmd,internal,providers}/**`. The merged tag document goes through `--tags all:` instead of `global:`; a tag write whose output says `No changes were made` is now an error.
- [x] testing `testing-strategy`. `TestWriteMKVWithMkvpropedit_WritesTagsBesidePerTrackTags`, run in-image: red against `global:`, green on `all:`. A row is in `docs/testing-strategy.md`.

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR after CI; HOLODEX-488 moves to Done via jira-sync
2. [ ] [—] On `:edge`, re-run writeback on the affected item and confirm studio/overview go `in_sync`

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-28 · session
- skills: code-review, handoff
- handoff: Fixed and verified in the runtime image. The trigger is a file with track Tags and no untargeted Tag, which `+bitexact` reproduces. The review skipped three findings: `mergeTagsXML` also stripping same-named track Simples (pre-existing design), the no-op check matching an English locale string, and no direct test of the no-op error. origin/main (8d8ce859) is merged in and the branch is pushed; the next move is opening the PR and merging after CI.

## Dropped — newest first (the reason is the point)

- [~] [backend] Post-write `mkvextract` verification of every written Simple — dropped 2026-09-28: `all:` removes the known refusal, and the output check covers its signal without a second subprocess per write
