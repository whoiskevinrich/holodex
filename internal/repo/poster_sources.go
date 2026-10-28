package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"holodex/internal/fieldsource"
	"holodex/internal/model"
)

// Poster source slots (F85, migration 0067, media-ingest.md "Poster source slots").
// One row per held local slot; the bytes live on disk beside the served pair, and
// the row carries the content hash the local:<kind>#<hash> decision token uses.

// PosterSource is one held local slot of a video.
type PosterSource struct {
	Kind      string   // fieldsource.LocalFrame | LocalUpload | LocalAutomatic
	Hash      string   // sha256 of the slot's bytes
	FrameAt   *float64 // picked time in seconds, frame only
	UpdatedAt string
}

// PosterSources returns a video's held local slots, keyed by kind.
func (r *Repo) PosterSources(ctx context.Context, videoID int64) (map[string]PosterSource, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT kind, hash, frame_at, updated_at FROM video_poster_sources WHERE video_id = ?`, videoID)
	if err != nil {
		return nil, fmt.Errorf("poster sources: %w", err)
	}
	defer rows.Close()
	out := map[string]PosterSource{}
	for rows.Next() {
		var ps PosterSource
		var at sql.NullFloat64
		if err := rows.Scan(&ps.Kind, &ps.Hash, &at, &ps.UpdatedAt); err != nil {
			return nil, fmt.Errorf("poster sources: %w", err)
		}
		if at.Valid {
			ps.FrameAt = &at.Float64
		}
		out[ps.Kind] = ps
	}
	return out, rows.Err()
}

// SetPosterSource upserts a slot's row. With decide set, the video's poster_url
// decision becomes local:<kind> in the same transaction — the frame and upload
// routes make their image the choice (field-resolution.md "Set where the image is
// made"); the generation queue's automatic slot never does.
func (r *Repo) SetPosterSource(ctx context.Context, videoID int64, kind, hash string, frameAt *float64, decide bool) error {
	if !fieldsource.ValidLocalKind(kind) {
		return fmt.Errorf("set poster source: unknown kind %q", kind)
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("set poster source: %w", err)
	}
	defer tx.Rollback()
	var at any
	if frameAt != nil {
		at = *frameAt
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO video_poster_sources (video_id, kind, hash, frame_at, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(video_id, kind) DO UPDATE SET hash = excluded.hash, frame_at = excluded.frame_at,
			updated_at = excluded.updated_at`,
		videoID, kind, hash, at, time.Now().UTC().Format(timeLayout)); err != nil {
		return fmt.Errorf("set poster source: %w", err)
	}
	if decide {
		if err := upsertDecision(ctx, tx, model.EnrichEntityVideo, videoID, fieldsource.LocalField, fieldsource.ForLocal(kind), ""); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeletePosterSource removes a slot's row and, in the same transaction, a poster_url
// decision that names it, so a removed upload never leaves a dangling choice (P0-9).
// cleared reports whether a decision was deleted.
func (r *Repo) DeletePosterSource(ctx context.Context, videoID int64, kind string) (cleared bool, err error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("delete poster source: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM video_poster_sources WHERE video_id = ? AND kind = ?`, videoID, kind); err != nil {
		return false, fmt.Errorf("delete poster source: %w", err)
	}
	res, err := tx.ExecContext(ctx, `
		DELETE FROM field_source_decisions
		WHERE entity_type = ? AND entity_id = ? AND field_key = ? AND source = ?`,
		model.EnrichEntityVideo, videoID, fieldsource.LocalField, fieldsource.ForLocal(kind))
	if err != nil {
		return false, fmt.Errorf("delete poster decision: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, tx.Commit()
}

// PosterDecision returns the video's standing poster_url decision source, "" when
// undecided — the one input Manager.Present reads besides the slots.
func (r *Repo) PosterDecision(ctx context.Context, videoID int64) (string, error) {
	var src string
	err := r.db.QueryRowContext(ctx, `
		SELECT source FROM field_source_decisions
		WHERE entity_type = ? AND entity_id = ? AND field_key = ?`,
		model.EnrichEntityVideo, videoID, fieldsource.LocalField).Scan(&src)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("poster decision: %w", err)
	}
	return src, nil
}

// ThumbnailStateRow is one video's automatic-pipeline state, for the slot backfill.
type ThumbnailStateRow struct {
	ID    int64
	State string
}

// VideosWithThumbnailImage returns every video whose thumbnail_state says the
// served pair holds an image (embedded, generated or the retired uploaded) — the
// F85 boot backfill's input.
func (r *Repo) VideosWithThumbnailImage(ctx context.Context) ([]ThumbnailStateRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, thumbnail_state FROM videos
		WHERE thumbnail_state IN (?, ?, ?)`,
		model.ThumbnailEmbedded, model.ThumbnailGenerated, model.ThumbnailUploaded)
	if err != nil {
		return nil, fmt.Errorf("videos with thumbnail image: %w", err)
	}
	defer rows.Close()
	var out []ThumbnailStateRow
	for rows.Next() {
		var t ThumbnailStateRow
		if err := rows.Scan(&t.ID, &t.State); err != nil {
			return nil, fmt.Errorf("videos with thumbnail image: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
