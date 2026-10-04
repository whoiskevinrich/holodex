# Architecture Decision Records

Index of ADRs for Holodex. Each records one decision, its rationale, and consequences.

> **Transition (HOLODEX-523):** these numbered ADRs are being folded into living topic docs. Write
> no new numbered ADR — edit the one that covers the decision in place, or start
> `docs/architecture/<topic>.md`. Rules: [`docs/reference/doc-types.md`](../reference/doc-types.md).

## Topic docs (current truth)

| Doc | Owns |
|---|---|
| [stack.md](stack.md) | Go, SvelteKit SPA, SQLite, migrations, REST, MCP transport, CSS token mechanism |
| [config-and-settings.md](config-and-settings.md) | Config layers, parallel YAMLs, `DATA_PATH` layout, caching posture, the `settings` store |
| [deployment.md](deployment.md) | Image, SPA embed, bundled media tools, GHCR, retag promotion, sidecar images |
| [security-perimeter.md](security-perimeter.md) | Owner gate, session cookie, provider SSRF and asset-host allowlists, read gates |
| [field-resolution.md](field-resolution.md) | Field mapping, the pure resolver, source decisions, promotion and claims, computed fields, completeness store |
| [metadata-providers.md](metadata-providers.md) | Sidecar HTTP contract, the enrichment shadow store, hints, assets, links, auto-apply, traffic pacing |
| [entity-identity.md](entity-identity.md) | Name keys, the alias spine, namespaced external ids, `kind:id` refs, merge, duplicate detection |
| [entity-relationships.md](entity-relationships.md) | Derived person/studio links, tag provenance and hierarchy, categories, film membership and cascades |
| [images.md](images.md) | Image stores and serve routes, normalization, dedup, provenance lock, studio roles, gallery bound, provider icons |
| [media-ingest.md](media-ingest.md) | Scanner and change detection, symlinks, extraction tools, thumbnails, Range serving, soft-delete and purge, refresh |
| [writeback.md](writeback.md) | Atomic file writes, the durable queue, snapshots and revert, read-back, the tag file contract, MKV and MP4 paths, sync state |
| [observability-and-jobs.md](observability-and-jobs.md) | Logging, metrics, health and readiness, `job_runs` history and attribution, dismissals, boot jobs, the refresh sweep |
| [browse-and-list-state.md](browse-and-list-state.md) | Browse query builder, FTS search, shuffle, related media, resolution buckets, URL list state, scroll snapshots, playlists |

An ADR row marked **Folded** is fully absorbed — read the topic doc, not the ADR. An "arch part"
row has its architecture absorbed; its product or UI remainder still awaits a spec or design doc.

The retired numbered ADRs are in [archive/](archive/README.md), kept as history only.
