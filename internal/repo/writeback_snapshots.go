package repo

import (
	"context"
	"fmt"
	"time"
)

// WritebackSnapshot is one field's pre-write value, captured before a
// writeback job overwrote it (F48.9, ADR-067) — the unit of rollback.
type WritebackSnapshot struct {
	VideoID    int64
	FieldKey   string
	PriorValue string
	WrittenAt  time.Time
}

// InsertWritebackSnapshots records the pre-write value of every field in one
// write operation, in a single transaction under the write lock (ADR-067:
// "same transaction as the job record"). Called immediately before the write
// itself — a snapshot describes what the file looked like *before* this
// write, independent of whether the write later succeeds. A nil/empty
// priorValues map is a no-op success.
func (r *Repo) InsertWritebackSnapshots(ctx context.Context, videoID int64, batchID string, priorValues map[string]string) error {
	if len(priorValues) == 0 {
		return nil
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	// One prepared statement for every field row — this runs synchronously on
	// every writeback write under the global writeMu, so it's worth sparing the
	// repeated prepare/bind/exec/close a field-at-a-time ExecContext would cost.
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO file_writeback_snapshots (video_id, batch_id, field_key, prior_value, written_at)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare writeback snapshot insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(timeLayout)
	for field, prior := range priorValues {
		if _, err := stmt.ExecContext(ctx, videoID, batchID, field, prior, now); err != nil {
			return fmt.Errorf("insert writeback snapshot: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit writeback snapshots: %w", err)
	}
	return nil
}

// SnapshotsForBatch returns every snapshot row from one write operation,
// ordered by video then field — the input to Revert (F48.9b): one inverse
// write per video, restoring each field to its prior_value. Returns an empty
// slice (not an error) when the batch id is unknown; the caller decides
// whether that's a 404.
func (r *Repo) SnapshotsForBatch(ctx context.Context, batchID string) ([]WritebackSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT video_id, field_key, prior_value, written_at
		FROM file_writeback_snapshots
		WHERE batch_id = ?
		ORDER BY video_id, field_key`, batchID)
	if err != nil {
		return nil, fmt.Errorf("snapshots for batch: %w", err)
	}
	defer rows.Close()

	var out []WritebackSnapshot
	for rows.Next() {
		var s WritebackSnapshot
		var writtenStr string
		if err := rows.Scan(&s.VideoID, &s.FieldKey, &s.PriorValue, &writtenStr); err != nil {
			return nil, err
		}
		s.WrittenAt, _ = time.Parse(timeLayout, writtenStr)
		out = append(out, s)
	}
	return out, rows.Err()
}

// SnapshotExistsForVideo reports whether one video already has a snapshot
// within one batch — snapshotBeforeWrite's own-job idempotency check
// (F48.9a), scoped narrower than SnapshotsForBatch. A batch id can now span
// several videos' jobs (merge propagation, F48.8, migration 0027): checking
// existence by batch id alone would make video B's job see video A's
// already-taken snapshot and wrongly skip taking its own, so the check must
// be scoped to (batch_id, video_id) instead.
func (r *Repo) SnapshotExistsForVideo(ctx context.Context, batchID string, videoID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM file_writeback_snapshots WHERE batch_id = ? AND video_id = ?)`,
		batchID, videoID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("snapshot exists for video: %w", err)
	}
	return exists, nil
}
