---
key: HOLODEX-541
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # documentation; no product surface
depends-on: []
release_note: ""
---

# HOLODEX-541 · Rule move batches 2–4 (remaining ADR leftovers)

This completes HOLODEX-527, scope agreed 2026-10-04: ADR leftovers only. About 100 items were
checked across five feature areas. Most were already in their spec or design doc. Added where
missing (17 files), each checked against the code:
- ADR-078's category conflict copy
- the film display-name surfaces and refetch on detach
- the display-name surfaces
- trash 404/409
- F5 scroll restore
- the tag-flag hierarchy rule
- the scan-speed NFR
- the fixed resolution buckets
- provider health marked not built
- the monogram fallback
- the halo handoff brought to the single skin
- Revert confirm and scope
- the duplicates chip
- the year/release-date note
- studio prune noted against HOLODEX-535
- ADR-090's inline-adoption rule in `ui-vocabulary.md`

Dropped: ADR-041's per-field write (superseded) and ADR-049's deferred provenance badge (never
built).

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge when CI passes, then close HOLODEX-527 by hand
2. [ ] [—] HOLODEX-528: retire the numbered ADRs (the checklist is in a comment on 528)

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-04 · batches 2–4 combined
- skills: code-review
- handoff: Every ADR product and UI rule now lives in a spec, a design doc or `docs/reference/`. Docs found misleading (retired-skin QA, self-contradictions) are filed as HOLODEX-542 rather than fixed here.

## Dropped — newest first (the reason is the point)
