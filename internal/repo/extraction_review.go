package repo

import (
	"context"
	"fmt"
	"time"
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
