# ADR-020: Frontend Embed Location, SPA Fallback Serving & BuildKit Caching

**Status**: Accepted
**Date**: 2026-06-09
**Deciders**: Project owner
**Supersedes**: the "Build Stages" and "Frontend Embedding" mechanics of [ADR-007](ADR-007-docker-structure.md) (its overall single-multi-stage-image + Vite-dev decision still stands)

---

## Context

[ADR-007](ADR-007-docker-structure.md) decided to embed the built SvelteKit SPA into
the Go binary under the `production` build tag and serve it for all non-`/api`,
non-`/mcp` routes with an `index.html` SPA fallback. That decision is sound, but the
build mechanics it documented do not compile and were never implemented:

- ADR-007 shows `COPY --from=frontend /app/web/dist ./web/dist` (repo root) and
  `//go:embed web/dist`, while building `./cmd/holodex`.
- `//go:embed` resolves **relative to the package directory** and **cannot reference a
  parent directory**. The only `package main` source lives in `cmd/holodex/`, so a
  root-level `web/dist` is unreachable from the embed directive.

The result in practice: the embed was never wired up. `cmd/holodex/main.go` built the
chi API router but never mounted a frontend, so `docker compose up --build` produced a
container that returned **404 at `/`**. This ADR records the implementation that makes
ADR-007's intent actually work.

## Decision

### 1. Embed source lives in the `cmd/holodex` package

Two build-tagged files split production vs. dev so dev builds never require `web/dist`:

- `cmd/holodex/frontend_prod.go` (`//go:build production`) — `//go:embed all:web/dist`,
  exposes `frontendFS() http.FileSystem` over the `web/dist` subtree.
- `cmd/holodex/frontend_dev.go` (`//go:build !production`) — `frontendFS()` returns `nil`.

Because the embed directive is in the `cmd/holodex` package, the built assets must be
copied to **`cmd/holodex/web/dist`** (not repo-root `web/dist`):

```dockerfile
COPY --from=frontend /app/web/dist ./cmd/holodex/web/dist
```

### 2. SPA fallback handler

`cmd/holodex/spa.go` serves any file that exists in the embedded FS directly, and falls
back to `index.html` for any other path so the SvelteKit client router can take over.
`cmd/holodex/main.go` installs it only when `frontendFS()` is non-nil, routing `/api*`,
`/healthz`, and `/readyz` to the API and everything else to the SPA handler. In dev
(no `production` tag) the handler is absent and Vite serves the frontend with HMR.

### 3. BuildKit cache mounts

The npm cache, Go module cache, and Go build cache persist across image builds:

```dockerfile
RUN --mount=type=cache,target=/root/.npm npm ci
RUN --mount=type=cache,target=/go/pkg/mod go mod download
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -tags production -o /out/holodex ./cmd/holodex
```

### 4. `.dockerignore`

A `.dockerignore` excludes `web/node_modules`, `web/dist`, `.git`, `.claude`, `data`,
and `*.db` from the build context so it stays small and host build artifacts never
leak into the image.

### 5. Startup logs the URL

The "listening" log line includes `"url":"http://localhost:<port>"` so the operator
sees where to connect without consulting docs.

## Consequences

- ADR-007's Dockerfile code blocks and "Frontend Embedding" section are **superseded**
  by this ADR; its base-image (Debian slim), port, and dev-workflow decisions are
  unaffected.
- `docker compose up --build` now serves the SPA at `http://localhost:7800/`.
- The `production` build tag remains mandatory for any image that should serve the UI;
  a plain `go build` yields an API-only binary by design (dev workflow).
- `MEDIA_PATH` is unrelated to UI serving: the binary reads it as the library root, and
  docker-compose substitutes the same `${MEDIA_PATH}` into the host media bind-mount
  (the in-container `MEDIA_PATH` stays `/media`).
  *(Updated 2026-06-15, PR #22: the compose host var was originally a separate
  `HOLODEX_MEDIA_PATH`; it was unified to `MEDIA_PATH` so one name serves both the bind
  source and the binary — reducing devex cognitive load. This note amends the original
  "remain distinct" wording rather than superseding the ADR's decision.)*
