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

// Saving a trim as an edition (F88, file-maintenance.md "Save as edition"): a new library item for
// a new file, copied once from its source. The two are independent from then on.

// CloneTables classifies every table holding per-video state, so a table added later can't be
// silently left out: TestCloneClassifiesEveryTable fails until someone decides. Copied rows are
// written by CloneVideoAsEdition. Derived rows come from the new file (ReExtract, thumbnails,
// montage). History stays with the source. NoVideos tables key on entity_id for other kinds only.
var CloneTables = struct{ Copied, Derived, History, NoVideos []string }{
	Copied: []string{
		"video_people", "video_tags", "video_studios", "film_videos", "playlist_videos",
		"metadata_curation", "field_source_decisions", "entity_enrichment", "facet_not_applicable",
		"enrichment_dismissals", "video_poster_sources",
	},
	Derived: []string{
		"video_metadata", "video_montages", "entity_completeness", "entity_completeness_missing",
		"completeness_dirty", "resolved_values",
	},
	History: []string{
		"file_writebacks", "file_writeback_snapshots", "writeback_queue", "held_originals",
		"metadata_extraction_review", "job_runs", "file_renames", "video_file_names",
		"video_trim_results", "video_trimmed_from", "video_plays", "video_watch",
	},
	NoVideos: []string{"entity_aliases", "entity_alias_suppressions", "entity_external_ids"},
}

// PathInLibrary reports whether a row, live or trashed, already claims path. A trashed row's state
// must not be merged into a new edition, so a trim refuses that name rather than reuse it.
func (r *Repo) PathInLibrary(ctx context.Context, path string) (bool, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM videos WHERE file_path = ?`, path).Scan(&n); err != nil {
		return false, fmt.Errorf("path in library: %w", err)
	}
	return n > 0, nil
}

// CloneVideoAsEdition creates or fills the videos row at targetPath from srcID, in one transaction,
// and returns its id. A scanner that saw the file first has already made the row, and running again
// after a crash finds its own, so every step tolerates existing rows and nothing is duplicated.
//
//   - A film link is kept, without its scene number: a film's numbers are unique, and the source
//     still holds that one. The owner can number the edition.
//   - edition becomes a manual decision, so neither the source's nor a stale filename read wins.
//   - The pair is labelled as editions, so it never enters the duplicates queue.
//
// Poster slot files live outside the database. The caller copies them, then presents the result.
func (r *Repo) CloneVideoAsEdition(ctx context.Context, srcID int64, targetPath, edition string) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	now := time.Now().UTC().Format(timeLayout)
	cols := fileLayerList("")
	res, err := tx.ExecContext(ctx, `
		INSERT INTO videos (file_path, `+cols+`, indexed_at, active)
		SELECT ?1, `+cols+`, ?2, 1 FROM videos WHERE id = ?3
		ON CONFLICT(file_path) DO UPDATE SET active = 1`, targetPath, now, srcID)
	if err != nil {
		return 0, fmt.Errorf("clone: video row: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return 0, err
	} else if n == 0 {
		return 0, fmt.Errorf("clone: %w: video %d", ErrNotFound, srcID)
	}
	var newID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM videos WHERE file_path = ?`, targetPath).Scan(&newID); err != nil {
		return 0, fmt.Errorf("clone: video id: %w", err)
	}

	steps := []struct{ name, sql string }{
		{"people", `INSERT OR IGNORE INTO video_people (video_id, person_id, role)
			SELECT ?1, person_id, role FROM video_people WHERE video_id = ?2`},
		{"tags", `INSERT INTO video_tags (video_id, tag_id, source)
			SELECT ?1, tag_id, source FROM video_tags WHERE video_id = ?2 AND true
			ON CONFLICT (video_id, tag_id) DO UPDATE SET source = excluded.source`},
		{"studios", `INSERT OR IGNORE INTO video_studios (video_id, studio_id)
			SELECT ?1, studio_id FROM video_studios WHERE video_id = ?2`},
		{"films", `INSERT OR IGNORE INTO film_videos (film_id, video_id, scene_number, is_full_film, created_at)
			SELECT film_id, ?1, NULL, is_full_film, created_at FROM film_videos WHERE video_id = ?2`},
		{"playlists", `INSERT OR IGNORE INTO playlist_videos (playlist_id, video_id, position)
			SELECT playlist_id, ?1, position FROM playlist_videos WHERE video_id = ?2`},
		{"curation", `INSERT OR IGNORE INTO metadata_curation
			(entity_type, entity_id, field_key, norm_value, value, action, source, created_at)
			SELECT entity_type, ?1, field_key, norm_value, value, action, source, created_at
			  FROM metadata_curation WHERE entity_type = 'video' AND entity_id = ?2`},
		{"decisions", `INSERT OR IGNORE INTO field_source_decisions
			(entity_type, entity_id, field_key, source, manual_value, created_at)
			SELECT entity_type, ?1, field_key, source, manual_value, created_at
			  FROM field_source_decisions WHERE entity_type = 'video' AND entity_id = ?2 AND field_key <> 'edition'`},
		{"enrichment", `INSERT OR IGNORE INTO entity_enrichment
			(entity_type, entity_id, provider, field_key, value, external_id, fetched_at)
			SELECT entity_type, ?1, provider, field_key, value, external_id, fetched_at
			  FROM entity_enrichment WHERE entity_type = 'video' AND entity_id = ?2`},
		{"not-applicable", `INSERT OR IGNORE INTO facet_not_applicable (entity_type, entity_id, canonical_field, created_at)
			SELECT entity_type, ?1, canonical_field, created_at
			  FROM facet_not_applicable WHERE entity_type = 'video' AND entity_id = ?2`},
		{"dismissals", `INSERT OR IGNORE INTO enrichment_dismissals (entity_type, entity_id, provider, dismissed_at)
			SELECT entity_type, ?1, provider, dismissed_at
			  FROM enrichment_dismissals WHERE entity_type = 'video' AND entity_id = ?2`},
		// An owner's frame and upload slots copy. The automatic slot is generated for the new file.
		{"posters", `INSERT OR IGNORE INTO video_poster_sources (video_id, kind, hash, frame_at, updated_at)
			SELECT ?1, kind, hash, frame_at, updated_at FROM video_poster_sources
			 WHERE video_id = ?2 AND kind IN ('frame', 'upload')`},
	}
	for _, s := range steps {
		if _, err := tx.ExecContext(ctx, s.sql, newID, srcID); err != nil {
			return 0, fmt.Errorf("clone %s: %w", s.name, err)
		}
	}
	if err := upsertDecision(ctx, tx, model.EnrichEntityVideo, newID, "edition", fieldsource.Manual, edition); err != nil {
		return 0, fmt.Errorf("clone edition: %w", err)
	}
	lo, hi := orderPair(srcID, newID)
	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO entity_keep_separate (entity_type, id_lo, id_hi) VALUES (?, ?, ?)`,
		model.EnrichEntityVideo, lo, hi); err != nil {
		return 0, fmt.Errorf("clone: keep separate: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO video_trimmed_from (video_id, source_video_id, created_at) VALUES (?, ?, ?)
		ON CONFLICT (video_id) DO NOTHING`, newID, srcID, now); err != nil {
		return 0, fmt.Errorf("clone: trimmed from: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit clone: %w", err)
	}
	return newID, nil
}

// TrimResult is the last trim's outcome, shown on the page of the video it ran from until the
// owner dismisses it. Skipped is JSON, one {"end","reason"} per frame the screen check skipped.
type TrimResult struct {
	VideoID        int64
	Mode           string
	EditionName    string
	EditionVideoID *int64
	Skipped        string
	CreatedAt      time.Time
}

// SetTrimResult replaces the video's trim outcome.
func (r *Repo) SetTrimResult(ctx context.Context, t TrimResult) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if t.Skipped == "" {
		t.Skipped = "[]"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO video_trim_results (video_id, mode, edition_name, edition_video_id, skipped, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, t.VideoID, t.Mode, t.EditionName, t.EditionVideoID, t.Skipped,
		time.Now().UTC().Format(timeLayout))
	if err != nil {
		return fmt.Errorf("set trim result: %w", err)
	}
	return nil
}

// GetTrimResult returns the video's trim outcome, or nil when it has none.
func (r *Repo) GetTrimResult(ctx context.Context, videoID int64) (*TrimResult, error) {
	t := TrimResult{VideoID: videoID}
	var edition sql.NullInt64
	var created string
	err := r.db.QueryRowContext(ctx, `
		SELECT mode, edition_name, edition_video_id, skipped, created_at
		  FROM video_trim_results WHERE video_id = ?`, videoID).
		Scan(&t.Mode, &t.EditionName, &edition, &t.Skipped, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get trim result: %w", err)
	}
	if edition.Valid {
		t.EditionVideoID = &edition.Int64
	}
	t.CreatedAt, _ = time.Parse(timeLayout, created)
	return &t, nil
}

// DismissTrimResult removes the video's trim outcome.
func (r *Repo) DismissTrimResult(ctx context.Context, videoID int64) (bool, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `DELETE FROM video_trim_results WHERE video_id = ?`, videoID)
	if err != nil {
		return false, fmt.Errorf("dismiss trim result: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ClearReplaceTrimResult removes a trimmed replace's outcome. A saved edition's outcome stays.
func (r *Repo) ClearReplaceTrimResult(ctx context.Context, videoID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx, `DELETE FROM video_trim_results WHERE video_id = ? AND mode = 'replace'`, videoID); err != nil {
		return fmt.Errorf("clear trim result: %w", err)
	}
	return nil
}

// TrimmedFrom is the source an edition was trimmed from.
type TrimmedFrom struct {
	VideoID int64
	Title   string
}

// GetTrimmedFrom returns the video's trim source, or nil when it wasn't trimmed from one or the
// source no longer exists. A trashed source still shows, since it can be restored.
func (r *Repo) GetTrimmedFrom(ctx context.Context, videoID int64) (*TrimmedFrom, error) {
	var t TrimmedFrom
	err := r.db.QueryRowContext(ctx, `
		SELECT v.id, v.title FROM video_trimmed_from f JOIN videos v ON v.id = f.source_video_id
		 WHERE f.video_id = ?`, videoID).Scan(&t.VideoID, &t.Title)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get trimmed from: %w", err)
	}
	return &t, nil
}

// UpdateMaintenancePayload rewrites a running job's payload: a trim's journal of how far it got.
func (r *Repo) UpdateMaintenancePayload(ctx context.Context, jobID int64, payload string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.db.ExecContext(ctx, `UPDATE writeback_queue SET payload = ?, updated_at = ? WHERE id = ?`,
		payload, time.Now().UTC().Format(timeLayout), jobID)
	if err != nil {
		return fmt.Errorf("update maintenance payload: %w", err)
	}
	return nil
}

// MaintenancePayloads returns every maintenance row's payload, so a sweep can tell which saved
// captures a queued or failed job still needs.
func (r *Repo) MaintenancePayloads(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT payload FROM writeback_queue WHERE kind = ?`, WritebackKindMaintenance)
	if err != nil {
		return nil, fmt.Errorf("maintenance payloads: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
