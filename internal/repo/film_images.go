package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"holodex/internal/model"
)

// Film images (F56/HOLODEX-280, ADR-086): the metadata index for a film's poster/
// thumb roles, shaped after studio_images (F51, ADR-079) with one deliberate
// difference carried from migration 0043's schema comment: UNIQUE is (film_id, role,
// source), not (film_id, role) alone, so an uploaded image and a provider-sourced
// image for the same role can coexist as distinct rows. Every function here is
// therefore scoped by source, not just role — this ticket only ever passes
// model.FilmImageSourceUpload; a provider-sourced writer/reader is HOLODEX-284's
// scope. Writes take writeMu like the rest of the write path; reads are unlocked
// (WAL).

// FilmImage is one stored image for a film.
type FilmImage struct {
	ID         int64
	FilmID     int64
	Role       string
	Source     string // 'upload' | 'provider:<name>'
	Provider   string
	ExternalID string
	Width      int
	Height     int
	ByteSize   int
	CreatedAt  time.Time
}

// FilmImageInsert is the payload for ReplaceFilmImage.
type FilmImageInsert struct {
	FilmID                  int64
	Role, Source            string
	Provider, ExternalID    string
	Width, Height, ByteSize int
}

// GetFilmImage returns the film's image row for one (role, source) slot, or
// ErrNotFound when that slot is empty.
func (r *Repo) GetFilmImage(ctx context.Context, filmID int64, role, source string) (FilmImage, error) {
	if !model.ValidFilmImageRole(role) {
		return FilmImage{}, fmt.Errorf("invalid film image role %q", role)
	}
	var (
		fi      FilmImage
		created string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, film_id, role, source, provider, external_id, width, height, byte_size, created_at
		FROM film_images WHERE film_id = ? AND role = ? AND source = ?`, filmID, role, source).
		Scan(&fi.ID, &fi.FilmID, &fi.Role, &fi.Source, &fi.Provider, &fi.ExternalID, &fi.Width, &fi.Height, &fi.ByteSize, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return FilmImage{}, ErrNotFound
	}
	if err != nil {
		return FilmImage{}, fmt.Errorf("get film image: %w", err)
	}
	fi.CreatedAt, _ = time.Parse(timeLayout, created)
	return fi, nil
}

// ReplaceFilmImage makes the film's image for `in.Role`+`in.Source` be exactly `in`
// and returns the new server-assigned id. Delete + insert in one transaction so the
// UNIQUE(film_id, role, source) slot invariant holds and the id always advances on a
// replace. The caller stores the bytes at the returned id and removes any superseded
// file (it holds the prior id from GetFilmImage).
func (r *Repo) ReplaceFilmImage(ctx context.Context, in FilmImageInsert) (int64, error) {
	if !model.ValidFilmImageRole(in.Role) {
		return 0, fmt.Errorf("invalid film image role %q", in.Role)
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM film_images WHERE film_id = ? AND role = ? AND source = ?`, in.FilmID, in.Role, in.Source); err != nil {
		return 0, fmt.Errorf("clear film image slot: %w", err)
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO film_images (film_id, role, source, provider, external_id, width, height, byte_size, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.FilmID, in.Role, in.Source, in.Provider, in.ExternalID, in.Width, in.Height, in.ByteSize,
		time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("insert film image: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit film image: %w", err)
	}
	return id, nil
}

// DeleteFilmImage removes the film's image row for one (role, source) slot.
// Idempotent — deleting an already-empty slot is a no-op success.
func (r *Repo) DeleteFilmImage(ctx context.Context, filmID int64, role, source string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM film_images WHERE film_id = ? AND role = ? AND source = ?`, filmID, role, source); err != nil {
		return fmt.Errorf("delete film image: %w", err)
	}
	return nil
}

// filmImageVersions returns filmID -> {role: rowID} for every film in ids, in ONE
// batch query — the list/detail read path's way of filling Film.ImageVersions
// without an N-way per-film lookup (mirrors studioImageVersions). Scoped to
// FilmImageSourceUpload: this ticket's owner-upload path is the only writer today,
// and a future provider-sourced row's display priority is HOLODEX-284's decision to
// make, not this function's to guess at.
func (r *Repo) filmImageVersions(ctx context.Context, ids []int64) (map[int64]map[string]int64, error) {
	out := make(map[int64]map[string]int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT film_id, role, id FROM film_images
		WHERE source = ? AND film_id IN (`+placeholders(len(ids))+`)`,
		append([]any{model.FilmImageSourceUpload}, toAnySlice(ids)...)...)
	if err != nil {
		return nil, fmt.Errorf("film image versions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var filmID, rowID int64
		var role string
		if err := rows.Scan(&filmID, &role, &rowID); err != nil {
			return nil, err
		}
		if out[filmID] == nil {
			out[filmID] = map[string]int64{}
		}
		out[filmID][role] = rowID
	}
	return out, rows.Err()
}

// attachFilmImages fills ImageVersions on each film from film_images in one batch
// query (F56/HOLODEX-280, ADR-086 — mirrors attachStudioImages).
func (r *Repo) attachFilmImages(ctx context.Context, films []model.Film) error {
	if len(films) == 0 {
		return nil
	}
	ids := make([]int64, len(films))
	for i, f := range films {
		ids[i] = f.ID
	}
	versions, err := r.filmImageVersions(ctx, ids)
	if err != nil {
		return err
	}
	for i := range films {
		films[i].ImageVersions = versions[films[i].ID]
	}
	return nil
}
