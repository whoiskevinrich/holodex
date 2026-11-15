---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-635
status: in-progress          # DERIVED from the Gates below; only `done` and `released` are read from here.
profile: backend             # a plan-payload bug; the only UI change is a null guard on the existing line
depends-on: []
release_note: The File names page no longer crashes when a template gains a token that a file wasn't named with.
---

# HOLODEX-635 · File names page crashes when a changed token has no recorded value (F86 bug)

Done means `/rename/plan` never sends `null` for a change's `recorded` or `current`, and the
File names page renders an out-of-date row whose token is on one side only.

## Gates — definition of done

- [~] spec `write-spec` → `docs/specs/**`. No behavior change: the payload already promised `string[]` (`web/src/lib/rename.ts`); this makes the server keep that promise.
- [x] backend → `{cmd,internal,providers}/**`. `changes` in `internal/rename/plan.go` encodes a one-sided token as `[]` via `nonNil`. `RenamePlanRow.svelte` also shows `—` for an empty or missing side, so an older server can't crash the page.
- [x] testing `testing-strategy`. `TestChangesOneSidedTokenEncodesEmpty` (failed with the live `null` payload before the fix); `RenamePlanRow` test "shows a dash for a token with no recorded value". `go test ./internal/rename ./internal/api`, the row tests and `npm run check` (0 errors) pass.

## Up next — ordered (position = priority)

1. [ ] [—] On merge, check HOLODEX-635 moved to Done (CI moves the branch's own key).

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-11-10 · null recorded value crashes File names
- skills: —
- handoff: Found from a live private-media crash (`Cannot read properties of null (reading 'join')`) after `{film?}`/`{edition}` were added to the first template. Fix and tests landed with every gate settled; next is merging.
