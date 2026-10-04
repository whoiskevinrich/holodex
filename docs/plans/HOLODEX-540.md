---
key: HOLODEX-540
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # documentation; no product surface
depends-on: []
release_note: ""
---

# HOLODEX-540 · Rule move batch 1 (foundation ADR leftovers)

This is batch 1 of HOLODEX-527, scope agreed 2026-10-04: ADR leftovers only. Most items were already
present in their spec or design doc. Added:
- the concrete session lifetimes (7/30/90 days, renew past half-life) to the owner-session spec
- writeback fidelity (free space, what a write preserves) to the F28 spec
- a "not built" marker on the runtime-settings spec
- MCP client and proxy notes to `configuration.md`
- first-boot re-index and GHCR-public notes to the README

Dropped: 006's endpoint list, which is code-derivable.

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge when CI passes
2. [ ] [—] Batches 2–4 of HOLODEX-527 (resolver/provider/identity, relationships/images/ingest, writeback/jobs/browse leftovers)

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-04 · batch 1
- skills: code-review
- handoff: Five files touched, each addition checked against the code. Specs flagged as stale beyond scope are left for when their feature is next touched: runtime-settings (table/endpoint language, migration 0021), owner-session-persistence (old paths, "ADR-046 to specify" notes), and the phase-3 F17.2/F17.3 backup model.

## Dropped — newest first (the reason is the point)
