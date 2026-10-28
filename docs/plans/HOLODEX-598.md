---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-598                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — only `done` and `released` are read from here
profile: feature             # the gate posture (see flightplan.yaml `postures:`)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The Duplicates page opens one entity type at a time, and people and studio pairs now compare side by side, with large headshots and where each studio name came from.
---

# HOLODEX-598 · Duplicates: one lane per entity type, Videos-style compare for People and Studios

Done means `/owner/duplicates` loads one lane at a time, with every lane's count on its tab. Person
and studio pairs merge from inside their compare panel ("Merge into this one" under each column), and
a studio merge can pick which spelling the survivor shows.

**Design package:** [spec F70](../specs/duplicates-pair-evidence.md) (598 revision) ·
[design handoff](../design/duplicates-pair-evidence-handoff.md) + its two SVGs · Videos lane:
[F76](../specs/duplicate-videos.md) P0-2 · testing-strategy § — not yet written

## Gates — definition of done

- [/] spec `write-spec` → `docs/specs/**` — the 598 revision and OQ7 are in, but the handoff's spec deltas (Up next 1) aren't applied yet
- [~] architecture — no technology fork: lanes and studio evidence reuse the queue and identity model, and Show as reuses merge + rename (spec "API")
- [x] design `design-handoff` → `docs/design/**` — handoff + `duplicates-pair-evidence-mockup.svg` + `duplicates-studios-compare-mockup.svg` (1547d0a1)
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] [spec] Apply the handoff's spec deltas: "Merge into this one", P0-4's accent scope, P0-10's posters — `docs/specs/duplicates-pair-evidence.md`
2. [ ] [spec] Probe OQ5 (load baseline) and OQ6 (studio name-source cost), anonymized — the dup-probes session may hold these
3. [ ] [design] `/implement`: put the handoff and both SVGs in front of the owner and record the sign-off — `docs/design/duplicates-pair-evidence-handoff.md`
4. [ ] [testing] `/testing-strategy` for lanes, the studio panel and Show as — `docs/testing-strategy.md`
5. [ ] [backend] Per-lane read with counts, studio name sources and posters, merge that renames the survivor — `internal/api`, `internal/repo`  ⛔ blocked on #3
6. [ ] [frontend] Build to the handoff's checklist — `web/src/routes/owner/duplicates/`, `web/src/lib/components/duplicates/`  ⛔ blocked on #3

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-27 · session
- skills: design-handoff (worked by hand, not via the Skill tool), handoff
- did: revised the F70 handoff and its SVG, and added a Studios SVG, for lanes, the People rework, the Studios panel and Show as. The owner chose: tab row, Videos accent grammar, 3 posters one per source then newest, Show as as a radio, and "Merge into this one" replacing "Keep this one". OQ7 answered in the spec. Cleared `needs-design` on Jira, and fired In Progress by hand.
- handoff: Design is committed (1547d0a1) but the spec still says "Keep this one", and P0-4 still says Keep separate is the accent on every pair: apply the handoff's "Spec deltas" table first, then /implement.

## Dropped — newest first (the reason is the point)
