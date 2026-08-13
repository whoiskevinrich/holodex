---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-216                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Writeback now shows a field's destination file tag and only offers fields it can actually write — a mixed batch tells you exactly what was written instead of reporting success for fields it silently skipped.
---

# HOLODEX-216 · Writeback hides the target file tag and reports success for fields it silently skipped

Bug fix, parent epic HOLODEX-167 (Writeback). Done means: the writeback dialog shows each row's
destination file tag, a field with no mapping for the video's container is visibly unwritable and
can't be checked, and the sync response distinguishes written from skipped fields instead of a bare
204 that lied about what happened.

**Design package:** spec: not required (restores/clarifies existing documented behavior, doesn't
introduce new scope) · ADR: not required · [design handoff](../design/writeback-target-visibility-handoff.md) ·
[testing-strategy](../testing-strategy.md) (HOLODEX-216 entry, §11)

## Gates — definition of done

- [~] spec `write-spec` — until: never; judged unnecessary, this extends the sync path's existing full-unmapped-422 policy consistently to the mixed case rather than deciding new behavior
- [~] architecture `architecture` — until: never; no data model/deployment/cross-cutting decision, only an API-layer field stamp + response shape
- [x] design `design-handoff` → `docs/design/writeback-target-visibility-handoff.md`
- [x] backend
- [x] frontend
- [x] testing `testing-strategy` → `docs/testing-strategy.md` §11
- [~] security `security-review` — until: never; no auth/access/infrastructure surface touched

## Up next — ordered (position = priority)

1. [ ] [—] Async (durable-queue) writeback path still can't report per-field written/skipped after
   the job completes — would need new `job_runs` columns. Filed as its own issue. → HOLODEX-276

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-13 · session
- skills: design-handoff, testing-strategy, simplify
- handoff: implementation + gates are green (including three-skin QA, verified live against
  `backend-films`/`web` previews — unwritable rows read at 4.67–18.5:1 contrast in all three
  skins); a regression test (`TestGetMedia_GenresRow_WriteTarget`) now pins the markWriteTargets
  call-order fix the `/simplify` altitude pass caught. The async per-field-outcome gap is filed
  as HOLODEX-276 (Up next #1, not this epic's blocker). Pushed and PR opened this session —
  HOLODEX-216 is ready for review.
