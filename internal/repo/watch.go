package repo

import (
	"context"
	"fmt"
	"time"
)

// The owner's watch history (F89 P0-7 to P0-9, watch-history.md). Nothing here deletes: a
// video's purge cascades to its plays and summary, the only delete path (SEC-10).

// watchedExpr is whether a video_watch row (aliased w) reads as watched: the owner's mark
// when there is one, otherwise whether any play counted.
const watchedExpr = "COALESCE(w.manual_watched, w.play_count > 0)"

// RecordPlay stores one counted play: the row and the summary in one transaction, so the
// two can't disagree after a crash. A counted play clears the owner's mark (P0-9). A video
// that doesn't exist or is deleted is ErrNotFound.
func (r *Repo) RecordPlay(ctx context.Context, videoID int64, progress float64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	now := time.Now().UTC().Format(timeLayout)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("record play: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO video_plays (video_id, played_at, progress)
		SELECT id, ?, ? FROM videos WHERE id = ? AND deleted_at IS NULL`, now, progress, videoID)
	if err != nil {
		return fmt.Errorf("record play: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("record play: %w", err)
	} else if n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO video_watch (video_id, play_count, last_played_at) VALUES (?, 1, ?)
		ON CONFLICT (video_id) DO UPDATE SET
			play_count = play_count + 1, last_played_at = excluded.last_played_at,
			manual_watched = NULL, manual_at = NULL`, videoID, now); err != nil {
		return fmt.Errorf("record play: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("record play: %w", err)
	}
	return nil
}

// SetWatchedMark sets the owner's mark. It adds and removes no plays (SEC-10). A video that
// doesn't exist or is deleted is ErrNotFound.
func (r *Repo) SetWatchedMark(ctx context.Context, videoID int64, watched bool) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	mark := 0
	if watched {
		mark = 1
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO video_watch (video_id, manual_watched, manual_at)
		SELECT id, ?, ? FROM videos WHERE id = ? AND deleted_at IS NULL
		ON CONFLICT (video_id) DO UPDATE SET
			manual_watched = excluded.manual_watched, manual_at = excluded.manual_at`,
		mark, time.Now().UTC().Format(timeLayout), videoID)
	if err != nil {
		return fmt.Errorf("set watched mark: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("set watched mark: %w", err)
	} else if n == 0 {
		return ErrNotFound
	}
	return nil
}

// WatchedVideos returns which of ids read as watched, for the owner's watched mark.
func (r *Repo) WatchedVideos(ctx context.Context, ids []int64) (map[int64]bool, error) {
	out := make(map[int64]bool)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT w.video_id FROM video_watch w
		WHERE `+watchedExpr+` AND w.video_id IN (`+placeholders(len(ids))+`)`, toAnySlice(ids)...)
	if err != nil {
		return nil, fmt.Errorf("watched videos: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
