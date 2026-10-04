# Stack

This doc owns the base technology choices every other part of Holodex sits on: the backend
language, the SPA framework, the database engine and its migrations, the HTTP API style, the MCP
transport, and the CSS design-token mechanism. How the pieces are packaged and shipped belongs to
[deployment.md](deployment.md); configuration keys and their precedence belong to
[config-and-settings.md](config-and-settings.md).

## Go backend in one process

The backend is Go, compiled to a single binary that runs the HTTP server, the background scanner
and its workers, and the MCP server as goroutines in one process sharing one `*sql.DB` and one
`repo.Repo`. Media work shells out to the tools bundled in the image (ffprobe/ffmpeg, exiftool,
MKVToolNix — see [deployment.md](deployment.md)) rather than linking media libraries. Data access
goes through `database/sql`; there is no ORM.

**Rejected:** Python/FastAPI — heavier image, and the GIL fights running the scanner and server in
parallel. **Rejected:** TypeScript/Bun — less ergonomic for subprocess and binary work.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## SvelteKit as a static SPA

The frontend is SvelteKit with TypeScript, built by `@sveltejs/adapter-static` in SPA mode
(`fallback: 'index.html'`, output in `web/dist`). There is no SSR: the Go server serves the built
assets and routes every non-API path to the fallback so the client router owns navigation and
URL-reflected state. The Go API contract is the only interface boundary between frontend and
backend; no types are shared or generated across it.

**Rejected:** Next.js — larger bundle, reconciler overhead on large grids, RSC complexity a local
SPA doesn't need. **Rejected:** Solid.js — smaller component ecosystem.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## SQLite (modernc) with WAL and FTS5

The database is a single SQLite file opened by `internal/db` with the pure-Go `modernc.org/sqlite`
driver (no CGo, no C toolchain in the build). Its path is `DATABASE_PATH`, defaulting to
`holodex.db` under the data directory. Every connection gets the pragmas `journal_mode(WAL)`,
`synchronous(NORMAL)`, `busy_timeout(5000)`, `foreign_keys(ON)`, `cache_size(-64000)` and
`temp_store(MEMORY)`.

- **Single writer.** SQLite allows one writer, so every write in `internal/repo` takes
  `Repo.writeMu`. Reads take no lock; WAL lets them run alongside a write.
- **Full-text search** uses FTS5 virtual tables (`videos_fts` and its siblings) kept in sync by
  triggers, created by migrations like any other schema object.
- **Backup is a file copy** of the database and its WAL.
- Graph-shaped queries are written as recursive CTEs rather than reaching for another engine.

**Rejected:** PostgreSQL — a sidecar container and volume to operate for a single-user tool.
**Rejected:** DuckDB — built for analytical workloads, not a scanner-writes/server-reads OLTP mix.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## golang-migrate with embedded versioned SQL, applied at boot

Schema changes are `golang-migrate` migrations in `internal/db/migrations/`, paired
`NNNN_name.up.sql` / `NNNN_name.down.sql` files embedded into the binary with `go:embed`.
`db.Open` runs every pending `up` migration before the server listens; a failed migration aborts
startup rather than serving a half-migrated schema. The applied version lives in
`schema_migrations`. `-migrate-only` applies migrations and exits.

The constraint this imposes: indexed data is rebuildable by a rescan, so destructive changes to it
are cheap, but owner-authored rows (enrichment, aliases, decisions, curation) are not, and every
up- and down-migration must preserve them.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## REST/JSON on chi under `/api/v1`

The HTTP API is REST over JSON, routed with `go-chi/chi/v5` and versioned under `/api/v1/`.
Filters, sort and paging are query parameters. Errors are `{"error": "<message>"}` with a matching
HTTP status (`writeError` in `internal/api/json.go`). List responses carry `items` and `total`,
plus `limit`/`offset` where the list pages.

REST is the frontend's concern only. The MCP server does not call the API over HTTP; it calls the
same `internal/repo` code in-process, so the two surfaces can't drift into separate data paths.

**Rejected:** GraphQL — codegen, a second schema language and a client library for a fixed query
surface. **Rejected:** tRPC — needs TypeScript on both ends.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## MCP over Streamable HTTP, with a stdio entrypoint

`internal/mcp` builds the MCP server on `mark3labs/mcp-go` over the same repository as the REST
API. It has two transports:

- **HTTP (primary).** With `MCP_ENABLED=true` and `MCP_TRANSPORT` `http` (the default) or `both`,
  the web process starts a second listener on `MCP_PORT` (default `7801`) serving Streamable HTTP
  at `/mcp`, plus the legacy SSE stream at `/mcp/sse` and `/mcp/message` for older clients.
- **stdio (secondary).** A separate entrypoint, `holodex -mcp-transport stdio`, runs the MCP
  server over stdin/stdout instead of the web server. It is meant for `docker exec -i` into the
  running container.

Streamable HTTP is primary because it works across containers, machines and reverse proxies with no
shell access to the host, and an exposed MCP port can be fronted by the same proxy auth as the web
UI.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## Semantic CSS tokens mapped by Tailwind v4 `@theme inline`

All colour, font and radius values are CSS custom properties defined once, in the `:root` block of
`web/src/app.css`. The vocabulary is semantic, not palette-named: `--bg`, `--surface`,
`--surface-2`, `--ink`, `--muted`, `--rule`, `--accent`, `--accent-ink`, `--warn`, `--warn-ink`,
`--logo-plate`, `--logo-plate-ink`, `--font-display`, `--font-ui`, `--radius`. There is one token
set and no theme selector: no `data-theme`, no runtime switch, no server-held theme.

- **Tailwind v4, CSS-first.** `tailwindcss` with the `@tailwindcss/vite` plugin; there is no
  `tailwind.config.*` and no PostCSS pipeline. A `@theme inline` block at the top of `app.css` maps
  each utility to its token (`--color-bg: var(--bg)`, `--radius-theme: var(--radius)`, …), so
  components use `bg-bg`, `text-ink`, `text-accent`, `font-display`, `rounded-theme`.
- **`inline` is load-bearing.** It makes each utility emit the `var(--x)` reference itself instead
  of a value resolved once at build time, so the token block stays the single source. Plain
  constants that are not token aliases (`--container-stage`) go in a separate plain `@theme` block.
- **Components never name a palette.** No hex, `zinc-*` or other raw scale values in a component.
  A colour change is an edit to the `:root` block and nothing else.
- **Visual flourishes live in CSS, not markup.** Components opt in by carrying a stable hook class
  (`.app-atmosphere`, `.video-frame`, `.video-grid`, `.skin-title`); `app.css` owns what the hook
  looks like.
- **Fonts are bundled offline** as `@fontsource-variable/*` npm dependencies imported in `app.css`,
  so the image never fetches a font at runtime.
- **One mirror outside CSS.** `internal/personimage/placeholder.go` copies `--surface-2`, `--muted`,
  `--accent` and `--rule` for the server-rendered placeholder SVG and must be changed with them.

**Rejected:** per-look token blocks behind a theme selector — every look multiplies QA and the
token mirrors, and a look nobody QAs rots silently. **Rejected:** a plain `@theme` for the token
map — it freezes each utility to one resolved value and breaks the single-source rule.

Decided in [`4fec2bab`](https://github.com/whoiskevinrich/holodex/commit/4fec2bab).
