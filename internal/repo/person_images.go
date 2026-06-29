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

// ErrDuplicateImage is returned when an enrichment gallery 'extra' would duplicate
// an image already stored for the person (matched by content_hash across any role,
// F34/ADR-050). The caller treats it as a silent skip — like ErrGalleryFull, it is a
// product-rule rejection, not a failure. Owner uploads are never deduped, so this is
// only ever returned for an enrichment-sourced extra.
var ErrDuplicateImage = errors.New("duplicate image")

// PersonImageInsert carries the fields for InsertPersonImage (F25). Role and Source
// are required; Provider/ExternalID/SourceURL are enrichment provenance, empty for
// owner uploads and promotes. ContentHash is the hex sha256 of the normalized bytes
// (F34/ADR-050) — populated on every ingest, and the dedup key for enrichment extras.
// OverCap lets an owner upload bypass the gallery cap deliberately (the cap still
// applies to enrichment, which never sets it).
type PersonImageInsert struct {
	PersonID                int64
	Role, Source            string
	Provider, ExternalID    string
	SourceURL               string // upstream asset URL (enrichment only); '' for uploads
	ContentHash             string // hex sha256 of the normalized JPEG (F34/ADR-050)
	Width, Height, ByteSize int
	OverCap                 bool
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
		// Gallery dedup (F34/ADR-050): an enrichment-sourced extra whose normalized
		// bytes already exist for this person — under ANY role — is a no-op, so a
		// re-enrich / second provider / size variant can't re-pile the same photo (or
		// echo the headshot) into the gallery. Owner uploads are deliberate and never
		// deduped (only source=enrichment is checked). Core roles never run this — their
		// single-slot replace and the F25.29 poster seed legitimately repeat a hash.
		if in.Source == model.PersonImageSourceEnrichment && in.ContentHash != "" {
			var dup int
			if err := tx.QueryRowContext(ctx,
				`SELECT 1 FROM person_images WHERE person_id = ? AND content_hash = ? LIMIT 1`,
				in.PersonID, in.ContentHash).Scan(&dup); err == nil {
				return 0, ErrDuplicateImage
			} else if !errors.Is(err, sql.ErrNoRows) {
				return 0, fmt.Errorf("dedup gallery: %w", err)
			}
		}
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
			(person_id, role, source, provider, external_id, source_url, content_hash, width, height, byte_size, sort_order, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.PersonID, in.Role, in.Source, in.Provider, in.ExternalID, in.SourceURL, in.ContentHash, in.Width, in.Height, in.ByteSize, sortOrder, now)
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

// ExistingPersonImageURLs returns the set of upstream asset URLs already stored for
// a person (non-empty source_url, any role), so enrichment can skip re-fetching a URL
// it already holds before downloading anything (the F34/ADR-050 URL fast-path). The
// content_hash check remains the authoritative guard for the same image under a
// different URL. Always non-nil on success.
func (r *Repo) ExistingPersonImageURLs(ctx context.Context, personID int64) (map[string]struct{}, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT source_url FROM person_images WHERE person_id = ? AND source_url <> ''`, personID)
	if err != nil {
		return nil, fmt.Errorf("list existing image urls: %w", err)
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

// PersonImageRef identifies one image row by its person and id — enough to locate the
// on-disk file (ImagePath) for cleanup after a backfill collapse (F34/ADR-050).
type PersonImageRef struct {
	PersonID, ID int64
}

// PersonImagesMissingHash returns the rows that have no content_hash yet — the
// pre-F34 images the one-time backfill must hash from their on-disk bytes
// (ADR-050). Returned in id order so the pass is deterministic. Always non-nil.
func (r *Repo) PersonImagesMissingHash(ctx context.Context) ([]PersonImageRef, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT person_id, id FROM person_images WHERE content_hash = '' ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list unhashed person images: %w", err)
	}
	defer rows.Close()
	out := []PersonImageRef{}
	for rows.Next() {
		var ref PersonImageRef
		if err := rows.Scan(&ref.PersonID, &ref.ID); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// SetPersonImageHash records the computed content_hash for one row (F34 backfill,
// ADR-050). Under the write lock like every mutation.
func (r *Repo) SetPersonImageHash(ctx context.Context, id int64, hash string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx,
		`UPDATE person_images SET content_hash = ? WHERE id = ?`, hash, id); err != nil {
		return fmt.Errorf("set person image hash: %w", err)
	}
	return nil
}

// CollapseDuplicateGalleryExtras deletes gallery 'extra' rows that duplicate another
// of the person's images by content_hash, keeping the earliest occurrence; an extra
// whose bytes match a CORE image is dropped in favor of the core image. Core images
// are never deleted. It returns the deleted rows so the caller can remove their disk
// files. One-time F34 backfill cleanup (ADR-050); idempotent (a deduped gallery has
// nothing left to collapse). Select + delete run in one write-locked transaction.
func (r *Repo) CollapseDuplicateGalleryExtras(ctx context.Context) ([]PersonImageRef, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	// An extra is a duplicate-to-delete when another image for the same person shares
	// its hash and is "preferred": any core image wins (o.role <> 'extra'), else the
	// earlier extra wins (o.id < e.id). So per (person, hash) the lowest-id extra
	// survives unless a core image shares the hash, in which case every extra goes.
	rows, err := tx.QueryContext(ctx, `
		SELECT e.person_id, e.id FROM person_images e
		WHERE e.role = ? AND e.content_hash <> ''
		  AND EXISTS (
			SELECT 1 FROM person_images o
			WHERE o.person_id = e.person_id AND o.content_hash = e.content_hash
			  AND o.id <> e.id AND (o.role <> ? OR o.id < e.id))
		ORDER BY e.id`, model.PersonImageExtra, model.PersonImageExtra)
	if err != nil {
		return nil, fmt.Errorf("find duplicate gallery extras: %w", err)
	}
	var victims []PersonImageRef
	for rows.Next() {
		var ref PersonImageRef
		if err := rows.Scan(&ref.PersonID, &ref.ID); err != nil {
			rows.Close()
			return nil, err
		}
		victims = append(victims, ref)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	for _, v := range victims {
		if _, err := tx.ExecContext(ctx, `DELETE FROM person_images WHERE id = ?`, v.ID); err != nil {
			return nil, fmt.Errorf("delete duplicate gallery extra: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit collapse duplicates: %w", err)
	}
	return victims, nil
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
