-- HOLODEX-207 (ADR-070, extends ADR-028): job_runs learns *what* a run touched.
--
-- Two of the nine kinds record one run per item (writeback: one per video;
-- enrich: one per provider x entity), but the row never said which entity, so
-- "what happened to this file?" was unanswerable and the only entity trace was
-- `detail` -- a display string, and only writeback put a filename in it.
--
-- Attribution is a polymorphic (entity_type, entity_id) pair reusing the
-- existing model.EnrichEntity* vocabulary ('video'/'person'/'studio'), with
-- DELIBERATELY NO FOREIGN KEY: job_runs is an audit table, and a run that says
-- "enriched person #42" is most interesting after person 42 is gone. An FK would
-- either block the delete or, with ON DELETE CASCADE, silently rewrite history.
-- The cost is that entity_id can dangle; the read side renders '#<id>' when the
-- entity no longer resolves. Library-wide kinds (scan, the backfills) leave the
-- pair at its zero value, which reads as "not attributed" -- no sentinel needed.
--
-- batch_id mirrors the writeback snapshot batch (migration 0027) onto the run
-- itself. It was previously recoverable only by regexing the free-text detail
-- line, which silently failed for merge-propagation batches: mergeBatchID names
-- those 'merge-person-N-M' and the frontend pattern required digits, so Revert
-- never appeared for the exact multi-video case 0027 was built to make
-- revertible.
--
-- All three are additive with zero-value defaults, so every existing row stays
-- valid; rows written before this migration have no attribution and cannot get
-- any (the information was never recorded), so entity-filtered views are correct
-- going forward only.
ALTER TABLE job_runs ADD COLUMN entity_type TEXT    NOT NULL DEFAULT '';
ALTER TABLE job_runs ADD COLUMN entity_id   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE job_runs ADD COLUMN batch_id    TEXT    NOT NULL DEFAULT '';

-- Trailing started_at DESC matches the list ORDER BY, so an entity-filtered page
-- is served from the index without a sort.
CREATE INDEX idx_job_runs_entity ON job_runs (entity_type, entity_id, started_at DESC);
