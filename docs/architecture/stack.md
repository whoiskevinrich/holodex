# Stack

This doc owns the base technology choices every other part of Holodex sits on: the backend
language, the SPA framework, its component-composition pattern and its component-test harness, its
long-list virtualizer, the database engine and its migrations, the HTTP API style, the MCP
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

## Frontend component tests in Vitest browser mode

`web/` runs Vitest as two projects in `vite.config.ts`. **`client`** runs `src/**/*.svelte.test.ts`
in headless Chromium through `@vitest/browser-playwright` and mounts components with
`vitest-browser-svelte`. Each test gets the real Svelte client runtime and a real DOM, with layout,
focus and events. **`server`** runs every other test file in Node: pure logic, the geometry
harness's own `.mjs` tests, and markup checks that use `svelte/server`'s `render()`.

A test goes by its filename. If it needs the component alive (mount effects, awaited fetches, a
click, where focus lands), it is a `*.svelte.test.ts`. If a rendered string answers the question,
it stays a `*.test.ts` in Node. Modules a page reaches outside itself, such as `#lib/api.ts`, the
`activity` store and `$app/*`, are replaced with `vi.mock`. There is no backend in the loop. CI
installs Chromium before `npm run test`.

**Rejected:** jsdom with `@testing-library/svelte`. It needs no browser download, but jsdom has no
layout and only approximates focus and `inert`. The SPA leans on exactly those for roving tabindex
and for parking focus after a dismiss, so a passing jsdom test can disagree with the app. It also
needs `resolve.conditions: ['browser']`, which changes how the Node tests resolve. **Rejected:** a
jsdom-plus-browser hybrid. It means two DOM environments and two sets of mocks, plus a fidelity
call on every new test, and it saves little: once Chromium is installed for any browser test, CI
has already paid for the download. **Rejected:** SSR `render()` alone. It never runs `onMount`
or awaits a fetch, so it can't test anything that happens after first paint.

Decided in [`d3b92f4c`](https://github.com/whoiskevinrich/holodex/commit/d3b92f4c) (HOLODEX-223).

## Frontend components: one sibling per data shape, over shared chrome

Two editing widgets that look alike but commit differently stay separate components. They share
their chrome; they are not one component configured by entity kind. The relationship editors on
the media page are the model. `StudioPicker` writes a single decided field through the
source-decision endpoint and runs the collision verdict. `PersonPicker` writes a multi-valued
curation list in which every link carries a role. What the two have in common is extracted, not
copied: `shared/Modal.svelte` owns the dialog mechanics, `entity/PickerShell` the header and close,
and `entity/AttachedChip` an attached entity with its detach. Tags differ in shape again: they are
added through a text input (`TagAddInput`), not a dialog, so they share none of that chrome. A look
change lands once, and each commit path reads top to bottom in one file.

- **Extend before you add.** Svelte has no inheritance, so props and snippets are the extension
  point. A new widget extends an existing component when its data shape and commit path match one,
  and becomes a new sibling only when one of them differs.
- **Shared pieces go in the narrowest folder that holds every caller,** by the classification rule
  in `web/src/lib/components/CLAUDE.md`. Each folder's own `CLAUDE.md` lists its files and names
  every sibling relationship. That table is the inventory: it sits beside the code and changes in
  the same commit as the component.

**Rejected:** one generic picker driven by an `entityKind` prop. It trades readable siblings for
one component that branches on kind inside: a role control that tags don't have, two commit paths,
and a verdict flow only one caller uses. **Rejected:** a separate component catalogue, either an
inventory doc or a showcase route. The per-folder tables already describe each file next to the
file itself, and a second copy would drift from them.

Decided in [`5e1da30c`](https://github.com/whoiskevinrich/holodex/commit/5e1da30c) (HOLODEX-287).

## Long lists: TanStack Virtual's core over a sparse page cache

The whole-list scroll on People and Studios (F80 EL11–EL13) has two halves, and only one of them
comes from a library.

**The range comes from `@tanstack/virtual-core`.** The core works out which entries are on screen,
places them, and scrolls to an index. A thin runes wrapper in `web/src/lib/` drives it in window
mode (`observeWindowRect`, `observeWindowOffset`, `windowScroll`), so the page keeps its own
scrollbar and the sticky toolbar and A–Z index keep working. `scrollMargin` is the list's offset from the top of the page. Columns are the core's
`lanes`: 1/2/3 for the People and Studios rows at the `sm`/`lg` breakpoints, `posterColumns()` for
Poster view and 1 for Table view. Pin an exact version and add nothing else from TanStack.
`@tanstack/svelte-virtual` is not used. It wraps the core in a Svelte store, and its releases lag
the core's.

**Fixed row heights are a contract, not a measurement.** Each view's row has one height, which the
design handoff fixes (one-line truncation, a fixed avatar or logo box). The wrapper gives the core
that height as `estimateSize` and never turns on `measureElement`. A poster card has a fixed aspect,
so its row height is a function of the list's width and the column count. A `ResizeObserver` on the
list recomputes it and calls `measure()`. A row that grows past its height (wrapping text, say) is a
bug in that row's markup. Measuring would hide it, and would also move the scrollbar while you drag.

**The data comes from a sparse page cache, hand-rolled beside `PagedList`.** No virtualizer fetches,
so this half is Holodex's own, and it reuses `PagedList`'s fetch contract unchanged. The `PageFetcher`
is `(offset, limit, fresh)`, with offsets aligned to 50-entry pages and `fresh` for the first read
that asks for `letters`. The server already pages by offset under every sort and seeded Random,
so the backend needs no change.

- **What is cached.** Each page is in one of three states: arriving, loaded, or failed. Each reply
  carries `total`, and a changed total resizes the core's `count`. The cache keeps at most 20
  pages. Past that, it evicts the page farthest from the visible range. Memory was never the
  constraint, since 1,500 entries are a few hundred KB. The cap keeps a long session from slowly
  becoming a whole-library copy that a mutation refresh would then have to re-read.
- **When it fetches.** Placeholders render at once for every visible index. Fetching waits until the
  core reports `isScrolling` false (its reset delay is 150ms), with a 300ms ceiling during a
  continuous slow scroll, so a fling across 1,500 entries fetches only where it lands. Missing
  pages in the visible range, plus one page of overscan each way, are fetched in parallel. A reply
  for a page that scrolled away is still cached, because it is valid data.
- **Staleness.** A sort, search or reshuffle bumps a generation number and empties the cache, the
  same guard as `PagedList`'s `#seq`. A reply from an older generation is dropped. A mutation
  refresh keeps the scroll offset, re-reads the visible pages, and drops the rest.
- **Back (EL7).** `listScroll` saves the first visible index and its offset within the row, not raw
  `scrollY`, because the row height depends on the width, which Back may not restore. The restore
  fetches that index's page first (which also brings `total`), sizes the list, then scrolls.

`PagedList` stays as it is for the lists that keep Load more: Tags, Films and the owner queues.

**Accessibility comes from the markup, not the library.** List, Poster and Studios rows are list
items carrying `aria-setsize` (the total) and `aria-posinset` (index + 1). Table view stays a real
`<table>`, so its rows can't be absolutely placed without breaking the fixed column widths. Instead,
two spacer rows above and below the rendered rows take the core's `start` and remaining height.
Its rows carry `aria-rowindex` (index + 2, since the header is row 1), and the table carries
`aria-rowcount` (total + 1), because `aria-setsize` doesn't apply to table rows. A placeholder is the same element with `aria-busy="true"`. The list is one tab stop
with roving tabindex, consistent with the other keyboard lists. The focus index is its own state,
and the core's `rangeExtractor` always includes it, so the focused row stays rendered when it
scrolls out of view. Each rendered row is keyed by absolute index, not entity id, so the element
that holds focus survives its placeholder being replaced by the loaded entry. The roving target is
the row element itself, and Enter follows the row's link.

**One ceiling, for later lists.** Browsers cap an element's height at about 17.9M px in Firefox and
33.5M px in Chrome. People and Studios at 60px rows are about 100,000px. A future whole-list Media
grid with hundreds of thousands of entries would cross it, and would need scaled offsets that the
core doesn't provide.

**Rejected:** hand-rolling the range calculation too. With fixed heights it is a few lines of
arithmetic, and the owner weighed that. The library still brings `scrollToIndex` alignment, lanes,
`rangeExtractor` and the scroll-state flag already tested, and it is where Films, Media and Playlists
would land later, when fixed heights may not hold. **Rejected:** `@tanstack/svelte-virtual`, for
its store-based adapter (above). **Rejected:** rendering every row as a placeholder and filling them
as they scroll in, even with `content-visibility: auto`. The DOM would still grow with the library,
which is the cost F80 removed. **Rejected:** infinite scroll that appends at the bottom. It can't
jump, and jumping is the point.

Decided in: pending squash (HOLODEX-601).

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
