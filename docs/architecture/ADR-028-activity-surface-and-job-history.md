# ADR-028: User-facing activity surface & job-history persistence

**Status**: Proposed
**Date**: 2026-06-14
**Deciders**: Project owner
**Extends**: ADR-019 (Observability — health endpoints, structured logging, `GET /metrics`)

---

## Context

ADR-019 gave Holodex its operational baseline: `/healthz`, `/readyz`, structured
`slog` summaries (`scan complete seen=… added=…`), and (via ADR-026) a Prometheus
`/metrics` endpoint. All of it is **operator-facing** — to answer *"what is the
system doing right now?"* the owner must tail container logs, scrape `/metrics`,
or hit the thin `GET /api/v1/admin/status` (which returns only
`{thumbnail_queue_depth}`).

Spec [F21 (System Activity)](../specs/system-activity.md) requires an **in-product**
view of the background work that already exists — the scanner (initial / periodic /
watch / manual passes, serialized by `scanMu`) and the thumbnail pipeline (two-tier
queue + workers) — including a short history of past runs so the owner can spot a
scan that is slowing down or repeatedly erroring. The raw signals exist in memory;
what is missing is (a) an aggregated read-model the UI can poll and (b) durable
history that survives a restart.

This ADR decides the **backend read-model and persistence**; the live-push transport
is deferred to ADR-029 (SSE, P1) and access control to ADR-030 (gating seam, P0).

## Decision

### 1. A new aggregated read-model endpoint
Add `GET /api/v1/admin/activity` returning a single snapshot aggregated from the
scanner, thumbnail pipeline, repository, and health subsystem (shape per F21.1:
`scan`, `thumbnails`, `library`, `system`). The existing
`GET /api/v1/admin/status` is **preserved unchanged** as the legacy minimal shape
so any current consumer keeps working; new clients use `activity`.

- **No secrets in the payload.** Paths/tokens/env values are never serialized —
  e.g. `system.media_path_present` is a boolean, not the path. This is an explicit
  invariant covered by a test.

### 2. Scanner status accessor
The scanner gains a `Status()` accessor reporting
`{state, trigger, started_at, last_run, next_scheduled_at}`, derived from the
existing `scanMu` / `stats` / ticker. It must add **no new lock on the scan hot
path** — `last_run` is published once per pass under the lock the pass already
holds; `state`/`trigger` are cheap atomics. `next_scheduled_at` is a best-effort
estimate (last tick + interval), `null` when `MEDIA_PATH` is unset.

### 3. Durable job history (`job_runs`), 30-day retention
A new embedded migration (golang-migrate, per ADR-016 — next sequential number)
adds a `job_runs` table:

```
job_runs(id PK, kind, trigger, status, started_at, finished_at,
         duration_ms, seen, added, updated, removed, skipped, errors,
         error_message NULLABLE)   -- index on started_at
```

- The scanner records one row per completed pass, **best-effort**: a failed write
  is logged and never aborts or blocks the scan (consistent with ADR-019's
  "never abort the scan" NFR).
- `kind` is `"scan"` in v1 but deliberately extensible (`thumbnail_backfill`,
  Phase-3 `enrichment` / `preview` / `writeback`) so future jobs reuse the table.
- **Retention is fixed at 30 days**, pruned automatically (a `DELETE … WHERE
  started_at < now-30d` run on insert and on startup). No configuration in v1.
- `GET /api/v1/admin/activity/history?days=30` (default and **max** 30) returns
  runs newest-first.

### 4. Library counts behind the existing cache seam
`videos_active / videos_inactive / people / tags` are computed for the read-model.
At personal scale these `COUNT(*)`s are cheap, but they are served through the
existing cache seam (ADR-008 / ADR-022 Noop) with a short TTL so per-poll cost
stays flat as a library grows — no new caching mechanism is introduced.

## Rationale

- **Reuse over reinvention.** The read-model is assembled from seams that already
  exist (the `SetMetrics`-style optional wiring, `QueueDepth()`, the repo, the
  health state). The scanner already accumulates the exact counts `job_runs` needs.
- **SQLite for history** (ADR-003) means durability across restarts for free — no
  new store, no new dependency, and the 30-day window keeps the table tiny.
- **Best-effort recording** preserves the cardinal scanner rule: observability must
  never compromise indexing.
- **A new endpoint, not a breaking change.** Keeping `admin/status` intact avoids a
  migration burden on anything already reading it (tests, future MCP, dashboards).
- **30 days, fixed** satisfies the "spot a pattern" goal without a retention-config
  UI; configurability is explicitly a P2 (F21.9).

## Consequences

- One new migration and a small `job_runs` repo method (insert + prune + history
  query); the scanner gains `Status()` and a record-on-completion hook.
- `internal/api` gains the `activity` and `activity/history` handlers; both sit
  behind the ADR-030 owner gate.
- This ADR **extends** ADR-019; it does not edit it. The live-push transport is
  **ADR-029** (SSE, P1) and is a clean add-on — the polled read-model here is the
  permanent fallback.
- When Phase 3 jobs land (F16–F18), they record into `job_runs` via the same path;
  no schema change beyond new `kind` values.
