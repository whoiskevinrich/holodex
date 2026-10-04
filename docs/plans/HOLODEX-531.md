---
key: HOLODEX-531
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # architecture-doc consolidation; no product surface
depends-on: [HOLODEX-524]
release_note: ""
---

# HOLODEX-531 · Topic docs batch 1 (stack, config-and-settings, deployment, security-perimeter)

This is batch 1 of HOLODEX-526, the work that folds the numbered ADRs into 13 living topic docs.
Batches land as separate PRs, each keyed to its own sub-task so the parent stays open. This batch
writes four topic docs and marks the 28 index rows they absorb.

The docs follow the code where an ADR had drifted:
- 006: the OpenAPI/swaggo setup was never built.
- 003: there are no split read/write pools; a single `writeMu` serializes writes.
- 014: the data layout and reload scope were stale.
- 060: the settings-promotion pattern is decided but nothing has been promoted yet.

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge after the HOLODEX-524 PR (#444). This branch stacks on it. HOLODEX-531 moves to Done via jira-sync
2. [ ] [—] Batch 2 on a new sub-task: field-resolution (absorbs completeness), metadata-providers, entity-identity
3. [ ] [—] Batch 3: entity-relationships, images, media-ingest
4. [ ] [—] Batch 4: writeback (absorbs sync-state), observability-and-jobs, browse-and-list-state. Then close HOLODEX-526 by hand

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-04 · topic list agreed, batch 1 written
- skills: code-review
- handoff: Four topic docs written from the ADRs and checked against the code; index rows marked Folded or "arch part". Leftovers are filed: spec/design items in a comment on HOLODEX-527, dead cache config as HOLODEX-530. The batch brief for the next batches lives in the session scratchpad. Re-derive it from this worklog plus `docs/reference/doc-types.md` if it's lost.

## Dropped — newest first (the reason is the point)
