# Spec: Job History — Digest, Pagination, and Entity Search (F21.3b)

**Status**: Draft
**Phase**: Post–Phase 3 (extends F21 System Activity)
**Owner**: Project owner
**Date**: 2026-07-22
**Jira**: [HOLODEX-203](https://whoiskevinrich.atlassian.net/browse/HOLODEX-203) (parent epic [HOLODEX-166](https://whoiskevinrich.atlassian.net/browse/HOLODEX-166) — System Activity)

**Depends on**: F21 System Activity ([spec](system-activity.md), [ADR-028](../architecture/ADR-028-activity-surface-and-job-history.md)) — this spec changes the read path and schema that ADR-028 established.

**New ADRs required**:
- **[ADR-071](../architecture/ADR-071-job-run-attribution-and-paginated-history.md) (P0)** — Job-run entity attribution and paginated history reads. Covers the `entity_type`/`entity_id`/`batch_id` columns, the decision to attribute rather than group, and the keyset read contract. Extends ADR-028 (which fixed the 30-day window and the unpaginated read); does not supersede it. *(This spec originally reserved ADR-069; that number was taken by [draft PRs for pre-implementation gates](../architecture/ADR-069-draft-prs-for-pre-implementation-gates.md) before the ADR was written.)*

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

- [x] Migration [`0028_job_runs_attribution.{up,down}.sql`](../../internal/db/migrations/0028_job_runs_attribution.up.sql) follows [`.claude/rules/migrations.md`](../../.claude/rules/migrations.md); up **and** down verified against a table that already holds rows
- [x] No foreign-key constraint is added; `job_runs` remains free of cascade semantics
- [ ] A job run whose referenced entity is later deleted still returns from the API, with the entity label rendered as `#<id> (deleted)` — *read-side rendering; lands with P0-4/P1-3, not with the columns*
- [x] Rows written before the migration have empty attribution and are excluded from entity-filtered results without error

**P0-2 — `batch_id` column**
Add `batch_id TEXT NOT NULL DEFAULT ''`, populated from the value `snapshotBeforeWrite` already computes. The Revert control reads this column.

- [x] The `/· batch (\d+)/` regex in [`JobHistory.svelte`](../../web/src/lib/components/JobHistory.svelte) is deleted
- [x] Revert continues to work for writeback runs recorded after the migration
- [x] Changing `writequeue.detailLine`'s format does not affect whether Revert is offered — nothing parses `detail` any more
- [x] Writeback runs recorded *before* the migration have an empty `batch_id` and offer no Revert control (accepted regression, bounded by the 30-day window)
- [x] **Fixes a live defect:** Revert was never offered for a merge-propagation batch, whose id is `merge-person-N-M` — `(\d+)` could not match it, so the one multi-video case shared batches exist for was the one case the UI could not revert

**P0-3 — Digest read**
A per-kind aggregate: last `started_at`, run count, and error count, plus the individual failed runs in the window.

- [x] Served by a `GROUP BY kind` query over the existing `idx_job_runs_started_at` index — `repo.JobRunDigest`; `last_status` is the newest run's status via SQLite's bare-column-with-MAX
- [x] Response size is independent of the number of runs in the window — asserted by `TestJobRunDigest` (500 extra runs leave `kinds`/`failures` lengths unchanged); the inline failure list is capped at `digestFailureCap`
- [x] Given a window containing only successful runs, the failures section is absent from the response — `TestJobRunDigestCleanWindow` returns an empty (non-nil) `failures`, and the UI hides the callout

**P0-4 — Keyset-paginated log read** *(deferred — re-justified against Q1 before building; see Timeline)*
`GET /api/v1/admin/activity/history` accepts a cursor over `(started_at, id)` plus `kind`, `status`, `entity_type`, and `entity_id` filters, and returns a bounded page with a next-cursor.

- [ ] Ordering remains `started_at DESC, id DESC`, matching the existing index
- [ ] Given a cursor, the first row of page N+1 immediately follows the last row of page N with no duplicates or gaps, including when rows share a `started_at`
- [ ] An absent or malformed cursor returns the first page rather than an error
- [ ] Unknown `kind` or `status` values return an empty page, not a 500
- [ ] The endpoint stays inside the existing `requireOwner` group

**P0-5 — Two-mode UI**
Digest is the default view; Log is one click away and preserves the full chronological stream.

- [x] Digest renders per-kind rows and a failures callout — `JobDigest.svelte`, the default view; the callout is a `border-warn` box and a kind whose *latest* run failed shows an `error` status badge even when older passes succeeded
- [ ] Log renders a paginated table with kind and status filters and a "load more" control — *the pagination/filter half waits on P0-4 (Q1); the Log tab currently reuses the existing full-window history table, fetched lazily only when opened*
- [x] The history section no longer waits on `/admin/activity` before painting — it is a sibling of that gate, hidden only on `needToken` (the same condition the endpoint itself enforces)
- [x] A history fetch failure renders a visible error state rather than the "No jobs recorded yet" empty state (previously swallowed); `ReauthError` is excluded so a pending re-auth doesn't flash an error
- [x] Digest, its failures callout, loading, and error states are QA'd in Cinémathèque, Broadcast, and Brutalist (warn elements 5.3–7.3:1, radius tracks each skin's token); the log-view states carried over from phase 1

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

Personal single-user server — adoption metrics don't apply. Timing the real library is out of scope (production instance, not reproducible here), so the leading criteria are stated as **invariants provable by test** rather than measurements. An invariant asserted against seeded data is a stronger guarantee than a stopwatch reading on one machine anyway: it holds for every library size, not the one that happened to be measured.

**Leading (asserted by test at merge)**
- The digest query returns a **fixed number of rows** regardless of window size — repo test seeding 100 and 100,000 runs returns identically-shaped results.
- The log endpoint returns **at most `limit` rows** for any window size, and issues a bounded number of queries per request.
- The history section renders **without awaiting** `/admin/activity` — component test asserts the section is reachable when the activity fetch is pending.
- Log-view DOM node count is bounded by page size, not window size.

**Lagging (owner judgment, verified in use)**
- "Did anything fail in the last 30 days?" is answerable **without scrolling or paginating**.
- "What touched video #412?" is answerable in **one filtered request**, returning runs from all attributing kinds — not just writeback.
- The page feels responsive on the real library after a bulk writeback — the original complaint, confirmed by the owner rather than by a number.

---

## Open Questions

**Q1 — Does ungating alone resolve the complaint? [owner, answered by shipping phase 1]**
The design assumes row volume dominates, but the render gate at [`+page.svelte:154`](../../web/src/routes/owner/status/+page.svelte) also delays first paint and is free to remove. Timing the real library to separate the two is out of scope, so phase 1 ships the ungating on its own and the owner judges whether the page still feels slow.

**This is a real decision point, not a formality.** If ungating alone fixes it, phases 2–5 — including the migration — are optional polish rather than a fix, and should be re-justified on the forensics and triage goals alone rather than on load time. Accepted risk of proceeding without the split: we may build the digest and pagination for a problem the gate was causing.

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

1. **Ungate** (P0-5's gating fix + error state). Cheapest possible change, independently correct, and the answer to Q1 — ship it and see whether the page still feels slow before committing to the rest.
2. **Migration + attribution + `batch_id`** (P0-1, P0-2). Schema first so the recording paths start populating; the Revert regex dies here.
3. **Digest + paginated log** (P0-3, P0-4, P0-5). The API and UI work.
4. **Adjacency rollup** (P0-6, gated on Q2).
5. **Entry points** (P1, gated on Q3).

Step 2 is the only irreversible one, and it is worth doing on the forensics and Revert-correctness goals even if step 1 turns out to have resolved the load time. Steps 3–5 should be re-justified against Q1's answer before starting.

---

## Gates

- [ ] **Spec** — this document
- [x] **[ADR-071](../architecture/ADR-071-job-run-attribution-and-paginated-history.md)** — entity attribution + paginated read contract
- [ ] **Design handoff** — two-mode UI, rollup affordance, three-skin states
- [ ] **Testing strategy** — [`docs/testing-strategy.md`](../testing-strategy.md) updated; keyset-cursor and rollup-boundary cases covered
- [x] **Security review** — not required; endpoints stay within the existing `requireOwner` group and no auth surface changes
- [ ] **Three-skin QA** — Cinémathèque, Broadcast, Brutalist
