# ADR-006: API Design — REST + OpenAPI 3.1

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

The Go backend must expose library data to the SvelteKit frontend and (indirectly) to the MCP server. The API design affects how filters are expressed, how pagination works, and how the frontend fetches data.

## Decision

**REST over HTTP/JSON**, versioned under `/api/v1/`, with an **OpenAPI 3.1 spec** as the authoritative contract.

## Rationale

- **Simplicity**: Fixed, well-defined filter dimensions (people, tags, duration, resolution, date) map cleanly to query parameters. No dynamic field selection is needed.
- **Native SvelteKit fit**: `fetch` in SvelteKit load functions and `+page.ts` files works directly against REST endpoints with no additional client library.
- **MCP independence**: MCP tools call Go **service layer functions directly** (same process, same binary) — they do not round-trip through HTTP. The REST API and MCP tools share the same repository/service code; REST is purely a frontend concern. This makes the API design choice irrelevant to MCP correctness.
- **Debuggability**: REST endpoints are trivially testable with `curl` and inspectable in browser DevTools, which matters for a self-hosted personal tool.
- **OpenAPI spec**: Generated via `swaggo/swag` annotations on handlers. Provides a machine-readable contract, enables auto-generated API docs at `/api/docs`, and documents query param shapes for future contributors.

## Rejected Alternatives

| Option | Reason rejected |
|--------|-----------------|
| GraphQL | Adds `gqlgen` codegen, a separate schema language, and a frontend GraphQL client — significant toolchain overhead for a fixed query surface |
| tRPC | Requires TypeScript on both ends; incompatible with Go backend |

## API Shape (Phase 1)

```
GET  /api/v1/media               # list + filter + search
GET  /api/v1/media/:id           # video detail
GET  /api/v1/people              # list people
GET  /api/v1/people/:id          # person detail + their videos
GET  /api/v1/tags                # list tags
GET  /api/v1/tags/:id            # tag detail + their videos

# Phase 2 additions
POST /api/v1/admin/rescan        # trigger full re-index
POST /api/v1/media/:id/thumbnail # regenerate thumbnail
GET  /metrics                    # Prometheus exposition
```

## Filter Query Parameters (`GET /api/v1/media`)

| Param | Type | Example |
|-------|------|---------|
| `q` | string | `?q=sunset` |
| `people` | comma-separated IDs | `?people=1,4,7` |
| `tags` | comma-separated IDs | `?tags=12,15` |
| `duration_min` | integer (seconds) | `?duration_min=3600` |
| `duration_max` | integer (seconds) | `?duration_max=7200` |
| `resolution` | enum: SD,HD,FHD,4K | `?resolution=4K` |
| `date_from` | ISO date | `?date_from=2020-01-01` |
| `date_to` | ISO date | `?date_to=2022-12-31` |
| `sort` | enum (see below) | `?sort=date_desc` |
| `page` | integer | `?page=2` |
| `page_size` | integer (max 100) | `?page_size=50` |

Sort values: `title_asc`, `title_desc`, `date_asc`, `date_desc`, `duration_asc`, `duration_desc`, `resolution_asc`, `resolution_desc`, `indexed_asc`, `indexed_desc`.

## Consequences

- The Go HTTP router is `chi` (lightweight, idiomatic, middleware-composable).
- All responses are `application/json`; errors follow `{"error": "message", "code": "SNAKE_CASE"}`.
- Pagination envelope: `{"data": [...], "total": N, "page": P, "page_size": S}`.
- The OpenAPI spec lives at `docs/api/openapi.yaml` and is regenerated as part of the build.
- MCP tools (`search_videos`, `get_video`, `list_people`, `list_tags`) call `service.VideoService` and `service.PeopleService` Go interfaces directly — no HTTP involved.
