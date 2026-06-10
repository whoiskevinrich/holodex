# Architecture Decision Records

Index of ADRs for Holodex. Each records one decision, its rationale, and consequences.

| ADR | Decision | Status |
|-----|----------|--------|
| [001](ADR-001-backend-language.md) | Backend language — **Go** | Accepted |
| [002](ADR-002-frontend-framework.md) | Frontend framework — **SvelteKit** (SPA) | Accepted |
| [003](ADR-003-database.md) | Database — **SQLite** (modernc) + FTS5 + WAL | Accepted |
| [004](ADR-004-metadata-extraction.md) | Metadata extraction — **exiftool + ffprobe + ffmpeg** | Accepted |
| [005](ADR-005-mcp-transport.md) | MCP transport — **HTTP/SSE primary**, stdio secondary | Accepted |
| [006](ADR-006-api-design.md) | API design — **REST + OpenAPI 3.1** (chi) | Accepted |
| [007](ADR-007-docker-structure.md) | Docker — **single multi-stage image** + Vite dev | Accepted (embed/build mechanics → ADR-020) |
| [008](ADR-008-caching.md) | Caching — **ristretto** in-process, Redis-ready | Accepted (in-process backend deferred → ADR-022) |
| [009](ADR-009-thumbnail-strategy.md) | Thumbnails — tiered, throttled, priority-aware backfill | Accepted |
| [010](ADR-010-mkv-tag-precedence.md) | MKV tag-target precedence — **level 50** authoritative | Accepted |
| [011](ADR-011-symlink-handling.md) | Symlinks — follow, dedup by canonical path | Accepted |
| [012](ADR-012-resolution-classification.md) | Resolution — **width-based** buckets, 10% tolerance | Accepted |
| [013](ADR-013-metadata-field-mapping.md) | Configurable metadata field mapping | Accepted |
| [014](ADR-014-configuration-and-data-layout.md) | Configuration strategy & data directory layout | Accepted |
| [015](ADR-015-media-file-serving.md) | Media file serving — **HTTP Range** requests | Accepted |
| [016](ADR-016-database-migrations.md) | DB migrations — **golang-migrate**, embedded | Accepted |
| [017](ADR-017-search-architecture.md) | Search — global mixed-entity + FTS5 | Accepted |
| [018](ADR-018-scanner-change-detection.md) | Scanner — incremental by (path, size, mtime) | Accepted |
| [019](ADR-019-observability-conventions.md) | Observability & operational conventions | Accepted |
| [020](ADR-020-frontend-embed-and-build.md) | Frontend embed location, SPA fallback serving & BuildKit caching | Accepted |
| [021](ADR-021-frontend-theming-and-skins.md) | Frontend theming — semantic design tokens & 3-skin system | Accepted |
| [022](ADR-022-defer-in-process-cache.md) | Defer the in-process cache to a measured need | Accepted |

## Phase specs
- [Phase 1 — MVP](../specs/phase-1-mvp.md)
- [Phase 2 — MCP + Polish](../specs/phase-2-mcp-polish.md)
- [Phase 3 — Enrichment](../specs/phase-3-enrichment.md)

## Cross-cutting
- [Testing Strategy](../testing-strategy.md) — pyramid, fixture corpus, per-component plan, CI, phasing
