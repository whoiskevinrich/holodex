# ADR-008: Caching Strategy — In-Process Cache with Redis-Ready Interface

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Holodex targets ≤ 300ms search response at p95 for 50k records. SQLite with proper indexes and WAL mode will meet this for most queries. However, repeated identical queries (popular filter combinations, people/tags index pages, thumbnail lookups) benefit from a cache layer. The project is currently personal-use but is planned for potential public release, which introduces the possibility of multiple concurrent users and eventually multiple replicas.

## Decision

**In-process Go cache** (backed by `ristretto`) for Phase 1 and Phase 2, behind a **`Cache` interface** that allows a Redis backend to be substituted without service layer changes.

## Cache Interface

```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, bool)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Invalidate(ctx context.Context, key string) error
    InvalidatePrefix(ctx context.Context, prefix string) error
}
```

Implementations:
- `InProcessCache` — backed by `dgraph-io/ristretto` (concurrent, LRU with admission policy, configurable max memory)
- `RedisCache` — backed by `redis/go-redis/v9` (future; enabled via `CACHE_BACKEND=redis` + `REDIS_URL`)
- `NoopCache` — disables caching entirely; useful for testing and `CACHE_BACKEND=none`

## What Gets Cached

| Cache key pattern | TTL | Invalidated by |
|-------------------|-----|----------------|
| `media:list:{hash of query params}` | 60s | Any file indexed/removed |
| `media:detail:{id}` | 5m | That file re-indexed |
| `people:list` | 5m | Any person added/removed |
| `people:detail:{id}` | 5m | That person's videos change |
| `tags:list` | 5m | Any tag added/removed |
| `tags:detail:{id}` | 5m | That tag's videos change |
| `thumbnail:{id}` | 1h | Thumbnail regenerated |

Cache keys are hashed from the full normalized query parameter set so that `?tags=1,2` and `?tags=2,1` resolve to the same key.

## Invalidation Strategy

The scanner and web server run in the same Go process. When the scanner indexes a file:
1. It calls `cache.InvalidatePrefix("media:list:")` — clears all list query results.
2. If the file already existed, it calls `cache.Invalidate("media:detail:{id}")`.
3. If the file's people or tags changed, it calls `cache.InvalidatePrefix("people:")` and `cache.InvalidatePrefix("tags:")`.

This is simple and correct for single-process deployments. For multi-replica deployments (future), Redis pub/sub handles cross-instance invalidation — the `RedisCache` implementation subscribes to an invalidation channel.

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `CACHE_BACKEND` | `memory` | `memory`, `redis`, or `none` |
| `CACHE_MAX_MEMORY_MB` | `128` | Max in-process cache size (ignored for Redis) |
| `REDIS_URL` | — | Required when `CACHE_BACKEND=redis` |

## Why Ristretto

- Concurrent-safe with no global lock on reads
- Admission policy (TinyLFU) avoids cache pollution from one-off queries
- Configurable memory bound prevents unbounded growth in long-running containers
- Well-maintained by Dgraph; used in production at scale

## Consequences

- Phase 1/2 Docker image adds no new containers or infrastructure for caching.
- If/when Holodex is released publicly and horizontal scaling is needed, adding `CACHE_BACKEND=redis` and a Redis container to `docker-compose.yml` is the only change required.
- The `Cache` interface is injected into service constructors — services never reference the concrete implementation, keeping the substitution clean.
- Cache hit/miss counters are exposed in the Phase 2 Prometheus metrics (`holodex_cache_hits_total`, `holodex_cache_misses_total` labeled by key prefix).
