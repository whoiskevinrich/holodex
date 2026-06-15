-- F22.6b (ADR-033, extends ADR-028): job_runs gains a free-text `detail` so
-- non-scan jobs (enrichment now; preview/writeback later) can describe themselves
-- — e.g. "tmdb → person #18 (5 fields)" — since the scan count columns are 0 for
-- them. Honours the no-secrets invariant: provider name + entity id + counts only,
-- never a filesystem path, env value, or token.
ALTER TABLE job_runs ADD COLUMN detail TEXT NOT NULL DEFAULT '';
