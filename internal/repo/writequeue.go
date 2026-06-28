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
)

// WritebackJob is one durable enqueued batch-write (F30, ADR-048).
type WritebackJob struct {
	ID       int64
	VideoID  int64
	Payload  string
	Attempts int
}

// EnqueueWriteback durably appends one batch-write job and returns its id (F30.4d).
func (r *Repo) EnqueueWriteback(ctx context.Context, videoID int64, payload string) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	now := time.Now().UTC().Format(timeLayout)
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO writeback_queue (video_id, payload, status, enqueued_at, updated_at)
		VALUES (?, ?, 'pending', ?, ?)`, videoID, payload, now, now)
	if err != nil {
		return 0, fmt.Errorf("enqueue writeback: %w", err)
	}
	return res.LastInsertId()
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
		SELECT id, video_id, payload, attempts FROM writeback_queue
		WHERE status = 'pending' ORDER BY enqueued_at, id LIMIT 1`).
		Scan(&job.ID, &job.VideoID, &job.Payload, &job.Attempts)
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
