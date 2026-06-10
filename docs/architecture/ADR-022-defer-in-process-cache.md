# ADR-022: Defer the In-Process Cache to a Measured Need

**Status**: Accepted
**Date**: 2026-06-10
**Deciders**: Project owner
**Supersedes**: the *Phase-1 timing* of [ADR-008](ADR-008-caching.md) (the cache abstraction, ristretto choice, and Redis-readiness all stand)

---

## Context

[ADR-008](ADR-008-caching.md) decided on a cache abstraction with an in-process
(ristretto) backend and a noop backend, and scoped *both* to Phase 1. In practice:

- The cache **interface** and **Noop** backend exist (`internal/cache`); `CACHE_BACKEND`
  defaults to `memory`, which currently falls back to Noop.
- The Phase-1 read paths (media list/detail/stream, people, tags, search) query SQLite
  directly. With WAL, the covering indexes from migration 0001, and FTS5, these comfortably
  meet the Phase-1 NFR (search p95 ≤ 300 ms at 50k rows) on personal-library hardware — no
  app-level cache is needed to hit the target.
- A real in-process cache requires **invalidation correctness**: every scanner write
  (upsert / deactivate) must invalidate the right entries, and ristretto offers no key
  enumeration (prefix invalidation would need a side index or a full flush). That is
  non-trivial machinery to add and test for a benefit we cannot currently measure.

## Decision

**Do not implement the in-process (ristretto) backend in Phase 1.** Keep the cache
*interface* and the *Noop* backend, so a real backend can drop in later without touching
the service layer (the portability ADR-008 was designed for). `CACHE_BACKEND=memory`
continues to resolve to Noop until the backend lands.

The ristretto `InProcess` backend will be built **when there is a measured need** — e.g. a
profiled endpoint exceeding the NFR on a real library — at which point invalidation can be
designed against the actual hot paths (likely flush-on-scan as the first cut).

## Consequences

- No dead/speculative cache code in the tree; `internal/cache` ships the interface + Noop,
  matching what the read paths actually use.
- The NFR is met by the database layer; revisit if profiling shows otherwise.
- When implemented, this ADR is superseded by one that specifies the backend and its
  invalidation strategy.
- `internal/cache/cache.go` references this ADR where it falls back to Noop, so the
  deferral is discoverable from the code.
