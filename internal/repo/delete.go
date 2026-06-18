package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// TrashItem is a soft-deleted video as shown in the owner's Trash view and as
// consumed by the purge sweep (F24, ADR-037). FilePath is included for the purge
// job (disk removal) — the Trash API response drops it (the detail page 404s).
type TrashItem struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	FilePath  string    `json:"-"`
	DeletedAt time.Time `json:"deleted_at"`
}

// SoftDelete marks a video soft-deleted (F24.1): it disappears from every read
// surface immediately but its row and file survive the grace period. Idempotent —
// a second delete leaves the original deleted_at untouched and still succeeds.
// Returns ErrNotFound only when the id doesn't exist at all.
func (r *Repo) SoftDelete(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	res, err := r.db.ExecContext(ctx,
		`UPDATE videos SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`,
		time.Now().UTC().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return nil // newly soft-deleted
	}
	// Zero rows updated: the row is either already soft-deleted (idempotent success)
	// or absent (ErrNotFound). Distinguish by existence.
	var x int
	if err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM videos WHERE id = ?`, id).Scan(&x); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("soft delete existence: %w", err)
	}
	return nil // already soft-deleted
}

// Restore clears deleted_at so the item returns to every view (F24.6). Returns
// ErrNotFound when the id isn't currently soft-deleted (nothing to restore).
func (r *Repo) Restore(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	res, err := r.db.ExecContext(ctx,
		`UPDATE videos SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Trash lists the soft-deleted items (newest deletion first) for the owner's Trash
// view (F24.7). FilePath is populated but the handler omits it from the response.
func (r *Repo) Trash(ctx context.Context) ([]TrashItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, title, file_path, deleted_at FROM videos
		WHERE deleted_at IS NOT NULL
		ORDER BY deleted_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("trash list: %w", err)
	}
	defer rows.Close()
	return scanTrash(rows)
}

// ExpiredSoftDeleted returns soft-deleted items whose deleted_at is at or before
// cutoff — the grace-period purge candidates (F24.4). deleted_at is stored as a
// fixed-width RFC3339 UTC string, so the lexical comparison is chronological.
func (r *Repo) ExpiredSoftDeleted(ctx context.Context, cutoff time.Time) ([]TrashItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, title, file_path, deleted_at FROM videos
		WHERE deleted_at IS NOT NULL AND deleted_at <= ?
		ORDER BY deleted_at ASC, id ASC`, cutoff.UTC().Format(timeLayout))
	if err != nil {
		return nil, fmt.Errorf("expired soft-deleted: %w", err)
	}
	defer rows.Close()
	return scanTrash(rows)
}

func scanTrash(rows *sql.Rows) ([]TrashItem, error) {
	var out []TrashItem
	for rows.Next() {
		var (
			it     TrashItem
			delStr string
		)
		if err := rows.Scan(&it.ID, &it.Title, &it.FilePath, &delStr); err != nil {
			return nil, err
		}
		it.DeletedAt, _ = time.Parse(timeLayout, delStr)
		out = append(out, it)
	}
	return out, rows.Err()
}

// PurgePath returns a video's file path regardless of soft-delete state — the one
// read that deliberately ignores deleted_at, for the purge job and purge-now
// (F24.5). Returns ErrNotFound if the row is already gone.
func (r *Repo) PurgePath(ctx context.Context, id int64) (string, error) {
	var path string
	err := r.db.QueryRowContext(ctx,
		`SELECT file_path FROM videos WHERE id = ?`, id).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return path, err
}

// HardDelete removes a video row permanently (F24.4/F24.5). The ON DELETE CASCADE
// foreign keys (video_people/video_tags/video_metadata) and the videos_ad FTS
// trigger clean up the junctions and search index automatically. A no-op (0 rows)
// is not an error — the desired end state is "the row is gone".
func (r *Repo) HardDelete(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if _, err := r.db.ExecContext(ctx, `DELETE FROM videos WHERE id = ?`, id); err != nil {
		return fmt.Errorf("hard delete: %w", err)
	}
	return nil
}

// VideoVisible reports whether a video exists and is not soft-deleted — the cheap
// guard the thumbnail-serving handler uses to 404 a soft-deleted cover during the
// grace window without resolving the full row (F24/ADR-037 §4).
func (r *Repo) VideoVisible(ctx context.Context, id int64) (bool, error) {
	var x int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM videos WHERE id = ? AND deleted_at IS NULL`, id).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("video visible: %w", err)
	}
	return true, nil
}
