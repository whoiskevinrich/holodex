---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-598                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — only `done` and `released` are read from here
profile: feature             # the gate posture (see flightplan.yaml `postures:`)
depends-on: []               # [KEY-…] cross-epic deps that must land first
approved:
  design:
    on: 2026-10-28
    at: b82427d9
release_note: The Duplicates page opens one entity type at a time, and people and studio pairs now compare side by side, with large headshots and where each studio name came from.
---

# HOLODEX-598 · Duplicates: one lane per entity type, Videos-style compare for People and Studios

Done means `/owner/duplicates` loads one lane at a time, with every lane's count on its tab. Person
and studio pairs merge from inside their compare panel ("Merge into this one" under each column), and
a studio merge can pick which spelling the survivor shows.

**Design package:** [spec F70](../specs/duplicates-pair-evidence.md) (598 revision) ·
[design handoff](../design/duplicates-pair-evidence-handoff.md) + its two SVGs · Videos lane:
[F76](../specs/duplicate-videos.md) P0-2 · [testing-strategy §16.2–16.3](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — the 598 revision, OQ7 and the handoff's spec deltas are in. OQ5/OQ6 are probes the build waits on, tracked in Up next
- [~] architecture — no technology fork: lanes and studio evidence reuse the queue and identity model, and Show as reuses merge + rename (spec "API")
- [x] design `design-handoff` → `docs/design/**` — handoff + `duplicates-pair-evidence-mockup.svg` + `duplicates-studios-compare-mockup.svg` (1547d0a1)
- [x] backend → `{cmd,internal,providers}/**` — planner stats at boot (ed193bd5), per-lane read + lane counts (1248e559), studio evidence (17e88124), Show as (6dea91e1); green `go test`
- [x] frontend → `web/src/**` — built to the handoff checklist (d050158b); the owner compared each lane with the approved SVGs and signed off on 2026-10-28 after the D1–D5 fixes (b82427d9)
- [x] testing `testing-strategy` — backend §16.2 (b956c058), frontend §16.3 (d050158b); `npm run test` 809/809, 4 mutations killed

## Up next — ordered (position = priority)

1. [ ] [backend] Record the owner's live OQ5 baseline in spec OQ5, then mark the PR ready ⛔ waiting on the owner running `oq5-live-baseline.sh` (read-only, counts and timings only) — `docs/specs/duplicates-pair-evidence.md`
2. [ ] [backend] Refresh planner statistics after a large scan, not only at boot → HOLODEX-603 — `internal/db/db.go`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-28 · session
- skills: code-review, handoff
- did: sign-off review in the stress-598 preview, each lane beside its approved SVG. The owner had five differences fixed: the "Possible duplicate people/studios." intro lead, no intro on a loading or empty lane, curly quotes in both confirms, a 6 px gap before the row's ↗, and no accent baseline on the person and tag placeholders (list-views handoff updated to match).
- did (later): the owner signed off the re-shown build against the approved mockups; frontend ticked, and the design sign-off re-pinned to b82427d9 (its only `docs/design` change is the placeholder line the owner chose).
- handoff: Every gate is settled and the owner signed off the build; the draft PR is held only for the OQ5 live baseline (Up next 1). Once it's recorded in the spec, mark #565 ready.

### 2026-10-27 · session
- skills: design-handoff (worked by hand, not via the Skill tool), handoff, implement, code-review, testing-strategy
- did: revised the F70 handoff and its SVG, and added a Studios SVG, for lanes, the People rework, the Studios panel and Show as. The owner chose: tab row, Videos accent grammar, 3 posters one per source then newest, Show as as a radio, and "Merge into this one" replacing "Keep this one". OQ7 answered in the spec. Cleared `needs-design` on Jira, and fired In Progress by hand.
- did (later): applied the handoff's spec deltas to F70 ("Merge into this one", P0-4's accent scoped to tag and film rows, P0-10's posters) and ticked spec.
- did (later): folded the OQ5/OQ6 probe answers into F70. OQ5 is closed with a ≤1 s target, and the live baseline is still to record. OQ6 is closed. P0-10 is reworded to current sources only, with provider identities, alias origins and a winner caption per poster. The Studios SVG and handoff are updated to match.
- did (backend session): planner statistics at boot (`PRAGMA optimize`), so the scaled 20k-video copy reads in 0.1 s, not 23 s; `?type=` reads one lane, paged in SQL, with `counts` for all five lanes on both lists; `GET /owner/duplicates/studios/{id}` (name sources, OQ7 posters with winner, aliases, provider links); `show_as` on the studio merge (merge + rename, one transaction). Tests + §16.2, mutation-checked. The OQ5 baseline script went to the owner; `/code-review high --fix` ran before each commit.
- did (frontend session): the page is a lane tab row with counts on every tab and one read per lane (`?type=` via `replaceState`; no type → Videos first, whose counts pick the lane). Person and studio rows take the Videos grammar (whole-row toggle, new-tab names, ghost Keep separate, "Merge into this one" per column behind a confirm). People get the 240 px lead + swappable strip; new `DuplicateStudioPanel` shows name sources, posters and alias origins; the studio confirm carries the Show as radio. First usable lane: 194 ms with 2 reads and 51 rows before, 131–148 ms with 1 read after (stress fixture, warm). Tests in §16.3; `/code-review high --fix` fixed a stuck-Loading race and stale evidence after a merge. No `/security-review`: the frontend adds no auth surface, and the reads it calls are owner-gated in the backend commits.
- handoff: Frontend built, QA'd and pushed; the draft PR stays draft. Next: the owner compares the build with the two approved SVGs (Up next 1), then frontend is ticked and the PR marked ready. The live OQ5 numbers are still owed.

## Dropped — newest first (the reason is the point)
