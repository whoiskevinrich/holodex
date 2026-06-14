# ADR-026: Prometheus metrics — hand-rolled exposition, no client library

**Status**: Accepted
**Date**: 2026-06-13
**Deciders**: Project owner
**Extends**: ADR-019 (Observability — established `GET /metrics`, `holodex_*` namespace)

---

## Context

ADR-019 committed Holodex to a Prometheus endpoint at `GET /metrics` for Phase 2
(spec F13), but deliberately left the *implementation* open. The canonical way to
expose Prometheus metrics in Go is `github.com/prometheus/client_golang`, which
pulls in a sizeable transitive tree (`client_model`, `common`, `procfs`,
`protobuf`, …).

Phase 2 requires exactly four metrics (F13.2):

| Metric | Type |
|--------|------|
| `holodex_indexed_files_total` | counter |
| `holodex_thumbnail_queue_depth` | gauge |
| `holodex_scan_duration_seconds` | histogram |
| `holodex_search_duration_seconds` | histogram |

The project keeps a deliberately lean `go.mod` (the ADR-019 rationale for `slog`
is literally "in the standard library — no dependency"; ADR-022 deferred the
in-process cache to a Noop; the pnpm monorepo was abandoned for maintainability).

## Decision

Implement a small **dependency-free** `internal/metrics` package that writes the
Prometheus **text exposition format (0.0.4)** by hand:

- A `Registry` holds an atomic counter, a pull-based gauge (read at scrape time
  from the thumbnail pipeline's `QueueDepth`), and two cumulative-bucket
  histograms with per-metric bucket layouts (scans: 0.1 s–300 s; searches:
  1 ms–2.5 s).
- Instrumentation is wired through optional seams (`Scanner.SetMetrics`,
  `Handlers.SetMetrics`), nil-safe so tests and health-only mode stay
  uninstrumented.
- The endpoint is mounted at the root (`/metrics`, outside `/api/v1`) by the
  router and added to the SPA pass-through allowlist.

## Rationale

- **Four simple metrics don't justify the dependency tree.** Counter/gauge are
  trivial; histograms are ~30 lines for cumulative buckets + `_sum`/`_count`.
- **Consistency with the codebase's minimal-dependency posture** (ADR-019/022).
- **Exposition format is stable** (0.0.4) and trivially scrapeable; we emit
  `# HELP`/`# TYPE` plus the series, which Prometheus and the OTEL collector
  ingest directly.

## Consequences

- If Holodex later needs richer instrumentation (exemplars, native histograms,
  many label dimensions, the Go runtime collectors), revisit and adopt
  `client_golang` — this ADR is **superseded** at that point, not edited.
- The hand-rolled package is responsible for format correctness; it is covered by
  unit tests asserting bucket monotonicity and the presence of each series.
- Pull-based gauge means the queue depth is always scrape-fresh and needs no
  push from the thumbnail workers.
