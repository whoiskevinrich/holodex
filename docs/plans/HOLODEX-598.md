---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-598                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — only `done` and `released` are read from here
profile: feature             # the gate posture (see flightplan.yaml `postures:`)
depends-on: []               # [KEY-…] cross-epic deps that must land first
approved:
  design:
    on: 2026-10-27
    at: 5464438f
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

- [x] spec `write-spec` → `docs/specs/**` — the 598 revision, OQ7 and the handoff's spec deltas are in. OQ5/OQ6 are probes the build waits on, tracked in Up next
- [~] architecture — no technology fork: lanes and studio evidence reuse the queue and identity model, and Show as reuses merge + rename (spec "API")
- [x] design `design-handoff` → `docs/design/**` — handoff + `duplicates-pair-evidence-mockup.svg` + `duplicates-studios-compare-mockup.svg` (1547d0a1)
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] [backend] Record the live OQ5 baseline (time to first usable lane, anonymized) before any build change — host `./data/holodex.db`
2. [ ] [testing] `/testing-strategy` for lanes, the studio panel and Show as — `docs/testing-strategy.md`
3. [ ] [backend] Run ANALYZE / PRAGMA optimize at boot, or count videos from the junction table: the correlated per-side count (review_queue.go:272) degrades without sqlite_stat1 — `internal/repo/review_queue.go`
4. [ ] [backend] Per-lane read with counts: `listDuplicates` (duplicates.go:65) always loads all four types, so ?type= saves nothing today. Then studio name sources and posters, and a merge that renames the survivor — `internal/api`, `internal/repo`
5. [ ] [frontend] Build to the handoff's checklist — `web/src/routes/owner/duplicates/`, `web/src/lib/components/duplicates/`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-27 · session
- skills: design-handoff (worked by hand, not via the Skill tool), handoff, implement
- did: revised the F70 handoff and its SVG, and added a Studios SVG, for lanes, the People rework, the Studios panel and Show as. The owner chose: tab row, Videos accent grammar, 3 posters one per source then newest, Show as as a radio, and "Merge into this one" replacing "Keep this one". OQ7 answered in the spec. Cleared `needs-design` on Jira, and fired In Progress by hand.
- did (later): applied the handoff's spec deltas to F70 ("Merge into this one", P0-4's accent scoped to tag and film rows, P0-10's posters) and ticked spec.
- did (later): folded the OQ5/OQ6 probe answers into F70. OQ5 is closed with a ≤1 s target, and the live baseline is still to record. OQ6 is closed. P0-10 is reworded to current sources only, with provider identities, alias origins and a winner caption per poster. The Studios SVG and handoff are updated to match.
- handoff: Crossed into build — design signed off at 5464438f (owner, 2026-10-27); origin/main merged; draft PR open. Start at Up next 1: record the live OQ5 baseline before any backend change.

## Dropped — newest first (the reason is the point)
