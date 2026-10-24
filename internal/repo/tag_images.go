package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Tag images (F82, list-views spec P0-10): the metadata index for a tag's one owner-
// uploaded poster image. Single-slot like a studio image role, so a new image always
// replaces (delete + insert). Writes take writeMu like the rest of the write path;
// reads are unlocked (WAL). A merge carries the loser's image to a survivor that has
// none (the tag_images idMove in identity_ops.go).

// TagImage is a tag's stored image. ID doubles as the ?v= cache-buster: a replace is
// delete + insert, so a new image gets a new id and the browser re-fetches past the
// immutable cache.
type TagImage struct {
	ID        int64
	TagID     int64
	Width     int
	Height    int
	ByteSize  int
	CreatedAt time.Time
}

// GetTagImage returns the tag's image row, or ErrNotFound when it has none.
func (r *Repo) GetTagImage(ctx context.Context, tagID int64) (TagImage, error) {
	var (
		ti      TagImage
		created string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, tag_id, width, height, byte_size, created_at FROM tag_images WHERE tag_id = ?`, tagID).
		Scan(&ti.ID, &ti.TagID, &ti.Width, &ti.Height, &ti.ByteSize, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return TagImage{}, ErrNotFound
	}
	if err != nil {
		return TagImage{}, fmt.Errorf("get tag image: %w", err)
	}
	ti.CreatedAt, _ = time.Parse(timeLayout, created)
	return ti, nil
}

// ReplaceTagImage makes the tag's image be a new row of the given size and returns its
// server-assigned id. Delete + insert in one transaction, so the one-per-tag invariant
// holds and the id always advances. The caller stores the bytes at the returned id and
// removes the superseded file (it holds the prior id from GetTagImage).
func (r *Repo) ReplaceTagImage(ctx context.Context, tagID int64, width, height, byteSize int) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.ExecContext(ctx, `DELETE FROM tag_images WHERE tag_id = ?`, tagID); err != nil {
		return 0, fmt.Errorf("clear tag image slot: %w", err)
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO tag_images (tag_id, width, height, byte_size, created_at) VALUES (?, ?, ?, ?, ?)`,
		tagID, width, height, byteSize, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("insert tag image: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tag image: %w", err)
	}
	return id, nil
}

// DeleteTagImage removes the tag's image row. Idempotent — deleting an empty slot is a
// no-op success.
func (r *Repo) DeleteTagImage(ctx context.Context, tagID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx, `DELETE FROM tag_images WHERE tag_id = ?`, tagID); err != nil {
		return fmt.Errorf("delete tag image: %w", err)
	}
	return nil
}

// tagImageVersions returns tagID -> image row id for every tag in ids that has an
// image, in one batch query (the list read's way of filling Tag.ImageVersion).
func (r *Repo) tagImageVersions(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT tag_id, id FROM tag_images WHERE tag_id IN (`+placeholders(len(ids))+`)`, toAnySlice(ids)...)
	if err != nil {
		return nil, fmt.Errorf("tag image versions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tagID, rowID int64
		if err := rows.Scan(&tagID, &rowID); err != nil {
			return nil, err
		}
		out[tagID] = rowID
	}
	return out, rows.Err()
}
