---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-497
status: in-review            # DERIVED; a zero-gate posture reads it from release_note alone (flightplan #40).
profile: chore               # dev tooling: a stress-fixture variant, a launch profile and a guard test; no product change
depends-on: []
release_note: Developer tooling only — the stress fixture gains a replace-studio variant and launch profile for exercising studio decisions locally.
---

# HOLODEX-497 · Stress fixture: a replace-studio variant for exercising studio decisions

This is done when the replace-studio setup HOLODEX-493's QA built by hand is committed and can't drift:
- the variant mapping sits beside the fixture's, with a test pinning that only `studio` differs;
- there's a launch profile serving it over a copy of the seeded data;
- the README says when to use it and how to make the copy.

Spun out of HOLODEX-493.

## Gates — definition of done

<!-- The chore posture has no gates (flightplan.yaml `postures.chore: []`). -->

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR once CI is green; HOLODEX-497 then moves to Done via jira-sync
2. [ ] [—] Fix the fixture's lowercase `mp4` container so writeback can be exercised on it → HOLODEX-498

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-29 · session
- skills: handoff, code-review
- handoff: Committed the replace-studio variant: the mapping, a drift test (mutation-checked), a `backend-stress-replace-9300` profile and a README section. The profile was smoke-tested (a clear returns 204). Start at Up next item 1: merge after CI.

## Dropped — newest first (the reason is the point)
