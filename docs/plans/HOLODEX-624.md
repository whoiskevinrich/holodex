---
key: HOLODEX-624
status: in-progress
profile: backend
depends-on: []
release_note:
---

# HOLODEX-624 · Trim a Matroska file with FLAC audio

A trim of a Matroska file whose audio is FLAC lands on the chosen frames with the audio intact and
in sync, instead of being refused at preflight. Child of epic HOLODEX-613 (F88 video trimming); the
branch merges into `HOLODEX-613-video-trimming`, not main.

**Design package:** [spec R3](../specs/video-trimming.md) · [file-maintenance.md, trim](../architecture/file-maintenance.md) · [testing-strategy §41](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — R3's FLAC refusal replaced by "trimmed like any other"
- [x] backend → `{cmd,internal,providers}/**` — FLAC re-encoded beside the video in step 1, left out of the cut, gate compares decoded samples; refusal and its UI copy removed
- [x] testing `testing-strategy` — §41 K9 rows, FLAC stream and sync oracles; full `maintenance` suite green in `holodex:test-image` 2026-11-07

## Up next — ordered (position = priority)

1. [ ] [—] parent session folds this into HOLODEX-613's worklog and merges the PR into `HOLODEX-613-video-trimming`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-11-07 · FLAC in Matroska trims instead of refusing
- skills: code-review
- handoff: Built and green in `holodex:test-image` (16/24/32-bit, bit-exact, in sync); the Draft PR targets `HOLODEX-613-video-trimming`, and the only UI change was deleting the `flac_in_matroska` refusal copy, so no frontend gate is opened here.

## Dropped — newest first (the reason is the point)
