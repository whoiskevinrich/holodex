package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"holodex/internal/model"
)

// Person images (F24, ADR-037): the metadata index for per-person images whose
// bytes live on disk. Core roles (headshot/banner/poster) are single-slot — a new
// image for a filled core role replaces the old one (delete + insert in a tx) so
// the partial unique index never trips. The 'extra' gallery is capped at
// GalleryCap, enforced transactionally. Writes take writeMu like the rest of the
// write path; reads are unlocked (WAL).

// GalleryCap bounds the per-person 'extra' gallery (ADR-037 F24). Beyond it,
// InsertPersonImage refuses an extra with ErrGalleryFull.
const GalleryCap = 20

// ErrGalleryFull is returned when an 'extra' insert would exceed GalleryCap.
var ErrGalleryFull = errors.New("gallery is full")

// InsertPersonImage stores one image row for a person and returns its id. For a
// core role it replaces any existing image in that slot (delete + insert) so the
// single-slot invariant holds; for 'extra' it appends, refusing past GalleryCap.
// All in one transaction under the write lock. A single INSERT + LastInsertId() is
// safe here: this table has no ON CONFLICT upsert (the warning in repo.go applies
// only to upserts whose LastInsertId can reflect an unrelated row), and the insert
// is the last statement on the tx connection.
//
// The caller (handler/enricher) writes the bytes to disk under the returned id
// AFTER this succeeds, since the id is the on-disk filename.
func (r *Repo) InsertPersonImage(ctx context.Context, personID int64, role, source, provider, externalID string, w, h, byteSize int) (int64, error) {
	if !model.ValidPersonImageRole(role) {
		return 0, fmt.Errorf("invalid person image role %q", role)
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	sortOrder := 0
	if model.CorePersonImageRole(role) {
		// Replace the core slot: remove any existing row (its on-disk file is cleaned
		// up by the handler, which lists-then-deletes; an orphaned file is harmless).
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM person_images WHERE person_id = ? AND role = ?`, personID, role); err != nil {
			return 0, fmt.Errorf("replace core slot: %w", err)
		}
	} else {
		// Gallery: enforce the cap and append after the current max sort_order.
		var count int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM person_images WHERE person_id = ? AND role = ?`,
			personID, model.PersonImageExtra).Scan(&count); err != nil {
			return 0, fmt.Errorf("count gallery: %w", err)
		}
		if count >= GalleryCap {
			return 0, ErrGalleryFull
		}
		var maxOrder sql.NullInt64
		if err := tx.QueryRowContext(ctx,
			`SELECT MAX(sort_order) FROM person_images WHERE person_id = ? AND role = ?`,
			personID, model.PersonImageExtra).Scan(&maxOrder); err != nil {
			return 0, fmt.Errorf("max gallery order: %w", err)
		}
		if maxOrder.Valid {
			sortOrder = int(maxOrder.Int64) + 1
		}
	}

	now := time.Now().UTC().Format(timeLayout)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO person_images
			(person_id, role, source, provider, external_id, width, height, byte_size, sort_order, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		personID, role, source, provider, externalID, w, h, byteSize, sortOrder, now)
	if err != nil {
		return 0, fmt.Errorf("insert person image: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("person image id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit person image: %w", err)
	}
	return id, nil
}

// GetPersonImage returns one image row scoped to its person, or ErrNotFound. The
// person scope means a mismatched (image, person) pair can't read another person's
// image (mirrors DeletePersonAlias's scoping).
func (r *Repo) GetPersonImage(ctx context.Context, personID, imageID int64) (model.PersonImage, error) {
	var (
		pi      model.PersonImage
		created string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, role, source, width, height, sort_order, created_at
		FROM person_images WHERE id = ? AND person_id = ?`, imageID, personID).
		Scan(&pi.ID, &pi.Role, &pi.Source, &pi.Width, &pi.Height, &pi.SortOrder, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return model.PersonImage{}, ErrNotFound
	}
	if err != nil {
		return model.PersonImage{}, fmt.Errorf("get person image: %w", err)
	}
	pi.Version = pi.ID
	pi.CreatedAt, _ = time.Parse(timeLayout, created)
	return pi, nil
}

// CorePersonImage returns the single image filling a core role for a person, or
// ErrNotFound when the slot is empty. role must be a core role.
func (r *Repo) CorePersonImage(ctx context.Context, personID int64, role string) (model.PersonImage, error) {
	if !model.CorePersonImageRole(role) {
		return model.PersonImage{}, fmt.Errorf("not a core role %q", role)
	}
	var (
		pi      model.PersonImage
		created string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, role, source, width, height, sort_order, created_at
		FROM person_images WHERE person_id = ? AND role = ?`, personID, role).
		Scan(&pi.ID, &pi.Role, &pi.Source, &pi.Width, &pi.Height, &pi.SortOrder, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return model.PersonImage{}, ErrNotFound
	}
	if err != nil {
		return model.PersonImage{}, fmt.Errorf("core person image: %w", err)
	}
	pi.Version = pi.ID
	pi.CreatedAt, _ = time.Parse(timeLayout, created)
	return pi, nil
}

// ListPersonImages returns all of a person's images ordered by role then gallery
// sort_order then id, so the serialized set is stable. Always non-nil on success.
func (r *Repo) ListPersonImages(ctx context.Context, personID int64) ([]model.PersonImage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, role, source, width, height, sort_order, created_at
		FROM person_images WHERE person_id = ?
		ORDER BY role, sort_order, id`, personID)
	if err != nil {
		return nil, fmt.Errorf("list person images: %w", err)
	}
	defer rows.Close()
	out := []model.PersonImage{}
	for rows.Next() {
		var (
			pi      model.PersonImage
			created string
		)
		if err := rows.Scan(&pi.ID, &pi.Role, &pi.Source, &pi.Width, &pi.Height, &pi.SortOrder, &created); err != nil {
			return nil, err
		}
		pi.Version = pi.ID
		pi.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, pi)
	}
	return out, rows.Err()
}

// CountGalleryImages returns the number of 'extra' gallery images for a person
// (the 20-cap counterpart, exposed for the handler's pre-check / tests).
func (r *Repo) CountGalleryImages(ctx context.Context, personID int64) (int, error) {
	var n int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM person_images WHERE person_id = ? AND role = ?`,
		personID, model.PersonImageExtra).Scan(&n); err != nil {
		return 0, fmt.Errorf("count gallery images: %w", err)
	}
	return n, nil
}

// DeletePersonImage removes one image row scoped to its person, returning the
// removed row's role/id (so the handler can also delete the on-disk file).
// ErrNotFound when no such image belongs to the person.
func (r *Repo) DeletePersonImage(ctx context.Context, personID, imageID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM person_images WHERE id = ? AND person_id = ?`, imageID, personID)
	if err != nil {
		return fmt.Errorf("delete person image: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReorderGallery sets the sort_order of a person's gallery images to match the
// given id order (index = sort_order), in one transaction under the write lock.
// Ids not belonging to the person (or not 'extra') are ignored. Reordering is
// purely cosmetic, so unknown ids are skipped rather than erroring the whole call.
func (r *Repo) ReorderGallery(ctx context.Context, personID int64, orderedIDs []int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	for i, id := range orderedIDs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE person_images SET sort_order = ?
			WHERE id = ? AND person_id = ? AND role = ?`,
			i, id, personID, model.PersonImageExtra); err != nil {
			return fmt.Errorf("reorder gallery: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder: %w", err)
	}
	return nil
}

// PersonImageSet builds the person-detail image read model (ADR-037 F24): which
// core roles are filled (with version) and the ordered gallery. One query, grouped
// in Go. Roles is always non-nil.
func (r *Repo) PersonImageSet(ctx context.Context, personID int64) (model.PersonImageSet, error) {
	imgs, err := r.ListPersonImages(ctx, personID)
	if err != nil {
		return model.PersonImageSet{}, err
	}
	set := model.PersonImageSet{
		Roles:   map[string]model.PersonImageSlot{},
		Gallery: []model.PersonImage{},
	}
	for _, pi := range imgs {
		if model.CorePersonImageRole(pi.Role) {
			set.Roles[pi.Role] = model.PersonImageSlot{Present: true, Version: pi.Version}
		} else {
			set.Gallery = append(set.Gallery, pi)
		}
	}
	return set, nil
}
