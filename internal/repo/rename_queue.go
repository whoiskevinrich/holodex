package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"holodex/internal/model"
)

// ErrRenameRaced refuses step 3 of a rename when the video's row no longer names either the old
// or the new path: something else moved it in between.
var ErrRenameRaced = errors.New("the video's path changed during the rename")

// RenameEnqueue is one video's rename job in an apply.
type RenameEnqueue struct {
	VideoID int64
	Payload string
}

// EnqueueRenames queues one kind='rename' row per video under a shared batch id (F86). A video's
// prior failed rename row is dropped, as EnqueueMaintenance does, so Retry can't run it twice;
// a video that already has a pending or running rename is skipped. It returns how many rows were
// queued.
func (r *Repo) EnqueueRenames(ctx context.Context, jobs []RenameEnqueue, batchID string) (int, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	now := time.Now().UTC().Format(timeLayout)
	n := 0
	for _, j := range jobs {
		var busy bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM writeback_queue
			WHERE video_id = ? AND kind = 'rename' AND status IN ('pending','running'))`, j.VideoID).Scan(&busy); err != nil {
			return 0, fmt.Errorf("enqueue rename: %w", err)
		}
		if busy {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM writeback_queue
			WHERE video_id = ? AND kind = 'rename' AND status = 'failed'`, j.VideoID); err != nil {
			return 0, fmt.Errorf("enqueue rename: clear prior failure: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO writeback_queue
			(video_id, kind, payload, status, batch_id, enqueued_at, updated_at)
			VALUES (?, 'rename', ?, 'pending', ?, ?, ?)`, j.VideoID, j.Payload, batchID, now, now); err != nil {
			return 0, fmt.Errorf("enqueue rename: %w", err)
		}
		n++
	}
	return n, tx.Commit()
}

// RequeueWriteback puts a running job back to pending behind every job queued so far, so the
// claim order runs it after the job it waits for (F86 batches). The claim orders by enqueued_at
// (whole seconds) then id, so the row is re-inserted to take a new, higher id; attempts carry over.
func (r *Repo) RequeueWriteback(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	now := time.Now().UTC().Format(timeLayout)
	if _, err := tx.ExecContext(ctx, `INSERT INTO writeback_queue
		(video_id, kind, payload, status, attempts, batch_id, enqueued_at, updated_at)
		SELECT video_id, kind, payload, 'pending', attempts, batch_id, ?, ? FROM writeback_queue WHERE id = ?`,
		now, now, id); err != nil {
		return fmt.Errorf("requeue %d: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM writeback_queue WHERE id = ?`, id); err != nil {
		return fmt.Errorf("requeue %d: %w", id, err)
	}
	return tx.Commit()
}

// BatchRenameJobs lists a batch's rename rows still queued or running.
func (r *Repo) BatchRenameJobs(ctx context.Context, batchID string) ([]WritebackJob, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, video_id, kind, payload, attempts, batch_id
		FROM writeback_queue WHERE kind = 'rename' AND batch_id = ? AND status IN ('pending','running')`, batchID)
	if err != nil {
		return nil, fmt.Errorf("batch rename jobs: %w", err)
	}
	defer rows.Close()
	var out []WritebackJob
	for rows.Next() {
		var j WritebackJob
		if err := rows.Scan(&j.ID, &j.VideoID, &j.Kind, &j.Payload, &j.Attempts, &j.BatchID); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// FinishRenameBatch completes an apply's summary run (P0-15) once no job of the batch is queued:
// renamed and failed are counted from each video's latest run in the batch, so a retried failure
// counts once. The apply-time counts and refused-by-reason detail are left as written. Calling it
// again recounts, so a later Retry in the batch keeps the summary current.
func (r *Repo) FinishRenameBatch(ctx context.Context, batchID string) error {
	if batchID == "" {
		return nil
	}
	// Mid-batch this is the only query, and it reads the small queue table without the lock.
	var queued bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM writeback_queue
		WHERE kind = 'rename' AND batch_id = ? AND status IN ('pending','running'))`, batchID).Scan(&queued); err != nil {
		return fmt.Errorf("rename batch queued: %w", err)
	}
	if queued {
		return nil
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	now := time.Now().UTC().Format(timeLayout)
	_, err := r.db.ExecContext(ctx, `
		WITH latest AS (SELECT updated AS u, errors AS e FROM job_runs
		                 WHERE id IN (SELECT max(id) FROM job_runs
		                               WHERE kind = 'rename' AND batch_id = ?1 AND entity_id <> 0
		                               GROUP BY entity_id))
		UPDATE job_runs
		   SET updated     = (SELECT count(*) FROM latest WHERE u > 0),
		       errors      = (SELECT count(*) FROM latest WHERE e > 0),
		       status      = CASE WHEN EXISTS(SELECT 1 FROM latest WHERE e > 0) THEN ?2 ELSE ?3 END,
		       finished_at = ?4,
		       duration_ms = CAST((julianday(?4) - julianday(started_at)) * 86400000 AS INTEGER)
		 WHERE kind = 'rename' AND batch_id = ?1 AND entity_id = 0`,
		batchID, model.JobStatusErr, model.JobStatusOK, now)
	if err != nil {
		return fmt.Errorf("rename batch summary: %w", err)
	}
	return nil
}

// HasRenameJob reports whether a video has a rename row in any state (boot recovery's "a job
// remains").
func (r *Repo) HasRenameJob(ctx context.Context, videoID int64) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM writeback_queue
		WHERE video_id = ? AND kind = 'rename' AND status IN ('pending','running'))`, videoID).Scan(&ok)
	return ok, err
}

// FailRenameJobs marks a video's queued rename rows failed with msg (boot recovery, R7).
func (r *Repo) FailRenameJobs(ctx context.Context, videoID int64, msg string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.db.ExecContext(ctx, `UPDATE writeback_queue SET status = 'failed', error = ?, updated_at = ?
		WHERE video_id = ? AND kind = 'rename' AND status IN ('pending','running')`,
		msg, time.Now().UTC().Format(timeLayout), videoID)
	return err
}

// NameLocked reports whether a video's name is locked (P0-13).
func (r *Repo) NameLocked(ctx context.Context, videoID int64) (bool, error) {
	var locked bool
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE((SELECT locked FROM video_file_names WHERE video_id = ?), 0)`,
		videoID).Scan(&locked)
	return locked, err
}

// RetryFailedRename and DismissFailedRename are the rename lane's Retry and Dismiss.
func (r *Repo) RetryFailedRename(ctx context.Context, videoID int64) (int64, error) {
	return r.retryFailed(ctx, videoID, WritebackKindRename)
}

func (r *Repo) DismissFailedRename(ctx context.Context, videoID int64) (int64, error) {
	return r.dismissFailed(ctx, videoID, WritebackKindRename)
}

// FailedRename is a video's failed rename job: owner reads only, the error may carry paths.
func (r *Repo) FailedRename(ctx context.Context, videoID int64) (string, bool, error) {
	var msg string
	err := r.db.QueryRowContext(ctx, `SELECT error FROM writeback_queue
		WHERE video_id = ? AND kind = 'rename' AND status = 'failed' ORDER BY id DESC LIMIT 1`, videoID).Scan(&msg)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return msg, err == nil, err
}

// InsertPendingRename writes step 1 of a rename: the journal row, before any filesystem call.
func (r *Repo) InsertPendingRename(ctx context.Context, fr FileRename) (int64, error) {
	tv, err := json.Marshal(fr.TokenValues)
	if err != nil {
		return 0, err
	}
	var undoOf any
	if fr.UndoOf != 0 {
		undoOf = fr.UndoOf
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `INSERT INTO file_renames
		(video_id, old_name, new_name, via_name, template, token_values, trigger, undo_of, state, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)`, fr.VideoID, fr.OldName, fr.NewName, fr.ViaName,
		fr.Template, string(tv), fr.Trigger, undoOf, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("insert pending rename: %w", err)
	}
	return res.LastInsertId()
}

// SetRenameVia journals the half-way name of a two-step rename.
func (r *Repo) SetRenameVia(ctx context.Context, id int64, via string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.db.ExecContext(ctx, `UPDATE file_renames SET via_name = ? WHERE id = ? AND state = 'pending'`, via, id)
	return err
}

// StepAsideRename journals a swap's step-aside: the file now sits at its via name, so the row
// gives up its old path in the same transaction, letting the partner take that name (P0-6).
func (r *Repo) StepAsideRename(ctx context.Context, fr FileRename, dir, via string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	if _, err := tx.ExecContext(ctx, `UPDATE file_renames SET via_name = ? WHERE id = ? AND state = 'pending'`, via, fr.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE videos SET file_path = ? WHERE id = ? AND file_path = ?`,
		filepath.Join(dir, via), fr.VideoID, filepath.Join(dir, fr.OldName)); err != nil {
		return err
	}
	return tx.Commit()
}

// AbandonRename drops a pending journal row once its file is back at the old name, pointing the
// row back at the old path if a step-aside had moved it.
func (r *Repo) AbandonRename(ctx context.Context, fr FileRename, dir string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	if fr.ViaName != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE videos SET file_path = ? WHERE id = ? AND file_path = ?`,
			filepath.Join(dir, fr.OldName), fr.VideoID, filepath.Join(dir, fr.ViaName)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM file_renames WHERE id = ? AND state = 'pending'`, fr.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// PendingRenameTarget returns the video whose pending rename writes path's name in path's folder:
// a walked file there belongs to that video and is never ingested as new (F86).
func (r *Repo) PendingRenameTarget(ctx context.Context, path string) (int64, bool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT fr.video_id, v.file_path FROM file_renames fr
		JOIN videos v ON v.id = fr.video_id WHERE fr.state = 'pending' AND fr.new_name = ?`, filepath.Base(path))
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var p string
		if err := rows.Scan(&id, &p); err != nil {
			return 0, false, err
		}
		if filepath.Dir(p) == filepath.Dir(path) {
			return id, true, nil
		}
	}
	return 0, false, rows.Err()
}

// NoteRepointed reconciles the rename journal after a scan repointed a video from oldPath to
// newPath (F79). A file that took the name a pending rename was writing finishes that rename (the
// F79 net, P0-9); a file at its via name is left for the job. Otherwise a new base name for a file
// Holodex last named is a hand edit, and the name locks (P0-13); a move to another folder with the
// same name is not.
func (r *Repo) NoteRepointed(ctx context.Context, id int64, oldPath, newPath string) error {
	pending, err := r.PendingRename(ctx, id)
	if err != nil {
		return err
	}
	newBase := filepath.Base(newPath)
	if pending != nil {
		if newBase != pending.NewName {
			return nil // at the via name, or elsewhere: boot recovery and the job settle it
		}
		r.writeMu.Lock()
		defer r.writeMu.Unlock()
		_, err := r.db.ExecContext(ctx, `UPDATE file_renames SET state = 'done', via_name = '' WHERE id = ?`, pending.ID)
		return err
	}
	if newBase == filepath.Base(oldPath) {
		return nil
	}
	last, err := r.LatestDoneRename(ctx, id)
	if err != nil || last == nil || last.NewName != filepath.Base(oldPath) {
		return err
	}
	if locked, err := r.NameLocked(ctx, id); err != nil || locked {
		return err
	}
	return r.SetNameLock(ctx, id, true, LockAuto, time.Now())
}

// DeletePendingRename drops a pending journal row whose rename did not happen.
func (r *Repo) DeletePendingRename(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.db.ExecContext(ctx, `DELETE FROM file_renames WHERE id = ? AND state = 'pending'`, id)
	return err
}

// PendingRenames lists every pending journal row, oldest first (boot recovery).
func (r *Repo) PendingRenames(ctx context.Context) ([]FileRename, error) {
	return r.queryRenames(ctx, `SELECT `+renameCols+` FROM file_renames WHERE state = 'pending' ORDER BY id`)
}

// PendingRename returns a video's pending journal row, or nil.
func (r *Repo) PendingRename(ctx context.Context, videoID int64) (*FileRename, error) {
	fr, err := scanRename(r.db.QueryRowContext(ctx, `SELECT `+renameCols+` FROM file_renames
		WHERE video_id = ? AND state = 'pending' ORDER BY id DESC LIMIT 1`, videoID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &fr, nil
}

// CompleteRename is step 3: in one transaction, point the video at its new name (and nothing
// else: a rename changes no bytes, so indexed_at and the file layer stay), mark the journal row
// done, and, for an undo, lock the name. The path update accepts the new path too, because a scan
// may already have repointed the row.
func (r *Repo) CompleteRename(ctx context.Context, fr FileRename, dir string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	oldPath, newPath, viaPath := filepath.Join(dir, fr.OldName), filepath.Join(dir, fr.NewName), filepath.Join(dir, fr.ViaName)
	res, err := tx.ExecContext(ctx, `UPDATE videos SET file_path = ? WHERE id = ? AND file_path IN (?, ?, ?)`,
		newPath, fr.VideoID, oldPath, newPath, viaPath)
	if err != nil {
		return fmt.Errorf("complete rename %d: %w", fr.ID, err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrRenameRaced
	}
	if _, err := tx.ExecContext(ctx, `UPDATE file_renames SET state = 'done', via_name = '' WHERE id = ?`, fr.ID); err != nil {
		return fmt.Errorf("complete rename %d: %w", fr.ID, err)
	}
	if fr.Trigger == RenameTriggerUndo {
		if err := setNameLock(ctx, tx, fr.VideoID, true, LockUndo, time.Now()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
