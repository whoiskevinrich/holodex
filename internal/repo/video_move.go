package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"holodex/internal/model"
)

// MoveCandidate is a stored row whose content matches a newly found file (F79,
// media-ingest.md "Moves recognised by a sampled content hash"). The scanner
// keeps only the ones that are missing this pass.
type MoveCandidate struct {
	ID       int64
	FilePath string
	Active   bool
}

// HasSizeTwin reports whether any non-deleted row has this byte size. Only a
// new file with a size twin can be a move, so the scanner holds just those
// until the walk ends and inserts every other new file at once.
func (r *Repo) HasSizeTwin(ctx context.Context, size int64) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM videos WHERE file_size = ? AND deleted_at IS NULL)`, size,
	).Scan(&one)
	if err != nil {
		return false, fmt.Errorf("size twin: %w", err)
	}
	return one == 1, nil
}

// MoveCandidates returns the non-deleted rows that a new file v is the same
// content as (sameContentSQL). A row with a fingerprint matches on it. A row with
// no fingerprint (one hidden before migration 0064, or not yet backfilled)
// matches only when every recorded fact is equal: byte size, mtime, duration,
// the stream facts and, where the row recorded them, the file tags.
func (r *Repo) MoveCandidates(ctx context.Context, v *model.Video) ([]MoveCandidate, error) {
	if v.ContentFP == "" {
		return nil, nil
	}
	tags, err := fileTagsJSON(v.Tags)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH n (file_size, content_fp, file_mtime, duration_sec, width, height,
		        video_codec, audio_codec, bitrate_kbps, container, file_tags)
		  AS (VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?))
		SELECT h.id, h.file_path, h.active FROM videos h, n
		 WHERE h.deleted_at IS NULL AND `+sameContentSQL+`
		 ORDER BY h.id`,
		v.FileSize, v.ContentFP,
		v.FileMtime.UTC().Format(timeLayout), v.Duration, v.Width, v.Height,
		v.VideoCodec, v.AudioCodec, v.BitrateKbps, v.Container, tags)
	if err != nil {
		return nil, fmt.Errorf("move candidates: %w", err)
	}
	defer rows.Close()
	var out []MoveCandidate
	for rows.Next() {
		var c MoveCandidate
		if err := rows.Scan(&c.ID, &c.FilePath, &c.Active); err != nil {
			return nil, fmt.Errorf("scan move candidate: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RepointVideo moves an existing row to the file a scan found at a new path: it
// rewrites file_path, the extracted facts and the fingerprint, and shows the row
// again. The id, indexed_at and everything attached to the row are untouched,
// because a move is neither a new arrival nor an update (F79 P0-3, HOLODEX-567).
// It refuses a soft-deleted row.
func (r *Repo) RepointVideo(ctx context.Context, id int64, v *model.Video, extra []model.ExtraMetadata) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var recorded any
	if v.RecordedAt != nil {
		recorded = v.RecordedAt.UTC().Format(timeLayout)
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE videos SET
			file_path    = ?,
			file_size    = ?,
			title        = ?,
			duration_sec = ?,
			width        = ?,
			height       = ?,
			video_codec  = ?,
			audio_codec  = ?,
			bitrate_kbps = ?,
			container    = ?,
			recorded_at  = ?,
			file_mtime   = ?,
			content_fp   = NULLIF(?, ''),
			active       = 1
		WHERE id = ? AND deleted_at IS NULL`,
		v.FilePath, v.FileSize, v.Title, v.Duration, v.Width, v.Height,
		v.VideoCodec, v.AudioCodec, v.BitrateKbps, v.Container,
		recorded, v.FileMtime.UTC().Format(timeLayout), v.ContentFP, id)
	if err != nil {
		return fmt.Errorf("repoint video %d: %w", id, err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("repoint video %d: %w", id, err)
	} else if n != 1 {
		return fmt.Errorf("repoint video %d: row missing or deleted", id)
	}
	if err := replaceAssociations(ctx, tx, id, v.Tags, extra); err != nil {
		return err
	}
	if err := recordFileTags(ctx, tx, id, v.Tags); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit repoint %d: %w", id, err)
	}
	return nil
}

// fileTagsJSON is the stored shape of videos.file_tags: the tag names as a JSON
// array, in extraction order.
func fileTagsJSON(tags []model.Tag) (string, error) {
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	raw, err := json.Marshal(names)
	if err != nil {
		return "", fmt.Errorf("encode file tags: %w", err)
	}
	return string(raw), nil
}
