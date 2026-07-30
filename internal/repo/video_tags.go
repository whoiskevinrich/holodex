package repo

import (
	"context"
	"fmt"

	"holodex/internal/fieldsource"
	"holodex/internal/model"
)

// Video↔tag attach/detach (F50, ADR-075 P0-7) — the owner's media-page add/remove
// chips. Resolves through the same shared name-identity spine as the scanner and
// materialization (resolveOrCreateByName), so a manually-typed name converges with
// any existing tag/alias exactly as a scanned one would. Rows land with
// source='manual' (fieldsource.Manual), which replaceAssociations' rescan-scoped
// delete (ADR-075 D3) never touches — a manual tag survives every future rescan.

// AttachTagToVideo resolves-or-creates a tag by name and links it to videoID with
// source='manual'. Idempotent: re-attaching an already-linked tag is a no-op.
// Returns the resolved tag. ErrNotFound if videoID doesn't exist (or is
// soft-deleted); ErrTagDenied / ErrTagNameTooLong propagate from
// resolveOrCreateByName so the caller can translate them to the owner-facing 422/400
// the manual-attach path gets (unlike the scanner's silent skip).
func (r *Repo) AttachTagToVideo(ctx context.Context, videoID int64, name string) (*model.Tag, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	// writeMu is already held, so no concurrent writer can delete videoID between
	// this check and the insert below -- a plain pre-tx read (reusing the same
	// existence guard the thumbnail handler uses) is as safe as a tx-scoped one.
	if visible, err := r.VideoVisible(ctx, videoID); err != nil {
		return nil, err
	} else if !visible {
		return nil, ErrNotFound
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	tid, err := resolveOrCreateByName(ctx, tx, model.EntityTag, name, "")
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO video_tags (video_id, tag_id, source) VALUES (?, ?, ?)`,
		videoID, tid, fieldsource.Manual); err != nil {
		return nil, fmt.Errorf("attach tag: %w", err)
	}
	// A plain name-only read, not the heavier GetTag (which also joins a video-count
	// aggregate and fetches aliases the response never uses).
	var tagName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM tags WHERE id = ?`, tid).Scan(&tagName); err != nil {
		return nil, fmt.Errorf("attach tag: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &model.Tag{ID: tid, Name: tagName}, nil
}

// DetachTagFromVideo removes tagID's link to videoID. ErrNotFound if the tag isn't
// currently attached to the video — surfaced, not a silent no-op, so the owner UI
// can tell a stale chip apart from a real removal.
func (r *Repo) DetachTagFromVideo(ctx context.Context, videoID, tagID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	res, err := r.db.ExecContext(ctx,
		`DELETE FROM video_tags WHERE video_id = ? AND tag_id = ?`, videoID, tagID)
	if err != nil {
		return fmt.Errorf("detach tag: %w", err)
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
