---
key: HOLODEX-595
status: in-progress
profile: full                # bug fix spanning backend + frontend; no posture holds just those, so spec/architecture/design/security are skipped below
depends-on: []
release_note: The Sync writeback progress bar on a tag's page now moves while the sync runs and finishes with a count of updated and failed files. Before, it could stay at "0 of N" until the sync ended or timed out.
---

# HOLODEX-595 · Tag writeback sync dialog progress hangs at "0 of N"

This is done when the tag sync dialog's bar advances while the batch runs and every settled job
counts toward done/failed.

**Root cause:** two bugs.
1. `WritebackBatchDialog` assigned `status` only once polling settled, so every poll in between was
   thrown away.
2. `writequeue.process` replaced the job's `batchID` with `snapshotBeforeWrite`'s return value,
   which is `""` when the pre-write snapshot read fails. That `job_runs` row was then filed under no
   batch, so `GET /writeback/batches/{id}/status` never counted it.

**Fix:** `waitForWritebackBatch` takes an `onProgress` callback. The worker keeps `job.BatchID`
unless a snapshot recorded an id. The detail line's revert pointer still reflects only a real
snapshot.

**Design package:** bug fix, no spec, architecture or design change ·
[testing-strategy row](../testing-strategy.md) (HOLODEX-595)

## Gates — definition of done

- [~] spec `write-spec` → `docs/specs/**`. Skipped deliberately: a bug fix restoring the specced progress behaviour (HOLODEX-239, ADR-077 D3). No requirement changes.
- [~] architecture `architecture` → `docs/architecture/**`. Skipped deliberately: no technology fork.
- [~] design `design-handoff` → `docs/design/**`. Skipped deliberately: no UX change; the dialog already drew live counts.
- [x] backend → `{cmd,internal,providers}/**`. `writequeue.process` keeps the job's batch id when the snapshot fails.
- [x] frontend → `web/src/**`. `waitForWritebackBatch` gains `onProgress`; the dialog feeds it into `status`.
- [x] testing `testing-strategy`. A Go test and a Vitest case, both red before the fix, plus a live run. A row is in `docs/testing-strategy.md`.
- [~] security `security-review`. Skipped deliberately: no auth, access or infrastructure change.

## Up next — ordered (position = priority)

1. [ ] [—] Push and open the PR; merge once CI is green
2. [ ] [—] Follow-up to consider: the dialog still stops polling at the 120s cap (`JOB_POLL_TIMEOUT_MS`), so a long MKV sync ends at "Still going". This is by design today.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-25 · fixed the tag-sync progress hang
- skills: code-review, handoff
- handoff: Both causes are fixed and tested (Go + Vitest, red before the fix) and verified live on the stress fixture. Every gate is settled and the release note is written, so the next step is to push, open the PR and merge on green.

## Dropped — newest first (the reason is the point)
