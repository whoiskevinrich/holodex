---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-498
status: in-review            # DERIVED; a zero-gate posture reads it from release_note alone (flightplan #40).
profile: chore               # dev tooling: a stress-fixture seed value and its guard test; no product change
depends-on: []
release_note: Developer tooling only — the stress fixture seeds the extractor's `MP4` container name, so the writeback dialog shows write targets on it.
---

# HOLODEX-498 · Stress fixture seeds container "mp4", so no fixture video ever gets a write target

This is done when every seeded video carries a container writeback has a tag table for, with a test
that fails if the seed drifts from the extractor's normalised name again.

Found while QA'ing HOLODEX-493; spun out of HOLODEX-497.

## Gates — definition of done

<!-- The chore posture has no gates (flightplan.yaml `postures.chore: []`). -->

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR once CI is green; HOLODEX-498 then moves to Done via jira-sync
2. [ ] [—] After merge, reseed local stress data: `rm -rf ./data/stress` then `go run ./testdata/stressseed` (and recopy `data/stress-replace`)

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-29 · session
- skills: code-review
- handoff: Seed now writes `MP4`; `TestGenerate_VideosCarryAWritableContainer` failed on `mp4` and passes after; full `go test ./testdata/stressseed` green; README gap note replaced. Start at Up next item 1: merge after CI.

## Dropped — newest first (the reason is the point)
