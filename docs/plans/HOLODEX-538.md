---
key: HOLODEX-538
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # docs-only correction; no product surface
depends-on: []
release_note: ""
---

# HOLODEX-538 · Activity history paging: correct the topic doc

The ticket said keyset paging and kind/status/entity filters on `/activity/history` had been
decided but never built. They had been built nowhere because they were dropped: the job-history
spec dropped P0-4 on 2026-07-29 after Q1 confirmed that removing the render gate alone fixed the
slow status page. The topic doc `observability-and-jobs.md`, written in HOLODEX-537 from ADR-071,
carried the pre-drop contract. Resolution: the doc now says the log is deliberately unpaged and
links the spec. No code change.

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge when CI passes; HOLODEX-538 moves to Done on merge

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-09 · decided: drop, not build
- skills: none
- handoff: Owner chose to fix the doc instead of building paging, since spec P0-4 was dropped on purpose; observability-and-jobs.md now says the log is unpaged by design.

## Dropped — newest first (the reason is the point)

- Keyset paging and kind/status/entity filters on `/activity/history`: dropped in the spec on 2026-07-29 (Q1). The log tab is rarely opened and the 30-day prune bounds it.
