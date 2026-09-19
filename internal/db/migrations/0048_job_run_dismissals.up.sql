-- HOLODEX-416 (ADR-100 D1/D2): an owner's "handled" verdict on one failed job run.
-- A sibling table rather than a column because job_runs is the audit record and is
-- never UPDATEd (ADR-071/091 posture) — a dismissal is triage state, not a fact about
-- the run. Structural sibling of enrichment_dismissals (0024), but with a real foreign
-- key instead of triggers: there is exactly one parent, and foreign_keys(ON) already
-- carries 0035–0037. ON DELETE CASCADE is the whole retention story — both sweep
-- sites (RecordJobRun's prune, PruneJobRuns at startup) stay a one-line DELETE and a
-- dismissal never outlives its run. dismissed_at is RFC3339 UTC like every other
-- timestamp column.
CREATE TABLE job_run_dismissals (
    job_run_id   INTEGER PRIMARY KEY REFERENCES job_runs(id) ON DELETE CASCADE,
    dismissed_at TEXT    NOT NULL
);
