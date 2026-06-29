-- F30 (ADR-048): durable, bounded-concurrency batch-writeback queue. One row per
-- enqueued "write to file" action (one file). The queue survives restart: on boot
-- the worker recovers rows left 'running' by a crash (their file is intact per the
-- ADR-041 copy→write→rename model) and re-runs them. Default concurrency is 1
-- (WRITEBACK_CONCURRENCY) so bulk writes don't thrash the filesystem.
--
-- payload is JSON: the curated, write-enabled canonical fields ({field,values,source}).
-- Tag names are re-resolved from the container at processing time (never trusted
-- from the payload — security condition C2). status ∈ pending|running|failed|done.
-- enqueued_at/updated_at are RFC3339 UTC.
CREATE TABLE writeback_queue (
    id          INTEGER PRIMARY KEY,
    video_id    INTEGER NOT NULL REFERENCES videos(id),
    payload     TEXT    NOT NULL,
    status      TEXT    NOT NULL DEFAULT 'pending',
    attempts    INTEGER NOT NULL DEFAULT 0,
    error       TEXT    NOT NULL DEFAULT '',
    enqueued_at TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);

CREATE INDEX idx_writeback_queue_status ON writeback_queue(status, enqueued_at);
