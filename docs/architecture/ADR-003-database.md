# ADR-003: Database — SQLite (modernc.org/sqlite) + FTS5

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Holodex needs a database that:
- Handles full-text search on titles (requirement F4.1)
- Supports filter queries across 50k+ records at ≤ 300ms p95 (requirement F4.9)
- Manages junction tables for people and tags
- Supports concurrent reads (web server) with single-writer inserts/updates (scanner)
- Ships inside a single Docker container with no sidecars
- Has a clear upgrade path to a more capable database if needed

Candidates evaluated: SQLite + FTS5, PostgreSQL, DuckDB.

## Decision

**SQLite** with the **`modernc.org/sqlite`** pure-Go driver, running in **WAL mode**, with **FTS5** for full-text search.

## Rationale

- **Zero-config**: Single `.db` file on the persistent volume — no connection strings, no sidecar container, no service discovery.
- **Pure-Go driver**: `modernc.org/sqlite` requires no CGo, keeping the Docker build simple (no C toolchain in the build stage) and the final image lean.
- **WAL mode**: Write-Ahead Logging allows concurrent reads from the HTTP server while the scanner writes, which matches the exact concurrency pattern of this application.
- **FTS5**: SQLite's built-in full-text search extension handles title substring and keyword search (requirement F4.1) without an external search service.
- **Backup simplicity**: A database backup is a file copy. `cp holodex.db holodex.db.bak` is sufficient; no `pg_dump` or snapshot coordination required.
- **Phase 3 compatibility**: Tag graph DAG traversal (requirement F15.2) is expressible as a recursive CTE, which SQLite fully supports.
- **Upgrade path**: If query complexity or concurrent-writer needs ever outgrow SQLite, the schema can be migrated to PostgreSQL with minimal changes — the Go `database/sql` interface is the same.

## Rejected Alternatives

| Option | Reason rejected |
|--------|-----------------|
| PostgreSQL | Requires sidecar container and persistent volume management; significant operational overhead for a single-user personal tool |
| DuckDB | Optimized for analytical/OLAP workloads; concurrent write semantics less suited to scanner + web server OLTP pattern |

## Consequences

- Database file path configured via `DATABASE_PATH` env var (default: `/data/holodex.db`).
- WAL mode enabled on first connection: `PRAGMA journal_mode=WAL`.
- Performance pragmas set on connection open: `PRAGMA synchronous=NORMAL`, `PRAGMA cache_size=-64000` (64 MB), `PRAGMA temp_store=MEMORY`.
- Schema migrations managed via embedded SQL files using a lightweight migrator (e.g., `golang-migrate/migrate` or hand-rolled versioned migrations).
- FTS5 virtual table `videos_fts` mirrors the `title` column of the `videos` table, kept in sync via triggers.
- A single write connection pool (max 1 writer) and a separate read connection pool (max N readers) avoids SQLITE_BUSY under load.
