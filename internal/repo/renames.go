package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Rename journal states and triggers (F86, migration 0068, docs/architecture/file-rename.md).
const (
	RenamePending = "pending"
	RenameDone    = "done"

	RenameTriggerManual = "manual"
	RenameTriggerUndo   = "undo"

	LockAuto  = "auto"
	LockOwner = "owner"
	LockUndo  = "undo"
)

// FileRename is one row of the rename journal. Names are base names in the video's own folder.
type FileRename struct {
	ID          int64
	VideoID     int64
	OldName     string
	NewName     string
	ViaName     string
	Template    string
	TokenValues map[string][]string
	Trigger     string
	UndoOf      int64 // 0 when the row is not an undo
	State       string
	CreatedAt   time.Time
}

// FileNameState is a video's name lock.
type FileNameState struct {
	Locked     bool
	LockSource string
	LockedAt   time.Time
}

const renameCols = `id, video_id, old_name, new_name, via_name, template, token_values, trigger,
	COALESCE(undo_of, 0), state, created_at`

func scanRename(s interface{ Scan(...any) error }) (FileRename, error) {
	var fr FileRename
	var tv, created string
	if err := s.Scan(&fr.ID, &fr.VideoID, &fr.OldName, &fr.NewName, &fr.ViaName, &fr.Template, &tv,
		&fr.Trigger, &fr.UndoOf, &fr.State, &created); err != nil {
		return fr, err
	}
	if err := json.Unmarshal([]byte(tv), &fr.TokenValues); err != nil {
		return fr, fmt.Errorf("rename %d token values: %w", fr.ID, err)
	}
	fr.CreatedAt, _ = time.Parse(timeLayout, created)
	return fr, nil
}

func (r *Repo) queryRenames(ctx context.Context, q string, args ...any) ([]FileRename, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query renames: %w", err)
	}
	defer rows.Close()
	var out []FileRename
	for rows.Next() {
		fr, err := scanRename(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, fr)
	}
	return out, rows.Err()
}

// LatestDoneRenames returns each video's latest done rename: the name Holodex last wrote.
func (r *Repo) LatestDoneRenames(ctx context.Context) (map[int64]FileRename, error) {
	rows, err := r.queryRenames(ctx, `SELECT `+renameCols+` FROM file_renames
		WHERE id IN (SELECT MAX(id) FROM file_renames WHERE state = 'done' GROUP BY video_id)`)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]FileRename, len(rows))
	for _, fr := range rows {
		out[fr.VideoID] = fr
	}
	return out, nil
}

// LatestDoneRename returns one video's latest done rename, or nil when it has none.
func (r *Repo) LatestDoneRename(ctx context.Context, videoID int64) (*FileRename, error) {
	fr, err := scanRename(r.db.QueryRowContext(ctx, `SELECT `+renameCols+` FROM file_renames
		WHERE video_id = ? AND state = 'done' ORDER BY id DESC LIMIT 1`, videoID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &fr, nil
}

// RenameHistory lists a video's done renames, oldest first (P1-2).
func (r *Repo) RenameHistory(ctx context.Context, videoID int64) ([]FileRename, error) {
	return r.queryRenames(ctx, `SELECT `+renameCols+` FROM file_renames
		WHERE video_id = ? AND state = 'done' ORDER BY id`, videoID)
}

// FileNameStates returns every video's name state that has one.
func (r *Repo) FileNameStates(ctx context.Context) (map[int64]FileNameState, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT video_id, locked, lock_source, locked_at FROM video_file_names`)
	if err != nil {
		return nil, fmt.Errorf("file name states: %w", err)
	}
	defer rows.Close()
	out := map[int64]FileNameState{}
	for rows.Next() {
		var id int64
		var st FileNameState
		var at string
		if err := rows.Scan(&id, &st.Locked, &st.LockSource, &at); err != nil {
			return nil, err
		}
		st.LockedAt, _ = time.Parse(timeLayout, at)
		out[id] = st
	}
	return out, rows.Err()
}

// SetNameLock locks or unlocks a video's name (P0-13). Unlocking clears the source.
func (r *Repo) SetNameLock(ctx context.Context, videoID int64, locked bool, source string, now time.Time) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	return setNameLock(ctx, r.db, videoID, locked, source, now)
}

func setNameLock(ctx context.Context, db execer, videoID int64, locked bool, source string, now time.Time) error {
	at := now.UTC().Format(timeLayout)
	if !locked {
		source, at = "", ""
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO video_file_names (video_id, locked, lock_source, locked_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(video_id) DO UPDATE SET locked = excluded.locked, lock_source = excluded.lock_source,
			locked_at = excluded.locked_at`, videoID, locked, source, at)
	if err != nil {
		return fmt.Errorf("set name lock %d: %w", videoID, err)
	}
	return nil
}

// HeldVideoIDs returns every video with a held-original row in any state (P0-8).
func (r *Repo) HeldVideoIDs(ctx context.Context) (map[int64]bool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT video_id FROM held_originals`)
	if err != nil {
		return nil, fmt.Errorf("held video ids: %w", err)
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// VideoPath is a video row's id and file path, whatever its state.
type VideoPath struct {
	ID       int64
	FilePath string
}

// AllVideoPaths returns every video row's path, active, missing or trashed: any of them holds its
// name in the folder, because file_path is unique (F86 P0-6).
func (r *Repo) AllVideoPaths(ctx context.Context) ([]VideoPath, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, file_path FROM videos`)
	if err != nil {
		return nil, fmt.Errorf("all video paths: %w", err)
	}
	defer rows.Close()
	var out []VideoPath
	for rows.Next() {
		var vp VideoPath
		if err := rows.Scan(&vp.ID, &vp.FilePath); err != nil {
			return nil, err
		}
		out = append(out, vp)
	}
	return out, rows.Err()
}
