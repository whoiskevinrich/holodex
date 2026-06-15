-- F21.3 (ADR-028): durable history of background job runs. Today every completed
-- scan pass records one row; `kind` is extensible to thumbnail backfill and the
-- Phase-3 jobs (enrichment / preview / writeback) without a schema change.
--
-- Retention is enforced by the application (30 days), pruned on insert and at
-- startup — not by the schema — so the table needs no triggers. started_at is
-- stored RFC3339 UTC (matching every other timestamp column), so the index also
-- serves the lexicographic retention cutoff.
CREATE TABLE job_runs (
    id            INTEGER PRIMARY KEY,
    kind          TEXT    NOT NULL,
    trigger       TEXT    NOT NULL,
    status        TEXT    NOT NULL,
    started_at    TEXT    NOT NULL,
    finished_at   TEXT    NOT NULL,
    duration_ms   INTEGER NOT NULL DEFAULT 0,
    seen          INTEGER NOT NULL DEFAULT 0,
    added         INTEGER NOT NULL DEFAULT 0,
    updated       INTEGER NOT NULL DEFAULT 0,
    removed       INTEGER NOT NULL DEFAULT 0,
    skipped       INTEGER NOT NULL DEFAULT 0,
    errors        INTEGER NOT NULL DEFAULT 0,
    error_message TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX idx_job_runs_started_at ON job_runs (started_at DESC);
