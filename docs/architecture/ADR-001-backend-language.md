# ADR-001: Backend Language — Go

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Holodex needs a backend that:
- Serves a REST/JSON API for the web UI and MCP server
- Runs a background file scanner with filesystem watch events
- Calls `ffprobe` as a subprocess to extract stream metadata
- Ships as a minimal Docker image

Three candidates were evaluated: Go, Python/FastAPI, TypeScript/Node.js (Bun).

## Decision

**Go** is the chosen backend language.

## Rationale

- **Single binary**: The entire backend compiles to one statically-linked executable, producing a Docker image under 30 MB (scratch or distroless base).
- **Concurrency model**: Goroutines and channels map naturally to the concurrent workloads — filesystem watcher, periodic scan loop, HTTP server, and (Phase 2) thumbnail generator — without the GIL or multiprocessing overhead of Python.
- **Performance**: Go's net/http is fast enough to meet the ≤ 300ms search p95 target with SQLite; no framework overhead required.
- **MCP compatibility**: `mark3labs/mcp-go` provides a production-ready MCP server SDK.
- **Operational simplicity**: No runtime to install in the container; `ffprobe` is the only external binary dependency for Phase 1.

## Rejected Alternatives

| Option | Reason rejected |
|--------|-----------------|
| Python/FastAPI | Heavier image; GIL complicates true parallelism between scanner and server; slower cold start |
| TypeScript/Bun | JS less ergonomic for binary/subprocess work; Bun ecosystem less mature for this use case |

## Consequences

- Frontend can be any language/framework (separate container or served as static files from the Go binary).
- `ffprobe` must be present in the Docker image as an external binary (already required for metadata extraction).
- ORM choice: use `database/sql` with a lightweight driver (see ADR-003 for database selection).
- MCP server will be embedded in the same Go process as the web server, toggled by `MCP_ENABLED` env var.
