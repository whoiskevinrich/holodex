# ADR-091: Writeback is fire-and-forget; job status is a property of the video, not of the dialog

**Status:** Proposed
**Date:** 2026-09-05
**Deciders:** Project owner

**Supersedes:** [ADR-073](ADR-073-post-write-baseline-resync.md) **D4** only — the decision that the
SPA polls a job's status to a terminal state *before reporting applied*. ADR-073's **D1** (the
post-write read-back runs after every successful write, ungated), **D2** (the synchronous branch does
the same), and **D3** (`GET /writeback/jobs/{id}` exists and is owner-gated) all stand unchanged. D1
in particular is load-bearing for this decision: without an unconditional post-write read-back, the
page could not re-resolve to a correct baseline when a job lands.
**Extends:** [ADR-048](ADR-048-metadata-curation-and-write-queue.md) (the durable write queue) ·
[ADR-041](ADR-041-metadata-writeback.md) (copy → write → rename).
**Relates to:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) /
[ADR-052](ADR-052-baseline-source-contract.md) (the baseline `in_sync` is computed against) ·
[ADR-090](ADR-090-two-layer-entity-metadata-management.md) (this is neither adoption nor precedence —
see *Scope* below).
**Issue:** [HOLODEX-323](https://whoiskevinrich.atlassian.net/browse/HOLODEX-323).

---

## Context

The writeback backend has been fire-and-forget since ADR-048. `POST /media/{id}/writeback`
sanitizes the submitted fields, inserts a `writeback_queue` row, and answers `202` with a job id;
the request context scopes only that insert. The worker pool runs on the application-lifetime
context passed to `Queue.Start(ctx)`, so no browser action — navigating away, closing the tab,
killing the process — can cancel an in-flight write. ADR-041's copy → write → rename guarantees the
original file is never left half-written, and `RecoverRunningWritebacks` re-runs anything a crash
interrupted.

The frontend does not reflect any of this. `WritebackFormDialog.svelte` gates Escape, the backdrop,
the close button and Cancel on `busy`, then polls `GET /writeback/jobs/{id}` for up to
`JOB_POLL_TIMEOUT_MS` (120 s). The user is held in front of a modal for a write that has already
been made durable and that they cannot affect.

ADR-073 D4 introduced that wait for a real reason. `in_sync` is `decided == fileVal` against the
*stored* baseline (ADR-052); refetching on the `202` reads the pre-write baseline and renders every
just-written field as "file out of sync", which reads as silent failure and invites a redundant
write. D4 made the dialog wait until the job was terminal so the refetch would see post-write state.

That solved the symptom by making the client block. The cost is that the *only* place a writeback's
outcome exists is a poll held in one component's memory: unmounting the dialog cancels it
(`cancelled: () => unmounted`), navigating away discards it, a reload loses it, and the 120 s cap
discards it even for a user who waits. A job that fails after the dialog is gone is visible only on
the Activity page. The owner has confirmed D4's premises no longer hold and that this supersedes it.

ADR-073's own consequences already flag the structural half of this: the `done`-means-absent contract
(D3) "is safe only for a poll seeded by a fresh enqueue. A second consumer — a poll resumed after a
reload, a batch/merge progress view..." cannot distinguish succeeded from never-existed. Any design
that reads job status outside the enqueueing component inherits that ambiguity.

## Decision

**D1. The dialog closes on the enqueue acknowledgement, not on job completion.** It holds only for
the `202` — a single fast insert — then closes. It renders no success state, because success is now
signalled by the absence of a signal.

The distinction is deliberate: the *write* is fire-and-forget, the *enqueue* is not. A failed enqueue
(expired owner session, dropped connection) leaves the dialog open with the error inline. This costs
one round trip and removes the only failure mode where the user would otherwise get no feedback at
all.

**D2. Writeback status is a property of the video, read from the queue.** The media payload carries
this video's pending and failed job state, derived from `writeback_queue` by `video_id` (the column
already exists and is already indexed by status). It is not a job id threaded through the client.

This makes the signal durable in exactly the ways the ADR-073 D4 poll was not: it survives reload,
tab close, a different tab, and a server restart, because it is derived from the same rows the worker
consumes rather than from client-held state.

**D3. Absence renders as silence, which resolves D3's second-consumer hazard for this consumer.**
`FinishWriteback` deletes the row on success and marks it `failed` otherwise. So for a given video:
a `pending`/`running` row means pending, a `failed` row means failed, and **no row means nothing to
say** — whether the job succeeded, was swept, or never existed.

ADR-073 flagged that conflation as a hazard for a second consumer. It is harmless here, because the
three absent cases all render identically and correctly: nothing. This design is safe *because* it
only ever asks "is there something to report?", never "did job N succeed?".

**D4. Failed jobs persist until acted on, and are retryable.** A failure stays visible until the
owner retries or dismisses it. `ClaimNextWriteback` selects `WHERE status = 'pending'` and
`RecoverRunningWritebacks` resets only `running`, so nothing currently moves `failed` back to
`pending` — there is no auto-retry, and `attempts` never exceeds 1 in practice. Retry is therefore a
new repo method: reset the row to `pending` and `kick()` the queue.

An error that clears itself would break D3: if a failure could vanish on its own, absence would no
longer mean "nothing to say".

**D5. The signal is job-level and section-scoped, never per-field.** A job is one `exiftool` or
`mkvpropedit` invocation followed by one `os.Rename` — it lands whole or not at all. One job yields
one chip in the Metadata section header; the error sentence names the fields the job carried.

## Scope

This ADR governs the *transport and lifecycle* of a writeback's outcome. It is not an ADR-090 layer:
it is neither adoption (should this candidate enter the shadow store?) nor precedence (which
namespace wins for this field?). It says nothing about which value gets written — that is settled
before the job is enqueued — only about how the result of writing it is reported.

It also does not change the per-field `file out of sync` pill from ADR-051. That pill continues to
mean "the decided value differs from the stored baseline"; this ADR adds a separate, section-level
statement about whether a write is in flight or has failed.

## Consequences

**Good**

- The user can navigate away the instant a write is submitted, which matches what the backend has
  always done.
- A failed write is visible on the page it belongs to, not only on the Activity page, and survives
  the session in which it failed.
- The `JOB_POLL_TIMEOUT_MS` cap stops being a correctness boundary. Today a job that outruns 120 s
  leaves the page stale even for a user who waited; with status on the payload, a slow job is just a
  chip that persists longer.
- Deleting the dialog's per-row status removes UI that was already fiction on the queued path.
- Materially reduces the motivation for [HOLODEX-276](https://whoiskevinrich.atlassian.net/browse/HOLODEX-276)
  (per-field written/skipped on the async path). If the write is atomic per job, per-field outcome
  is not a distinction the queue can honestly report. That ticket should be re-evaluated, not
  assumed still wanted.

**Bad / accepted**

- The media payload grows by a small status object, and `GET /media/{id}` now reads
  `writeback_queue`. Accepted: it is an indexed lookup on an owner-facing detail route.
- A dismissed failure is gone from the page. The `job_runs` row persists independently, so the
  Activity page remains the audit trail; the page-level chip is a notification, not a record.
- Pending must be polled while visible, since there is no push channel. Reuse `pollUntilSettled`
  from `$lib/writebackJob.ts` rather than adding a second polling idiom.
- Absence-means-silence is correct only while `FinishWriteback` deletes on success. If a future
  change retains completed rows, D3 must be revisited — a retained `done` row would start rendering
  as something rather than nothing.

## Alternatives considered

**Keep D4, but allow the dialog to be closed.** Minimal change, and wrong: it reintroduces exactly
the stale "N out of sync" reading that D4 was written to prevent, for any user who closes early.

**Thread the `job_id` up to the page and poll there.** Zero backend work and it reuses
`waitForWritebackJob` verbatim. Rejected because the state still dies on reload and cannot show a
failure from a previous session — it moves the fragility up one level rather than removing it. It
also inherits ADR-073 D3's second-consumer ambiguity without D3's mitigation, since a resumed poll
genuinely cannot tell success from never-existed.

**Server-sent events for job completion.** The best interaction, and over-built for a single-owner
personal server: new transport, new reconnect semantics, new failure modes, to replace a poll that
runs only while a chip is on screen.

**A global "N writes pending" indicator only.** Cheapest, and it answers the wrong question — it
tells the owner that *something* is writing, not whether *this* video is affected.
