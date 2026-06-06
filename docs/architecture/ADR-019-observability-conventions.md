# ADR-019: Observability & Operational Conventions

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Beyond features, a self-hosted service needs baseline operational hygiene: health/readiness signaling for container orchestration, consistent structured logging, and clean shutdown. Setting these conventions before building keeps them uniform across the codebase.

## Decision

### Health & readiness endpoints
| Endpoint | Meaning |
|----------|---------|
| `GET /healthz` | Liveness — process is up and serving HTTP. Always 200 if the server runs. |
| `GET /readyz` | Readiness — DB is open, migrations applied, initial scan bootstrap complete. 503 until ready. |

- `docker-compose.yml` and the Dockerfile `HEALTHCHECK` use `/healthz`.

### Structured logging
- Standard library **`slog`** with JSON handler in production, text handler in dev.
- Level via `LOG_LEVEL` (default `info`; `debug`, `warn`, `error`).
- Every scan logs a summary (files seen / added / updated / removed / skipped / errors); per-file extraction failures log at `warn` and are skipped (never abort the scan — Phase 1 NFR).

### Graceful shutdown
- On `SIGTERM`/`SIGINT`: stop accepting new HTTP connections, drain in-flight requests (bounded timeout), signal the scanner and worker pools to stop, flush, then exit.
- SQLite WAL is checkpointed on clean shutdown.

### Prometheus metrics (Phase 2)
- Exposition at `GET /metrics` (ADR / spec F13). Metric names are namespaced `holodex_*`.

## Rationale

- **`slog`** is in the standard library — no dependency, structured by default, plays well with log aggregators if Holodex is later released and run at scale.
- **Distinct liveness vs readiness** prevents orchestrators from routing traffic before migrations/bootstrap finish.
- **Graceful shutdown** avoids corrupting in-flight thumbnail jobs and leaves the WAL in a clean state, reducing startup recovery time.

## Consequences

- A small `internal/health` package exposes the two endpoints and tracks readiness state transitions.
- Logging is injected (an `*slog.Logger` on the app context), not package-global, so tests can capture output.
- These conventions are referenced by ADR-007 (Docker `HEALTHCHECK`) and the Phase 2 metrics requirements.
