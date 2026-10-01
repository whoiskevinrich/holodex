package repo

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"holodex/internal/model"
)

// Video playlists (F69, ADR-104): a container, not an entity. A playlist is
// membership + a sort (D2): playlist_videos is a set with a position that only
// the 'manual' sort reads, and every read order comes from the same orderBy the
// browse list uses, so a snapshot playlist and a curated one are indistinguishable
// at read time. Reads join the ADR-037 `active = 1 AND deleted_at IS NULL` seam so
// trash is inherited, never restated (D5). Nothing here touches the identity
// spine, the resolver, or enrichment.

// playlistVisibility is the SQL filter a visitor's read applies (ADR-104 D5).
// publicOnly=false (the owner) applies nothing.
func playlistVisibility(publicOnly bool) string {
	if publicOnly {
		return ` AND p.visibility = '` + model.PlaylistPublic + `'`
	}
	return ""
}

// liveMemberCount counts a playlist's members that browse would show — the
// trash seam applied in the count so a trashed video drops out of item_count the
// moment it is trashed and returns on restore.
const liveMemberCount = `(SELECT COUNT(*) FROM playlist_videos pv
	JOIN videos v ON v.id = pv.video_id
	WHERE pv.playlist_id = p.id AND v.active = 1 AND v.deleted_at IS NULL)`

const playlistColumns = `p.id, p.name, p.sort, p.visibility, p.query, p.query_version, p.play_shuffled,
	p.created_at, p.updated_at, ` + liveMemberCount

func scanPlaylist(s rowScanner) (model.Playlist, error) {
	var p model.Playlist
	var query sql.NullString
	var version sql.NullInt64
	err := s.Scan(&p.ID, &p.Name, &p.Sort, &p.Visibility, &query, &version, &p.PlayShuffled,
		&p.CreatedAt, &p.UpdatedAt, &p.ItemCount)
	if query.Valid {
		p.Query = &query.String
		v := int(version.Int64)
		p.QueryVersion = &v
	}
	return p, err
}

// ErrSmartPlaylist refuses a membership write to a smart playlist, which has no
// membership (ADR-121 D1, spec P0-6).
var ErrSmartPlaylist = errors.New("smart playlist has no membership")

// PlaylistIDFacets maps each browse id facet a stored query can name to the table its
// ids live in (ADR-121 D5): the keys a merge rewrites and the read-time stale check
// looks up. All four tables hard-delete, so a missing row is a deleted (or merged-away)
// entity.
var PlaylistIDFacets = map[string]string{
	"person":      "people",
	"tag":         "tags",
	"studio_id":   "studios",
	"category_id": "categories",
}

// mergeFacetKey is the stored-query key a merge of each entity type rewrites (D5).
// Films have no browse facet, so a film merge touches no stored query.
var mergeFacetKey = map[string]string{
	model.EnrichEntityPerson: "person",
	model.EnrichEntityStudio: "studio_id",
	model.EntityTag:          "tag",
}

// EncodePlaylistQuery is the normalising half of the canonical stored query (ADR-121
// D2): empty values dropped, repeated values de-duplicated, id facets sorted
// numerically and other repeated keys lexically, keys sorted by url.Values.Encode. The
// API's canonicalPlaylistQuery validates first; the merge rewrite re-normalises with
// this alone, so both writers produce the same string for the same set. Id values must
// already be valid positive integers.
func EncodePlaylistQuery(q url.Values) string {
	out := url.Values{}
	for key, vals := range q {
		seen := map[string]bool{}
		var kept []string
		for _, v := range vals {
			if v == "" || seen[v] {
				continue
			}
			seen[v] = true
			kept = append(kept, v)
		}
		if len(kept) == 0 {
			continue
		}
		if _, isID := PlaylistIDFacets[key]; isID {
			slices.SortFunc(kept, func(a, b string) int {
				x, _ := strconv.ParseInt(a, 10, 64)
				y, _ := strconv.ParseInt(b, 10, 64)
				return cmp.Compare(x, y)
			})
		} else {
			slices.Sort(kept)
		}
		out[key] = kept
	}
	return out.Encode()
}

// ListPlaylists returns every playlist (owner) or only the public ones (visitor),
// most recently updated first.
func (r *Repo) ListPlaylists(ctx context.Context, publicOnly bool) ([]model.Playlist, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+playlistColumns+` FROM playlists p WHERE 1=1`+playlistVisibility(publicOnly)+
			` ORDER BY p.updated_at DESC, p.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list playlists: %w", err)
	}
	defer rows.Close()
	out := []model.Playlist{} // `[]`, never `null` (HOLODEX-275)
	for rows.Next() {
		p, err := scanPlaylist(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPlaylist returns one playlist. For a visitor (publicOnly) a private playlist
// is ErrNotFound — indistinguishable from an unknown id, so existence never leaks
// (ADR-104 D5).
func (r *Repo) GetPlaylist(ctx context.Context, id int64, publicOnly bool) (*model.Playlist, error) {
	p, err := scanPlaylist(r.db.QueryRowContext(ctx,
		`SELECT `+playlistColumns+` FROM playlists p WHERE p.id = ?`+playlistVisibility(publicOnly), id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get playlist: %w", err)
	}
	return &p, nil
}

// CountPublicPlaylists is the ungated /capabilities count the SPA uses to hide
// the nav item from a visitor with nothing to see (spec P0-5).
func (r *Repo) CountPublicPlaylists(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM playlists WHERE visibility = ?`, model.PlaylistPublic).Scan(&n)
	return n, err
}

// CreatePlaylist creates a playlist and, when videoIDs is non-empty, its
// membership in the same transaction with position = rank in videoIDs (the
// snapshot producer, ADR-104 D3). A non-nil query makes it a smart playlist
// (ADR-121 D1) at the current query version; the caller passes no videoIDs then.
// Validation of name/sort/visibility/query is the handler's; this trusts its arguments.
func (r *Repo) CreatePlaylist(ctx context.Context, name, sort, visibility string, query *string, videoIDs []int64) (*model.Playlist, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var version *int
	if query != nil {
		v := model.PlaylistQueryVersion
		version = &v
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO playlists (name, sort, visibility, query, query_version) VALUES (?, ?, ?, ?, ?)`,
		name, sort, visibility, query, version)
	if err != nil {
		return nil, fmt.Errorf("create playlist: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if len(videoIDs) > 0 {
		if err := insertPlaylistVideos(ctx, tx, id, videoIDs); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPlaylist(ctx, id, false)
}

// insertPlaylistVideos appends videoIDs in order after the playlist's current
// last position, in one multi-row statement. INSERT OR IGNORE keeps the set
// property: a repeated id keeps its first position.
func insertPlaylistVideos(ctx context.Context, tx *sql.Tx, playlistID int64, videoIDs []int64) error {
	var base int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), 0) FROM playlist_videos WHERE playlist_id = ?`, playlistID).Scan(&base); err != nil {
		return fmt.Errorf("playlist position: %w", err)
	}
	// SQLite's default variable cap is 32766 (three per row); a personal library
	// is far below it, but chunk anyway so a large snapshot can never hit it.
	const chunk = 5000
	for start := 0; start < len(videoIDs); start += chunk {
		end := min(start+chunk, len(videoIDs))
		var sb strings.Builder
		sb.WriteString(`INSERT OR IGNORE INTO playlist_videos (playlist_id, video_id, position) VALUES `)
		args := make([]any, 0, 3*(end-start))
		for i, vid := range videoIDs[start:end] {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("(?, ?, ?)")
			base++
			args = append(args, playlistID, vid, base)
		}
		if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
			return fmt.Errorf("insert playlist videos: %w", err)
		}
	}
	return nil
}

// PlaylistPatch carries the optional fields UpdatePlaylist sets; nil = unchanged.
// Query replaces a smart playlist's stored query (Edit filter → Update); the handler
// only sends it for a playlist that is already smart.
type PlaylistPatch struct {
	Name, Sort, Visibility, Query *string
	PlayShuffled                  *bool
}

// UpdatePlaylist applies patch and bumps updated_at. ErrNotFound for an unknown id.
func (r *Repo) UpdatePlaylist(ctx context.Context, id int64, patch PlaylistPatch) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	sets := []string{`updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`}
	var args []any
	if patch.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *patch.Name)
	}
	if patch.Sort != nil {
		sets = append(sets, "sort = ?")
		args = append(args, *patch.Sort)
	}
	if patch.Visibility != nil {
		sets = append(sets, "visibility = ?")
		args = append(args, *patch.Visibility)
	}
	if patch.Query != nil {
		sets = append(sets, "query = ?", "query_version = ?")
		args = append(args, *patch.Query, model.PlaylistQueryVersion)
	}
	if patch.PlayShuffled != nil {
		sets = append(sets, "play_shuffled = ?")
		args = append(args, *patch.PlayShuffled)
	}
	args = append(args, id)
	res, err := r.db.ExecContext(ctx, `UPDATE playlists SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update playlist: %w", err)
	}
	return rowsAffectedOrNotFound(res)
}

// DeletePlaylist removes a playlist; playlist_videos cascades. Videos are untouched.
func (r *Repo) DeletePlaylist(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	res, err := r.db.ExecContext(ctx, `DELETE FROM playlists WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete playlist: %w", err)
	}
	return rowsAffectedOrNotFound(res)
}

// AddPlaylistVideo appends videoID to playlistID (idempotent — an existing member
// keeps its position, spec RD9) and bumps updated_at. ErrNotFound when either the
// playlist or the video does not exist; a soft-deleted video is still a valid
// target (its membership simply stays hidden until restore, ADR-104 D5).
func (r *Repo) AddPlaylistVideo(ctx context.Context, playlistID, videoID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists, smart int
	if err := tx.QueryRowContext(ctx,
		`SELECT (SELECT COUNT(*) FROM playlists WHERE id = ?) * (SELECT COUNT(*) FROM videos WHERE id = ?),
		        (SELECT COUNT(*) FROM playlists WHERE id = ? AND query IS NOT NULL)`,
		playlistID, videoID, playlistID).Scan(&exists, &smart); err != nil {
		return err
	}
	if exists == 0 {
		return ErrNotFound
	}
	if smart > 0 {
		return ErrSmartPlaylist
	}
	if err := insertPlaylistVideos(ctx, tx, playlistID, []int64{videoID}); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE playlists SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?`, playlistID); err != nil {
		return err
	}
	return tx.Commit()
}

// RemovePlaylistVideo drops one membership row. ErrNotFound when it was not a member;
// ErrSmartPlaylist for a smart playlist, which has no members to remove.
func (r *Repo) RemovePlaylistVideo(ctx context.Context, playlistID, videoID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	var smart int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM playlists WHERE id = ? AND query IS NOT NULL`, playlistID).Scan(&smart); err != nil {
		return err
	}
	if smart > 0 {
		return ErrSmartPlaylist
	}
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM playlist_videos WHERE playlist_id = ? AND video_id = ?`, playlistID, videoID)
	if err != nil {
		return fmt.Errorf("remove playlist video: %w", err)
	}
	if err := rowsAffectedOrNotFound(res); err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE playlists SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?`, playlistID)
	return err
}

// PlaylistVideos returns a playlist's un-trashed members in the playlist's order:
// 'manual' → position; any browse sort key → the same orderBy browse uses (seed
// parameterizes 'random', ADR-045). Rows are hydrated like ListVideos so the
// browse tile renders them unchanged.
func (r *Repo) PlaylistVideos(ctx context.Context, playlistID int64, sort string, seed int64) ([]model.Video, error) {
	orderClause, orderArgs := VideoFilter{Sort: sort, Seed: seed}.orderBy()
	if sort == model.PlaylistSortManual {
		orderClause, orderArgs = "pv.position ASC, v.id ASC", nil
	}
	q := `SELECT v.id, v.file_path, v.file_size, v.title, v.duration_sec, v.width,
	             v.height, v.video_codec, v.audio_codec, v.bitrate_kbps, v.container,
	             v.recorded_at, v.indexed_at, v.file_mtime, v.thumbnail_state
	      FROM playlist_videos pv
	      JOIN videos v ON v.id = pv.video_id
	      WHERE pv.playlist_id = ? AND v.active = 1 AND v.deleted_at IS NULL
	      ORDER BY ` + orderClause
	args := append([]any{playlistID}, orderArgs...)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("playlist videos: %w", err)
	}
	defer rows.Close()
	out := []model.Video{}
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachAssociations(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// PlaylistsForVideo returns the playlists a video belongs to (owner: all;
// visitor: public only) — the detail rail's PLAYLISTS row (spec P0-7).
func (r *Repo) PlaylistsForVideo(ctx context.Context, videoID int64, publicOnly bool) ([]model.Playlist, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+playlistColumns+` FROM playlists p
		 JOIN playlist_videos pv ON pv.playlist_id = p.id
		 WHERE pv.video_id = ?`+playlistVisibility(publicOnly)+`
		 ORDER BY p.updated_at DESC, p.id DESC`, videoID)
	if err != nil {
		return nil, fmt.Errorf("playlists for video: %w", err)
	}
	defer rows.Close()
	out := []model.Playlist{}
	for rows.Next() {
		p, err := scanPlaylist(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// FreezePlaylist turns a smart playlist into an ADR-104 snapshot (ADR-121 D1, spec
// P0-7): videoIDs — the query's result in the playlist's order, evaluated by the
// caller — become its membership with position = rank, and query/query_version are
// nulled, in one transaction. ErrNotFound for an unknown id; ErrNotSmart if it is
// already a snapshot (a second Freeze must not append the set again).
func (r *Repo) FreezePlaylist(ctx context.Context, id int64, videoIDs []int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE playlists SET query = NULL, query_version = NULL,
		        updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		 WHERE id = ? AND query IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("freeze playlist: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM playlists WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNotFound
		}
		return ErrNotSmart
	}
	if err := insertPlaylistVideos(ctx, tx, id, videoIDs); err != nil {
		return err
	}
	return tx.Commit()
}

// ErrNotSmart refuses a smart-only operation (Freeze, a query update) on a snapshot playlist.
var ErrNotSmart = errors.New("not a smart playlist")

// rewriteSmartPlaylistRefs follows a merge into every stored query (ADR-121 D5): each
// smart playlist naming mergedID under the entity type's facet key is rewritten to
// canonicalID and re-normalised. Runs inside the merge's transaction so a merge and
// its query rewrite land together. Smart playlists number in the tens, so a scan of
// them is cheaper than keeping a second copy of their references.
func rewriteSmartPlaylistRefs(ctx context.Context, tx *sql.Tx, entityType string, canonicalID, mergedID int64) error {
	key, ok := mergeFacetKey[entityType]
	if !ok {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, query FROM playlists WHERE query IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("smart playlist refs: %w", err)
	}
	type change struct {
		id    int64
		query string
	}
	var changes []change
	from, to := strconv.FormatInt(mergedID, 10), strconv.FormatInt(canonicalID, 10)
	for rows.Next() {
		var id int64
		var stored string
		if err := rows.Scan(&id, &stored); err != nil {
			rows.Close()
			return err
		}
		q, err := url.ParseQuery(stored)
		if err != nil || !slices.Contains(q[key], from) {
			continue // a stored query is always canonical; an unparsable one is left for the stale check
		}
		for i, v := range q[key] {
			if v == from {
				q[key][i] = to
			}
		}
		changes = append(changes, change{id, EncodePlaylistQuery(q)})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range changes {
		if _, err := tx.ExecContext(ctx, `UPDATE playlists SET query = ? WHERE id = ?`, c.query, c.id); err != nil {
			return fmt.Errorf("rewrite smart playlist %d: %w", c.id, err)
		}
	}
	return nil
}

// MissingFacetIDs returns, per id facet key, the ids in refs whose entity no longer
// exists (ADR-121 D5's read-time stale check). Keys outside PlaylistIDFacets are ignored.
func (r *Repo) MissingFacetIDs(ctx context.Context, refs map[string][]int64) (map[string][]int64, error) {
	missing := map[string][]int64{}
	for key, ids := range refs {
		table, ok := PlaylistIDFacets[key]
		if !ok || len(ids) == 0 {
			continue
		}
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		rows, err := r.db.QueryContext(ctx,
			`SELECT id FROM `+table+` WHERE id IN (`+placeholders(len(ids))+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("stale refs %s: %w", key, err)
		}
		found := map[int64]bool{}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			found[id] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for _, id := range ids {
			if !found[id] {
				missing[key] = append(missing[key], id)
			}
		}
	}
	return missing, nil
}

func rowsAffectedOrNotFound(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
