---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-542
status: in-progress
profile: chore               # docs-only: specs and design docs brought to current truth, no code
depends-on: []
release_note: "No user-facing change: the specs and design docs now describe only the Cinémathèque look and match what the app does."
---

# HOLODEX-542 · Specs and design docs still describe retired skins and contain self-contradictions

Done when every doc the ticket names states current truth (code wins): no retired-skin QA or
`skin ×` placeholder matrix, and the four self-contradictions resolved against the code.

**Design package:** [HOLODEX-542](https://whoiskevinrich.atlassian.net/browse/HOLODEX-542) — the ticket lists each doc and the conflict it holds.

## Gates — definition of done

<!-- Posture `chore` has an empty gate roster: status comes from the release note alone. -->

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge the PR once CI is green, then move HOLODEX-542 to Done if CI doesn't — docs only
2. [ ] [—] The repo-wide retired-skin sweep (~190 docs), dead `SourceSelect` and stale `PersonImageFrame` comments → HOLODEX-549

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-08 · retired-skin text and four contradictions fixed
- skills: code-review, handoff
- handoff: All seven docs the ticket named now match the code (placeholders are role × gender, 409 only, SourceBadge is the live control, failed writeback line names no fields); PR open — merge when CI is green, and the wider sweep is HOLODEX-549.

## Dropped — newest first (the reason is the point)
