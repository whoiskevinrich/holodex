# ADR-016: Database Migrations — golang-migrate with Embedded Versioned SQL

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

The schema evolves across every phase: Phase 1 adds `video_metadata`; Phase 3 adds person aliases, person metadata, tag aliases, the tag graph, and enrichment tables. A self-hosted app that auto-updates its Docker image must apply schema changes safely on startup without manual user intervention, and must never lose user-supplied data (Phase 3 enrichment/mappings).

## Decision

Use **`golang-migrate/migrate`** with **embedded, versioned, forward-and-backward SQL migrations**, applied automatically on startup.

- Migrations live in `internal/db/migrations/` as paired files: `NNNN_description.up.sql` / `NNNN_description.down.sql`.
- They are embedded into the binary via `go:embed` (no external files needed at runtime).
- On startup the app runs all pending `up` migrations against the SQLite database before serving traffic.
- The current schema version is tracked in `schema_migrations` (managed by the library).

## Rationale

- **Auto-apply on startup** matches the self-hosted, image-pull update model — users don't run manual migration commands.
- **Embedded** keeps the single-binary deployment intact (ADR-001/ADR-007).
- **Versioned up/down** gives deterministic, reviewable schema evolution and a rollback path during development.
- **SQLite-supported.** `golang-migrate` has a native SQLite driver compatible with the `modernc.org/sqlite` choice (ADR-003).

## Consequences

- A failed migration aborts startup with a clear error rather than serving against a half-migrated schema.
- Because the index is **rebuildable from source files** (metadata is the source of truth), destructive schema changes to *indexed* data are low-risk — a re-scan repopulates. **User-authored data** (mappings live in YAML, not the DB; Phase 3 enrichment/aliases live in the DB) must be preserved across migrations — down-migrations and data-preserving up-migrations are written with that in mind.
- A `--migrate-only` CLI flag allows running migrations without starting the server (useful for diagnostics).
- FTS5 virtual tables and their sync triggers (ADR-017) are created via migrations like any other schema object.
