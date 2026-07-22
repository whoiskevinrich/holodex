# ADR-071: Job runs attribute to an entity; history reads through a keyset cursor

**Status:** Proposed
**Date:** 2026-07-22
**Deciders:** Project owner

**Extends:** [ADR-028](ADR-028-activity-surface-and-job-history.md) (the `job_runs` audit table and the
activity read-model this adds columns and a read contract to) · [ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md)
(writeback snapshot batches — this promotes their id out of a free-text line).
**Relates to:** [ADR-033](ADR-033-metadata-source-plugins.md) (enrich runs, which already know their
entity) · [ADR-047](ADR-047-per-item-metadata-refresh.md) (per-item refresh) · [ADR-048](ADR-048-metadata-curation-and-write-queue.md)
(the write queue whose worker records one run per video) · [ADR-061](ADR-061-unified-entity-name-identity.md)
(entity merge, whose propagation batches are the case the current Revert control silently misses).
**Spec:** [job-history-digest-and-search.md](../specs/job-history-digest-and-search.md).
**Issue:** [HOLODEX-207](https://whoiskevinrich.atlassian.net/browse/HOLODEX-207) (phase 2 of
[HOLODEX-203](https://whoiskevinrich.atlassian.net/browse/HOLODEX-203)).

---

## Context

`job_runs` ([migration 0004](../../internal/db/migrations/0004_job_runs.up.sql), ADR-028) is a flat
audit log: `kind`, `trigger`, `status`, timings, counters, and a free-text `detail`
([0006](../../internal/db/migrations/0006_job_detail.up.sql)). It was designed when every recorded
run was a whole *pass* — one scan, one backfill. Two kinds now record one run per *item*:

- **writeback** — [`Queue.process`](../../internal/writequeue/writequeue.go) records one run per
  video, and extraction auto-apply enqueues per field per video, so one bulk operation can produce
  thousands of rows.
- **enrich** — `recordEnrichJob` records one run per provider × entity.

Nothing in the row says *which* entity a run touched. The only entity trace is `detail`, a display
string — and only `writeback` puts anything identifying in it (`filepath.Base(path)`).

Two consequences follow, and they are separable:

**Reading.** `ListJobRuns` has no `LIMIT`. The endpoint returns the whole 30-day window and the
page renders every row plus a second `<tr>` of detail for each. The query itself is healthy — one
indexed table, no joins, no blobs — the cost is row count, payload size, and DOM nodes.

**Forensics and Revert.** "What happened to this file?" cannot be answered, because three of the
four kinds that touch an entity don't record which one. And the Revert control parses the batch id
back out of the display string:

```js
// web/src/lib/components/JobHistory.svelte
const m = /· batch (\d+)/.exec(r.detail);
```

`detailLine` writes `· batch <id>`, and for a single-video write that id is the job's own numeric id,
so the regex matches. But merge propagation — the *reason* migration 0027 added a caller-supplied
batch id — names its batch `merge-person-12-34` ([`mergeBatchID`](../../internal/api/merge_writeback.go)).
`(\d+)` does not match `merge-…`, so **the Revert button never appears for a merge-propagation
batch**: verified, `/· batch (\d+)/.exec('… · batch merge-person-12-34')` returns `null`. The exact
multi-video case the shared batch id was built to make revertible is the one case the UI can't
revert. This is a live defect, not a hypothetical fragility of parsing display strings.

Note what is *not* broken: `snapshotBeforeWrite`'s fallback to the job id when no batch id was
supplied is deliberate and load-bearing — it makes a crash-recovered retry find its own earlier
snapshot instead of re-deriving a wrong one. That behavior stays exactly as it is.

### Forces

- **`job_runs` is an audit table.** Its rows must survive the deletion of whatever they describe.
  A run that says "enriched person #42" is *most* interesting after person 42 is gone.
- **Attribution has to be entity-generic.** Video, Person, and Studio are already one decision
  model (ADR-051/052/061); a `video_id` column would encode the video slice and force a second
  mechanism for the other two.
- **The recording sites already hold the identity.** `recordEnrichJob` has `entityType` and
  `entityID` in hand; the write-queue worker has `job.VideoID`. Attribution is a plumbing change,
  not a lookup.
- **`detail` must stay free text.** It is a human sentence for the operator. Anything a *program*
  needs to read has to be a column — that is the general lesson the Revert bug teaches.
- **Reads must be bounded by page size, not window size.** Retention is 30 days but density is
  unbounded; the read contract has to be correct for a window containing 12,000 rows.
- **Rows share `started_at`.** Runs are recorded at millisecond resolution and a burst writes many
  per timestamp, so any cursor keyed on time alone will skip or duplicate rows at a page boundary.

---

## Decision

### D1 — Attribution is a polymorphic `(entity_type, entity_id)` pair with no foreign key

`job_runs` gains two columns, populated by every kind that acts on a single entity:

| Column | Type | Populated by |
|---|---|---|
| `entity_type` | `TEXT NOT NULL DEFAULT ''` | writeback, refresh (`video`); enrich (`video`/`person`/`studio`) |
| `entity_id` | `INTEGER NOT NULL DEFAULT 0` | same |

`entity_type` reuses the existing `model.EnrichEntity*` vocabulary rather than minting a parallel
one. Library-wide kinds (scan, the backfills) leave the pair at its zero value, which reads as
"not attributed" — no sentinel, no nullable column.

**There is deliberately no foreign key.** A `REFERENCES videos(id)` would either block deleting a
video or, with `ON DELETE CASCADE`, silently rewrite history when one is removed. Both are wrong
for an audit table. The cost is that `entity_id` can dangle; the read side treats an unresolvable
id as exactly that and renders `#<id>` rather than failing.

Reads filtered by entity are served by a composite index matching the sort order:

```sql
CREATE INDEX idx_job_runs_entity ON job_runs (entity_type, entity_id, started_at DESC);
```

### D2 — `batch_id` becomes a column; the Revert control reads it structurally

`job_runs` gains `batch_id TEXT NOT NULL DEFAULT ''`, set by the writeback worker from the value it
already computed for the snapshot. `JobRun` carries it as a JSON field, the frontend reads
`r.batch_id`, and the regex is deleted — which fixes merge-batch Revert as a direct consequence, not
as a separate change.

`detail` keeps its `· batch <id>` suffix. It is now purely human-readable, and no code parses it.

### D3 — The log reads through a keyset cursor over `(started_at, id)`; the digest is a separate bounded query

Two endpoints replace the single unbounded list, both inside the existing `requireOwner` group — no
new auth surface:

- **`GET /api/v1/admin/activity/digest?days=`** — per-kind aggregate (`GROUP BY kind`: last run,
  run count, error count) plus the window's failures. Its response size is a function of the number
  of job *kinds*, which is bounded by the code, not by the number of rows.
- **`GET /api/v1/admin/activity/history?cursor=&limit=&kind=&status=&entity_type=&entity_id=&days=`**
  — one page of runs plus `next_cursor`, ordered `started_at DESC, id DESC` to match
  `idx_job_runs_started_at`.

The cursor is **keyset, not offset**. Offset pagination degrades as the offset grows and, worse,
skips or repeats rows when new runs are inserted mid-read — which is the normal case here, since
the owner reads this page *while* a bulk job is writing to it. The cursor carries both `started_at`
and `id` because a burst writes many rows per timestamp; comparing on time alone would drop rows at
every page boundary. The predicate is the standard tuple comparison:

```sql
WHERE (started_at, id) < (:cursor_started_at, :cursor_id)
```

The cursor is opaque to the client and **advisory to the server**: absent, malformed, or
unparseable, it yields the first page rather than a 400. A paginated audit log should degrade to
"start over", never to an error page. Unknown `kind`/`status` filter values likewise return an empty
page — they are user input, not a contract violation.

---

## Options Considered

### D1 — how a run says what it touched

| Option | Verdict |
|---|---|
| **Polymorphic `(entity_type, entity_id)`, no FK** | **Chosen.** Entity-generic, survives deletion, one index serves all three types. |
| `video_id` FK only | Rejected — encodes the video slice; enrich already attributes to people and studios, so this would need a second mechanism immediately. |
| Polymorphic pair *with* an FK per type | Rejected — impossible as one column pair, and either blocks deletion or cascades away history. |
| Join table `job_run_entities` | Rejected — buys many-to-many nobody needs, and costs a join on the hot read path. Revisit only if a run genuinely spans entities. |
| Parse `detail` with a stricter grammar | Rejected — this is what the Revert bug *is*. Making the display string a parsing target again repeats the mistake with more steps. |

### D2 — where the batch id lives

| Option | Verdict |
|---|---|
| **`batch_id` column on `job_runs`** | **Chosen.** The worker already has the value; the UI reads a field. Fixes merge-batch Revert as a side effect. |
| Widen the regex to `([\w-]+)` | Rejected — fixes today's symptom and leaves the next `detailLine` edit free to break Revert again silently. |
| Structured JSON in `detail` | Rejected — `detail` is the operator's sentence. Making it a serialization format costs its readability and still isn't queryable in SQL. |

### D3 — the read contract

| Option | Verdict |
|---|---|
| **Keyset cursor on `(started_at, id)`** | **Chosen.** Stable under concurrent inserts, correct across ties, uses the existing index. |
| `LIMIT/OFFSET` | Rejected — skips and repeats rows when the log is being written to during the read, which is exactly when the owner is reading it. |
| Cursor on `started_at` only | Rejected — a burst writes many rows per timestamp; every page boundary would drop or duplicate the tie group. |
| Cursor on `id` only | Rejected — correct only while id order matches `started_at` order. True today, but it silently couples the read contract to insert ordering. |
| Keep one endpoint, add `LIMIT` | Rejected — bounds the payload but not the *question*: "did anything fail in 30 days" still requires paging to the end. |

---

## Consequences

**Good**

- "What touched video #412?" becomes one indexed query returning runs from every attributing kind.
- Revert works for merge-propagation batches for the first time, and can no longer be broken by an
  edit to a display string.
- Digest response size is bounded by the number of job kinds; log response size by page size. Both
  are independent of window density.
- The attribution columns are additive with zero-value defaults — every existing row and every
  library-wide kind stays valid without a backfill.

**Bad / accepted**

- `entity_id` can dangle. Accepted deliberately: it is the price of an audit trail that deletion
  cannot rewrite. The read side renders `#<id>` when the entity is gone.
- Rows written before this migration have no attribution and cannot get any — the information was
  never recorded. Entity-filtered views are correct going forward only, and empty for old rows.
- A second index on `job_runs` costs write throughput on a table written once per job. Judged
  negligible against a single-writer SQLite queue already doing file I/O per job.
- Two read endpoints instead of one is more API surface to keep consistent.

**Neutral**

- `snapshotBeforeWrite`'s job-id fallback is untouched, so crash-recovery idempotence is unchanged.
- `detail` keeps its current text. Nothing about the operator's view of it changes.

---

## Action Items

- [ ] Migration `0028_job_runs_attribution.{up,down}.sql` — `entity_type`, `entity_id`, `batch_id`,
      and `idx_job_runs_entity`. Down drops the index and the three columns.
- [ ] `model.JobRun` gains the three fields; `RecordJobRun` persists them.
- [ ] Populate at the recording sites: `writequeue.Queue.process` (`video` + the batch id it already
      computed), `enrich.recordEnrichJob` (its existing `entityType`/`entityID`), per-item refresh.
- [ ] `JobHistory.svelte` reads `r.batch_id`; delete `batchId()` and its regex.
- [ ] Repo: keyset `ListJobRunsPage` + `JobRunDigest`; keep `PruneJobRuns` untouched.
- [ ] Tests — migration up/down; attribution asserted per recording path; Revert on a
      `merge-person-*` batch (the regression this ADR names); cursor paging across a tie group of
      rows sharing one `started_at`.
- [ ] Update [`docs/specs/job-history-digest-and-search.md`](../specs/job-history-digest-and-search.md)
      and the parent worklog: this is **ADR-071**, not the ADR-069 the spec reserved — 069 was taken
      by [draft PRs for pre-implementation gates](ADR-069-draft-prs-for-pre-implementation-gates.md).
