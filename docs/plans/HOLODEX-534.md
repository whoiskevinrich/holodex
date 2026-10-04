---
key: HOLODEX-534
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # architecture-doc consolidation; no product surface
depends-on: []
release_note: ""
---

# HOLODEX-534 · Topic docs batch 3 (entity-relationships, images, media-ingest)

This is batch 3 of HOLODEX-526. It writes three topic docs from the link-model, image and ingest
ADRs, and marks 27 index rows. 057 is dead (superseded by the studio image roles). 012's resolution
buckets are computed at read time, so they move to batch 4's browse-and-list-state.

Where the ADRs had drifted, the docs follow the code:
- exiftool is the only tag source.
- Thumbnails are served `no-cache` plus 304, not long-lived.
- Refresh uses `BuildVideoFromFile`.
- The placeholder skin matrix was never built.

Two real findings are filed separately:
- HOLODEX-535 (Bug): studio prune-on-empty deletes authored studio data.
- HOLODEX-536: MKV target-level precedence was never implemented.

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge when CI passes (merge-as-green for this epic)
2. [ ] [—] Batch 4: writeback (absorb sync-state, 120 D4 tag deletion, 096 D4 edition, decision endpoints), observability-and-jobs (absorb 033 §6, 103 D6–D9, 047 job rows), browse-and-list-state (absorb 012's read-time buckets). Then close HOLODEX-526 by hand

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-04 · batch 3 written
- skills: code-review
- handoff: Three topic docs written and checked against the code; 27 index rows marked. Leftovers are in a comment on HOLODEX-527. Studio prune (535) and MKV target levels (536) are filed as their own tickets and linked.

## Dropped — newest first (the reason is the point)
