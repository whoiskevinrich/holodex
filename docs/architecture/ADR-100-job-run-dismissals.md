# ADR-100: Job-run dismissals — a sibling table with a cascading foreign key, never a column on the audit row

**Status:** Proposed
**Date:** 2026-09-18
**Deciders:** Project owner (design handoff + spec 2026-09-18)

**Extends:** [ADR-028](ADR-028-activity-surface-and-job-history.md) (the `job_runs` table, its 30-day
retention and the two places the sweep runs) · [ADR-071](ADR-071-job-run-attribution-and-paginated-history.md)
(the per-kind digest read this ADR adds a `LEFT JOIN` to; its "an audit row must survive deletion of what it
describes" posture is the reason the dismissal is not a column) · [ADR-091](ADR-091-fire-and-forget-writeback-status.md)
D4 ("a failure never clears itself" — a dismissal is the owner clearing it, the one path that invariant allows).
**Relates to:** [ADR-066](ADR-066-enrichment-auto-apply-and-dismissal.md) D2 (`enrichment_dismissals`, the
durable-negative-assertion table this one is the structural sibling of — and whose *trigger* cleanup this ADR
deliberately does not copy, see D2) · [ADR-061](ADR-061-unified-entity-name-identity.md) (`entity_keep_separate`,
the first of that family).
**Spec:** [Job history — digest, pagination, and entity search](../specs/job-history-digest-and-search.md) P0-7
(answers Q4) · **Design:** [status-dismiss-failures-handoff.md](../design/status-dismiss-failures-handoff.md)
D1–D5 · **Jira:** [HOLODEX-416](https://whoiskevinrich.atlassian.net/browse/HOLODEX-416), Draft PR #354.

---

## Context

`/owner/status` → Recent jobs → Summary renders a `Recent failures` callout from
`repo.JobRunDigest` (ADR-071): every `status = 'error'` run in the 30-day window, plus a per-kind
`errors` count. Nothing clears a failure once the owner has handled it, so a bad batch keeps the
callout and the Errors column warn-coloured for a month and a new failure hides among old ones.
The spec's Q4 deferred this as "does the failure list need its own window?"; the answer, after
two months of use, is that the window is the wrong lever — the noise is *handled* failures, not
*old* ones, and any shorter window hides unhandled failures just as readily.

The design handoff locked the UX (per-row **Dismiss**, header **Dismiss all N**, no confirm, the
Log tab keeps every run) and the storage shape (**D4**: a new `job_run_dismissals` table, not a
column). The spec added **D5**: when a kind's *newest* run is a dismissed error, the digest's
Status badge renders muted with the Log's `· dismissed` marker rather than staying warn or
lying with an older `ok`. This ADR records why the storage shape is what it is and settles the
one question the handoff left to it — how a dismissal is kept from outliving its run.

Forces:

- **`job_runs` is an audit table and is treated as immutable.** 0028 gave it attribution columns
  with *no* foreign key so a run survives deletion of what it describes (ADR-071); ADR-091 leans
  on the row being the durable record that lets `writeback_queue` stay a work queue. Every write
  to it today is an `INSERT` followed by the retention `DELETE`; nothing `UPDATE`s a run.
- **Retention runs in two places** — `RecordJobRun` prunes after every insert and `PruneJobRuns`
  runs once at startup — both as `DELETE FROM job_runs WHERE started_at < cutoff`. Anything that
  has to happen "when a run is swept" has two call sites today and would have to be remembered
  at any third.
- **`foreign_keys(ON)` is set on the DSN** (`internal/db/db.go`) and is already load-bearing:
  0035 (`category_tags`), 0036 (`studio_images`) and 0037 (`person_link_derivation`) all declare
  `REFERENCES … ON DELETE CASCADE` and rely on it. Triggers were used for `enrichment_dismissals`
  (0024) only because that table is polymorphic over three parent tables and a FK cannot express
  `entity_type` — not as a house preference for triggers.
- **The digest's `last_status` is a bare column paired with `MAX(started_at)`**, so SQLite reads
  it from the newest row per kind. D5 needs a second fact about that same row (was it dismissed?),
  and the same bare-column mechanics can deliver it without a subquery.
- **Dismissal is per-run, never per-kind.** The owner's stated model is "I only care about
  failures after dismissal": a failure that lands after a dismissal must always surface. Whatever
  the storage, it must be impossible for a dismissal to swallow a run that did not exist yet.
- **Personal single-user server.** `job_runs` holds at most a month of runs; a bulk-writeback burst
  is a few thousand rows. No storage option here is distinguishable on size or speed.

## Decision

**D1 — A dismissal is a row in a new `job_run_dismissals` table, keyed by the run.**

```sql
CREATE TABLE job_run_dismissals (
    job_run_id   INTEGER PRIMARY KEY REFERENCES job_runs(id) ON DELETE CASCADE,
    dismissed_at TEXT    NOT NULL   -- RFC 3339 UTC, like every other timestamp column
);
```

`job_runs` gains no column and is never updated. Dismissing is `INSERT OR IGNORE`; the handler
reports `dismissed: true` only when a row was actually inserted, so a second dismissal of the
same run — two tabs, a retried request — is a no-op `200 {dismissed: false}`, never a 409 or 404,
mirroring `DismissFailedWriteback`'s "absent row is fine" posture. Only `status = 'error'` runs are
dismissable; the insert is guarded by `SELECT … WHERE id = ? AND status = ?`, so a dismissal of
an `ok` run is the same no-op, not a stored-but-meaningless row.

**D2 — The retention sweep cascades through the foreign key, not through code.**

`ON DELETE CASCADE` is the whole mechanism. Both existing sweep sites (`RecordJobRun`,
`PruneJobRuns`) keep their one-line `DELETE FROM job_runs` and the dismissal goes with the run;
a future third sweep gets the same guarantee for free. The repo test that pins this
(`TestPruneJobRuns_CascadesDismissal`) sweeps a dismissed run and asserts both rows are gone —
and, because the cascade only exists while `foreign_keys(ON)` is set, it doubles as the guard
that the pragma stays on the test DSN (`fold_test.go` already sets it).

A trigger, as 0024 used, would be the right tool only if the parent were polymorphic; here it
has exactly one parent. An explicit `DELETE FROM job_run_dismissals WHERE job_run_id NOT IN
(SELECT id FROM job_runs)` in each sweep would work but is the kind of two-site bookkeeping this
codebase has been burned by (ADR-075's `replaceAssociations` note) — and would leave a window
between the two statements in `RecordJobRun` where an orphan is observable.

**D3 — The digest excludes dismissed runs by `LEFT JOIN`, and reports D5 from the same row that
gives `last_status`.**

```sql
SELECT r.kind, COUNT(*) AS runs,
       SUM(CASE WHEN r.status = 'error' AND d.job_run_id IS NULL THEN 1 ELSE 0 END) AS errors,
       MAX(r.started_at) AS last_run, r.status AS last_status,
       (r.status = 'error' AND d.job_run_id IS NOT NULL) AS last_dismissed
FROM job_runs r LEFT JOIN job_run_dismissals d ON d.job_run_id = r.id
WHERE r.started_at >= ? GROUP BY r.kind ORDER BY last_run DESC
```

`runs` still counts every run — dismissing is triage, not erasure. `errors` counts only
undismissed errors (D2 of the handoff). `last_status` is unchanged: the newest run's status,
dismissed or not, so the digest never shows an older `ok` for a kind whose latest run failed.
`last_dismissed` rides the same bare-column rule as `last_status` — SQLite takes it from the row
that produced `MAX(started_at)` — so it is true exactly when the newest run is a dismissed error,
which is the D5 condition, with no correlated subquery. The failures list gets `AND d.job_run_id
IS NULL`. The history read gains `d.dismissed_at` via the same `LEFT JOIN` and returns every run.

**D4 — "Dismiss all" is a window-scoped `INSERT … SELECT`, evaluated server-side at request time.**

```sql
INSERT OR IGNORE INTO job_run_dismissals (job_run_id, dismissed_at)
SELECT id, ? FROM job_runs WHERE status = 'error' AND started_at >= ?
```

with the same `days` clamp the digest uses. No client-side id list is sent, so the 50-row
digest cap is irrelevant to the bulk path, and a run that starts after the request is by
construction not in the `SELECT`. `RowsAffected` is the `dismissed: n` the handler returns.

## Options Considered

### Option A: `dismissed_at` column on `job_runs` (rejected — D4 of the handoff)

| Dimension | Assessment |
|---|---|
| Complexity | Lowest — one `ALTER TABLE`, one `UPDATE`, no join |
| Cost | None |
| Scalability | Irrelevant at this size |
| Team familiarity | High |

**Pros:** simplest possible query; no cascade question at all.
**Cons:** the first `UPDATE` ever issued against `job_runs`. Breaks the "audit rows are written
once" posture that ADR-071 and ADR-091 both rest on, and mixes an owner's *triage state* into a
row that otherwise records only *what happened*. Every future reader of `job_runs` would have to
know that one column is mutable.

### Option B: Sibling table `job_run_dismissals` + `ON DELETE CASCADE` (chosen)

| Dimension | Assessment |
|---|---|
| Complexity | Low — one table, one `LEFT JOIN`, cascade is declarative |
| Cost | None |
| Scalability | Irrelevant at this size |
| Team familiarity | High — 0035/0036/0037 use the identical FK form |

**Pros:** `job_runs` stays immutable; the sweep needs no code; the dismissal is a durable
negative assertion in the same family as `entity_keep_separate` / `enrichment_dismissals` /
`denied_tags`, which is where a reader would look for it.
**Cons:** one more table; the digest and history queries each gain a join (over the PK, so
free). The cascade silently depends on `foreign_keys(ON)` — mitigated by the test in D2.

### Option C: Sibling table + explicit orphan delete in each sweep

Same table without the `REFERENCES`, and `DELETE FROM job_run_dismissals WHERE job_run_id NOT IN
(SELECT id FROM job_runs)` appended to both sweep sites.

**Pros:** no reliance on the pragma.
**Cons:** two call sites to keep in step plus any future one; an observable orphan window
between the two statements; and it re-implements by hand what the schema already expresses
elsewhere in this database. Nothing about `job_runs` makes the FK inappropriate — it is the
*parent* here, not the audit-row-that-must-not-reference-anything.

### Option D: Per-kind watermark — `dismissed_before(kind, started_at)`

One row per kind: "everything of this kind before *T* is handled."

**Pros:** smallest possible store; "Dismiss all" is one upsert; the owner's "I only care about
failures after dismissal" is literally the data model.
**Cons:** cannot express the handoff's **D1** per-row Dismiss — clearing a middle failure while
keeping an older, still-open one is impossible, and that is the one-off case (a single bad
file) the row control exists for. Would also need its own sweep logic (a watermark older than
the retention cutoff is dead weight). Rejected on D1; noted because it is the shape a future
"snooze this kind" feature would want, and it composes with Option B rather than replacing it.

## Trade-off Analysis

The only real fork was B vs C — cascade by schema or by code — and it is decided by where the
guarantee should live. A dismissal has exactly one parent and no meaning without it; that is
the textbook case for a declarative FK, and this database already trusts the pragma for three
tables. Putting the same rule in two Go functions instead buys independence from a pragma that
is set unconditionally on the only DSN, at the price of the two-site drift this codebase has
already paid for once. The 0024 precedent of triggers is not a counter-argument: it was forced
by a polymorphic parent, and its comment says so.

A is rejected on principle rather than mechanics: it is *easier*, and it would be the first
crack in a posture two ADRs depend on. D is the more interesting rejection — it matches the
owner's mental model better than B does — but it cannot do the per-row half of the design, and
per-row was locked first.

## Consequences

- **Easier:** the sweep stays a one-liner at every site; `job_runs` keeps its "insert then
  prune, never update" shape; a future undo (spec non-goal, HOLODEX-416 "not in v1") is a
  `DELETE` of one row with no audit-row edit; a "snooze kind" feature (Option D) can be added
  beside this table without touching it.
- **Harder / to watch:** the digest and history reads now depend on a `LEFT JOIN`, and the D5
  `last_dismissed` value depends on the same bare-column-with-`MAX` behaviour `last_status`
  already relies on — the existing `TestJobRunDigest` comment documents it, and the new test
  must assert it for both columns so a future rewrite of the query (say to a window function)
  cannot silently break one and not the other.
- **Invariant added to the testing strategy:** a dismissal never outlives its run, and a run
  that starts after a dismissal is never dismissed by it. Both are provable by repo test; neither
  needs the HTTP layer.
- **Revisit if:** `job_runs` ever grows a second mutable concern (it should not — that is the
  moment to ask whether it is still an audit table), or if a per-kind snooze is wanted, in which
  case Option D lands as a second table, not a rewrite of this one.

## Action Items

1. [ ] Migration `NNNN_job_run_dismissals.{up,down}.sql` — the D1 DDL; down drops the table
   (`.claude/rules/migrations.md`). Take the number at merge time: `0048` is held by HOLODEX-412's
   in-flight branch, so this lands as `0049` unless something else merges first
2. [ ] `repo.DismissJobRun(ctx, id) (bool, error)` — guarded `INSERT OR IGNORE`, `RowsAffected == 1`
3. [ ] `repo.DismissJobFailures(ctx, days) (int64, error)` — D4 `INSERT … SELECT`
4. [ ] `repo.JobRunDigest` / `ListJobRuns` — D3 `LEFT JOIN`, `last_dismissed` on `JobKindDigest`,
   `DismissedAt *time.Time` on `model.JobRun` (`omitempty`)
5. [ ] `POST /admin/activity/runs/{id}/dismiss` + `POST /admin/activity/failures/dismiss` inside
   the existing `requireOwner` group
6. [ ] Tests: `TestPruneJobRuns_CascadesDismissal`, `TestJobRunDigest_ExcludesDismissed` (asserts
   `errors`, `failures`, `last_status` *and* `last_dismissed` for the newest-run case), double
   dismiss → `false`, non-error run → `false`, Dismiss-all does not touch a run inserted after the
   window snapshot; handler 401 without the token
7. [ ] `docs/testing-strategy.md` row for the two invariants above
8. [ ] Index entry in [README.md](README.md); spec P0-7 gate → `[x]` once merged
