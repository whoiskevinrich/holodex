---
key: HOLODEX-537
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # architecture-doc consolidation; no product surface
depends-on: []
release_note: ""
---

# HOLODEX-537 · Topic docs batch 4 (writeback, observability-and-jobs, browse-and-list-state)

This is the final batch of HOLODEX-526. It writes the last three of the 13 topic docs and marks 33
index rows. Every architecture ADR is now in a topic doc. The only unmarked rows are the 11 process
ADRs and 065 (Won't Do, never adopted), which HOLODEX-528 retires.

Where the ADRs had drifted, the docs follow the code:
- 067: the snapshot is taken by the worker before the write.
- 041: the endpoint returns 202 plus a job id.
- 019: no WAL checkpoint and no text handler.
- 017: no bm25 ranking.

Keyset paging that was decided but never built is filed as HOLODEX-538.

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge when CI passes (merge-as-green for this epic)
2. [ ] [—] Close HOLODEX-526 by hand (its sub-tasks 531/533/534/537 each closed on merge). Next in the epic: HOLODEX-525 (skill boundaries), 527 (move product/UI rules into specs and design docs, using the batch comments on 527), 528 (retire the numbered ADRs)

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-04 · batch 4 written; folding complete
- skills: code-review
- handoff: Last three topic docs written and checked against the code; 33 rows marked. Coverage check: everything is folded except the process ADRs and 065. The leftover lists are on HOLODEX-527, and the retirement list is on HOLODEX-528.

## Dropped — newest first (the reason is the point)
