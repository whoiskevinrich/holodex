---
key: HOLODEX-524
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # repo working rules + Flightplan config; no product surface
depends-on: []
release_note: ""
---

# HOLODEX-524 · Rules and gates for living spec / architecture / design docs

First child of epic HOLODEX-523. This lands the working rules so that new work follows the model
while the migration (HOLODEX-525…528) runs:

- Specs, architecture docs and design docs never comingle.
- All three are living docs that hold the current truth; git is the history.
- No new numbered ADRs.
- A feature with no technology fork records `[~] architecture — no technology fork`.

Done when CLAUDE.md, `flightplan.yaml` and `docs/reference/doc-types.md` say so.

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR after CI. Then move HOLODEX-524 to Done (jira-sync moves the branch key)
2. [ ] [—] HOLODEX-528 must keep the shared helpers `scripts/hooks/feature-claims-guard.mjs` imports from `adr-claims.mjs` when it deletes the ADR half

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-04 · brainstorm → epic → rules
- skills: product-brainstorming
- handoff: Rules landed. CLAUDE.md change-routing and conventions, the `flightplan.yaml` architecture glob (`docs/architecture/**`), a transition banner on the ADR index, and the new `docs/reference/doc-types.md`. Next in the epic is HOLODEX-525 (skill boundaries) or 526 (topic list to agree with the owner first).

## Dropped — newest first (the reason is the point)
