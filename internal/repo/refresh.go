package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrDeleted is returned when a row exists but is soft-deleted (F31, ADR-047).
// Callers distinguish it from ErrNotFound to answer 409 (the item is in Trash)
// versus 404 (no such item) — a soft-deleted row must never be re-read or
// reactivated by a refresh (ADR-037 #26 guard).
var ErrDeleted = errors.New("deleted")

// RefreshTarget resolves a video id to its canonical file path for a forced
// per-item re-extract (F31, ADR-047). It distinguishes a missing row
// (ErrNotFound) from a soft-deleted one (ErrDeleted) so the refresh endpoint can
// answer 404 vs 409 without a second query. It deliberately does no
// change-detection — the caller always re-extracts.
func (r *Repo) RefreshTarget(ctx context.Context, id int64) (string, error) {
	var (
		path    string
		deleted bool
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT file_path, deleted_at IS NOT NULL FROM videos WHERE id = ?`, id).
		Scan(&path, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("refresh target: %w", err)
	}
	if deleted {
		return "", ErrDeleted
	}
	return path, nil
}
