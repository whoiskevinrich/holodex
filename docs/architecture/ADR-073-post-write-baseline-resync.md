# ADR-073: The post-write read-back is unconditional; queued writes expose a job status

**Status:** Proposed
**Date:** 2026-07-26
**Deciders:** Project owner

**Supersedes:** [ADR-068](ADR-068-extraction-resolve-entity-materialization.md) **D1** only — the
decision to *gate* the post-write re-extract to entity fields from `filename`/`manual` sources.
ADR-068's substantive decision (a resolved "create new" materializes by re-reading the written file
through `UpsertVideo`, not by an inline insert) stands unchanged; only its gate is reversed.
**Extends:** [ADR-048](ADR-048-metadata-curation-and-write-queue.md) (the durable write queue this
adds a status read to) · [ADR-041](ADR-041-metadata-writeback.md) (writeback) ·
[ADR-047](ADR-047-per-item-metadata-refresh.md) (`refresh.ReExtract`). **Relates to:**
[ADR-051](ADR-051-per-field-source-of-truth-decisions.md)/[ADR-052](ADR-052-baseline-source-contract.md)
(the baseline contract whose staleness is the bug). **Issue:**
[HOLODEX-214](https://whoiskevinrich.atlassian.net/browse/HOLODEX-214).

---

## Context

`/media/{id}` kept reporting "Write decisions to file · N out of sync" after a successful write,
with the same N, and each written field kept its per-field "file out of sync" pill. The only
feedback that the write had succeeded was the dialog closing — which reads as a silent failure and
invites a second redundant write (on MKV, another full remux).

Two independent causes, both structural:

**1. The stored baseline was never refreshed.** `in_sync` is computed in `internal/resolver` as
`decided == fileVal`, where `fileVal` comes from `NewVideoBaseline(v, extra)` — the DB's stored copy
of the file's tags, not a live read of the file. ADR-052 makes that copy the baseline layer by
design. Nothing in the write path updated it: the post-write hook re-extracted embedded cover art
always, and full metadata *only* when ADR-068 D1's `MayIntroduceEntity` predicate held (an
`actors`/`studio` field, from a source other than `merge`/`revert`). So writing `title`, `year`,
`description` — anything non-entity — updated the file on disk and left the DB asserting the
pre-write value indefinitely, through reloads, until an unrelated rescan or a manual Refresh.

ADR-068 D1 gated the hook for a real reason: *"Merge propagation must not regain a per-video cost."*
But merge propagation rewrites the tag on every affected file, so excluding it is the same staleness
across every video in the merge — the case where a correct baseline matters most, not least. The
gate optimized the wrong side of the trade.

**2. The write is asynchronous, and the SPA treated 202 as done.** ADR-048's queue is wired
unconditionally in production, so `POST /media/{id}/writeback` returns `202 {job_id}` at enqueue
time. The dialog marked every row done and handed control back on that answer — before a byte was
written. Any refetch at that instant is guaranteed to read pre-write state, so fixing (1) alone
would not have fixed the symptom. There was no way to observe a job's completion: the queue has a
depth count and a days-windowed activity history, but nothing answers "did *my* job land."

## Decision

**D1. The post-write hook runs after every successful write, with no field or source gate.**
`refreshSvc.ReExtract` is called unconditionally, replacing `writequeue.MayIntroduceEntity` (deleted,
with its unit test). `PostWriteFunc` drops its `fields` parameter — it existed only to feed the gate,
and keeping it invites the same conditional logic back.

Re-reading the file, rather than patching the stored tags from the values we sent, is deliberate:
the tools normalize (multi-value joining, container tag aliasing, the `WritebackField` people→actors
mapping), so writing back our own inputs would diverge from what the next real scan reads and make
`in_sync` flap — reintroducing this bug more subtly. It is also what materializes entities per
ADR-068.

**D2. The synchronous writeback branch does the same read-back.** `writebackMedia`'s non-queued path
had its own inline post-write block that extracted cover art and nothing else, so it carried this
bug independently. It is dead in production (the queue is always wired) but is kept consistent
rather than left as a second, drifting post-write path.

**D3. Queued writes expose a per-job status: `GET /writeback/jobs/{id}`,** owner-gated with the rest
of the writeback surface. `pending`/`running` in flight, `failed` + the queue's error when it gave
up, `done` otherwise. Flat like the revert route — the id alone identifies the job.

Because `FinishWriteback` deletes the row on success, **an absent row reads as `done`**, conflating
succeeded / never-existed / already-swept. That holds only for the intended caller: a poll started
from the job id the enqueue just returned. A failed job keeps its row, so a real failure is never
reported as success.

**D4. The SPA polls that status to a terminal state before reporting applied.**
`$lib/writebackJob.waitForWritebackJob` (pure, injected fetcher) backs off 250 ms × 1.5 to a 5 s
ceiling, capped at 120 s. A status *fetch* failure is not a write failure — the job is unaffected by
our inability to read it — so it keeps polling. On timeout it resolves rather than throwing: a write
still in flight is not a failure, and the caller refetching then shows the true current state rather
than a stale one, which is the property that was broken.

## Consequences

- **A merge propagation regains a per-video re-extract**, the cost ADR-068 D1 avoided: `+2` process
  spawns (exiftool `-j`, ffprobe), `+2` write transactions, ≈150–600 ms per video, serialized at the
  default `WRITEBACK_CONCURRENCY=1`. Accepted, because each of those jobs already copies the whole
  media file byte-for-byte (ADR-041 copy→write→rename) plus two other exiftool passes — the added
  work is ~0.5–1.5% of what the job already spends, and it is entirely off the request path. The
  gate was never buying much, because the dominant per-job cost was never gated.
- The hook now runs two exiftool passes over the same file on every job (cover art, then the
  re-extract). Collapsing them onto one parsed `-j` dump is a worthwhile follow-up, not done here.
- `in_sync` is now correct after any write, from any path, without a rescan.
- The `done`-means-absent contract (D3) is safe only for a poll seeded by a fresh enqueue. A second
  consumer — a poll resumed after a reload, a batch/merge progress view, the revert flow (which
  returns `job_ids` and has no completion signal at all) — would silently read `done` for a job that
  never ran. Resolving terminal state from `job_runs` (durable, already carries status, error, and
  ADR-071 attribution) with `writeback_queue` consulted only for in-flight state is the deeper fix
  when a second consumer appears; it needs `job_runs` to carry the queue job id explicitly.
- ADR-068's Consequences section says its gate predicate is "load-bearing … covered by a unit test."
  That test is deleted here; `TestQueue_PostWriteHookRunsForEveryWrite` asserts the opposite
  contract across plain, entity, merge, and revert writes.
