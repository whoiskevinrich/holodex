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
| [023](ADR-023-image-distribution.md) | Image distribution — **published GHCR image + pull-based compose** | Accepted |
| [024](ADR-024-ci-cd-pipeline.md) | CI/CD — **PR gate + supply-chain scanning + tag-driven releases** | Accepted |
| [025](ADR-025-tailwind-v4-css-first.md) | Tailwind v4 — **CSS-first config** (`@theme inline`) + Vite plugin | Accepted (supersedes ADR-021 §2 mechanism) |
| [026](ADR-026-metrics-exposition.md) | Prometheus metrics — **hand-rolled exposition, no client_golang** | Accepted (extends ADR-019) |
| [027](ADR-027-dotenv-local-config.md) | Local **`.env` loading** for dev config | Accepted (extends ADR-014) |
| [028](ADR-028-activity-surface-and-job-history.md) | User-facing **activity surface** & **job-history** persistence | Accepted (extends ADR-019; spec F21) |
| [030](ADR-030-access-control-gating-seam.md) | Access-control / **"Pro mode" gating seam** for owner-only surfaces | Accepted (spec F21; `ADMIN_TOKEN` + `requireOwner`) |
| [031](ADR-031-related-media-endpoint.md) | Related-media endpoint — **one combined call + `ORDER BY RANDOM()` seam** | Proposed (spec Quick Wins QW2; extends ADR-006/017) |
| [032](ADR-032-browse-state-preservation.md) | Browse-state preservation — **module-scoped grid+scroll cache for fluid Back** | Proposed (spec Quick Wins QW4; extends ADR-002) |
| [033](ADR-033-metadata-source-plugins.md) | Metadata source plugins — **sidecar providers** over a unified resolution layer | Accepted (spec F22; generalizes ADR-013) |
| [034](ADR-034-release-notes-and-deployments.md) | Release notes — **git-cliff changelog** + GHCR **deployment linkage** + package link | Accepted (extends ADR-023/024) |
| [035](ADR-035-ci-cd-scoping-and-release-gate.md) | CI/CD refinements — **image path-scoping** + CodeQL concurrency + **release test-gate** | Accepted (extends ADR-023/024) |
| [036](ADR-036-person-alias-search-indexing.md) | Person aliases — **dedicated `person_aliases_fts` mirror** (not a denormalized column) | Accepted (spec F23; extends ADR-017) |
| [038](ADR-038-person-images.md) | Person images — **on-disk store + typed real-or-placeholder serving** + shared ingest normalization | Proposed (spec F25; extends ADR-009/030/033) |

> **Reserved:** ADR-029 — live activity transport (Server-Sent Events) for F21.8 (P1), to be drafted when SSE is scheduled.

## Phase specs
- [Phase 1 — MVP](../specs/phase-1-mvp.md)
- [Phase 2 — MCP + Polish](../specs/phase-2-mcp-polish.md)
- [Phase 3 — Enrichment](../specs/phase-3-enrichment.md)
- [System Activity — "Under the Hood" (F21)](../specs/system-activity.md)
- [Quick Wins — Search history & "More with …" shelves](../specs/quick-wins.md)
- [Metadata Source Plugins (F22)](../specs/metadata-plugins.md) — keystone of Phase 3; detailed F16
- [Person Aliases (F23)](../specs/person-aliases.md) — first Phase-3 People slice; detailed F14.1

## Cross-cutting
- [Testing Strategy](../testing-strategy.md) — pyramid, fixture corpus, per-component plan, CI, phasing
