package repo

import (
	"context"
	"database/sql"
	"errors"
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

// resolveOrCreateTagName resolves-or-creates a tag by name inside an already-open
// tx and reads back its canonical name -- a plain name-only read, not the heavier
// GetTag (which also joins a video-count aggregate and fetches aliases neither
// caller uses). The shared step behind attachTagTx (video attach/materialization)
// and ResolveOrCreateTag (categories.go's bare resolve, no video link), which
// differ only in what happens with the resolved id afterward.
func resolveOrCreateTagName(ctx context.Context, tx *sql.Tx, name string) (int64, string, error) {
	tid, err := resolveOrCreateByName(ctx, tx, model.EntityTag, name, "")
	if err != nil {
		return 0, "", err
	}
	var tagName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM tags WHERE id = ?`, tid).Scan(&tagName); err != nil {
		return 0, "", fmt.Errorf("resolve or create tag: %w", err)
	}
	return tid, tagName, nil
}

// attachTagTx resolves-or-creates name and links it to videoID with the given
// provenance, inside an already-open tx. The shared step behind AttachTagToVideo
// (owner manual attach, one name at a time) and AttachMaterializedTags (F50 P0-9
// enrichment materialization, a whole resolved-field batch at once) — the two
// callers differ only in how a denied/oversized name is handled (surfaced vs.
// silently skipped) and whether a video-existence check runs first.
func attachTagTx(ctx context.Context, tx *sql.Tx, videoID int64, name, source string) (int64, string, error) {
	tid, tagName, err := resolveOrCreateTagName(ctx, tx, name)
	if err != nil {
		return 0, "", err
	}
	// ON CONFLICT upgrades an existing link's source away from 'file' (so a
	// scanner-discovered tag that's later manually attached or materialized
	// survives future rescans, ADR-075 D3) but never downgrades one that's
	// already durable -- re-running materialization must not clobber a manual
	// attach, and vice versa.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO video_tags (video_id, tag_id, source) VALUES (?, ?, ?)
		 ON CONFLICT (video_id, tag_id) DO UPDATE SET source = excluded.source
		 WHERE video_tags.source = ?`,
		videoID, tid, source, fieldsource.File); err != nil {
		return 0, "", fmt.Errorf("attach tag: %w", err)
	}
	return tid, tagName, nil
}

// AttachTagToVideo resolves-or-creates a tag by name and links it to videoID with
// source='manual'. Idempotent: re-attaching an already-linked tag is a no-op.
// Returns the resolved tag. ErrNotFound if videoID doesn't exist (or is
// soft-deleted); ErrTagDenied / ErrTagNameTooLong / ErrTagNameCollidesWithCategory
// propagate from resolveOrCreateByName so the caller can translate them to the
// owner-facing 422/400/409 the manual-attach path gets (unlike the scanner's
// silent skip).
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

	tid, tagName, err := attachTagTx(ctx, tx, videoID, name, fieldsource.Manual)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &model.Tag{ID: tid, Name: tagName}, nil
}

// MaterializedTag is one enrichment-resolved value to attach as a tag, with the
// provenance to record on its video_tags row (F50 P0-9, ADR-075 D4).
type MaterializedTag struct {
	Name   string
	Source string
}

// AttachMaterializedTags attaches every value in tags to videoID in a single
// transaction (F50 P0-9, ADR-075 D4) — the enrichment-materialization counterpart to
// AttachTagToVideo, called once per enrich-apply with the video's whole resolved
// `genres` set rather than once per value. A denied, oversized, or category-colliding
// name is silently skipped, not surfaced: enrichment is unattended, so there is no
// owner to show a 422/400/409 to (ADR-075 D2; ADR-078 D3), matching
// replaceAssociations' precedent for the scanner.
// INSERT OR IGNORE makes re-running against an already-materialized video a no-op
// (idempotent). No video-existence check: the caller (MaterializeVideoTags) already
// resolved the video's fields, which only succeeds for a video that exists.
func (r *Repo) AttachMaterializedTags(ctx context.Context, videoID int64, tags []MaterializedTag) error {
	if len(tags) == 0 {
		return nil
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	for _, t := range tags {
		if _, _, err := attachTagTx(ctx, tx, videoID, t.Name, t.Source); err != nil {
			if errors.Is(err, ErrTagDenied) || errors.Is(err, ErrTagNameTooLong) || errors.Is(err, ErrTagNameCollidesWithCategory) {
				continue
			}
			return err
		}
	}
	return tx.Commit()
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
