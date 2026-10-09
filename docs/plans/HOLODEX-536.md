---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-536
status: in-progress
profile: backend             # metadata-extraction behaviour, no UI surface
depends-on: []
release_note: "MKV files with tags at several levels now show the episode's own title, cast and genres, not a track's or the whole series'."
---

# HOLODEX-536 · MKV tag target-level selection

Done when, for Matroska files, track-, edition-, chapter- and attachment-bound tags never describe
the video; single-value fields fall back 50 → 60 → 70; multi-value fields take every level-50 value
and never inherit. Files the reader can't walk keep exiftool's values.

**Design package:** [HOLODEX-536](https://whoiskevinrich.atlassian.net/browse/HOLODEX-536) · [media-ingest.md](../architecture/media-ingest.md) ("Only container-level Matroska tags describe the video") · [phase-1-mvp.md](../specs/phase-1-mvp.md) (open question 2)

## Gates — definition of done

- [x] spec — phase-1-mvp.md OQ2 gains the single-value 60/70 fallback clause
- [x] backend — `internal/metadata/matroska.go` reader + `selectMatroskaTags`, wired into `Extract`
- [x] testing — unit (`matroska_test.go`) + real-exiftool integration test; testing-strategy.md gap closed

## Up next — ordered (position = priority)

1. [ ] Squash-merge once CI is green
2. [ ] After merge: record the deciding squash sha on the media-ingest.md section ("Decided in")

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-09 · Go Matroska Tags reader replaces incidental first-wins
- skills: code-review
- handoff: Bug reproduced with crafted MKVs (track/collection/chapter tags ahead of the episode's won in exiftool; ffprobe is last-wins), owner chose the Go reader; built, reviewed and tested against real exiftool — ready for PR.

## Dropped — newest first (the reason is the point)

- `exiftool -G1` partial fix — leaves collection/season/chapter tags indistinguishable from the episode's.
