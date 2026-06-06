# ADR-007: Docker Image Structure — Single Multi-Stage Image + Vite Dev Server

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Holodex consists of a Go backend (API + MCP server + file scanner) and a SvelteKit frontend (static SPA). The Docker image must include both, plus external binaries (`ffmpeg`/`ffprobe` and `exiftool`). The structure must support fast development iteration and a clean single-container production deployment.

## Decision

**Single multi-stage Docker image** for production. **Vite dev server** (port 5173) for frontend development, proxying `/api/` to the Go server (port 7800).

## Build Stages

```dockerfile
# Stage 1 — Frontend build
FROM node:22-slim AS frontend
WORKDIR /app/web
COPY web/package*.json .
RUN npm ci
COPY web/ .
RUN npm run build         # outputs to /app/web/dist

# Stage 2 — Go build
FROM golang:1.23-bookworm AS backend
WORKDIR /app
COPY go.mod go.sum .
RUN go mod download
COPY --from=frontend /app/web/dist ./web/dist
COPY . .
RUN CGO_ENABLED=0 go build -tags production -o holodex ./cmd/holodex

# Stage 3 — Runtime
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    exiftool \
    ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY --from=backend /app/holodex /usr/local/bin/holodex
EXPOSE 7800 7801
ENTRYPOINT ["holodex"]
```

## Frontend Embedding

- SvelteKit builds to `web/dist/` at compile time.
- The Go binary embeds `web/dist` via `//go:embed web/dist` under the `production` build tag.
- In production, Go serves static files from the embedded FS for all non-`/api/` and non-`/mcp` routes, with `index.html` as the SPA fallback.
- In development (no `production` tag), the embed is skipped; Go serves only the API. Vite handles the frontend with HMR.

## Development Workflow

```
Terminal 1: go run ./cmd/holodex          # API on :7800
Terminal 2: cd web && npm run dev         # Vite on :5173, proxies /api/ → :7800
```

`vite.config.ts` proxy:
```ts
server: {
  proxy: {
    '/api': 'http://localhost:7800',
    '/mcp': 'http://localhost:7801',
  }
}
```

## Base Image: Debian Slim (not Alpine)

**Debian bookworm-slim** is chosen over Alpine because:
- `exiftool` is Perl-based; Perl on Alpine (musl libc) has known compatibility issues with some CPAN modules exiftool depends on.
- `ffmpeg` Debian packages are more complete and better maintained than Alpine's.
- Adds ~40 MB vs Alpine but eliminates an entire class of runtime surprises.

## Consequences

- Final image size estimate: ~180 MB (Debian slim ~80 MB + ffmpeg ~70 MB + exiftool ~20 MB + Go binary ~15 MB).
- `docker-compose.yml` exposes ports 7800 (web UI) and 7801 (MCP, optional).
- A `docker-compose.override.yml` pattern supports local development volume mounts without modifying the production compose file.
- Build cache: Go module download and `npm ci` layers are cached independently — frontend-only changes don't invalidate the Go module cache and vice versa.
- **Test/CI image** additionally requires `mkvtoolnix` (for `mkvpropedit`) on top of `ffmpeg` + `exiftool`, so the test fixture corpus can author MKV files with tags at multiple target levels (see [testing-strategy.md](../testing-strategy.md) §3). This dependency is **test-only** — the runtime image does not need it (Phase 1–2 do not write MKV tags; Phase 3 writeback tooling is a separate future ADR).
