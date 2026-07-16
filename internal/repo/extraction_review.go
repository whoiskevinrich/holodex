package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"holodex/internal/model"
)

// Extraction review queue (F48.4, ADR-067): one pending row per (video,
// field) where a scored extraction candidate didn't clear its tier's
// AutoApplyThreshold, or failed the entity exact-match gate (F48.3d), or lost
// to a standing manual: decision (F48.3e). Structurally closest to
// decisions.go (a per-entity-field row with an upsert path) rather than
// enrichment_dismissals.go's separate dismissal table — dismiss/resolve are
// just terminal values of this row's own status column.

const (
	ExtractionReviewPending   = "pending"
	ExtractionReviewDismissed = "dismissed"
	ExtractionReviewResolved  = "resolved"
)

// ExtractionReviewRow is one row of metadata_extraction_review.
// SuggestedEntityID is 0 when there is no Jaro-Winkler suggestion (F48.3d).
type ExtractionReviewRow struct {
	ID                int64
	VideoID           int64
	FieldKey          string
	FilenameValue     string
	TagValue          string
	Confidence        float64
	SuggestedEntityID int64
	Status            string
	CreatedAt         string
	ResolvedAt        string
}

// UpsertExtractionReview creates the pending review row for (video, field),
// or updates it in place if one is already pending (F48.4b: re-running
// extraction on an already-pending field updates rather than duplicates).
// The partial unique index (status = 'pending') means a field whose prior
// review was already dismissed or resolved gets a fresh pending row instead
// — re-triggering extraction is what F48.4d says un-suppresses a dismissal.
//
// Uses SQLite's ON CONFLICT ... WHERE (a partial-index conflict target,
// supported since 3.35) rather than person_images.go's delete-then-insert for
// the same "one live row per key" shape: this row's id/created_at need to
// survive an in-place update (it's what the owner's review-queue UI points
// at), where person_images' delete+insert is fine because nothing external
// references a slot row's id.
func (r *Repo) UpsertExtractionReview(ctx context.Context, videoID int64, fieldKey, filenameValue, tagValue string, confidence float64, suggestedEntityID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	var suggested any
	if suggestedEntityID != 0 {
		suggested = suggestedEntityID
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO metadata_extraction_review
			(video_id, field_key, filename_value, tag_value, confidence, suggested_entity_id, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'pending', ?)
		ON CONFLICT(video_id, field_key) WHERE status = 'pending' DO UPDATE SET
			filename_value       = excluded.filename_value,
			tag_value            = excluded.tag_value,
			confidence           = excluded.confidence,
			suggested_entity_id  = excluded.suggested_entity_id,
			created_at           = excluded.created_at`,
		videoID, fieldKey, filenameValue, tagValue, confidence, suggested,
		time.Now().UTC().Format(timeLayout))
	if err != nil {
		return fmt.Errorf("upsert extraction review: %w", err)
	}
	return nil
}

// ListExtractionReviews returns every review row for status, newest first.
func (r *Repo) ListExtractionReviews(ctx context.Context, status string) ([]ExtractionReviewRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, video_id, field_key, filename_value, tag_value, confidence,
		       COALESCE(suggested_entity_id, 0), status, created_at, COALESCE(resolved_at, '')
		FROM metadata_extraction_review
		WHERE status = ?
		ORDER BY created_at DESC, id DESC`, status)
	if err != nil {
		return nil, fmt.Errorf("list extraction reviews: %w", err)
	}
	defer rows.Close()

	var out []ExtractionReviewRow
	for rows.Next() {
		var row ExtractionReviewRow
		if err := rows.Scan(&row.ID, &row.VideoID, &row.FieldKey, &row.FilenameValue, &row.TagValue,
			&row.Confidence, &row.SuggestedEntityID, &row.Status, &row.CreatedAt, &row.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ResolveExtractionReview marks a pending row resolved (F48.4c) — the owner
// picked a value and the resulting write has been enqueued by the caller.
// Idempotent no-op success if the row is already resolved/dismissed or
// doesn't exist (mirrors ClearDecision's idempotency).
func (r *Repo) ResolveExtractionReview(ctx context.Context, id int64) error {
	return r.setExtractionReviewStatus(ctx, id, ExtractionReviewResolved)
}

// DismissExtractionReview marks a pending row dismissed (F48.4d) — durable
// until the owner re-triggers extraction for the file, which opens a fresh
// pending row for the same field rather than resurrecting this one.
func (r *Repo) DismissExtractionReview(ctx context.Context, id int64) error {
	return r.setExtractionReviewStatus(ctx, id, ExtractionReviewDismissed)
}

func (r *Repo) setExtractionReviewStatus(ctx context.Context, id int64, status string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.db.ExecContext(ctx, `
		UPDATE metadata_extraction_review
		SET status = ?, resolved_at = ?
		WHERE id = ?`, status, time.Now().UTC().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("set extraction review status: %w", err)
	}
	return nil
}

// GetExtractionReview returns one review row by id, or ErrNotFound — the
// resolve/dismiss API handlers (F48.6c/d) need the row's field_key and
// values before they can act on it.
func (r *Repo) GetExtractionReview(ctx context.Context, id int64) (ExtractionReviewRow, error) {
	var row ExtractionReviewRow
	err := r.db.QueryRowContext(ctx, `
		SELECT id, video_id, field_key, filename_value, tag_value, confidence,
		       COALESCE(suggested_entity_id, 0), status, created_at, COALESCE(resolved_at, '')
		FROM metadata_extraction_review
		WHERE id = ?`, id).Scan(&row.ID, &row.VideoID, &row.FieldKey, &row.FilenameValue, &row.TagValue,
		&row.Confidence, &row.SuggestedEntityID, &row.Status, &row.CreatedAt, &row.ResolvedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ExtractionReviewRow{}, ErrNotFound
	}
	if err != nil {
		return ExtractionReviewRow{}, fmt.Errorf("get extraction review: %w", err)
	}
	return row, nil
}

// ExtractionQueueRow is one pending review row, video-joined for the owner's
// grouped-by-video queue (F48.6) — the extraction-review analogue of
// enrich_queue.go's EnrichQueueRow. Serialized directly by the API handler,
// so every field carries the JSON tag the frontend expects.
type ExtractionQueueRow struct {
	ID                  int64   `json:"id"`
	VideoID             int64   `json:"video_id"`
	VideoTitle          string  `json:"video_title"`
	FilePath            string  `json:"file_path"`
	FieldKey            string  `json:"field_key"`
	FilenameValue       string  `json:"filename_value"`
	TagValue            string  `json:"tag_value"`
	Confidence          float64 `json:"confidence"`
	SuggestedEntityID   int64   `json:"suggested_entity_id,omitempty"`
	SuggestedEntityName string  `json:"suggested_entity_name,omitempty"`
}

// ExtractionQueue lists every pending review row, joined with its video's
// title/path, ordered per-video so the frontend's grouping stays stable
// across reloads. Video-level ordering (most-pending-fields-first, per the
// design handoff) is a client-side derivation over this flat list, same as
// enrichment's client-side kind grouping.
func (r *Repo) ExtractionQueue(ctx context.Context) ([]ExtractionQueueRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT er.id, er.video_id, v.title, v.file_path, er.field_key,
		       er.filename_value, er.tag_value, er.confidence,
		       COALESCE(er.suggested_entity_id, 0)
		FROM metadata_extraction_review er
		JOIN videos v ON v.id = er.video_id
		WHERE er.status = 'pending'
		ORDER BY er.video_id, er.created_at`)
	if err != nil {
		return nil, fmt.Errorf("extraction queue: %w", err)
	}
	defer rows.Close()

	var out []ExtractionQueueRow
	for rows.Next() {
		var row ExtractionQueueRow
		if err := rows.Scan(&row.ID, &row.VideoID, &row.VideoTitle, &row.FilePath, &row.FieldKey,
			&row.FilenameValue, &row.TagValue, &row.Confidence, &row.SuggestedEntityID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Resolve suggested-entity names: one EntityNames call per entity type
	// actually present (at most two — person, studio), not one lookup per
	// row or per distinct id. A stale suggestion (the entity was merged/
	// deleted since extraction last ran) simply has no entry in the map and
	// is left with an empty SuggestedEntityName.
	names := make(map[string]map[int64]string, 2)
	for i := range out {
		row := &out[i]
		if row.SuggestedEntityID == 0 {
			continue
		}
		entityType, ok := model.EntityTypeForField[row.FieldKey]
		if !ok {
			continue
		}
		byID, loaded := names[entityType]
		if !loaded {
			var err error
			byID, err = r.EntityNames(ctx, entityType)
			if err != nil {
				return nil, err
			}
			names[entityType] = byID
		}
		row.SuggestedEntityName = byID[row.SuggestedEntityID]
	}
	return out, nil
}
