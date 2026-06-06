# ADR-014: Configuration Strategy & Data Directory Layout

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Holodex has accumulated ~20 environment variables plus structured YAML config (`metadata-mappings.yaml`, future plugin config). It also writes several kinds of generated/persistent data (database, thumbnails, previews, images, backups). Without a single configuration model and a defined on-disk layout, deployment and volume mounts will churn across phases.

## Decision

### Configuration precedence (highest wins)
```
CLI flags  >  environment variables  >  config file (holodex.yaml)  >  built-in defaults
```
- **Environment variables** are the primary mechanism for deployment/ops knobs (paths, ports, worker counts, feature switches) — natural for Docker.
- **YAML config files** hold richer structured config that doesn't fit env vars cleanly: `metadata-mappings.yaml` (ADR-013) and future plugin definitions (Phase 3). The base file is `holodex.yaml`.
- A single config loader resolves all sources at startup into one immutable `Config` struct passed through the app. Reloadable subsets (e.g. mappings, ADR-013 F20.10) are re-read on demand.

### Data directory layout
A single root data directory, `DATA_PATH` (default `/data`), contains everything persistent:

```
/data/
  holodex.db            # SQLite database (+ -wal, -shm)
  config/
    holodex.yaml        # base config (optional; env can supply everything)
    metadata-mappings.yaml
  thumbnails/           # generated + extracted cover art (ADR-009)
  previews/             # Phase 3 preview trailers
  images/
    people/             # Phase 3 person images
    tags/               # Phase 3 tag images
  backups/              # Phase 3 writeback backups
```

### Key path variables
| Env var | Default | Description |
|---------|---------|-------------|
| `DATA_PATH` | `/data` | Root persistent directory (single volume mount) |
| `DATABASE_PATH` | `${DATA_PATH}/holodex.db` | DB file (override allowed) |
| `MEDIA_PATH` | (required) | Read-only media library root |

## Consequences

- `docker-compose.yml` mounts exactly two volumes: `MEDIA_PATH` (read-only) and `DATA_PATH` (read-write).
- This ADR **supersedes** the looser `DATABASE_PATH/thumbnails`, `DATABASE_PATH/previews`, `DATABASE_PATH/images`, `DATABASE_PATH/backups` references in ADR-008/ADR-009/Phase specs: those subdirectories now hang off `DATA_PATH`, not the DB file path.
- The media volume is mounted read-only in Phase 1–2 (no writeback); Phase 3 writeback (ADR-008 TBD) requires read-write and is gated by `WRITEBACK_ENABLED`.
- All config keys are documented in one place (README + `holodex.yaml.example`).
