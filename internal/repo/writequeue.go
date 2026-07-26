package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
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
