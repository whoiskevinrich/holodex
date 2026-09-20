-- Dismissals are triage state only; dropping them resurfaces every error run still
-- inside the retention window in the digest. job_runs itself is untouched.
DROP TABLE job_run_dismissals;
