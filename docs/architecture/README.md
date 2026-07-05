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
| [037](ADR-037-soft-delete-and-purge.md) | Soft-delete media — **`deleted_at` axis orthogonal to `active`** + dedicated purge job | Accepted (spec F24; extends ADR-018/028/030) |
| [038](ADR-038-person-images.md) | Person images — **on-disk store + typed real-or-placeholder serving** + shared ingest normalization | Proposed (spec F25; extends ADR-009/030/033) |
| [039](ADR-039-provider-asset-urls.md) | Provider asset URLs — contract clarification + **operator-configured `asset_hosts` allowlist** | Accepted (extends ADR-033/038; updates metadata-provider-contract) |
| [040](ADR-040-tmdb-provider-repo-placement.md) | TMDB provider source placement — **monorepo `providers/tmdb/` subdirectory** | Accepted (extends ADR-033/039/023/024) |
| [041](ADR-041-metadata-writeback.md) | Metadata writeback — **explicit per-field write-back** to media files via copy→write→rename | Proposed (extends ADR-004/013/033; spec F28) |
| [042](ADR-042-windows-asInvoker-manifest.md) | Windows build — **`asInvoker` application manifest** embedded via `.syso` to suppress UAC prompt | Accepted (spec windows-uac-manifest) |
| [043](ADR-043-gallery-cap-and-enrichment-suppression.md) | Person gallery — **configurable cap (`PERSON_GALLERY_MAX`) + owner over-cap override + enrichment URL suppression on delete** | Accepted (spec F25.23–25; extends ADR-038/039) |
| [044](ADR-044-automated-version-and-release-pr.md) | Release versioning — **Release Please** release-PR + PAT-driven tag in front of unchanged `release.yml` | Accepted (extends ADR-024/034/035; revisits ADR-034's "no release-please") |
| [045](ADR-045-seeded-random-ordering.md) | Seeded random ordering — **deterministic `holo_shuffle(id, seed)` SQL function** for paginated Media + named-sort enum for People/Tags | Proposed (spec sort-persistence SP2/SP3; extends ADR-006/003/017; relates ADR-031) |
| [046](ADR-046-owner-session-persistence.md) | Owner session persistence — **HttpOnly signed token-exchange cookie** (`POST /session`), gate accepts cookie-or-header | Proposed (spec owner-session-persistence; amends ADR-030's "in-memory only" condition) |
| [047](ADR-047-per-item-metadata-refresh.md) | Per-item metadata refresh — **unified forced re-extract + re-enrich** over a `plan`/`apply` seam; flat `kind=refresh` job row; non-destructive layering invariant | Proposed (spec F31; extends ADR-004/018/033/028/030; relates ADR-013/037/041) |
| [048](ADR-048-metadata-curation-and-write-queue.md) | Granular metadata curation — **cross-source dedup merge + `manual` source/tombstones** + **durable bounded-concurrency batch-writeback queue** | Proposed (spec F30; generalizes ADR-013; extends ADR-033/041/028/030; partially realizes ADR-041 Option C) |
| [049](ADR-049-manual-image-precedence.md) | Owner-set person images — **enrichment never overwrites an `upload`/`promoted` core slot** (provenance-implicit lock, no migration) | Proposed (spec F25.31/F33; extends ADR-038; sibling of ADR-043; image twin of ADR-048) |
| [050](ADR-050-image-content-dedup.md) | Deduplicate enrichment photos by **image content hash** — gallery `extra` skipped when its sha256 matches any of the person's images; `source_url` fast-path; app-layer enforcement + one-time backfill (migration 0015) | Proposed (spec F34; extends ADR-038; byte-level sibling of ADR-043/ADR-049) |
| [051](ADR-051-per-field-source-of-truth-decisions.md) | Per-field **source-of-truth decisions** — file-baseline default + standing per-item `{file, provider:<name>, manual}` decision overriding precedence (source-pin, not value); drives display + writeback; **entity-generic + multi-provider** (migration 0016) | Proposed (supersedes ADR-047 F31.11 slice; extends ADR-013/033/041/048; relates ADR-030/036) |
| [052](ADR-052-baseline-source-contract.md) | **`BaselineSource` contract** — resolver baseline-layer seam: entity-agnostic `ResolveFields` core over a `BaselineSource` interface, `Resolve` reduced to the video (file-layer) wrapper; behavior-preserving | Accepted (realizes ADR-051 §9 fast-follow ①; extends ADR-033/013; spec F36) |
| [053](ADR-053-studio-entity-and-resolved-link-derivation.md) | **Studio entity** — `studios` + `video_studios` join + `studios_fts` (migration 0017); `video_studios` **derived from the resolved `studio` field** (re-linked on scan/enrich/decision/curation, prune-on-empty), not raw extraction; `studioBaseline`; identity ops deferred | Proposed (realizes ADR-051 §9 fast-follow ③; extends ADR-052/017/013; relates ADR-036; spec F38) |
| [054](ADR-054-studio-external-id-dedup.md) | **Studio external-id de-dup** — `studio_external_ids(external_id PK, studio_id FK)` join (migration 0018); capture TMDB `production_companies[].id` via a self-describing **internal sidecar** enrichment field (`_`-prefix, never displayed/resolved); thread a name→external_id side-map into an **external-id-first** `resolveOrCreateStudio` so same-company spellings converge (refines ADR-053 RD1) | Proposed (extends ADR-053; relates ADR-033/036/047; spec F38 P2-5; HOLODEX-122) |
| [055](ADR-055-enrichment-unique-key-invariant.md) | **Universal enrichment unique-key invariant** — every source MUST supply a namespaced id `<namespace>:<id>` and it is the **sole identity/de-dup key** for the entity (no name fallback); the namespace is a **shared identity space** so providers converge cross-provider; generalizes ADR-054 to all canonical entities (studio ✓, person → HOLODEX-125, video enrichment, future tags) | Proposed (generalizes ADR-054; extends ADR-033 contract; relates ADR-036/047/051; HOLODEX-123; impl HOLODEX-124/125) |
| [056](ADR-056-person-link-resolved-derivation.md) | **Person link resolved-derivation** — `video_people` migrates from scan-time raw-extraction to **resolved-value derivation** (`RelinkVideoEntity`, sole writer; `RelinkVideoStudios` generalized in) with a **`role` column derived from the source person-typed field** (PK → `(video_id, person_id, role)`); **`entity: person` field marker** drives derivation/`peopleKeys`/link-picker; **lossless cutover** (derivation sources ⊇ extraction `peopleKeys`, backfill loss-guard); **30-day orphan grace + authored-identity prune guard** (vs. studio's immediate prune); `role` unset-capable (empty sentinel); writeback reuses `actors→Artist` (canonical name, no new perimeter); **studio parity (P0, ships with people)** adds owner-view studio link + `studio→Publisher` writeback | Proposed (applies+generalizes ADR-053, **amends its no-studio-writeback non-goal**; extends ADR-052/051/013; relates ADR-036/041/030/055; spec F39; foundation for F32) |

> **Reserved:** ADR-029 — live activity transport (Server-Sent Events) for F21.8 (P1), to be drafted when SSE is scheduled.

## Phase specs
- [Phase 1 — MVP](../specs/phase-1-mvp.md)
- [Phase 2 — MCP + Polish](../specs/phase-2-mcp-polish.md)
- [Phase 3 — Enrichment](../specs/phase-3-enrichment.md)
- [System Activity — "Under the Hood" (F21)](../specs/system-activity.md)
- [Quick Wins — Search history & "More with …" shelves](../specs/quick-wins.md)
- [Metadata Source Plugins (F22)](../specs/metadata-plugins.md) — keystone of Phase 3; detailed F16
- [Person Aliases (F23)](../specs/person-aliases.md) — first Phase-3 People slice; detailed F14.1
- [Sticky sort + Random sort](../specs/sort-persistence.md) — per-page localStorage sort + seeded Random (ADR-045)
- [Refresh Metadata (F31)](../specs/metadata-refresh.md) — per-item forced re-extract + re-enrich (ADR-047)
- [Per-field source-of-truth (F36)](../specs/field-source-of-truth.md) — standing per-item, per-field decision over precedence; file-first default (ADR-051)

## Cross-cutting
- [Testing Strategy](../testing-strategy.md) — pyramid, fixture corpus, per-component plan, CI, phasing
