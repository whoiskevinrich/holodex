---
key: HOLODEX-533
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # architecture-doc consolidation; no product surface
depends-on: [HOLODEX-531]
release_note: ""
---

# HOLODEX-533 · Topic docs batch 2 (field-resolution, metadata-providers, entity-identity)

This is batch 2 of HOLODEX-526. It writes three topic docs from the resolver, completeness,
provider-contract and identity ADRs, and marks 33 index rows. Completeness has no doc of its own:
it lives in `field-resolution.md` as a registry flag, a trigger-invalidated store and a boot
fingerprint.

Where the ADRs had drifted, the docs follow the code:
- 083: link templates accept http(s).
- 039: the recognized asset kinds have grown, so the doc names `assetRoleFor` rather than listing them.
- 061: the tag key folds spaces only.
- 107: the memo backfill shipped as migration 0052.
- 055: the namespace invariant is only partly enforced, and the doc says so.

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge after HOLODEX-531 (#446). This branch stacks on it
2. [ ] [—] Batch 3: entity-relationships, images (absorb 059 §2/§4), media-ingest
3. [ ] [—] Batch 4: writeback (absorb sync-state, 120 D4 tag deletion, 096 D4 edition), observability-and-jobs (absorb 033 §6, 103 D6–D9), browse-and-list-state. Then close HOLODEX-526 by hand

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-04 · batch 2 written
- skills: code-review
- handoff: Three topic docs written and checked against the code; 33 index rows marked. Spec and design leftovers, plus the items for later batches, are in a comment on HOLODEX-527.

## Dropped — newest first (the reason is the point)
