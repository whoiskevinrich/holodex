# Spec: Job History — Digest, Pagination, and Entity Search (F21.3b)

**Status**: Draft
**Phase**: Post–Phase 3 (extends F21 System Activity)
**Owner**: Project owner
**Date**: 2026-07-22
**Jira**: [HOLODEX-203](https://whoiskevinrich.atlassian.net/browse/HOLODEX-203) (parent epic [HOLODEX-166](https://whoiskevinrich.atlassian.net/browse/HOLODEX-166) — System Activity)

**Depends on**: F21 System Activity ([spec](system-activity.md), [ADR-028](../architecture/ADR-028-activity-surface-and-job-history.md)) — this spec changes the read path and schema that ADR-028 established.

**New ADRs required**:
- **ADR-069 (reserved, P0)** — Job-run entity attribution and paginated history reads. Covers the `entity_type`/`entity_id`/`batch_id` columns, the decision to attribute rather than group, and the keyset read contract. Extends ADR-028 (which fixed the 30-day window and the unpaginated read); does not supersede it.

---

## Problem Statement

The 30-day job history on `/owner/status` is slow to load and hard to get an answer out of. `ListJobRuns` has no `LIMIT` ([`internal/repo/jobruns.go:86`](../../internal/repo/jobruns.go)) and the API exposes only `?days=`, so the page fetches and renders every run in the window. Seven of the nine job kinds record one row per operation, but **`writeback` records one row per video** ([`internal/writequeue/writequeue.go:222`](../../internal/writequeue/writequeue.go)) and **`enrich` one row per provider × entity** ([`internal/enrich/service.go:442`](../../internal/enrich/service.go)) — and extraction auto-apply enqueues a writeback *per field per video* ([`internal/extract/process.go:157`](../../internal/extract/process.go)). One "extract all" across a 3,000-video library can therefore produce on the order of 12,000 job-run rows, each rendering 1–2 `<tr>` with no virtualization ([`JobHistory.svelte:64`](../../web/src/lib/components/JobHistory.svelte)).

The result is that the surface built to answer *"is it working?"* becomes least usable exactly when the most work has happened. The owner's three real questions — **did anything fail**, **did the last run of X work**, and **what happened to this file** — are all answered today by scrolling.

The underlying query is healthy: one indexed table, no joins, no N+1, no blob columns. The cost is row count, payload size, and DOM nodes — not query shape.

## Goals

1. **Answer "did anything fail?" without scrolling.** Failures across the whole window are visible on first paint, regardless of how many successful runs sit between them.
2. **Answer "did the last run of X work?" in one glance.** Per-kind last-run time, run count, and error count are shown as an aggregate, not reconstructed by eye from a chronological list.
3. **Keep the audit trail readable.** The full chronological stream stays available and becomes navigable — paginated, filterable, and with high-volume runs collapsed — rather than being replaced by a summary.
4. **Make "what happened to this file?" an exact lookup.** History can be filtered to a specific video, person, or studio instead of scanned for a filename that only some job kinds record.
5. **Bound first paint.** Time-to-first-paint of the history section becomes independent of how many runs are in the window.

## Non-Goals

- **A stored grouping / correlation id.** Today's `batch_id` is a revert handle, not a grouping key — `snapshotBeforeWrite` falls back to the queue row's own id ([`internal/writequeue/writequeue.go:311`](../../internal/writequeue/writequeue.go)), so every single-video write is a "batch" of one. A true group id would mean changing `enrich.Service.Enrich`'s public signature and every `Enqueue` call site. *(Why: read-time adjacency rollup delivers most of the log readability for a fraction of the blast radius. Revisit if rollup boundaries prove wrong in practice.)*
- **Changing retention.** The 30-day window and the hard-coded `jobRunRetentionDays` stay as they are. *(Why: once the read is paginated, table size is a disk concern, not a UI one. A count cap would also let a writeback storm evict an older failed scan — the opposite of goal 1.)*
- **A retention config knob.** The `GalleryCap` pattern ([`internal/repo/repo.go:33`](../../internal/repo/repo.go)) is the template if this is ever wanted. *(Why: no demand; adds config surface to document and test.)*
- **Virtualized rendering.** Pagination bounds the DOM; virtualization would be a second mechanism for the same problem. *(Why: redundant once page size is capped.)*
- **Reducing what gets recorded.** `writeback` and `enrich` keep writing one row per item. *(Why: the per-item rows are the audit trail; the problem is how they are read, not that they exist.)*
- **Fixing the snapshot-retention mismatch.** `file_writeback_snapshots` has no retention prune ([migration 0026](../../internal/db/migrations/0026_file_writeback_snapshots.up.sql)), so a 60-day-old batch stays revertible while its `job_run` is gone — the revert window is 30 days by UI accident, not by design. *(Why: a real defect, but independent of this work and deserving its own decision about the intended window.)*

---

## User Stories

**Owner — triage**
- As the **library owner**, I want failures from the whole window surfaced on first paint so that a silent writeback error doesn't hide behind 12,000 successful ones.
- As the **library owner**, I want per-kind last-run time and error count so that I can confirm the nightly scan is still running without reading the log.

**Owner — audit**
- As the **library owner**, I want the full chronological history still available so that I can read what happened in order when I'm investigating something.
- As the **library owner**, I want a bulk writeback to appear as one collapsed entry I can expand so that a single click's worth of work doesn't cost me 150 pages.
- As the **library owner**, I want to filter the log by job kind and status so that I can look at only enrich failures.

**Owner — forensics**
- As the **library owner**, I want to see every job run that touched a specific video, person, or studio so that I can find out what changed it and when.
- As the **library owner**, I want the Revert control to keep working on writeback batches so that this change doesn't cost me the ability to undo.

**Edge cases**
- As the **library owner**, when a job run references a video I've since deleted, I want the run to remain in the history so that the audit trail isn't rewritten by deletion.
- As the **library owner**, when there are no failures, I want the failures section to be absent rather than an empty box so that "clean" reads as clean.

---

## Requirements

### Must-Have (P0)

**P0-1 — Entity attribution columns**
Add `entity_type TEXT NOT NULL DEFAULT ''` and `entity_id INTEGER NOT NULL DEFAULT 0` to `job_runs`, indexed as a pair. Populate from identifiers already in local scope at record time: `job.VideoID` for writeback, `report.VideoID` for refresh, `entityType`/`entityID` for enrich. The remaining six kinds leave them empty.

- [ ] Migration `NNNN_job_runs_entity_attribution.{up,down}.sql` follows [`.claude/rules/migrations.md`](../../.claude/rules/migrations.md)
- [ ] No foreign-key constraint is added; `job_runs` remains free of cascade semantics
- [ ] A job run whose referenced entity is later deleted still returns from the API, with the entity label rendered as `#<id> (deleted)`
- [ ] Rows written before the migration have empty attribution and are excluded from entity-filtered results without error

**P0-2 — `batch_id` column**
Add `batch_id TEXT NOT NULL DEFAULT ''`, populated from the value `snapshotBeforeWrite` already computes. The Revert control reads this column.

- [ ] The `/· batch (\d+)/` regex in [`JobHistory.svelte:17`](../../web/src/lib/components/JobHistory.svelte) is deleted
- [ ] Revert continues to work for writeback runs recorded after the migration
- [ ] Changing `writequeue.detailLine`'s format does not affect whether Revert is offered
- [ ] Writeback runs recorded *before* the migration have an empty `batch_id` and offer no Revert control (accepted regression, bounded by the 30-day window)

**P0-3 — Digest read**
A per-kind aggregate: last `started_at`, run count, and error count, plus the individual failed runs in the window.

- [ ] Served by a `GROUP BY kind` query over the existing `idx_job_runs_started_at` index
- [ ] Response size is independent of the number of runs in the window
- [ ] Given a window containing only successful runs, the failures section is absent from the response

**P0-4 — Keyset-paginated log read**
`GET /api/v1/admin/activity/history` accepts a cursor over `(started_at, id)` plus `kind`, `status`, `entity_type`, and `entity_id` filters, and returns a bounded page with a next-cursor.

- [ ] Ordering remains `started_at DESC, id DESC`, matching the existing index
- [ ] Given a cursor, the first row of page N+1 immediately follows the last row of page N with no duplicates or gaps, including when rows share a `started_at`
- [ ] An absent or malformed cursor returns the first page rather than an error
- [ ] Unknown `kind` or `status` values return an empty page, not a 500
- [ ] The endpoint stays inside the existing `requireOwner` group

**P0-5 — Two-mode UI**
Digest is the default view; Log is one click away and preserves the full chronological stream.

- [ ] Digest renders per-kind rows and a failures callout
- [ ] Log renders a paginated table with kind and status filters and a "load more" control
- [ ] The history section no longer waits on `/admin/activity` before painting ([`+page.svelte:154`](../../web/src/routes/owner/status/+page.svelte))
- [ ] A history fetch failure renders a visible error state rather than the "No jobs recorded yet" empty state (current behavior swallows it at [`+page.svelte:38`](../../web/src/routes/owner/status/+page.svelte))
- [ ] Empty, loading, error, digest, and log states are QA'd in Cinémathèque, Broadcast, and Brutalist

**P0-6 — Adjacency rollup in the log**
Consecutive runs sharing `(kind, status, and — for enrich — provider)` within a time window collapse into one expandable entry showing the count and span.

- [ ] A 2,847-row writeback burst renders as one collapsed entry
- [ ] Concurrent `enrichRefreshAll` providers ([`enrich_review.go:173`](../../internal/api/enrich_review.go)) roll up per provider, not into one interleaved blob
- [ ] Expanding an entry reveals its member runs without an additional fetch beyond the current page
- [ ] A failed run is never absorbed into a successful rollup

### Nice-to-Have (P1)

**P1-1 — Entity search entry point.** A control on `/owner/status` to look up an entity by name and filter the log to it, rather than only arriving pre-filtered from elsewhere. *(See open question Q3.)*

**P1-2 — Deep link from entity pages.** A "history" affordance on a video, person, or studio page that opens the log filtered to that entity.

**P1-3 — Entity label resolution.** `LEFT JOIN` to resolve `entity_type`/`entity_id` to a title in the log view, replacing the `#<id>` rendering.

### Future Considerations (P2)

**P2-1 — Stored group id.** If adjacency rollup produces wrong boundaries in practice, mint a real group id at each initiating call site. The attribution columns added here are orthogonal and would not need to change.

**P2-2 — Export.** CSV or JSON export of a filtered log view.

**P2-3 — Retention configurability.** Only if disk pressure becomes real.

---

## Data & API Contract

**Schema** (one migration, additive):

| Column | Type | Populated by |
|---|---|---|
| `entity_type` | `TEXT NOT NULL DEFAULT ''` | writeback, refresh (`video`); enrich (`video`/`person`/`studio`) |
| `entity_id` | `INTEGER NOT NULL DEFAULT 0` | same |
| `batch_id` | `TEXT NOT NULL DEFAULT ''` | writeback |

Index: `(entity_type, entity_id)`. No foreign key — `job_runs` is an audit table and must survive deletion of what it describes.

**Endpoints** (both owner-gated, unchanged):

- `GET /admin/activity/digest` → per-kind aggregate + failed runs in the window.
- `GET /admin/activity/history` → `?cursor=&limit=&kind=&status=&entity_type=&entity_id=&days=` → `{runs: [...], next_cursor: string|null}`. `days` retains its existing 30-day clamp.

---

## Success Metrics

Personal single-user server — adoption metrics don't apply. These are verifiable acceptance targets, measured on a testbed loaded with a representative post-`extract-all` history.

**Leading (verify at merge)**
- Time-to-first-paint of the history section is **flat across history size** — measured at ~100 and ~12,000 rows, within noise of each other.
- History section paints without waiting on `/admin/activity`, verified by network waterfall.
- Digest response payload is **< 10 KB** regardless of row count.
- Log page DOM node count is bounded by page size, not window size.

**Lagging (verify in use)**
- "Did anything fail in the last 30 days?" is answerable **without scrolling or paginating**.
- "What touched video #412?" is answerable in **one filtered request**, returning runs from all attributing kinds — not just writeback.

**Baseline to capture before implementing**: current wall-clock split (time to history response / payload size / time from response to paint) at real library scale, so the flatness claim has a before-number.

---

## Open Questions

**Q1 — What is the current wall-clock split? [engineering, non-blocking but do first]**
The design assumes row volume dominates, but the render gate at [`+page.svelte:154`](../../web/src/routes/owner/status/+page.svelte) is free to remove and may account for a meaningful share. Measure before implementing so the targets above have a baseline — and so we know whether P0-5's ungating alone would have sufficed.

**Q2 — What time gap ends an adjacency rollup? [engineering + design, blocking P0-6]**
Too tight and a bulk writeback fragments into many entries; too loose and unrelated runs merge. Needs a value chosen against real burst timings, not guessed. Consider gap-based (e.g. no more than N seconds between consecutive runs) rather than fixed-bucket, which would split a burst that straddles a boundary.

**Q3 — How is entity search reached? [design, blocking P1-1]**
Two shapes: a search box on the status page that resolves a typed name to an entity, or a "view history" affordance on video/person/studio pages that deep-links into a pre-filtered log. The second is cheaper and matches where the question is actually asked ("what happened to *this*"), but only works if you already have the entity open. Not blocking P0 — the API filter lands either way.

**Q4 — Does the digest's failure list need its own window? [design, non-blocking]**
Failures across a full 30 days may be too noisy after a bad batch, or exactly right. Default to the full window and revisit.

**Q5 — Page size for the log? [engineering, non-blocking]**
50 is the assumed default. Worth confirming against the collapsed-row count once rollup exists — 50 *rolled-up* entries may represent far more runs than intended.

---

## Timeline Considerations

No external deadline. Suggested phasing — each is independently shippable and independently useful:

1. **Ungate + measure** (P0-5's gating fix, Q1). Cheapest possible change; establishes the baseline.
2. **Migration + attribution + `batch_id`** (P0-1, P0-2). Schema first so the recording paths start populating; the Revert regex dies here.
3. **Digest + paginated log** (P0-3, P0-4, P0-5). The API and UI work.
4. **Adjacency rollup** (P0-6, gated on Q2).
5. **Entry points** (P1, gated on Q3).

Step 2 is the only irreversible one. Steps 1 and 3 can ship without it if the measurement in Q1 shows the render gate dominates.

---

## Gates

- [ ] **Spec** — this document
- [ ] **ADR-069** — entity attribution + paginated read contract
- [ ] **Design handoff** — two-mode UI, rollup affordance, three-skin states
- [ ] **Testing strategy** — [`docs/testing-strategy.md`](../testing-strategy.md) updated; keyset-cursor and rollup-boundary cases covered
- [x] **Security review** — not required; endpoints stay within the existing `requireOwner` group and no auth surface changes
- [ ] **Three-skin QA** — Cinémathèque, Broadcast, Brutalist
