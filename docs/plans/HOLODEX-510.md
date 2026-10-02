---
key: HOLODEX-510
status: in-progress
profile: backend             # behavior with no UI surface — a writeback queue id-reuse bug fix
depends-on: []
release_note: Each write to a file now saves its own copy of the values it replaces, so Revert restores exactly what that write changed. Before, a write that came right after another could skip saving its copy, and Revert then mixed the two writes.
---

# HOLODEX-510 · Writeback snapshot batch id collides when a job id is reused

This is done when no two writeback jobs ever share an id, so a job's id-derived snapshot batch
(`snapshotBeforeWrite`'s fallback, F48.9a) is unique. The crash-recovery property must still hold:
a retry of the *same* job finds its own snapshot and doesn't take a new one.

**Root cause (found under HOLODEX-505, media #3545):** `writeback_queue.id` was a plain
`INTEGER PRIMARY KEY`, and `FinishWriteback` deletes a job's row on success. SQLite then gave the
next job the same id. The 22:36:59 genres write found the 22:36:15 write's snapshot under batch "N"
and skipped its own.

**Fix:** migration 0057 rebuilds the queue with `AUTOINCREMENT`, which this schema already uses
elsewhere, and starts the id counter above every all-digit batch id still held in snapshots or
`job_runs`. The batch id stays the job id, so the UI and Revert are unchanged.

**Design package:** bug fix, no spec or ADR (it makes ADR-067's job-id batch premise true again) ·
[testing-strategy row](../testing-strategy.md) (HOLODEX-510)

## Gates — definition of done

- [~] spec `write-spec` → `docs/specs/**`. Skipped deliberately: a bug fix restoring documented snapshot/Revert behavior (F48.9a, ADR-067). No requirement changes.
- [x] backend → `{cmd,internal,providers}/**`. Migration 0057 (`writeback_queue` AUTOINCREMENT, sequence seeded above used batch ids).
- [x] testing `testing-strategy`. Migration round-trip test plus a repo test that was red without 0057. A row is in `docs/testing-strategy.md`.

## Up next — ordered (position = priority)

1. [ ] [—] **Merge only after PR #428 (HOLODEX-507)**, which claims migration 0056. If this lands first, prod moves to 57 and golang-migrate never runs 0056. If #428 stalls, renumber here instead.
2. [ ] [—] Batches that collided before this fix still mix two writes' snapshots (e.g. media #3545's 2026-10-01 genres batch). Don't trust Revert on those. Repairing them is out of scope — prod

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-01 · made writeback job ids unique (migration 0057)
- skills: code-review, handoff
- handoff: The fix and tests are committed and every gate is settled. Merge after #428 (migration 0056) lands; otherwise renumber 0057.

## Dropped — newest first (the reason is the point)
