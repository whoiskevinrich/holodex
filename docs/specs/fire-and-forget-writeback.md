# Spec: Fire-and-forget writeback with page-level status

**Status**: Draft
**Phase**: Jira [HOLODEX-323](https://whoiskevinrich.atlassian.net/browse/HOLODEX-323) (Story under the Writeback epic, HOLODEX-167)
**Owner**: Project owner
**Date**: 2026-09-06

Writing metadata to a file stops holding the owner in a modal. The dialog closes as soon as the
job is accepted, and the job's outcome moves to the Metadata section of the media detail page,
where it survives reload, tab close and server restart.

**ADR**: **[ADR-091](../architecture/ADR-091-fire-and-forget-writeback-status.md) (Proposed)** —
records the transport decision and supersedes **[ADR-073](../architecture/ADR-073-post-write-baseline-resync.md) D4 only**.
**Design handoff**: [fire-and-forget-writeback-handoff.md](../design/fire-and-forget-writeback-handoff.md)
(+ committed SVG mockup).

**Depends on** (all shipped):
- the durable write queue ([ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md)) —
  `writeback_queue`, `writequeue.Queue`, and the worker pool started from `cmd/holodex`
- atomic file writes ([ADR-041](../architecture/ADR-041-metadata-writeback.md)) — copy → write →
  rename, plus `RecoverRunningWritebacks` for crash recovery
- the unconditional post-write read-back ([ADR-073](../architecture/ADR-073-post-write-baseline-resync.md) **D1**,
  which this spec relies on and does not touch) — without it the page could not re-resolve to a
  correct baseline when a job lands
- the per-field decision model ([ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)/[ADR-052](../architecture/ADR-052-baseline-source-contract.md)) —
  `in_sync` and the existing per-field `file out of sync` pill
- the owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md), `requireOwner`)

---

## Context

The backend has been fire-and-forget since ADR-048. `POST /media/{id}/writeback` sanitizes the
submitted fields, inserts a `writeback_queue` row, and answers `202 {job_id, queued}`; `r.Context()`
scopes only that insert. The worker pool runs on the application-lifetime context passed to
`Queue.Start(ctx)`, so navigating away, closing the tab, or killing the browser cannot cancel an
in-flight write, and ADR-041's copy → write → rename means the original file is never left
half-written.

The frontend does not reflect this. `WritebackFormDialog.svelte` gates Escape, the backdrop, the
close button and Cancel on `busy`, then polls `GET /writeback/jobs/{id}` for up to
`JOB_POLL_TIMEOUT_MS` (120 s). The owner is held in front of a modal for a write that is already
durable and that they cannot affect. If they leave anyway, the poll is cancelled with the component
and a failure becomes visible only on the Activity page.

ADR-073 D4 introduced that wait for a real reason — refetching on the `202` reads the pre-write
baseline and renders just-written fields as "file out of sync" — so removing the wait without
replacing the signal would reintroduce that bug. The owner has confirmed D4's premises no longer
hold.

## Goals

1. Submitting a writeback returns control immediately; the owner can navigate away with no loss,
   no warning, and no lost outcome.
2. A pending or failed write for a video is visible on that video's page, and survives reload,
   another tab, and a server restart.
3. A successful write is signalled by the *absence* of any signal.
4. The dialog becomes a pre-flight confirm step: what will be written, where it lands, and what it
   replaces — reviewable before committing.

## Non-goals

- **The N-video batch dialog is unchanged** (RD3). `WritebackBatchDialog` and the ADR-077 batch
  status path keep their current blocking progress view.
- **No per-field write outcomes.** The write is atomic per job; see RD4 and the HOLODEX-276 note.
- **No push transport.** Pending is polled while the page is open; SSE is out of scope (ADR-091
  Alternatives).
- **No change to which value is written.** That is settled before the job is enqueued. This spec
  covers only how the result of writing it is reported.
- **No change to the per-field `file out of sync` pill** on individual rows.

## Requirements

### R1 — The dialog closes on the enqueue acknowledgement

**R1.1** On submit, the dialog awaits only the `202` from `POST /media/{id}/writeback`, then closes.
It does not poll for job completion.

**R1.2** Close affordances (Escape, backdrop, close button, Cancel) remain gated for that single
round trip, then behave normally. Focus returns to the trigger on close, as today.

**R1.3** If the enqueue fails — network error, expired owner session, `4xx`/`5xx` — the dialog stays
open, shows the error inline, and unlocks its close affordances. The write is fire-and-forget; the
*enqueue* is not.

**R1.4** No success toast. Silence is the success signal (R2.4).

**R1.5** `waitForWritebackJob` is no longer called by the dialog. It remains exported for any other
consumer; `pollUntilSettled` is reused by R2.5.

### R2 — Writeback status is a property of the video

**R2.1** `GET /media/{id}` carries this video's writeback status, derived from `writeback_queue`
by `video_id`: whether a job is pending (`pending` or `running`) and whether one has failed,
with the failed job's error message.

**R2.2** Absence means nothing to report. `FinishWriteback` deletes the row on success, so no row
covers succeeded, swept, and never-existed alike — all three correctly render nothing (ADR-091 D3).

**R2.3** The Metadata section header renders, beside the "Write decisions to file" action:

| Condition | Badge | Weight |
|---|---|---|
| Any field out of sync | `out of sync` | Outline, warn text, no fill |
| A job pending for this video | `writing to file` + spinner | Filled, accent |
| A job failed for this video | `couldn't write` + warning triangle | Filled, warn |
| None of the above | *(nothing)* | — |

**R2.4** Badges carry no counts (RD1). Out-of-sync and an event badge co-occur whenever both are
true — a pending write does not hide out-of-sync, because until the job lands the file genuinely
still differs.

**R2.5** While a job is pending and the page is open, the page re-checks status using
`pollUntilSettled` from `$lib/writebackJob.ts`. On reaching zero pending it re-resolves the detail
payload so `in_sync` recomputes against the post-write baseline (ADR-073 D1) and the out-of-sync
badge clears without a manual refresh.

**R2.6** Both badges and the failed detail line are owner-only, gated on the same condition as the
existing `canWriteback` — not a blanket owner check around the section.

### R3 — Failed jobs persist and are actionable

**R3.1** A failed job's badge persists across reloads and sessions until retried or dismissed. It
never clears itself; a self-clearing error would break R2.2's "absence means nothing to report".

**R3.2** A failed job renders one detail line beneath the header: the cause, then **Retry** and
**Dismiss**, both real buttons with link affordance.

**R3.3** **Retry** resets the failed row to `pending` and kicks the queue. This needs a new repo
method: `ClaimNextWriteback` selects `WHERE status = 'pending'` and `RecoverRunningWritebacks`
resets only `running`, so nothing moves `failed` → `pending` today.

**R3.4** **Dismiss** deletes the failed row (RD2). `job_runs` retains the permanent audit record.

**R3.5** Enqueuing a new write for a video **clears any existing failed row for that video** (RD5).
Submitting a new write is an implicit acknowledgment of the prior failure, so the page shows one
job's worth of truth at a time.

**R3.6** Retry and Dismiss are owner-gated endpoints.

### R4 — The dialog is a pre-flight confirm step

**R4.1** Row content is unchanged: destination tag (`→ write_target`), the `was:` current file
value, the editable new value, the no-op state, and the unwritable state. This preview already
exists and is not rebuilt.

**R4.2** The transient status icons leave the gutter (`isWriting` spinner, `isError` cross), and
`row.status` collapses accordingly.

**R4.3** Gutter glyphs are mutually distinct — checkbox/check = will be written, `=` = already
matches, circle-minus = unwritable. No two glyphs share a meaning, and none rely on colour alone.

**R4.4** Row order: writable fields in registry order, then no-ops, then unwritable.

**R4.5** Long values never grow the modal. Overview clamps to one line with a `more` link; the
poster collapses to a `compare` link. Both disclose on **hover, keyboard focus, and tap**, and
dismiss on `Escape` without closing the dialog. Hover-only is not acceptable.

**R4.6** The CTA keeps its count ("Write 9 fields"). Pre-action scope is decision-relevant; status
counts are not (RD1).

## Resolved decisions

**RD1 — Badges carry no counts.** The write is atomic over the fields submitted, so the job is the
unit and a per-field tally answers a question nobody acts on: 3 versus 1 changes neither the
decision nor the action. This also dissolves the residual-count question an earlier draft raised —
with no number, there is nothing to reconcile about which fields a job carries. The dialog's CTA
count is the exception and is justified separately (R4.6).

**RD2 — Dismiss deletes the row.** `job_runs` already holds the permanent audit record, so nothing
is lost, and `writeback_queue` stays a work queue rather than becoming a log. This matches how
success already behaves (`FinishWriteback` deletes) and avoids a fourth status plus a migration.

**RD3 — Media detail only for v1.** `WritebackBatchDialog` (tag-sync, merge propagation, film studio
cascade — ADR-077) keeps its blocking progress view. Those are deliberate bulk operations where
watching progress is the point, and an N-video status has no single page to live on. Revisit if the
batch dialog starts being dismissed mid-run in practice.

**RD4 — Signals are job-level, never per-field.** One `exiftool`/`mkvpropedit` invocation plus one
`os.Rename` lands whole or not at all. Per-field granularity would be fiction — the existing dialog
comment already concedes this for the queued path. The failed detail line gives the cause, not a
field list; which fields were in the job is visible in the rows below it.

**RD5 — A new write supersedes an unacknowledged failure.** Enqueuing clears the video's failed row.
The alternative (both persist) lets the header carry two event badges plus out-of-sync
simultaneously, for no gain — the owner has already responded to the failure by writing again.

**RD6 — Out of sync is never hidden.** An intermediate draft had pending replace it. That was wrong
on the facts: until the job lands the file still differs, and a queued write can sit behind a large
`EnqueueMany` batch. The badge-weight split (filled events, outline steady state) removes the reason
it was hidden — a quiet pill beside a filled one is not noise.

## Acceptance criteria

1. Submitting a writeback closes the dialog on the `202`; the owner can navigate away immediately
   and the file is written regardless.
2. A failed enqueue keeps the dialog open with the error inline and close affordances unlocked.
3. While a job is pending for the video, the Metadata header shows `writing to file` alongside
   `out of sync`; when the job lands, both clear without a manual refresh.
4. After a failed job, the header shows `couldn't write` alongside `out of sync`, plus a detail line
   with working Retry and Dismiss — and all of it is still there after a full page reload.
5. Retry moves the job back to `pending` and the queue picks it up.
6. Dismiss removes the failed row; the `job_runs` row remains and is visible on the Activity page.
7. Submitting a new write for a video with an undismissed failure clears the failure.
8. With neither pending nor failed work and no field out of sync, the Metadata header renders no
   badge at all.
9. A visitor sees no writeback badges and no Retry/Dismiss.
10. Gutter glyphs are distinct; the no-op row does not use a check.
11. Overview and poster disclose on hover, on keyboard focus, and on tap, and dismiss on `Escape`.
12. Three-skin QA passes per `.claude/rules/frontend-theming.md`.

## Testing

Per `docs/testing-strategy.md`; this spec adds:

- **Repo**: the per-video status query (pending only, failed only, both, neither); the
  `failed` → `pending` reset; delete-on-dismiss; clear-failed-on-enqueue (R3.5).
- **API**: `GET /media/{id}` carries status; Retry and Dismiss are owner-gated and 401/403 for a
  visitor; Retry on an absent or already-pending row is a safe no-op.
- **Frontend unit**: `pollUntilSettled` reuse for the page-level poll, including the cancelled and
  timeout paths (already covered for the dialog case; extend to the page consumer).
- **Adversarial**: a job that fails, is retried, and fails again; a dismiss racing a worker that is
  mid-write on that row; a page open in two tabs where one dismisses.
- **Regression**: after a write lands, `in_sync` recomputes against the post-write baseline and the
  out-of-sync badge clears — the ADR-073 D1 behaviour this spec depends on.

## Open items

1. **Poll cadence and lifetime.** Reuse `pollUntilSettled`; decide whether the page-level poll
   pauses on tab blur. Not blocking — the current backoff is adequate.
2. **HOLODEX-276 should be re-evaluated, not assumed.** Per-field written/skipped on the async path
   is not a distinction an atomic job can honestly report (RD4). Close it or narrow it once this
   lands.
3. **Disclosure component reuse.** The overview and poster popovers are one pattern with two
   payloads. Check for an existing popover component before writing a new one.
4. **`outOfSyncN` may become orphaned.** It currently feeds the `· {outOfSyncN} out of sync` span.
   If nothing else consumes the count once the badge stops rendering it, remove the computation
   rather than leaving it behind.
