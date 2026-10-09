---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-550
status: in-progress
profile: chore               # CI tooling fix: no spec/design surface
depends-on: []
release_note: "No user-facing change: a docs-only ticket now reaches Done in Jira when its pull request merges."
---

# HOLODEX-550 · jira-sync docs-only guard strands docs tickets

Done when a docs-only merge fires Done for a key whose worklog posture has no design-phase gate,
while a gate-artifact PR (any posture with one, or no readable worklog) still skips it.

**Design package:** [HOLODEX-550](https://whoiskevinrich.atlassian.net/browse/HOLODEX-550) · [jira-pipeline.md](../reference/jira-pipeline.md) (the docs-only guard entry)

## Gates — definition of done

<!-- Posture `chore` has an empty gate roster: status comes from the release note alone. -->

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge once CI is green; the merge itself is not docs-only, so Done should fire on its own

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-09 · docsOnlyExempt added, reviewed, security-reviewed
- skills: security-review, code-review, handoff
- handoff: Exemption built and tested (152/152, mutation-checked, true for 542/549 and false for 548 on real files); security review clean; PR open — merge when green.

## Dropped — newest first (the reason is the point)
