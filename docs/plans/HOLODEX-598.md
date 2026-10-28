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
- [x] backend → `{cmd,internal,providers}/**` — planner stats at boot (ed193bd5), per-lane read + lane counts (1248e559), studio evidence (17e88124), Show as (6dea91e1); green `go test`
- [ ] frontend → `web/src/**`
- [/] testing `testing-strategy` — backend half in §16.2 (b956c058); the frontend build adds its rows

## Up next — ordered (position = priority)

1. [ ] [frontend] Build to the handoff's checklist against the backend shapes (side-session note `598-backend-api-shapes.md`) — `web/src/routes/owner/duplicates/`, `web/src/lib/components/duplicates/`
2. [ ] [testing] Add the frontend rows to §16.2 (lanes, studio panel, Show as radio) — `docs/testing-strategy.md`
3. [ ] [backend] Record the owner's live OQ5 baseline in spec OQ5 ⛔ waiting on the owner running `oq5-live-baseline.sh` (read-only, counts and timings only) — `docs/specs/duplicates-pair-evidence.md`
4. [ ] [backend] Refresh planner statistics after a large scan, not only at boot → HOLODEX-603 — `internal/db/db.go`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-27 · session
- skills: design-handoff (worked by hand, not via the Skill tool), handoff, implement, code-review, testing-strategy
- did: revised the F70 handoff and its SVG, and added a Studios SVG, for lanes, the People rework, the Studios panel and Show as. The owner chose: tab row, Videos accent grammar, 3 posters one per source then newest, Show as as a radio, and "Merge into this one" replacing "Keep this one". OQ7 answered in the spec. Cleared `needs-design` on Jira, and fired In Progress by hand.
- did (later): applied the handoff's spec deltas to F70 ("Merge into this one", P0-4's accent scoped to tag and film rows, P0-10's posters) and ticked spec.
- did (later): folded the OQ5/OQ6 probe answers into F70. OQ5 is closed with a ≤1 s target, and the live baseline is still to record. OQ6 is closed. P0-10 is reworded to current sources only, with provider identities, alias origins and a winner caption per poster. The Studios SVG and handoff are updated to match.
- did (backend session): planner statistics at boot (`PRAGMA optimize`), so the scaled 20k-video copy reads in 0.1 s, not 23 s; `?type=` reads one lane, paged in SQL, with `counts` for all five lanes on both lists; `GET /owner/duplicates/studios/{id}` (name sources, OQ7 posters with winner, aliases, provider links); `show_as` on the studio merge (merge + rename, one transaction). Tests + §16.2, mutation-checked. The OQ5 baseline script went to the owner; `/code-review high --fix` ran before each commit.
- handoff: Backend gate done and pushed; draft PR stays draft. Next: the frontend build against `598-backend-api-shapes.md` (Up next 1). The live OQ5 numbers are still owed by the owner.

## Dropped — newest first (the reason is the point)
