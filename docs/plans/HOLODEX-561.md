---
key: HOLODEX-561
status: in-progress
profile: backend
depends-on: []
release_note: "Removing a video's last tag on its detail page can now be written back to the file on its own. Before, the removal could only be written alongside some other changed field."
---

# HOLODEX-561 · Removing a video's last tag can't be written back

Owner report (2026-10-11): an owner removes a tag on Media Detail and can't write it back unless
some other field also changed. Cause: removing the **last** tag leaves an empty write set, so
`applyTagSetSync` synthesizes an empty genres row (the file still has the tag). `markWriteTargets`
skipped it, because the mapper drops a valueless field. The row went out with no `write_target`,
so the dialog and the out-of-sync count treated it as unwritable. Fix: ask for the empty tag set
row's target, as for cleared rows. The queued write already deletes Genre on an empty union
(ADR-110 D6).

## Gates — definition of done

- [~] spec — no behavior change; restores the existing tag-set writeback rule (ADR-110 D6 / HOLODEX-401)
- [x] backend → `internal/api/writeback.go`
- [x] testing `testing-strategy` — `TestGetMedia_EmptyGenresRow_WriteTarget` + emptied-set case in `tagSetRow.test.ts`

## Up next — ordered (position = priority)

1. [ ] Squash-merge the PR on green CI, then confirm CI moved HOLODEX-561 to Done

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-11 · empty tag set row gets its write target
- skills: code-review high --fix
- handoff: Reproduced with a failing Go test, fixed `markWriteTargets`, and added a frontend test for the emptied set. Every gate is settled. Start at squash-merge on green CI.
