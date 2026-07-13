---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-173                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Cast members on a video's page now show their age at the time of that video's release.
---

# HOLODEX-173 · Age-in-media: person's age at the time of a video

Cross-entity derived field: joins a person's resolved `birthdate` with a video's resolved `release_date`
at the API layer to show each cast member's age at release time, on the video's cast poster grid. Done
means the backend join + frontend badge + tests land — not just the design decided.

**Design package:** [spec](../specs/age-in-media.md) · ADR N/A (reuses [ADR-063](../architecture/ADR-063-derived-computed-fields.md)) · [handoff](../design/age-in-media-handoff.md) · testing-strategy §TBD

**Note (2026-07-12):** Jira shows this issue's status as **Done**, fired by CI when PR #137 (the
design-handoff docs only) merged. That's premature — the gates below show backend/frontend/testing still
open. The PR-merge→Done automation appears to trigger on any merge of a linked branch rather than on this
story's own gates being satisfied; flagged to the project owner, not corrected here.

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → [docs/specs/age-in-media.md](../specs/age-in-media.md)
- [x] architecture `architecture` → N/A, reuses ADR-063 (no new ADR; a bespoke API-layer join introduces no new architecture)
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [x] security `security-review` → N/A, read-only computed value, no auth/access/infra change

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [ ] [backend] Extend cast query to select `p.birthdate, p.deathdate`; add `ageInMedia` helper (exports/wraps `wholeYearsBetween`/`parseDate`) and wire into `getMedia` (FR1–FR4) — `internal/repo/repo.go`, `internal/api/handlers.go`, `internal/resolver/derive.go`
2. [ ] [frontend] Corner badge on `PersonPoster` per the landed design (bottom-right, number only, `bg-black/70`) — `web/src/lib/components/PersonPoster.svelte`, `web/src/routes/media/[id]/+page.svelte`
3. [ ] [testing] Unit (`ageInMedia` cases) + API integration + no-`recorded_at`-fallback guard + frontend skin QA, per spec Test Notes — `docs/testing-strategy.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-07-12 · Design handoff landed; PR #134 closed as redundant; Jira Done-status mismatch found
- skills: design-handoff
- handoff: Backend (FR1–FR4) and frontend (FR5) implementation is next — the design is decided (corner
  badge, number only, bottom-right on `PersonPoster`) but nothing is wired yet. Also: Jira shows this
  issue Done from the docs-only PR merge — don't trust that status; the gates above are the real state.
