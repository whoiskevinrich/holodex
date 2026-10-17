---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-581
status: in-progress
profile: chore               # docs-only correction: two docs restated to match shipped code
depends-on: []
release_note: ""
---

# HOLODEX-581 · Correct film-enrichment claims in the provider contract and entity-relationships

Found during HOLODEX-95. The provider contract (§3 note, §4.2c) and `entity-relationships.md`
(Films section, Vocabulary bullet) described film enrichment wrongly. Provider `aliases` is not
ignored on a film (HOLODEX-376 writes it to the film's aliases). A provider `title` is a display
spelling of the name (F60 RD9), not ignored. Unknown keys do not auto-register on a film; they stay
in the shadow store unshown, and wiring that up is HOLODEX-579. Docs only, no code.

**Design package:** none (docs correction)

## Gates — definition of done

_Posture `chore` holds no gates._

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR and confirm CI moves HOLODEX-581 to Done
2. [ ] [—] Optional follow-up: add `title`/`aliases` rows to the §4.2c canonical-field table (left out to keep this change scoped)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-16 · session
- skills: code-review (high --fix)
- handoff: Both docs now match main for film `title`, `aliases` and unknown keys, and link HOLODEX-579. Next: merge the PR.

## Dropped — newest first (the reason is the point)
