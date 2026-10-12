package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Film soft-delete (F78, HOLODEX-562): films ride the same deleted_at axis videos do
// (F24; migration 0062). A trashed film keeps every row it owns — scene links, cast
// roles, images, aliases, enrichment, decisions — so RestoreFilm is lossless; it only
// disappears from reads. Every film read carries liveFilm by hand (there is no film
// filter builder to hang it on); the functions in this file and the identity
// collision probes are the only reads that see a trashed film.

// liveFilm is the visibility predicate for a films row aliased `f`.
const liveFilm = `f.deleted_at IS NULL`

// SoftDeleteFilm moves a film to Trash. Idempotent like SoftDelete: a second call
// keeps the original deleted_at. ErrNotFound only when the id doesn't exist.
func (r *Repo) SoftDeleteFilm(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	res, err := r.db.ExecContext(ctx,
		`UPDATE films SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`,
		time.Now().UTC().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("soft delete film: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return nil
	}
	return r.FilmExistsAny(ctx, id) // already trashed (nil) or absent (ErrNotFound)
}

// RestoreFilm takes a film out of Trash. ErrNotFound when it isn't trashed.
func (r *Repo) RestoreFilm(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	res, err := r.db.ExecContext(ctx,
		`UPDATE films SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("restore film: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// FilmTrash lists trashed films, newest deletion first, titled "Name (Year)".
func (r *Repo) FilmTrash(ctx context.Context) ([]TrashItem, error) {
	return r.queryFilmTrash(ctx, `WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC, id DESC`)
}

// ExpiredSoftDeletedFilms returns trashed films whose deleted_at is at or before
// cutoff — the grace-period purge candidates (same lexical-time rule as videos).
func (r *Repo) ExpiredSoftDeletedFilms(ctx context.Context, cutoff time.Time) ([]TrashItem, error) {
	return r.queryFilmTrash(ctx,
		`WHERE deleted_at IS NOT NULL AND deleted_at <= ? ORDER BY deleted_at ASC, id ASC`,
		cutoff.UTC().Format(timeLayout))
}

func (r *Repo) queryFilmTrash(ctx context.Context, tail string, args ...any) ([]TrashItem, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, year, deleted_at FROM films `+tail, args...)
	if err != nil {
		return nil, fmt.Errorf("film trash: %w", err)
	}
	defer rows.Close()
	var out []TrashItem
	for rows.Next() {
		var (
			it     TrashItem
			year   sql.NullInt64
			delStr string
		)
		if err := rows.Scan(&it.ID, &it.Title, &year, &delStr); err != nil {
			return nil, err
		}
		it.Kind = TrashKindFilm
		it.Title = FilmLabel(it.Title, int(year.Int64))
		it.DeletedAt, _ = time.Parse(timeLayout, delStr)
		out = append(out, it)
	}
	return out, rows.Err()
}

// FilmLabel is the "Name (Year)" form a film is named by in Trash; the bare name when
// the film has no year.
func FilmLabel(name string, year int) string {
	if year > 0 {
		return fmt.Sprintf("%s (%d)", name, year)
	}
	return name
}

// FilmExistsAny reports whether a film row exists regardless of Trash — SoftDeleteFilm's
// already-trashed vs. absent check.
func (r *Repo) FilmExistsAny(ctx context.Context, id int64) error {
	var x int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM films WHERE id = ?`, id).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// HardDeleteFilm removes a film row permanently (spec P0-3/P0-7). The FK cascades take
// film_videos (the videos themselves are untouched, RD2), roles and image rows; the
// films AFTER DELETE triggers take FTS, external ids, aliases, keep-separate, the
// film's FK-less per-entity rows and every video's standing choice of this film as a
// source (0062, RD10). Image files on disk are the purger's. A no-op is not an error.
func (r *Repo) HardDeleteFilm(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx, `DELETE FROM films WHERE id = ?`, id); err != nil {
		return fmt.Errorf("hard delete film: %w", err)
	}
	return nil
}

// FilmVisible reports whether a film exists and is not in Trash — the cheap guard the
// film-image handler uses so a trashed film's poster 404s like its page.
func (r *Repo) FilmVisible(ctx context.Context, id int64) (bool, error) {
	var x int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM films f WHERE f.id = ? AND `+liveFilm, id).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("film visible: %w", err)
	}
	return true, nil
}

// TrashedFilm returns the collision shape for a film in Trash, or ok=false when the
// film is live or absent. A create/rename/year edit whose occupant is trashed answers
// with a restore offer instead of a merge or "view that film" (spec RD6).
func (r *Repo) TrashedFilm(ctx context.Context, id int64) (FilmYearCollision, bool, error) {
	var (
		c    FilmYearCollision
		year sql.NullInt64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, year FROM films WHERE id = ? AND deleted_at IS NOT NULL`, id).Scan(&c.FilmID, &c.FilmName, &year)
	if errors.Is(err, sql.ErrNoRows) {
		return FilmYearCollision{}, false, nil
	}
	if err != nil {
		return FilmYearCollision{}, false, fmt.Errorf("trashed film: %w", err)
	}
	c.Year = int(year.Int64)
	c.Trashed = true
	return c, true, nil
}
