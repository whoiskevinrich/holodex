package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Hover-montage state (F83, migration 0066). Kept in its own table beside the
// still thumbnail's videos.thumbnail_state so neither lifecycle can reach the
// other (media-ingest.md, "Hover montages").

// MontageRow is one video's montage outcome. No row means never attempted.
type MontageRow struct {
	VideoID   int64
	State     string // "generated" | "failed"
	Error     string
	UpdatedAt string
}

// MontageCandidate is the minimum a montage job needs about a video.
type MontageCandidate struct {
	ID          int64
	FilePath    string
	DurationSec int
}

// SetMontageState upserts a video's montage outcome.
func (r *Repo) SetMontageState(ctx context.Context, id int64, state, errText string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO video_montages (video_id, state, error, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(video_id) DO UPDATE SET state = excluded.state, error = excluded.error,
		 updated_at = excluded.updated_at`,
		id, state, errText, time.Now().UTC().Format(timeLayout)); err != nil {
		return fmt.Errorf("set montage state: %w", err)
	}
	return nil
}

// MontageState returns a video's montage row, ok=false when never attempted.
func (r *Repo) MontageState(ctx context.Context, id int64) (MontageRow, bool, error) {
	var m MontageRow
	err := r.db.QueryRowContext(ctx,
		`SELECT video_id, state, error, updated_at FROM video_montages WHERE video_id = ?`, id).
		Scan(&m.VideoID, &m.State, &m.Error, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return MontageRow{}, false, nil
	}
	if err != nil {
		return MontageRow{}, false, fmt.Errorf("montage state: %w", err)
	}
	return m, true, nil
}

// MontageAttempted returns the subset of ids that already have a montage row.
func (r *Repo) MontageAttempted(ctx context.Context, ids []int64) (map[int64]bool, error) {
	out := make(map[int64]bool, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT video_id FROM video_montages WHERE video_id IN (?`+strings.Repeat(",?", len(ids)-1)+`)`,
		args...)
	if err != nil {
		return nil, fmt.Errorf("montage attempted: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("montage attempted: %w", err)
		}
		out[id] = true
	}
	return out, rows.Err()
}

// MontageBackfillCandidates lists live videos at least minDurationSec long that
// have never had a montage attempted, newest first (automatic mode only).
func (r *Repo) MontageBackfillCandidates(ctx context.Context, minDurationSec, limit int) ([]MontageCandidate, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT v.id, v.file_path, v.duration_sec FROM videos v
		 WHERE v.active = 1 AND v.deleted_at IS NULL AND v.duration_sec >= ?
		   AND NOT EXISTS (SELECT 1 FROM video_montages m WHERE m.video_id = v.id)
		 ORDER BY v.indexed_at DESC, v.id DESC LIMIT ?`, minDurationSec, limit)
	if err != nil {
		return nil, fmt.Errorf("montage backfill candidates: %w", err)
	}
	defer rows.Close()
	var out []MontageCandidate
	for rows.Next() {
		var c MontageCandidate
		if err := rows.Scan(&c.ID, &c.FilePath, &c.DurationSec); err != nil {
			return nil, fmt.Errorf("montage backfill candidates: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MontageCandidateByID resolves one live video for a montage job, ok=false when
// it is gone or soft-deleted.
func (r *Repo) MontageCandidateByID(ctx context.Context, id int64) (MontageCandidate, bool, error) {
	var c MontageCandidate
	err := r.db.QueryRowContext(ctx,
		`SELECT id, file_path, duration_sec FROM videos
		 WHERE id = ? AND active = 1 AND deleted_at IS NULL`, id).
		Scan(&c.ID, &c.FilePath, &c.DurationSec)
	if errors.Is(err, sql.ErrNoRows) {
		return MontageCandidate{}, false, nil
	}
	if err != nil {
		return MontageCandidate{}, false, fmt.Errorf("montage candidate: %w", err)
	}
	return c, true, nil
}
