DROP INDEX IF EXISTS idx_job_runs_entity;
ALTER TABLE job_runs DROP COLUMN batch_id;
ALTER TABLE job_runs DROP COLUMN entity_id;
ALTER TABLE job_runs DROP COLUMN entity_type;
