---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-549
status: in-progress
profile: chore               # docs + dead-code removal and comment fixes; no behavior change
depends-on: []
release_note: "No user-facing change: the docs and code comments no longer describe the retired Broadcast and Brutalist skins, and an unused source-picker component is removed."
---

# HOLODEX-549 · Sweep remaining retired-skin references and dead SourceSelect out of docs and code

Done when no spec, design doc or code comment instructs or describes Broadcast, Brutalist, a skin
picker or three-skin QA as current (the instance-skin docs and `theming.md`, which record the
retirement, are exempt), and the unmounted `SourceSelect` is gone with its stale references.
Spun out of HOLODEX-542.

**Design package:** [HOLODEX-549](https://whoiskevinrich.atlassian.net/browse/HOLODEX-549)

## Gates — definition of done

<!-- Posture `chore` has an empty gate roster: status comes from the release note alone. -->

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge the PR once CI is green; move HOLODEX-549 to Done if CI doesn't — docs + comments only

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-08 · SourceSelect deleted; retired-skin sweep across ~200 docs and 30 comments
- skills: code-review, handoff
- handoff: Sweep done and reviewed: leftover skin mentions are only dated QA records, mockup alt text describing three-skin SVGs, and the instance-skin/theming docs that record the retirement; PR open, merge when green.

## Dropped — newest first (the reason is the point)
