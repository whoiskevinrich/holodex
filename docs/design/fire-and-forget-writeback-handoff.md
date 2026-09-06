# Design handoff: Fire-and-forget writeback

**Jira:** HOLODEX-323 · **ADR:** [ADR-091](../architecture/ADR-091-fire-and-forget-writeback-status.md) ·
**Supersedes:** ADR-073's synchronous-wait rule

![Fire-and-forget writeback mockup](fire-and-forget-writeback-mockup.svg)

## Overview

Writing metadata to a file is already asynchronous and durable on the server: the handler
enqueues a `writeback_queue` row and answers `202` in milliseconds, and the worker runs on
the application-lifetime context, not the request context. Nothing the browser does can
cancel an in-flight write, and the copy → write → rename model (ADR-041) means the original
file is never left half-written.

The dialog nevertheless behaves as though the write were synchronous: it gates every close
affordance on `busy` and polls for up to 120 s. This change stops that pretence. The dialog
becomes a **pre-flight confirm step** that closes as soon as the job is accepted, and the
job's outcome moves to a **page-level signal near the Metadata section**.

**The governing principle: silence means success.** The page renders a chip only while a
write is pending or after one has failed. No chip and no "file out of sync" pill together
mean the file matches the decided values.

## Why the pending signal is not optional

ADR-073 introduced the dialog's wait because refetching on the `202` returns *pre-write*
state — `resolved[]` recomputes `in_sync` against the old baseline, and fields that were
just written render as "file out of sync". Closing the dialog early **without** a page-level
pending signal reproduces exactly that failure: the user submits, the dialog vanishes, the
section still reads "3 out of sync", and the reasonable response is to write again.

Fire-and-forget and the pending chip are therefore one change, not a change plus a
follow-up. Shipping the first without the second is a regression.

## Layout

Both signals live in the Metadata section header row, immediately after the "Metadata"
label and before the existing "Write decisions to file" action. They are section-scoped,
never per-field — see *Job-level, not per-field* below.

The failed state adds a second line beneath the header row carrying the error sentence and
the Retry control. The header row itself does not grow.

## Design tokens used

Tokens only — no hardcoded values. QA all three skins per `.claude/rules/frontend-theming.md`.

| Token | Usage |
|---|---|
| `text-accent` / `border-accent` | Pending chip text and outline |
| `text-warn` / `border-warn` | Failed chip text and outline, error sentence |
| `text-ink` | Section label, field values |
| `text-muted` | Secondary copy, `was:` line, action row |
| `bg-surface` | Chip fill |
| `rounded-full` | Chip shape, matching the existing `file out of sync` pill |
| `text-[0.65rem]` | Chip label size, matching `SourceBadge`'s pill |

The pending chip deliberately reuses the geometry of the existing `file out of sync` pill in
`SourceBadge.svelte` so the two read as one family.

## States and interactions

| Element | State | Behavior |
|---|---|---|
| Metadata header | Clean | No chip. Nothing renders. |
| Metadata header | Pending | Accent chip, "writing to file…", with the spinner idiom already used in the dialog. Rendered while `pending > 0` for this video. |
| Metadata header | Failed | Warn chip, "write failed". Persists until retried or dismissed. |
| Failed detail line | Visible | One sentence naming the affected fields plus cause, then a Retry link. |
| Retry | Click | Resets the failed row to `pending`, kicks the queue, swaps the chip to pending. |
| Dismiss | Click | Clears the failed row without retrying. The write is *not* reattempted. |
| Dialog | Submitting | Holds only for the `202`. Close affordances stay locked for that one round trip. |
| Dialog | Enqueued | Closes immediately. No success toast — silence is the success signal. |
| Dialog | Enqueue failed | Stays open, shows the error inline, close affordances unlock. |

### Dialog rows — what changes

Row **content is unchanged**. The existing pre-flight preview already shows everything
needed to catch a bad value before committing:

- destination tag via `→ {field.write_target}`
- current file value via the `was:` line
- the new value in an editable field
- "already in file, nothing to write" for no-ops
- "No file tag for this container — can't be written" for unmappable fields, shown but not
  checkable

Only the **transient status icons** leave the left gutter: the `isWriting` spinner and the
`isError` cross. The gutter reverts to the checkbox, the no-op check, and the unwritable
marker. `row.status` collapses accordingly.

## Job-level, not per-field

The write is a single `exiftool` or `mkvpropedit` invocation followed by one `os.Rename`.
It lands whole or not at all. Per-field success or failure is therefore not a real
distinction on this path, and per-field status UI would be fiction — the existing dialog
comment already concedes this for the queued path.

One job produces one chip. The error sentence names the fields the job carried, which is
the honest granularity.

## Edge cases

- **Queue depth.** Merge propagation and tag-sync enqueue one job per affected video via
  `EnqueueMany`, so a single-video write can sit behind a large batch. The pending chip must
  not imply immediacy — "writing to file…" with no ETA, not a progress bar.
- **Multiple pending jobs for one video.** Render one chip, not N. The condition is
  `pending > 0`.
- **Pending and failed at once** (a retry queued while an older failure is undismissed):
  pending wins in the header chip; the failed detail line remains until resolved.
- **Long field lists in the error sentence.** Truncate to the first three field labels plus
  "and N more" so the line stays single-height.
- **Visitor view.** Both chips are owner-only — a visitor has no writeback affordance. Gate
  on the same condition as the existing `canWriteback`, not on a blanket owner check around
  the section.
- **Job completes while the page is open.** The page re-resolves when pending reaches zero,
  so the warn pills clear without a manual refresh.
- **Job completes while the page is closed.** Next load reads current state from the queue —
  nothing is lost, because the signal is server-side rather than session-held.
- **Stale failed row from a previous session.** Renders normally on load. This is the point
  of sourcing from the queue rather than from a job id held in memory.

## Accessibility

- The pending chip carries `aria-live="polite"` so its appearance and disappearance are
  announced without stealing focus.
- The failed chip is **not** a live region — it persists and is reachable by reading order.
  The error sentence is associated with the section, not announced on a timer.
- Retry and Dismiss are real `<button>`s with discernible names ("Retry writing metadata",
  "Dismiss writeback error"), not icon-only affordances.
- Chip text never encodes state by color alone: "writing to file…" and "write failed" are
  distinguishable without hue.
- Dialog focus trap and focus return on close are unchanged. Closing on the `202` must still
  return focus to the trigger — the existing `onMount` cleanup already does this, and must
  not regress when the close path changes.
- Escape stays blocked for the single round trip the dialog is awaiting the ack, then
  behaves normally.

## Motion

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| Pending chip | Appears | Fade in | 150 ms | ease-out |
| Pending chip | Job lands | Fade out, then section re-resolve | 150 ms | ease-out |
| Failed chip | Appears | Fade in | 150 ms | ease-out |
| Spinner | While pending | Existing `animate-spin` idiom | — | linear |

No layout shift on chip appearance: reserve the chip's height in the header row so the
action link does not jump.

## Open items for implementation

1. **Retry needs a new repo method.** `ClaimNextWriteback` selects `WHERE status = 'pending'`,
   and `RecoverRunningWritebacks` only resets `running`. Nothing moves `failed` back to
   `pending` today, so Retry requires a new query plus a `kick()`.
2. **Dismiss semantics.** Deleting the failed row loses the audit trail; the `job_runs` row
   persists independently, so deletion is acceptable. Confirm before implementing.
3. **Poll cadence while pending.** Reuse `pollUntilSettled` from `$lib/writebackJob.ts`
   rather than introducing a second polling idiom.
