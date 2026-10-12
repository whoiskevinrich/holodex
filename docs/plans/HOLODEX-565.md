---
key: HOLODEX-565
status: in-progress
profile: backend
depends-on: []
release_note: ""               # none — an internal test fixture only, no user-facing behaviour (test is hidden from the changelog)
---

# HOLODEX-565 · Fix TestExtract_MatroskaTargetLevels under exiftool 13.25

Done when the HOLODEX-536 integration test again proves Holodex's own Matroska target-level selection
under the exiftool the runtime image ships (13.25), and fails when that selection is removed. Spun
out of HOLODEX-556, where `make test-image` surfaced it.

**Design package:** [media-ingest.md](../architecture/media-ingest.md) ("A small Go reader enforces it") · [testing-strategy.md](../testing-strategy.md) (metadata integration tests)

## Gates — definition of done

- [~] spec `write-spec` → `docs/specs/**` — n/a: test fixture only, no requirement or behaviour change
- [x] backend → `{cmd,internal,providers}/**` — `internal/metadata/matroska_integration_test.go`: episode-level tag moved between the track- and collection-level ones, so neither exiftool flattening order (first-wins in older versions, last-wins in 13.25/13.59) lands on it
- [x] testing `testing-strategy` — passes in `holodex:test-image` (exiftool 13.25) and locally (13.59); disabling `selectMatroskaTags` turns it red ("The Series" / "Series Regular")

## Up next — ordered (position = priority)

1. [ ] [—] Review and squash-merge the PR; sweep HOLODEX-565 to Done on merge if jira-sync misses it — Jira

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-11 · exiftool 13.25 flattens Matroska tags last-wins
- skills: code-review, handoff
- handoff: Fixture reordered so the test exercises HOLODEX-536 under both exiftool orders; green in the image and locally, mutation-checked; media-ingest.md corrected. Next move: review and merge the PR.

## Dropped — newest first (the reason is the point)
