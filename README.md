# Holodex

A Docker-based personal video media server. Browse, search, and navigate your local/networked
video library with the file's own embedded metadata as the source of truth.

> Design docs: [`docs/architecture`](docs/architecture/README.md) (19 ADRs) · [`docs/specs`](docs/specs) (phase specs) · [`docs/testing-strategy.md`](docs/testing-strategy.md)

## Status

Phase 1 (MVP) scaffold. The architecture is locked; implementation is in progress.

## Quick start (development)

Requires Go 1.23+ and Node 20+.

```bash
# 1. Resolve Go modules (first time)
go mod tidy

# 2. Run the backend API (defaults: DATA_PATH=./data, PORT=7800)
go run ./cmd/holodex
#    -> http://localhost:7800/healthz

# 3. Run the frontend dev server (separate terminal)
cd web && npm install && npm run dev
#    -> http://localhost:5173 (proxies /api -> :7800)
```

Set `MEDIA_PATH` to a folder of `.mp4`/`.mkv` files to enable scanning:

```bash
MEDIA_PATH=/path/to/videos go run ./cmd/holodex
```

## Docker

```bash
docker compose up --build
#    -> http://localhost:7800
```

Configure via environment (see [`holodex.yaml.example`](holodex.yaml.example) and ADR-014):
`MEDIA_PATH` (read-only library), `DATA_PATH` (read-write index/thumbnails/config), `PORT`.

## Testing

```bash
go test ./...                 # unit (fast, no external binaries)
go test -tags integration ./... # integration (needs ffmpeg, exiftool, mkvtoolnix)
./testdata/gen.sh             # (re)generate the media fixture corpus
```

See [docs/testing-strategy.md](docs/testing-strategy.md).

## Layout

```
cmd/holodex          entrypoint (config, db, migrations, server, graceful shutdown)
internal/
  config             configuration loader (CLI > env > yaml > defaults) — ADR-014
  model              core domain types
  metadata           extraction (exiftool/ffprobe) + resolution classifier — ADR-004/010/012
  scanner            filesystem scan + incremental change detection — ADR-011/018
  db                 SQLite open + embedded migrations — ADR-003/016
  cache              cache interface (in-process / noop) — ADR-008
  api                chi router, REST handlers, health — ADR-006/019
web/                 SvelteKit SPA (Tailwind) — ADR-002
testdata/            fixture generator + golden files
docs/                architecture, specs, testing strategy
```
