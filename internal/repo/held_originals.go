package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Held-original journal states (F77, migration 0063, docs/architecture/file-maintenance.md).
const (
	HeldSwapping   = "swapping"
	HeldHeld       = "held"
	HeldRestoring  = "restoring"
	HeldDiscarding = "discarding"
	HeldPathSuffix = ".holodex-orig"
)

// HeldOriginal is one video's pre-maintenance original, set aside beside the file.
type HeldOriginal struct {
	VideoID   int64
	HeldPath  string
	Op        string
	State     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// GetHeldOriginal returns the video's journal row, or nil when it has none.
func (r *Repo) GetHeldOriginal(ctx context.Context, videoID int64) (*HeldOriginal, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT video_id, held_path, op, state, created_at, expires_at
		FROM held_originals WHERE video_id = ?`, videoID)
	h, err := scanHeld(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return h, err
}

// ListHeldOriginals returns every journal row in state, oldest first (boot recovery reads
// the non-held states; the expiry pass reads held).
func (r *Repo) ListHeldOriginals(ctx context.Context, state string) ([]HeldOriginal, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT video_id, held_path, op, state, created_at, expires_at
		FROM held_originals WHERE state = ? ORDER BY created_at, video_id`, state)
	if err != nil {
		return nil, fmt.Errorf("list held originals: %w", err)
	}
	defer rows.Close()
	var out []HeldOriginal
	for rows.Next() {
		h, err := scanHeld(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *h)
	}
	return out, rows.Err()
}

// BeginSwap journals a swap before its first rename: the row exists in state swapping, so a
// crash between the two renames is finished or undone at boot. It fails if the video already
// has a row (a second operation while one is held never journals; it renames once).
func (r *Repo) BeginSwap(ctx context.Context, videoID int64, heldPath, op string, now, expires time.Time) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO held_originals (video_id, held_path, op, state, created_at, expires_at)
		VALUES (?, ?, ?, 'swapping', ?, ?)`, videoID, heldPath, op,
		now.UTC().Format(timeLayout), expires.UTC().Format(timeLayout))
	if err != nil {
		return fmt.Errorf("begin swap: %w", err)
	}
	return nil
}

// SetHeldState moves a journal row to state.
func (r *Repo) SetHeldState(ctx context.Context, videoID int64, state string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `UPDATE held_originals SET state = ? WHERE video_id = ?`, state, videoID)
	if err != nil {
		return fmt.Errorf("set held state: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("set held state: video %d has no held original", videoID)
	}
	return nil
}

// DeleteHeldOriginal removes the journal row; a missing row is a no-op.
func (r *Repo) DeleteHeldOriginal(ctx context.Context, videoID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx, `DELETE FROM held_originals WHERE video_id = ?`, videoID); err != nil {
		return fmt.Errorf("delete held original: %w", err)
	}
	return nil
}

func scanHeld(s rowScanner) (*HeldOriginal, error) {
	var h HeldOriginal
	var created, expires string
	if err := s.Scan(&h.VideoID, &h.HeldPath, &h.Op, &h.State, &created, &expires); err != nil {
		return nil, err
	}
	h.CreatedAt, _ = time.Parse(timeLayout, created)
	h.ExpiresAt, _ = time.Parse(timeLayout, expires)
	return &h, nil
}
