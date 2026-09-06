package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"holodex/internal/model"
)

// Writeback queue statuses (F30, ADR-048).
const (
	WritebackPending = "pending"
	WritebackRunning = "running"
	WritebackFailed  = "failed"
	// WritebackDone is not a stored value — FinishWriteback deletes the row on
	// success. GetWritebackJobStatus synthesizes it for an absent row.
	WritebackDone = "done"
)

// WritebackJob is one durable enqueued batch-write (F30, ADR-048).
type WritebackJob struct {
	ID       int64
	VideoID  int64
	Payload  string
	Attempts int
	// BatchID is the caller-supplied snapshot batch id (F48.8, ADR-067,
	// migration 0027) — empty for every pre-F48.8 caller, in which case
	// snapshotBeforeWrite derives one from the job's own id instead.
	BatchID string
}

// EnqueueWriteback durably appends one batch-write job and returns its id
// (F30.4d). batchID is normally "" (the job's own id becomes its snapshot
// batch); a caller propagating one multi-video operation (a Person/Studio
// merge, F48.8) supplies the same batchID across several videos' jobs so
// they share one snapshot batch and one Revert.
func (r *Repo) EnqueueWriteback(ctx context.Context, videoID int64, payload, batchID string) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	now := time.Now().UTC().Format(timeLayout)
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO writeback_queue (video_id, payload, batch_id, status, enqueued_at, updated_at)
		VALUES (?, ?, ?, 'pending', ?, ?)`, videoID, payload, batchID, now, now)
	if err != nil {
		return 0, fmt.Errorf("enqueue writeback: %w", err)
	}
	return res.LastInsertId()
}

// WritebackJobInsert is one row for EnqueueWritebackBatch.
type WritebackJobInsert struct {
	VideoID int64
	Payload string
	BatchID string
}

// EnqueueWritebackBatch durably appends several batch-write jobs in a single
// transaction and returns their ids in the same order as jobs (F48.8): a
// multi-video operation (Person/Studio merge propagation) enqueues one job
// per affected video without paying one writeMu acquisition and one commit
// per video on the owner-facing request path — mirrors
// InsertWritebackSnapshots's single prepared-statement transaction for the
// same reason. A nil/empty jobs is a no-op success.
func (r *Repo) EnqueueWritebackBatch(ctx context.Context, jobs []WritebackJobInsert) ([]int64, error) {
	if len(jobs) == 0 {
		return nil, nil
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO writeback_queue (video_id, payload, batch_id, status, enqueued_at, updated_at)
		VALUES (?, ?, ?, 'pending', ?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("prepare writeback batch insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(timeLayout)
	ids := make([]int64, len(jobs))
	for i, j := range jobs {
		res, err := stmt.ExecContext(ctx, j.VideoID, j.Payload, j.BatchID, now, now)
		if err != nil {
			return nil, fmt.Errorf("enqueue writeback batch: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("enqueue writeback batch: %w", err)
		}
		ids[i] = id
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit writeback batch: %w", err)
	}
	return ids, nil
}

// ClaimNextWriteback atomically picks the oldest pending job and marks it running,
// incrementing attempts. Returns (nil, nil) when the queue is empty. The
// select-then-update runs under the write lock so two workers can't claim the same
// row (F30.4c).
func (r *Repo) ClaimNextWriteback(ctx context.Context) (*WritebackJob, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	var job WritebackJob
	err := r.db.QueryRowContext(ctx, `
		SELECT id, video_id, payload, attempts, batch_id FROM writeback_queue
		WHERE status = 'pending' ORDER BY enqueued_at, id LIMIT 1`).
		Scan(&job.ID, &job.VideoID, &job.Payload, &job.Attempts, &job.BatchID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim writeback: %w", err)
	}
	job.Attempts++
	if _, err := r.db.ExecContext(ctx, `
		UPDATE writeback_queue SET status = 'running', attempts = ?, updated_at = ?
		WHERE id = ?`, job.Attempts, time.Now().UTC().Format(timeLayout), job.ID); err != nil {
		return nil, fmt.Errorf("mark writeback running: %w", err)
	}
	return &job, nil
}

// FinishWriteback completes a job. On success the row is deleted (job_runs holds the
// durable history; file_writebacks holds the per-field audit). On failure the row is
// marked 'failed' with the error so it is inspectable (F30.4f).
func (r *Repo) FinishWriteback(ctx context.Context, id int64, ok bool, errMsg string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if ok {
		if _, err := r.db.ExecContext(ctx, `DELETE FROM writeback_queue WHERE id = ?`, id); err != nil {
			return fmt.Errorf("finish writeback: %w", err)
		}
		return nil
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE writeback_queue SET status = 'failed', error = ?, updated_at = ?
		WHERE id = ?`, errMsg, time.Now().UTC().Format(timeLayout), id); err != nil {
		return fmt.Errorf("fail writeback: %w", err)
	}
	return nil
}

// GetWritebackJobStatus reports one job's terminal-or-not state, so a caller can
// poll an enqueued write to completion (ADR-073).
//
// Because FinishWriteback deletes the row on success, an absent row reads as
// WritebackDone. That conflates "succeeded" with "never existed" / "already
// swept", which holds only for the intended caller: a poll started from the job
// id the enqueue just handed back. A failed job keeps its row, so a real failure
// is never mistaken for success.
func (r *Repo) GetWritebackJobStatus(ctx context.Context, id int64) (status, errMsg string, err error) {
	var msg sql.NullString
	err = r.db.QueryRowContext(ctx, `
		SELECT status, error FROM writeback_queue WHERE id = ?`, id).Scan(&status, &msg)
	if errors.Is(err, sql.ErrNoRows) {
		return WritebackDone, "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("get writeback job %d: %w", id, err)
	}
	return status, msg.String, nil
}

// statusCounts runs a "SELECT status, COUNT(*) ... GROUP BY status"-shaped
// query and returns each status's count keyed by the status string —
// shared by GetWritebackBatchStatus's two identically-shaped tallies (one
// over writeback_queue, one over job_runs).
func statusCounts(ctx context.Context, db *sql.DB, query string, args ...any) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

// GetWritebackBatchStatus aggregates a shared batchID's progress across every
// job that was enqueued under it (HOLODEX-239, ADR-077 D3): pending/running
// come from writeback_queue's still-live rows, done/failed from job_runs
// (kind=writeback) — a row's absence from writeback_queue combined with a
// job_runs hit is GetWritebackJobStatus's own "row is gone = done" rule,
// applied across every job sharing the batch instead of one job id. Lets a
// tag-scoped sync's dialog poll one endpoint instead of fanning out to N.
func (r *Repo) GetWritebackBatchStatus(ctx context.Context, batchID string) (pending, running, done, failed int, err error) {
	queue, err := statusCounts(ctx, r.db,
		`SELECT status, COUNT(*) FROM writeback_queue WHERE batch_id = ? GROUP BY status`, batchID)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("get writeback batch status (queue): %w", err)
	}
	runs, err := statusCounts(ctx, r.db,
		`SELECT status, COUNT(*) FROM job_runs WHERE kind = ? AND batch_id = ? GROUP BY status`,
		model.JobKindWriteback, batchID)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("get writeback batch status (job_runs): %w", err)
	}
	return queue[WritebackPending], queue[WritebackRunning], runs[model.JobStatusOK], runs[model.JobStatusErr], nil
}

// RecoverRunningWritebacks resets any 'running' rows back to 'pending' on boot —
// they were interrupted by a crash/shutdown; the original file is intact (ADR-041
// copy→write→rename), so the job is safe to re-run (F30.4e). Returns the count.
func (r *Repo) RecoverRunningWritebacks(ctx context.Context) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `
		UPDATE writeback_queue SET status = 'pending', updated_at = ?
		WHERE status = 'running'`, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("recover writebacks: %w", err)
	}
	return res.RowsAffected()
}

// PendingWritebackCount returns the number of jobs waiting or in-flight — the queue
// depth surfaced for observability and the SPA's "queued (position N)" hint.
func (r *Repo) PendingWritebackCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM writeback_queue WHERE status IN ('pending','running')`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("pending writeback count: %w", err)
	}
	return n, nil
}

// VideoWritebackStatus is one video's writeback queue state (ADR-091, HOLODEX-323, spec
// R2.1): whether a job is pending/running, and whether one has failed. Derived from
// writeback_queue by video_id rather than a job id held client-side, so it survives
// reload, another tab, and a server restart. The zero value ({false, false, ""}) is
// exactly "nothing to report" (R2.2/ADR-091 D3) — FinishWriteback deletes the row on
// success, so a video with no row here reads identically whether the write succeeded,
// was swept, or never happened.
//
// Error is the raw message from writeback.WriteBatch, and every failure path in
// internal/writeback/writeback.go embeds absolute filesystem paths (copy/rename errors
// wrap os errors carrying both paths; the exiftool/mkvpropedit/ffmpeg branches append
// raw tool stderr containing the .holodex-tmp path). Spec R2.1a: callers MUST NOT
// serialize this field to an unauthenticated/non-owner request — GetMedia is the one
// call site and redacts it inline for that reason, mirroring
// redactFileMetadataForVisitor's existing posture on the same response.
type VideoWritebackStatus struct {
	Pending bool   `json:"pending"`
	Failed  bool   `json:"failed"`
	Error   string `json:"error,omitempty"`
}

// GetVideoWritebackStatus reports a video's writeback status (spec R2.1). Enqueuing a
// new write for the video (writebackMedia) clears any prior failed row first (spec
// R3.5), so pending and failed should not normally coexist for one video in practice —
// but this aggregates defensively over every row rather than assuming that invariant,
// taking the most recently updated failure's message if more than one somehow exists.
func (r *Repo) GetVideoWritebackStatus(ctx context.Context, videoID int64) (VideoWritebackStatus, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT status, error FROM writeback_queue
		WHERE video_id = ? ORDER BY updated_at ASC`, videoID)
	if err != nil {
		return VideoWritebackStatus{}, fmt.Errorf("get video writeback status: %w", err)
	}
	defer rows.Close()

	var out VideoWritebackStatus
	for rows.Next() {
		var status, errMsg string
		if err := rows.Scan(&status, &errMsg); err != nil {
			return VideoWritebackStatus{}, fmt.Errorf("get video writeback status: %w", err)
		}
		switch status {
		case WritebackPending, WritebackRunning:
			out.Pending = true
		case WritebackFailed:
			// Ascending order means the last failed row scanned is the most recent —
			// unconditional overwrite is simplest way to keep only that one's message.
			out.Failed = true
			out.Error = errMsg
		}
	}
	if err := rows.Err(); err != nil {
		return VideoWritebackStatus{}, fmt.Errorf("get video writeback status: %w", err)
	}
	return out, nil
}

// RetryFailedWriteback resets a video's failed writeback row(s) back to pending (spec
// R3.3) so the worker's normal ClaimNextWriteback picks them up again — nothing else
// does this today: ClaimNextWriteback only ever claims 'pending' rows, and
// RecoverRunningWritebacks resets only 'running' ones left by a crash. Returns the
// number of rows reset; 0 is a safe no-op (nothing was failed for this video), not an
// error, mirroring GetWritebackJobStatus's "absent row" posture.
func (r *Repo) RetryFailedWriteback(ctx context.Context, videoID int64) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `
		UPDATE writeback_queue SET status = 'pending', error = '', updated_at = ?
		WHERE video_id = ? AND status = 'failed'`,
		time.Now().UTC().Format(timeLayout), videoID)
	if err != nil {
		return 0, fmt.Errorf("retry writeback: %w", err)
	}
	return res.RowsAffected()
}

// DismissFailedWriteback deletes a video's failed writeback row(s) without retrying
// (spec R3.4/RD2). job_runs already holds the permanent audit record for the failure
// (kind=writeback, status=err), so writeback_queue stays a work queue rather than a
// log — this mirrors FinishWriteback's own delete-on-success. Returns the number of
// rows removed; 0 is a safe no-op.
func (r *Repo) DismissFailedWriteback(ctx context.Context, videoID int64) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM writeback_queue WHERE video_id = ? AND status = 'failed'`, videoID)
	if err != nil {
		return 0, fmt.Errorf("dismiss writeback: %w", err)
	}
	return res.RowsAffected()
}
