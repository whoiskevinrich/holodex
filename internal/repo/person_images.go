package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"holodex/internal/model"
)

// Person images (F25, ADR-038): the metadata index for per-person images whose
// bytes live on disk. Core roles (headshot/banner/poster) are single-slot — a new
// image for a filled core role replaces the old one (delete + insert in a tx) so
// the partial unique index never trips. The 'extra' gallery is capped at
// GalleryCap, enforced transactionally. Writes take writeMu like the rest of the
// write path; reads are unlocked (WAL).

// GalleryCap is the built-in default per-person 'extra' gallery bound (ADR-038
// F25), used when no PERSON_GALLERY_MAX override is configured. Beyond the
// effective cap, InsertPersonImage refuses an extra with ErrGalleryFull unless the
// insert is marked OverCap.
const GalleryCap = 20

// ErrGalleryFull is returned when an 'extra' insert would exceed the gallery cap
// and the insert is not an explicit owner over-cap upload.
var ErrGalleryFull = errors.New("gallery is full")

// PersonImageInsert carries the fields for InsertPersonImage (F25). Role and Source
// are required; Provider/ExternalID/SourceURL are enrichment provenance, empty for
// owner uploads and promotes. OverCap lets an owner upload bypass the gallery cap
// deliberately (the cap still applies to enrichment, which never sets it).
type PersonImageInsert struct {
	PersonID            int64
	Role, Source        string
	Provider, ExternalID string
	SourceURL           string // upstream asset URL (enrichment only); '' for uploads
	Width, Height, ByteSize int
	OverCap             bool
}

// InsertPersonImage stores one image row for a person and returns its id. For a
// core role it replaces any existing image in that slot (delete + insert) so the
// single-slot invariant holds; for 'extra' it appends, refusing past the effective
// gallery cap (unless in.OverCap). All in one transaction under the write lock. A
// single INSERT + LastInsertId() is safe here: this table has no ON CONFLICT upsert
// (the warning in repo.go applies only to upserts whose LastInsertId can reflect an
// unrelated row), and the insert is the last statement on the tx connection.
//
// The caller (handler/enricher) writes the bytes to disk under the returned id
// AFTER this succeeds, since the id is the on-disk filename.
func (r *Repo) InsertPersonImage(ctx context.Context, in PersonImageInsert) (int64, error) {
	if !model.ValidPersonImageRole(in.Role) {
		return 0, fmt.Errorf("invalid person image role %q", in.Role)
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	sortOrder := 0
	if model.CorePersonImageRole(in.Role) {
		// Replace the core slot: remove any existing row (its on-disk file is cleaned
		// up by the handler, which lists-then-deletes; an orphaned file is harmless).
		// Core roles are single-slot and never counted against the gallery cap.
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM person_images WHERE person_id = ? AND role = ?`, in.PersonID, in.Role); err != nil {
			return 0, fmt.Errorf("replace core slot: %w", err)
		}
	} else {
		// Gallery: enforce the cap (unless an explicit owner over-cap upload) and
		// append after the current max sort_order.
		if !in.OverCap {
			var count int
			if err := tx.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM person_images WHERE person_id = ? AND role = ?`,
				in.PersonID, model.PersonImageExtra).Scan(&count); err != nil {
				return 0, fmt.Errorf("count gallery: %w", err)
			}
			if count >= r.GalleryCapValue() {
				return 0, ErrGalleryFull
			}
		}
		var maxOrder sql.NullInt64
		if err := tx.QueryRowContext(ctx,
			`SELECT MAX(sort_order) FROM person_images WHERE person_id = ? AND role = ?`,
			in.PersonID, model.PersonImageExtra).Scan(&maxOrder); err != nil {
			return 0, fmt.Errorf("max gallery order: %w", err)
		}
		if maxOrder.Valid {
			sortOrder = int(maxOrder.Int64) + 1
		}
	}

	now := time.Now().UTC().Format(timeLayout)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO person_images
			(person_id, role, source, provider, external_id, source_url, width, height, byte_size, sort_order, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.PersonID, in.Role, in.Source, in.Provider, in.ExternalID, in.SourceURL, in.Width, in.Height, in.ByteSize, sortOrder, now)
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

// LockedCoreRoles returns the set of core roles whose current image the owner set by
// hand — source 'upload' or 'promoted' (F33, ADR-049). Enrichment treats these as
// locked and never replaces them, so a re-enrich can't clobber an image the owner
// chose; an empty or provider-set ('enrichment') slot is absent from the set and
// stays refreshable. Always non-nil on success.
func (r *Repo) LockedCoreRoles(ctx context.Context, personID int64) (map[string]struct{}, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT role FROM person_images
		WHERE person_id = ? AND role IN (?, ?, ?) AND source IN (?, ?)`,
		personID, model.PersonImageHeadshot, model.PersonImageBanner, model.PersonImagePoster,
		model.PersonImageSourceUpload, model.PersonImageSourcePromoted)
	if err != nil {
		return nil, fmt.Errorf("locked core roles: %w", err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		out[role] = struct{}{}
	}
	return out, rows.Err()
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

// DeletePersonImage removes one image row scoped to its person. ErrNotFound when no
// such image belongs to the person. When the removed row is a gallery 'extra' that
// arrived from enrichment (non-empty source_url), its URL is recorded in the
// per-person suppression list so a later re-enrich does not silently re-add it
// (F25, ADR-043). Core-slot deletions never suppress — a re-enrich may legitimately
// refill an empty headshot/banner/poster. The read + delete + suppress run in one
// transaction under the write lock.
func (r *Repo) DeletePersonImage(ctx context.Context, personID, imageID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var role, sourceURL string
	err = tx.QueryRowContext(ctx,
		`SELECT role, source_url FROM person_images WHERE id = ? AND person_id = ?`,
		imageID, personID).Scan(&role, &sourceURL)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup person image for delete: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM person_images WHERE id = ? AND person_id = ?`, imageID, personID); err != nil {
		return fmt.Errorf("delete person image: %w", err)
	}

	if role == model.PersonImageExtra && sourceURL != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO person_image_suppressions (person_id, source_url, created_at)
			VALUES (?, ?, ?)`, personID, sourceURL, time.Now().UTC().Format(timeLayout)); err != nil {
			return fmt.Errorf("suppress image url: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete person image: %w", err)
	}
	return nil
}

// SuppressedPersonImageURLs returns the set of asset URLs the owner has deleted for
// a person, so re-enrichment skips re-adding them (F25, ADR-043). Always non-nil.
func (r *Repo) SuppressedPersonImageURLs(ctx context.Context, personID int64) (map[string]struct{}, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT source_url FROM person_image_suppressions WHERE person_id = ?`, personID)
	if err != nil {
		return nil, fmt.Errorf("list suppressed image urls: %w", err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out[u] = struct{}{}
	}
	return out, rows.Err()
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

// PersonImageSet builds the person-detail image read model (ADR-038 F25): which
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
