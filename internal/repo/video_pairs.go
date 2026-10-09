package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"holodex/internal/fieldsource"
	"holodex/internal/model"
)

// Duplicate videos (F76, HOLODEX-521): two live files matched to the same item from the
// same provider. Unlike the entity duplicates queue, video pairs are computed on demand
// rather than stored — a pair's liveness (trash, restore, re-match, a file going missing)
// is a property of the files, so a stored row would need a cleanup hook on every one of
// those paths. Keep both is an entity_keep_separate('video') row; keep one moves the other
// file to Trash and carries the owner's work to the kept copy in one transaction.

// ErrVideoPairNotLive is returned when a keep-one or label targets a pair that no longer
// exists: a side was trashed, re-matched or kept-both since the owner loaded it.
var ErrVideoPairNotLive = errors.New("video pair is no longer a duplicate")

// videoSharedMatchSQL lists every live pair of videos whose current match for one provider
// is the same external id, minus kept-both pairs. A video's current match per provider is
// its newest non-empty memo (the F71 rule): a narrower re-enrich can leave older rows
// carrying an older id behind. Keyed on the provider column, so the same id from two
// providers never pairs. HOLODEX-457 re-homes this memo; this is the one place to change.
const videoSharedMatchSQL = `
WITH winner AS (
    SELECT e.entity_id AS vid, e.provider,
           (SELECT f.external_id
              FROM entity_enrichment f
             WHERE f.entity_type = 'video'
               AND f.entity_id   = e.entity_id
               AND f.provider    = e.provider
               AND f.external_id <> ''
             ORDER BY f.fetched_at DESC, f.external_id
             LIMIT 1) AS ext
      FROM entity_enrichment e
      JOIN videos v ON v.id = e.entity_id AND v.active = 1 AND v.deleted_at IS NULL
     WHERE e.entity_type = 'video'
       AND e.external_id <> ''
     GROUP BY e.entity_id, e.provider
)
SELECT a.vid, b.vid,
       -- The shared provider item's own title: the one label true of both files (F76).
       -- min(a.provider) makes SQLite take a.provider/a.ext from that same row.
       min(a.provider),
       coalesce((SELECT t.value FROM entity_enrichment t
                  WHERE t.entity_type = 'video' AND t.entity_id IN (a.vid, b.vid)
                    AND t.provider = a.provider AND t.external_id = a.ext
                    AND t.field_key = 'title' AND t.value <> ''
                  ORDER BY t.fetched_at DESC LIMIT 1), '')
  FROM winner a
  JOIN winner b ON a.provider = b.provider AND a.ext = b.ext AND a.vid < b.vid
 WHERE NOT EXISTS (SELECT 1 FROM entity_keep_separate ks
                    WHERE ks.entity_type = 'video' AND ks.id_lo = a.vid AND ks.id_hi = b.vid)
 GROUP BY a.vid, b.vid
 ORDER BY a.vid, b.vid`

// carryExcludedFields never carry: part and edition describe the trashed file, not the
// scene (spec RD5).
const carryExcludedFields = `'edition', 'part'`

// VideoPair is one listed duplicate: the two video ids, low first, and the shared provider
// item's title (” when the provider gave none).
type VideoPair struct {
	A     int64
	B     int64
	Title string
}

// VideoFileFacts are the file-level facts the compare panel lines up per side.
type VideoFileFacts struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	FilePath    string `json:"file_path"`
	FileSize    int64  `json:"file_size"`
	Duration    int    `json:"duration_sec"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	VideoCodec  string `json:"video_codec,omitempty"`
	BitrateKbps int    `json:"bitrate_kbps,omitempty"`
	Container   string `json:"container,omitempty"`
}

// FilmLink is one film membership as the compare panel and the confirm name it.
type FilmLink struct {
	FilmID      int64  `json:"film_id"`
	Name        string `json:"name"`
	SceneNumber *int64 `json:"scene_number,omitempty"`
	IsFullFilm  bool   `json:"is_full_film"`
}

// VideoWork summarizes the owner's own work on one video ("Your work" row).
type VideoWork struct {
	Playlists  int        `json:"playlists"`
	Films      []FilmLink `json:"films"`
	Edits      int        `json:"edits"`
	ManualTags int        `json:"manual_tags"`
}

// CarryPreview is what keeping one copy would move onto it from the other — and, from
// KeepOneVideo, what did move. Both come from planCarry, so they cannot disagree.
type CarryPreview struct {
	Playlists  int        `json:"playlists"`
	Films      []FilmLink `json:"films"`
	Edits      int        `json:"edits"`
	ManualTags int        `json:"manual_tags"`
}

type carryQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// ListVideoPairs returns every live duplicate-video pair, low id first.
func (r *Repo) ListVideoPairs(ctx context.Context) ([]VideoPair, error) {
	return listVideoPairs(ctx, r.db)
}

func listVideoPairs(ctx context.Context, q carryQuerier) ([]VideoPair, error) {
	rows, err := q.QueryContext(ctx, videoSharedMatchSQL)
	if err != nil {
		return nil, fmt.Errorf("list video pairs: %w", err)
	}
	defer rows.Close()
	var out []VideoPair
	for rows.Next() {
		var (
			p        VideoPair
			provider string
		)
		if err := rows.Scan(&p.A, &p.B, &provider, &p.Title); err != nil {
			return nil, err
		}
		// A title is one value, but the column joins multi-values; keep the first.
		p.Title, _, _ = strings.Cut(p.Title, enrichMultiSep)
		out = append(out, p)
	}
	return out, rows.Err()
}

// VideoPairLive reports whether (a, b) is currently a listed duplicate pair.
func (r *Repo) VideoPairLive(ctx context.Context, a, b int64) (bool, error) {
	return videoPairLive(ctx, r.db, a, b)
}

func videoPairLive(ctx context.Context, q carryQuerier, a, b int64) (bool, error) {
	lo, hi := orderPair(a, b)
	pairs, err := listVideoPairs(ctx, q)
	if err != nil {
		return false, err
	}
	for _, p := range pairs {
		if p.A == lo && p.B == hi {
			return true, nil
		}
	}
	return false, nil
}

// VideoFacts loads the compare-panel facts for live videos, keyed by id. Ids that are
// missing or trashed are absent.
func (r *Repo) VideoFacts(ctx context.Context, ids []int64) (map[int64]VideoFileFacts, error) {
	out := make(map[int64]VideoFileFacts, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, title, file_path, file_size, duration_sec, width, height,
		       coalesce(video_codec, ''), coalesce(bitrate_kbps, 0), coalesce(container, '')
		  FROM videos
		 WHERE deleted_at IS NULL AND id IN (`+placeholders(len(ids))+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("video facts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var f VideoFileFacts
		if err := rows.Scan(&f.ID, &f.Title, &f.FilePath, &f.FileSize, &f.Duration, &f.Width,
			&f.Height, &f.VideoCodec, &f.BitrateKbps, &f.Container); err != nil {
			return nil, err
		}
		out[f.ID] = f
	}
	return out, rows.Err()
}

// VideoWorkFor summarizes the owner's own work on one video.
func (r *Repo) VideoWorkFor(ctx context.Context, id int64) (VideoWork, error) {
	var w VideoWork
	if err := r.db.QueryRowContext(ctx,
		`SELECT count(*) FROM playlist_videos WHERE video_id = ?`, id).Scan(&w.Playlists); err != nil {
		return w, fmt.Errorf("work playlists: %w", err)
	}
	films, err := filmLinks(ctx, r.db, `fv.video_id = ?1`, id)
	if err != nil {
		return w, err
	}
	w.Films = films
	if err := r.db.QueryRowContext(ctx, `
		SELECT count(*) FROM (
			SELECT field_key FROM field_source_decisions WHERE entity_type = 'video' AND entity_id = ?1
			UNION SELECT field_key FROM metadata_curation WHERE entity_type = 'video' AND entity_id = ?1
			UNION SELECT canonical_field FROM facet_not_applicable WHERE entity_type = 'video' AND entity_id = ?1)`,
		id).Scan(&w.Edits); err != nil {
		return w, fmt.Errorf("work edits: %w", err)
	}
	if err := r.db.QueryRowContext(ctx,
		`SELECT count(*) FROM video_tags WHERE video_id = ? AND source = ?`,
		id, fieldsource.Manual).Scan(&w.ManualTags); err != nil {
		return w, fmt.Errorf("work manual tags: %w", err)
	}
	return w, nil
}

// PreviewCarry reports what keeping keepID would move onto it from trashID.
func (r *Repo) PreviewCarry(ctx context.Context, keepID, trashID int64) (CarryPreview, error) {
	p, _, err := planCarry(ctx, r.db, keepID, trashID)
	return p, err
}

// The carry rules (spec P0-5), each one SELECT shared by the preview and the apply.
// ?1 is the kept video, ?2 the trashed one.
const (
	// A playlist place moves when the kept copy isn't already in that playlist.
	carryPlaylistWhere = `video_id = ?2 AND NOT EXISTS (
		SELECT 1 FROM playlist_videos k WHERE k.playlist_id = playlist_videos.playlist_id AND k.video_id = ?1)`
	// A film link moves when the kept copy has no link to that film.
	carryFilmWhere = `fv.video_id = ?2 AND NOT EXISTS (
		SELECT 1 FROM film_videos k WHERE k.film_id = fv.film_id AND k.video_id = ?1)`
	// A manual tag carries unless the kept copy already holds that tag durably; a
	// file-sourced link on the kept copy is upgraded so the tag survives a rescan.
	carryTagWhere = `t.video_id = ?2 AND t.source = 'manual' AND NOT EXISTS (
		SELECT 1 FROM video_tags k WHERE k.video_id = ?1 AND k.tag_id = t.tag_id AND k.source <> 'file')`
	// A decision pinned to a provider carries only when the kept copy is matched to that
	// provider; one pinned to a film only when either copy is linked to it (the link moves
	// first). Otherwise the kept copy would resolve the field to nothing and lose it.
	carryDecisionOK = `(d.source NOT LIKE 'provider:%'
		OR (d.source LIKE 'provider:film:%' AND EXISTS (SELECT 1 FROM film_videos cf
			WHERE cf.film_id = CAST(substr(d.source, 15) AS INTEGER) AND cf.video_id IN (?1, ?2)))
		OR (d.source NOT LIKE 'provider:film:%' AND EXISTS (SELECT 1 FROM entity_enrichment ce
			WHERE ce.entity_type = 'video' AND ce.entity_id = ?1 AND ce.provider = substr(d.source, 10))))`
	// A field's edits carry when the kept copy has no edit of any kind on that field.
	carryFieldsSQL = `
		SELECT d.field_key FROM field_source_decisions d
		 WHERE d.entity_type = 'video' AND d.entity_id = ?2 AND ` + carryDecisionOK + `
		UNION SELECT field_key FROM metadata_curation WHERE entity_type = 'video' AND entity_id = ?2
		UNION SELECT canonical_field FROM facet_not_applicable WHERE entity_type = 'video' AND entity_id = ?2
		EXCEPT SELECT field_key FROM field_source_decisions WHERE entity_type = 'video' AND entity_id = ?1
		EXCEPT SELECT field_key FROM metadata_curation WHERE entity_type = 'video' AND entity_id = ?1
		EXCEPT SELECT canonical_field FROM facet_not_applicable WHERE entity_type = 'video' AND entity_id = ?1`
)

func filmLinks(ctx context.Context, q carryQuerier, where string, args ...any) ([]FilmLink, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT fv.film_id, f.name, fv.scene_number, fv.is_full_film
		  FROM film_videos fv JOIN films f ON f.id = fv.film_id
		 WHERE `+where+`
		 ORDER BY f.name, fv.film_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("film links: %w", err)
	}
	defer rows.Close()
	out := []FilmLink{}
	for rows.Next() {
		var (
			l     FilmLink
			scene sql.NullInt64
		)
		if err := rows.Scan(&l.FilmID, &l.Name, &scene, &l.IsFullFilm); err != nil {
			return nil, err
		}
		if scene.Valid {
			l.SceneNumber = &scene.Int64
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// planCarry computes what keeping keepID would move from trashID, returning the field
// keys whose edits carry so applyCarry copies exactly those.
func planCarry(ctx context.Context, q carryQuerier, keepID, trashID int64) (CarryPreview, []string, error) {
	p := CarryPreview{}
	if err := q.QueryRowContext(ctx,
		`SELECT count(*) FROM playlist_videos WHERE `+carryPlaylistWhere, keepID, trashID).Scan(&p.Playlists); err != nil {
		return p, nil, fmt.Errorf("plan carry playlists: %w", err)
	}
	films, err := filmLinks(ctx, q, carryFilmWhere, keepID, trashID)
	if err != nil {
		return p, nil, err
	}
	p.Films = films
	if err := q.QueryRowContext(ctx,
		`SELECT count(*) FROM video_tags t WHERE `+carryTagWhere, keepID, trashID).Scan(&p.ManualTags); err != nil {
		return p, nil, fmt.Errorf("plan carry tags: %w", err)
	}
	rows, err := q.QueryContext(ctx, `SELECT field_key FROM (`+carryFieldsSQL+`)
		WHERE field_key NOT IN (`+carryExcludedFields+`) ORDER BY field_key`, keepID, trashID)
	if err != nil {
		return p, nil, fmt.Errorf("plan carry edits: %w", err)
	}
	defer rows.Close()
	var fields []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return p, nil, err
		}
		fields = append(fields, k)
	}
	if err := rows.Err(); err != nil {
		return p, nil, err
	}
	p.Edits = len(fields)
	return p, fields, nil
}

// applyCarry moves the planned work from trashID onto keepID inside tx. Playlist places
// and film links MOVE (a film's scene number is unique, and the trashed copy leaves the
// playlist); edits and manual tags are COPIED, so a restored copy keeps its own.
func applyCarry(ctx context.Context, tx *sql.Tx, keepID, trashID int64, fields []string) error {
	steps := []struct{ name, sql string }{
		{"playlists", `UPDATE playlist_videos SET video_id = ?1 WHERE ` + carryPlaylistWhere},
		{"films", `UPDATE film_videos AS fv SET video_id = ?1 WHERE ` + carryFilmWhere},
		{"tags", `INSERT INTO video_tags (video_id, tag_id, source)
			SELECT ?1, t.tag_id, 'manual' FROM video_tags t WHERE ` + carryTagWhere + `
			ON CONFLICT (video_id, tag_id) DO UPDATE SET source = 'manual' WHERE video_tags.source = 'file'`},
	}
	for _, s := range steps {
		if _, err := tx.ExecContext(ctx, s.sql, keepID, trashID); err != nil {
			return fmt.Errorf("carry %s: %w", s.name, err)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	args := []any{keepID, trashID}
	for _, f := range fields {
		args = append(args, f)
	}
	in := numberedPlaceholders(3, len(fields))
	editSteps := []struct{ name, sql string }{
		{"decisions", `INSERT OR IGNORE INTO field_source_decisions
			(entity_type, entity_id, field_key, source, manual_value, created_at)
			SELECT 'video', ?1, d.field_key, d.source, d.manual_value, d.created_at
			  FROM field_source_decisions d
			 WHERE d.entity_type = 'video' AND d.entity_id = ?2 AND d.field_key IN (` + in + `) AND ` + carryDecisionOK},
		{"curation", `INSERT OR IGNORE INTO metadata_curation
			(entity_type, entity_id, field_key, norm_value, value, action, source, created_at)
			SELECT 'video', ?1, field_key, norm_value, value, action, source, created_at
			  FROM metadata_curation WHERE entity_type = 'video' AND entity_id = ?2 AND field_key IN (` + in + `)`},
		{"not-applicable", `INSERT OR IGNORE INTO facet_not_applicable
			(entity_type, entity_id, canonical_field, created_at)
			SELECT 'video', ?1, canonical_field, created_at
			  FROM facet_not_applicable WHERE entity_type = 'video' AND entity_id = ?2 AND canonical_field IN (` + in + `)`},
	}
	for _, s := range editSteps {
		if _, err := tx.ExecContext(ctx, s.sql, args...); err != nil {
			return fmt.Errorf("carry %s: %w", s.name, err)
		}
	}
	return nil
}

func numberedPlaceholders(from, n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("?%d", from+i)
	}
	return strings.Join(parts, ", ")
}

// KeepOneVideo resolves a pair by keeping keepID: the owner's work on trashID moves to
// it and trashID goes to Trash, all in one transaction — a failure changes nothing.
// Returns what moved. ErrVideoPairNotLive when the pair is no longer listed.
func (r *Repo) KeepOneVideo(ctx context.Context, keepID, trashID int64) (CarryPreview, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return CarryPreview{}, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	live, err := videoPairLive(ctx, tx, keepID, trashID)
	if err != nil {
		return CarryPreview{}, err
	}
	if !live {
		return CarryPreview{}, ErrVideoPairNotLive
	}
	plan, fields, err := planCarry(ctx, tx, keepID, trashID)
	if err != nil {
		return CarryPreview{}, err
	}
	if err := applyCarry(ctx, tx, keepID, trashID, fields); err != nil {
		return CarryPreview{}, err
	}
	// No keep-separate row: a restored copy's pair returns (spec P0-1).
	if _, err := tx.ExecContext(ctx,
		`UPDATE videos SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`,
		time.Now().UTC().Format(timeLayout), trashID); err != nil {
		return CarryPreview{}, fmt.Errorf("keep one: trash: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return CarryPreview{}, fmt.Errorf("commit keep one: %w", err)
	}
	return plan, nil
}

// DismissVideoPair records keep-both for a video pair: it never returns for these two
// files. Idempotent, and deliberately not liveness-checked.
func (r *Repo) DismissVideoPair(ctx context.Context, a, b int64) error {
	lo, hi := orderPair(a, b)
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO entity_keep_separate (entity_type, id_lo, id_hi) VALUES (?, ?, ?)`,
		model.EnrichEntityVideo, lo, hi); err != nil {
		return fmt.Errorf("keep both: %w", err)
	}
	return nil
}

// VideoLabel is one side of a label: a value to set, Clear to store an owner-cleared
// field (a manual decision with no value), or neither to leave the field untouched.
type VideoLabel struct {
	ID    int64
	Value string
	Clear bool
}

// LabelVideoPair writes fieldKey (edition or part) on each side as its label says and
// resolves the pair as keep-both, in one transaction.
func (r *Repo) LabelVideoPair(ctx context.Context, fieldKey string, labels [2]VideoLabel) error {
	lo, hi := orderPair(labels[0].ID, labels[1].ID)
	if lo == hi {
		return fmt.Errorf("label: want two distinct videos")
	}

	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	live, err := videoPairLive(ctx, tx, lo, hi)
	if err != nil {
		return err
	}
	if !live {
		return ErrVideoPairNotLive
	}
	for _, l := range labels {
		if l.Value == "" && !l.Clear {
			continue
		}
		if err := upsertDecision(ctx, tx, model.EnrichEntityVideo, l.ID, fieldKey, fieldsource.Manual, l.Value); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO entity_keep_separate (entity_type, id_lo, id_hi) VALUES (?, ?, ?)`,
		model.EnrichEntityVideo, lo, hi); err != nil {
		return fmt.Errorf("label: keep both: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit label: %w", err)
	}
	return nil
}
