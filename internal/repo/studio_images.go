package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"holodex/internal/model"
)

// Studio images (F51, ADR-079): the metadata index for a studio's three core image
// roles (icon/logo/poster), generalizing the single-role studio_logos cache (ADR-057).
// Unlike person_images there is no gallery — every role is single-slot, so a new image
// for a role always replaces (delete + insert) with no partial-index/cap distinction to
// make. Writes take writeMu like the rest of the write path; reads are unlocked (WAL).

// StudioImage is one stored image for a studio. ID doubles as the ?v= cache-buster: a
// replace is delete + insert, so a new image gets a new id and the browser re-fetches
// past the immutable cache (mirrors StudioLogo/PersonImage).
type StudioImage struct {
	ID         int64
	StudioID   int64
	Role       string
	Source     string // 'upload' | 'enrichment'
	Provider   string
	ExternalID string
	Width      int
	Height     int
	ByteSize   int
	CreatedAt  time.Time
}

// StudioImageInsert is the payload for ReplaceStudioImage.
type StudioImageInsert struct {
	StudioID                int64
	Role, Source            string
	Provider, ExternalID    string
	Width, Height, ByteSize int
}

// GetStudioImage returns the studio's image row for one role, or ErrNotFound when
// that slot is empty.
func (r *Repo) GetStudioImage(ctx context.Context, studioID int64, role string) (StudioImage, error) {
	if !model.ValidStudioImageRole(role) {
		return StudioImage{}, fmt.Errorf("invalid studio image role %q", role)
	}
	var (
		si      StudioImage
		created string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, studio_id, role, source, provider, external_id, width, height, byte_size, created_at
		FROM studio_images WHERE studio_id = ? AND role = ?`, studioID, role).
		Scan(&si.ID, &si.StudioID, &si.Role, &si.Source, &si.Provider, &si.ExternalID, &si.Width, &si.Height, &si.ByteSize, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return StudioImage{}, ErrNotFound
	}
	if err != nil {
		return StudioImage{}, fmt.Errorf("get studio image: %w", err)
	}
	si.CreatedAt, _ = time.Parse(timeLayout, created)
	return si, nil
}

// ReplaceStudioImage makes the studio's image for `in.Role` be exactly `in` and
// returns the new server-assigned id. Delete + insert in one transaction so the
// UNIQUE(studio_id, role) single-slot invariant holds and the id always advances on
// a replace. The caller stores the bytes at the returned id and removes any
// superseded file (it holds the prior id from GetStudioImage).
func (r *Repo) ReplaceStudioImage(ctx context.Context, in StudioImageInsert) (int64, error) {
	if !model.ValidStudioImageRole(in.Role) {
		return 0, fmt.Errorf("invalid studio image role %q", in.Role)
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM studio_images WHERE studio_id = ? AND role = ?`, in.StudioID, in.Role); err != nil {
		return 0, fmt.Errorf("clear studio image slot: %w", err)
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO studio_images (studio_id, role, source, provider, external_id, width, height, byte_size, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.StudioID, in.Role, in.Source, in.Provider, in.ExternalID, in.Width, in.Height, in.ByteSize,
		time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("insert studio image: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit studio image: %w", err)
	}
	return id, nil
}

// DeleteStudioImage removes the studio's image row for one role. Idempotent —
// deleting an already-empty slot is a no-op success.
func (r *Repo) DeleteStudioImage(ctx context.Context, studioID int64, role string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM studio_images WHERE studio_id = ? AND role = ?`, studioID, role); err != nil {
		return fmt.Errorf("delete studio image: %w", err)
	}
	return nil
}

// LockedStudioImageRoles returns the set of roles whose current image the owner set
// by hand (source='upload'), which enrichment must never overwrite (F51, the
// ADR-049 provenance-lock pattern generalized to a second entity). An empty or
// provider-set ('enrichment') slot is absent from the set and stays refreshable.
// Always non-nil on success.
func (r *Repo) LockedStudioImageRoles(ctx context.Context, studioID int64) (map[string]struct{}, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT role FROM studio_images WHERE studio_id = ? AND source = ?`,
		studioID, model.StudioImageSourceUpload)
	if err != nil {
		return nil, fmt.Errorf("locked studio image roles: %w", err)
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

// studioImageVersions returns studioID -> {role: rowID} for every studio in ids, in
// ONE batch query — the list/detail read path's way of filling Studio.ImageVersions
// without an N-way per-studio lookup (mirrors the pre-F51 attachStudioLogos).
func (r *Repo) studioImageVersions(ctx context.Context, ids []int64) (map[int64]map[string]int64, error) {
	out := make(map[int64]map[string]int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT studio_id, role, id FROM studio_images
		WHERE studio_id IN (`+placeholders(len(ids))+`)`, toAnySlice(ids)...)
	if err != nil {
		return nil, fmt.Errorf("studio image versions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var studioID, rowID int64
		var role string
		if err := rows.Scan(&studioID, &role, &rowID); err != nil {
			return nil, err
		}
		if out[studioID] == nil {
			out[studioID] = map[string]int64{}
		}
		out[studioID][role] = rowID
	}
	return out, rows.Err()
}

// attachStudioImages fills ImageVersions on each studio from studio_images in one
// batch query (F51, ADR-079 — generalizes attachStudioLogos to three roles).
func (r *Repo) attachStudioImages(ctx context.Context, studios []model.Studio) error {
	if len(studios) == 0 {
		return nil
	}
	ids := make([]int64, len(studios))
	for i, s := range studios {
		ids[i] = s.ID
	}
	versions, err := r.studioImageVersions(ctx, ids)
	if err != nil {
		return err
	}
	for i := range studios {
		studios[i].ImageVersions = versions[studios[i].ID]
	}
	return nil
}
