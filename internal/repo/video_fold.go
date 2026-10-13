package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Healing moves that already happened (F79 P0-5, media-ingest.md "Moves
// recognised by a sampled content hash"): a hidden older row and a visible newer
// row with the same content are one video whose file moved before Holodex could
// tell. The newer row is folded into the older one, which keeps its id (the
// arrival order HOLODEX-567's Date added sorts by) and its indexed_at.

// ErrFoldNotLive is returned when a fold targets a pair that no longer qualifies
// (a side was shown, hidden, trashed or kept separate since it was listed), or
// that has writeback or file-maintenance work in flight.
var ErrFoldNotLive = errors.New("fold pair no longer qualifies")

// FoldPair is a hidden older row and the visible newer row to fold into it.
type FoldPair struct {
	Older, Newer int64
}

// sameContentSQL is the one test of "the same video" (F79 P0-1) between a stored
// row h and a file or newer row n: equal byte size, and the fingerprint when h
// has one, otherwise every recorded fact (the legacy rule for rows with none).
// MoveCandidates and the fold both use it, so the two can't disagree.
const sameContentSQL = `
	h.file_size = n.file_size
	AND ((h.content_fp IS NOT NULL AND h.content_fp = n.content_fp)
	  OR (h.content_fp IS NULL
	      AND h.file_mtime = n.file_mtime AND h.duration_sec = n.duration_sec
	      AND h.width = n.width AND h.height = n.height
	      AND h.video_codec = n.video_codec AND h.audio_codec = n.audio_codec
	      AND h.bitrate_kbps = n.bitrate_kbps AND h.container = n.container
	      AND (h.file_tags IS NULL OR h.file_tags = n.file_tags)))`

// foldEvidenceSQL pairs hidden row h with a visible newer row n of the same
// content, unless the two were kept separate.
const foldEvidenceSQL = `
	n.id > h.id
	AND h.active = 0 AND h.deleted_at IS NULL
	AND n.active = 1 AND n.deleted_at IS NULL
	AND ` + sameContentSQL + `
	AND NOT EXISTS (SELECT 1 FROM entity_keep_separate ks
	                 WHERE ks.entity_type = 'video' AND ks.id_lo = h.id AND ks.id_hi = n.id)`

// foldIdleSQL holds a pair back while either side has queued or running
// writeback or maintenance work, or a held original: those jobs carry the id
// and the path, and folding under them would race the file swap. A later pass
// folds it once they settle.
const foldIdleSQL = `
	NOT EXISTS (SELECT 1 FROM writeback_queue q
	             WHERE q.video_id IN (h.id, n.id) AND q.status IN ('pending', 'running'))
	AND NOT EXISTS (SELECT 1 FROM held_originals ho WHERE ho.video_id IN (h.id, n.id))`

// FoldPairs lists the pairs to fold and the newer rows left out as ambiguous: a
// pair folds only when it is one-to-one on both sides (P0-4).
func (r *Repo) FoldPairs(ctx context.Context) (pairs []FoldPair, ambiguousNewer []int64, err error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT h.id, n.id FROM videos h JOIN videos n ON `+foldEvidenceSQL+`
		 WHERE `+foldIdleSQL+`
		 ORDER BY h.id, n.id`)
	if err != nil {
		return nil, nil, fmt.Errorf("fold pairs: %w", err)
	}
	defer rows.Close()
	var all []FoldPair
	perOlder, perNewer := map[int64]int{}, map[int64]int{}
	for rows.Next() {
		var p FoldPair
		if err := rows.Scan(&p.Older, &p.Newer); err != nil {
			return nil, nil, fmt.Errorf("scan fold pair: %w", err)
		}
		all = append(all, p)
		perOlder[p.Older]++
		perNewer[p.Newer]++
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	skipped := map[int64]bool{}
	for _, p := range all {
		if perOlder[p.Older] == 1 && perNewer[p.Newer] == 1 {
			pairs = append(pairs, p)
		} else if !skipped[p.Newer] {
			skipped[p.Newer] = true
			ambiguousNewer = append(ambiguousNewer, p.Newer)
		}
	}
	return pairs, ambiguousNewer, nil
}

// FoldMovedVideo folds newer into older in one transaction under the write lock
// and hard-deletes newer. Everything attached to newer is carried BEFORE the
// delete, because the delete's cascades and the videos_ad_* triggers remove
// whatever is still keyed to it:
//
//   - provider data: per provider, the more recently fetched side wins;
//   - dismissed provider suggestions: the union;
//   - owner edits (decisions, curation, not-applicable marks) per field: newer
//     wins wherever it has one. That is "the newest edit wins" (P0-6): older
//     was hidden from the move until now, so every edit on newer came after
//     every edit on older;
//   - playlist places, film links and manual tags: via applyCarry, as for a
//     duplicate kept as one (older keeps its own position or link on a clash);
//   - writeback history, snapshots, extraction reviews and job attribution:
//     moved to older.
//
// older then takes newer's file path, facts and fingerprint and is shown again;
// its id and indexed_at stay. ErrFoldNotLive when the pair no longer qualifies.
func (r *Repo) FoldMovedVideo(ctx context.Context, older, newer int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var ok bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM videos h JOIN videos n ON `+foldEvidenceSQL+`
		               WHERE h.id = ? AND n.id = ? AND `+foldIdleSQL+`)`, older, newer).Scan(&ok); err != nil {
		return fmt.Errorf("fold %d<-%d: check: %w", older, newer, err)
	}
	if !ok {
		return ErrFoldNotLive
	}

	steps := []struct{ name, sql string }{
		// Provider data first, so carryDecisionOK sees the providers newer brings.
		{"enrichment: drop older's stale providers", `
			DELETE FROM entity_enrichment
			 WHERE entity_type = 'video' AND entity_id = ?1
			   AND provider IN (
			       SELECT n.provider FROM entity_enrichment n
			        WHERE n.entity_type = 'video' AND n.entity_id = ?2
			        GROUP BY n.provider
			       HAVING max(n.fetched_at) > coalesce((
			              SELECT max(o.fetched_at) FROM entity_enrichment o
			               WHERE o.entity_type = 'video' AND o.entity_id = ?1 AND o.provider = n.provider), ''))`},
		{"enrichment: move newer's winning providers", `
			UPDATE entity_enrichment SET entity_id = ?1
			 WHERE entity_type = 'video' AND entity_id = ?2
			   AND provider NOT IN (SELECT provider FROM entity_enrichment
			                         WHERE entity_type = 'video' AND entity_id = ?1)`},
		{"dismissals", `UPDATE OR IGNORE enrichment_dismissals SET entity_id = ?1
			WHERE entity_type = 'video' AND entity_id = ?2`},
	}
	for _, s := range steps {
		if _, err := tx.ExecContext(ctx, s.sql, older, newer); err != nil {
			return fmt.Errorf("fold %d<-%d: %s: %w", older, newer, s.name, err)
		}
	}

	fields, err := foldWinningFields(ctx, tx, older, newer)
	if err != nil {
		return err
	}
	if len(fields) > 0 {
		args := []any{older}
		for _, f := range fields {
			args = append(args, f)
		}
		in := numberedPlaceholders(2, len(fields))
		for _, s := range []struct{ name, sql string }{
			{"decisions", `DELETE FROM field_source_decisions WHERE entity_type = 'video' AND entity_id = ?1 AND field_key IN (` + in + `)`},
			{"curation", `DELETE FROM metadata_curation WHERE entity_type = 'video' AND entity_id = ?1 AND field_key IN (` + in + `)`},
			{"not-applicable", `DELETE FROM facet_not_applicable WHERE entity_type = 'video' AND entity_id = ?1 AND canonical_field IN (` + in + `)`},
		} {
			if _, err := tx.ExecContext(ctx, s.sql, args...); err != nil {
				return fmt.Errorf("fold %d<-%d: clear older %s: %w", older, newer, s.name, err)
			}
		}
	}
	if err := applyCarry(ctx, tx, older, newer, fields); err != nil {
		return fmt.Errorf("fold %d<-%d: %w", older, newer, err)
	}

	for _, s := range []struct{ name, sql string }{
		{"writeback queue", `UPDATE writeback_queue SET video_id = ?1 WHERE video_id = ?2`},
		{"writeback history", `UPDATE file_writebacks SET video_id = ?1 WHERE video_id = ?2`},
		{"writeback snapshots", `UPDATE file_writeback_snapshots SET video_id = ?1 WHERE video_id = ?2`},
		{"extraction reviews", `UPDATE metadata_extraction_review SET video_id = ?1 WHERE video_id = ?2`},
		{"job attribution", `UPDATE job_runs SET entity_id = ?1 WHERE entity_type = 'video' AND entity_id = ?2`},
		{"keep-separate", `DELETE FROM entity_keep_separate
			WHERE entity_type = 'video' AND (id_lo = ?2 OR id_hi = ?2)`},
	} {
		if _, err := tx.ExecContext(ctx, s.sql, older, newer); err != nil {
			return fmt.Errorf("fold %d<-%d: %s: %w", older, newer, s.name, err)
		}
	}

	// older takes newer's file layer. Copy everything but the path first, while
	// newer still exists; file_path is unique, so it moves only after the delete.
	fileCols := fileLayerList("") + ", content_fp, file_tags"
	if _, err := tx.ExecContext(ctx, `
		UPDATE videos SET (`+fileCols+`) = (SELECT `+fileCols+` FROM videos WHERE id = ?)
		 WHERE id = ?`, newer, older); err != nil {
		return fmt.Errorf("fold %d<-%d: take file layer: %w", older, newer, err)
	}
	var path string
	if err := tx.QueryRowContext(ctx, `SELECT file_path FROM videos WHERE id = ?`, newer).Scan(&path); err != nil {
		return fmt.Errorf("fold %d<-%d: read newer path: %w", older, newer, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM videos WHERE id = ?`, newer); err != nil {
		return fmt.Errorf("fold %d<-%d: delete newer: %w", older, newer, err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE videos SET file_path = ?, active = 1 WHERE id = ?`, path, older); err != nil {
		return fmt.Errorf("fold %d<-%d: take path: %w", older, newer, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit fold %d<-%d: %w", older, newer, err)
	}
	return nil
}

// foldWinningFields returns the fields whose owner edits newer carries over
// older's: every field newer has a readable edit on. A provider-pinned decision
// newer can't carry (carryDecisionOK) doesn't count, so it never clears older's.
func foldWinningFields(ctx context.Context, tx *sql.Tx, older, newer int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT d.field_key FROM field_source_decisions d
		 WHERE d.entity_type = 'video' AND d.entity_id = ?2 AND `+carryDecisionOK+`
		UNION SELECT field_key FROM metadata_curation WHERE entity_type = 'video' AND entity_id = ?2
		UNION SELECT canonical_field FROM facet_not_applicable WHERE entity_type = 'video' AND entity_id = ?2
		ORDER BY 1`, older, newer)
	if err != nil {
		return nil, fmt.Errorf("fold %d<-%d: winning fields: %w", older, newer, err)
	}
	defer rows.Close()
	var fields []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		fields = append(fields, k)
	}
	return fields, rows.Err()
}
