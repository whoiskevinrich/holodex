# Observability and jobs

This doc owns how Holodex reports on itself: structured logging, the Prometheus `/metrics`
endpoint, liveness and readiness, the `job_runs` history model (kinds, attribution, reads,
retention, dismissals), boot-time jobs, the single-flight pattern for long-running jobs, and the bulk
enrichment sweep runner. The scanner, purge and per-item refresh operations themselves are
[media-ingest.md](media-ingest.md); this doc owns only how they record runs. Per-provider pacing and
the `429` pause are [metadata-providers.md](metadata-providers.md).

## Structured logging with `log/slog`, injected

The standard library's `slog` with a JSON handler (`newLogger` in `cmd/holodex`), level from
`LOG_LEVEL` (or `-log-level`; default `info`). The logger is passed in, never package-global, so
tests can capture output. A per-item failure inside a pass logs at `warn` and is counted, never
fatal to the pass.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## Separate liveness and readiness; graceful shutdown

`GET /healthz` is liveness: always 200 while the process serves HTTP. The container `HEALTHCHECK`
probes it (`runHealthcheck`). `GET /readyz` is readiness, backed by `api.Health` (an atomic flag):
503 until migrations and the synchronous boot jobs have run and the server is about to listen.
Both sit at the root, outside `/api/v1`, and pass through the SPA handler.

On `SIGTERM`/`SIGINT` readiness drops, `http.Server.Shutdown` drains in-flight requests within a
bounded timeout, and cancelling the root context stops every background worker and running job.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## Hand-rolled Prometheus exposition in `internal/metrics`

`GET /metrics` (root, not `/api/v1`) serves text exposition format 0.0.4 from a dependency-free
`metrics.Registry`: an atomic counter (`holodex_indexed_files_total`), a gauge read at scrape time
from the thumbnail queue (`holodex_thumbnail_queue_depth`), and two cumulative-bucket histograms
(`holodex_scan_duration_seconds`, `holodex_search_duration_seconds`). Every name is `holodex_*`.
Instrumentation hangs off optional nil-safe seams (`Scanner.SetMetrics`, the handlers' metrics
field), so tests and health-only mode stay uninstrumented. A nil handler omits the route.

**Rejected:** `prometheus/client_golang` — a large transitive tree for four metrics. Adopt it (and
rewrite this section) if exemplars, many label dimensions or runtime collectors are needed.

Decided in [`6a40aeae`](https://github.com/whoiskevinrich/holodex/commit/6a40aeae).

## `job_runs`: an append-only audit table with fixed 30-day retention

Every background or owner-triggered job records into one SQLite table, `job_runs` (`id`, `kind`,
`trigger`, `status`, `started_at`, `finished_at`, `duration_ms`, the counters `seen`/`added`/
`updated`/`removed`/`skipped`/`errors`, `error_message`, `detail`, plus the attribution columns
below). `kind` is open-ended: a new job is a new `model.JobKind*` constant, never a schema change.

- **Write-once.** Rows are inserted and pruned, never updated (triage state is a sibling table).
- **Best-effort.** `repo.RecordJobRun` failures are logged and never abort or block the job.
  Recording uses a detached context, so a cancelled or failed operation still records.
- **Retention is a constant** (`jobRunRetentionDays = 30`): `RecordJobRun` prunes after every
  insert under `writeMu`, and `PruneJobRuns` runs once at startup. The same constant clamps every
  `days=` read.
- **`detail` is a human sentence and nothing parses it.** Anything a program reads is a column.
  No row carries paths, env values or tokens; entities appear as ids, provider errors as a generic
  message.
- **Grain.** Whole-library passes (scan, purge, extraction, backfills, orphan sweep)
  record one row per pass. Per-item kinds record one row per item: `writeback` per video,
  `enrich` per provider × entity, `refresh` one row per refresh covering both halves.

Decided in [`1882a895`](https://github.com/whoiskevinrich/holodex/commit/1882a895).

## Attribution as a polymorphic `(entity_type, entity_id)` pair plus `batch_id`; bounded reads

`job_runs.entity_type` (the `model.EnrichEntity*` vocabulary) and `entity_id` name the single entity
a run touched. Library-wide kinds leave them at `''`/`0`, which means unattributed. Index
`idx_job_runs_entity (entity_type, entity_id, started_at DESC)` serves per-entity reads.
`batch_id` ties the runs of one operation together: writeback sets it from its snapshot batch, so
Revert reads the column and never the `detail` text, and the enrich sweep sets it on every run it
produces.

There is deliberately no foreign key. An audit row must outlive what it describes, and a
`REFERENCES` would either block deletion or cascade history away. An `entity_id` may dangle, and the
read side renders a gone entity as `#<id>`.

Reads are bounded by page or kind count, never by window density. The owner-gated `GET /api/v1/admin/activity/digest` is one `GROUP BY kind` (run count, undismissed
error count, last run, `last_status` taken from the newest row by SQLite's bare-column-with-`MAX`
rule) plus the window's failures, capped at `digestFailureCap`. Its size scales with the number of
job kinds, not rows. `GET …/activity/history` returns runs ordered `started_at DESC, id DESC`
(matching `idx_job_runs_started_at`), optionally narrowed to one `?batch=`.

The log is not yet paged. Its paging contract is a keyset cursor over `(started_at, id)`, opaque,
degrading to the first page when malformed; both columns because a burst shares one timestamp.

**Rejected:** a `video_id` FK — encodes one entity type. **Rejected:** a join table — many-to-many
nobody needs. **Rejected:** parsing ids from `detail`. **Rejected:** `LIMIT/OFFSET` — skips and
repeats rows while a bulk job writes during the read.

Decided in [`97ee9917`](https://github.com/whoiskevinrich/holodex/commit/97ee9917).

## Dismissals as a sibling table with a cascading foreign key

An owner marking a failed run handled is a row in `job_run_dismissals (job_run_id PRIMARY KEY
REFERENCES job_runs(id) ON DELETE CASCADE, dismissed_at)`. `job_runs` gains no column.

- `DismissJobRun` is `INSERT OR IGNORE … SELECT` guarded on `status = 'error'`; a repeat is a
  no-op, never a 404/409. `DismissJobFailures` is one window-scoped `INSERT … SELECT` evaluated at
  request time, so a later run is never dismissed by it. Dismissal is per run, never per kind.
- Every read shares `jobRunSelect`, which `LEFT JOIN`s the dismissal. The digest excludes dismissed
  errors from `errors` and failures and reports `last_dismissed` off the same newest row as
  `last_status`. History returns every run with `dismissed_at`.
- Retention needs no code: the cascade removes a dismissal with its run at every sweep site. This
  relies on `foreign_keys(ON)` in the DSN, which `TestPruneJobRuns_CascadesDismissal` pins.

**Rejected:** a `dismissed_at` column — the first `UPDATE` on an audit table. **Rejected:** orphan
deletes in each sweep — call sites to keep in step.

Decided in [`f8e7e8af`](https://github.com/whoiskevinrich/holodex/commit/f8e7e8af).

## Boot jobs gated on their own job history

Startup data passes run in `cmd/holodex` after migrations and before the server listens, each
best-effort (logged, never fatal) and each recording one `job_runs` row. A one-time backfill
(`studio-backfill`, `person-backfill`, `identity-backfill`, `alias-backfill`) skips when
`HasSuccessfulJobRun(kind)` finds a prior `ok` run, after any cheaper "already done" check. An
`error` row leaves the gate open, so the next boot retries. Because retention prunes the marker,
every one-time pass must also be idempotent. Every-boot passes (`shared-id-sweep`) skip the gate.
Work that must not delay boot (provider icon refresh, completeness drain) runs in a bounded
goroutine instead.

Decided in [`16a621af`](https://github.com/whoiskevinrich/holodex/commit/16a621af).

## Long-running jobs: `TryLock` single-flight with a `Status()` read-model on the activity poll

An owner-triggered bulk job (`Scanner.TriggerRescan`, `extract.BatchRunner.TriggerAll`,
`enrich.SweepRunner.Trigger`) takes a `sync.Mutex.TryLock`, runs in a goroutine on the
server-lifetime context and unlocks on exit. A repeat trigger while running reports "not started",
which is informational, never an error. Live progress comes from an in-memory `Status()` read-model.
The owner-gated `GET /api/v1/admin/activity` aggregates scanner status, thumbnail queue, library
counts, system facts and the sweep block into one polled snapshot. It carries no secrets
(`media_path_present` is a boolean, not the path). Live state is not persisted. After a restart a
job is reconstructed from `job_runs`.

**Rejected:** a dedicated progress endpoint or push channel — the poll already reaches every page.

Decided in [`a9815b1d`](https://github.com/whoiskevinrich/holodex/commit/a9815b1d).

## Entity refresh sweep: `RefreshPair`, a per-sweep breaker, `batch_id` as the audit key

`enrich.SweepRunner` refreshes every person or studio, one entity at a time, fanning out across that
entity's providers concurrently as one click does. One `TryLock` covers both kinds.

- **One per-pair step.** `Service.RefreshPair(…, RefreshOpts{Force, BatchID}) PairOutcome` owns the
  routing (linked → enrich; unlinked → dismissed skip, or resolve → single strong match → apply). The
  interactive handler (`refreshOneProvider`) only renders its outcome. `PairOutcome` carries the
  typed error, so the sweep can tell a pause from a failure. A linked pair fetched within
  `StaleWindow` (24 h) returns `stale_skipped` unless `Force`, and the interactive path always
  forces.
- **The breaker is per-sweep state in the runner, never in the client.** Per provider, any success
  resets two streaks. `breakerFailures` (5) non-`429` errors or `breakerPauses` (3) pairs still
  paused after their one retry trip it, and the provider's remaining pairs count as skipped with
  a named reason. Clicks are never subject to it, and each sweep starts clean.
- **Audit.** Each sweep mints a `sweep-<hex>` `batch_id`, threads it explicitly through `RefreshOpts`
  into every per-entity `enrich` run, and records one `enrich-sweep` summary run at the end. A sweep
  killed mid-way leaves its per-entity runs and no summary; `?batch=` reconstructs it.

**Rejected:** a service-level or persisted breaker — a bad sweep would fail the owner's next click
with nothing to reset it. **Rejected:** a context value or `wait` flag on `Resolve`/`Enrich` —
invisible at the call site. The pause stays a typed error and each caller sets its own policy.

Decided in [`a9815b1d`](https://github.com/whoiskevinrich/holodex/commit/a9815b1d).

## Provider activity observed through `enrich` job runs

Every provider resolve-and-apply or enrich records an attributed `enrich` run. No per-provider
Prometheus series or health block exists yet; when added, they go through `internal/metrics` and the
activity read-model under its no-secrets rule.

Decided in [`90b3b4a0`](https://github.com/whoiskevinrich/holodex/commit/90b3b4a0).
