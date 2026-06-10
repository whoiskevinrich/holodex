// Package repo is the data-access layer: all SQL lives here so the scanner and
// API handlers work against typed methods, never raw queries (ADR-003/006).
//
// The videos/people/tags FTS5 mirrors are external-content tables kept in sync
// by triggers (migration 0001), so writes here only touch the base tables.
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"holodex/internal/model"
)

// ErrNotFound is returned by single-row getters when no row matches.
var ErrNotFound = errors.New("not found")

// Repo is the data-access layer. SQLite permits only one writer at a time, so
// concurrent scanner workers are serialized through writeMu to avoid SQLITE_BUSY
// (ADR-003/018). Reads are not locked — WAL allows them to run concurrently.
type Repo struct {
	db      *sql.DB
	writeMu sync.Mutex
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

// timeLayout is the storage format for timestamps (ISO-8601, UTC).
const timeLayout = time.RFC3339

// ---------------------------------------------------------------------------
// Write path (scanner)
// ---------------------------------------------------------------------------

// VideoStat is the minimal row the scanner reads to decide whether a file needs
// re-extraction (ADR-018).
type VideoStat struct {
	ID    int64
	Size  int64
	Mtime time.Time
}

// StatByPath returns the stored (id, size, mtime) for a canonical path, or
// ok=false if the file has never been indexed.
func (r *Repo) StatByPath(ctx context.Context, path string) (VideoStat, bool, error) {
	var (
		st       VideoStat
		mtimeStr string
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, file_size, file_mtime FROM videos WHERE file_path = ?`, path,
	).Scan(&st.ID, &st.Size, &mtimeStr)
	if errors.Is(err, sql.ErrNoRows) {
		return VideoStat{}, false, nil
	}
	if err != nil {
		return VideoStat{}, false, fmt.Errorf("stat by path: %w", err)
	}
	st.Mtime, _ = time.Parse(timeLayout, mtimeStr)
	return st, true, nil
}

// UpsertVideo inserts or updates a video and fully replaces its people, tags,
// and raw metadata associations, in a single transaction. Returns the video id.
func (r *Repo) UpsertVideo(ctx context.Context, v *model.Video, extra []model.ExtraMetadata) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	now := time.Now().UTC().Format(timeLayout)
	var recorded any
	if v.RecordedAt != nil {
		recorded = v.RecordedAt.UTC().Format(timeLayout)
	}

	// Upsert the base row by unique file_path.
	res, err := tx.ExecContext(ctx, `
		INSERT INTO videos (file_path, file_size, title, duration_sec, width, height,
		                    recorded_at, indexed_at, file_mtime, active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(file_path) DO UPDATE SET
			file_size    = excluded.file_size,
			title        = excluded.title,
			duration_sec = excluded.duration_sec,
			width        = excluded.width,
			height       = excluded.height,
			recorded_at  = excluded.recorded_at,
			indexed_at   = excluded.indexed_at,
			file_mtime   = excluded.file_mtime,
			active       = 1`,
		v.FilePath, v.FileSize, v.Title, v.Duration, v.Width, v.Height,
		recorded, now, v.FileMtime.UTC().Format(timeLayout),
	)
	if err != nil {
		return 0, fmt.Errorf("upsert video: %w", err)
	}

	// ON CONFLICT does not report LastInsertId for the updated row, so resolve id.
	id, _ := res.LastInsertId()
	if id == 0 {
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM videos WHERE file_path = ?`, v.FilePath).Scan(&id); err != nil {
			return 0, fmt.Errorf("resolve video id: %w", err)
		}
	}

	if err := replaceAssociations(ctx, tx, id, v.People, v.Tags, extra); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit upsert: %w", err)
	}
	return id, nil
}

// replaceAssociations clears and re-links people, tags, and raw metadata for a
// video so re-extraction is idempotent.
func replaceAssociations(ctx context.Context, tx *sql.Tx, videoID int64, people []model.Person, tags []model.Tag, extra []model.ExtraMetadata) error {
	for _, stmt := range []string{
		`DELETE FROM video_people   WHERE video_id = ?`,
		`DELETE FROM video_tags     WHERE video_id = ?`,
		`DELETE FROM video_metadata WHERE video_id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, videoID); err != nil {
			return fmt.Errorf("clear associations: %w", err)
		}
	}

	for _, p := range people {
		pid, err := getOrCreateByName(ctx, tx, "people", p.Name)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO video_people (video_id, person_id) VALUES (?, ?)`, videoID, pid); err != nil {
			return fmt.Errorf("link person: %w", err)
		}
	}
	for _, t := range tags {
		tid, err := getOrCreateByName(ctx, tx, "tags", t.Name)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO video_tags (video_id, tag_id) VALUES (?, ?)`, videoID, tid); err != nil {
			return fmt.Errorf("link tag: %w", err)
		}
	}
	for _, m := range extra {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO video_metadata (video_id, source_key, value) VALUES (?, ?, ?)`,
			videoID, m.SourceKey, m.Value); err != nil {
			return fmt.Errorf("insert metadata: %w", err)
		}
	}
	return nil
}

// getOrCreateByName resolves a people/tags row id by unique name, creating it if
// absent. table is a trusted literal ("people" | "tags"), never user input.
func getOrCreateByName(ctx context.Context, tx *sql.Tx, table, name string) (int64, error) {
	name = strings.TrimSpace(name)
	var id int64
	err := tx.QueryRowContext(ctx, "SELECT id FROM "+table+" WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("lookup %s: %w", table, err)
	}
	res, err := tx.ExecContext(ctx, "INSERT INTO "+table+" (name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("insert %s: %w", table, err)
	}
	return res.LastInsertId()
}

// DeactivateExcept marks active=0 for every active video whose id is not in
// seenIDs, returning the number deactivated (ADR-018 removal reconciliation).
func (r *Repo) DeactivateExcept(ctx context.Context, seenIDs []int64) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if len(seenIDs) == 0 {
		res, err := r.db.ExecContext(ctx, `UPDATE videos SET active = 0 WHERE active = 1`)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}
	q := `UPDATE videos SET active = 0 WHERE active = 1 AND id NOT IN (` +
		placeholders(len(seenIDs)) + `)`
	res, err := r.db.ExecContext(ctx, q, int64sToAny(seenIDs)...)
	if err != nil {
		return 0, fmt.Errorf("deactivate missing: %w", err)
	}
	return res.RowsAffected()
}

// ---------------------------------------------------------------------------
// Read path (API)
// ---------------------------------------------------------------------------

// VideoFilter expresses the browse/search query (F4). Zero-valued fields are
// ignored. People/Tags use AND semantics: a video must match every selected id.
type VideoFilter struct {
	Query                          string
	PersonIDs                      []int64
	TagIDs                         []int64
	DurationMinSec, DurationMaxSec int
	WidthMin, WidthMax             int
	YearMin, YearMax               int
	Limit, Offset                  int
}

// ListVideos returns a page of active videos matching filter plus the total
// match count (for pagination). Results are ordered newest-indexed first (F3.4).
func (r *Repo) ListVideos(ctx context.Context, f VideoFilter) ([]model.Video, int, error) {
	where, args := f.build()

	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM videos v `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count videos: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT v.id, v.file_path, v.file_size, v.title, v.duration_sec, v.width,
	             v.height, v.recorded_at, v.indexed_at, v.file_mtime
	      FROM videos v ` + where +
		` ORDER BY v.indexed_at DESC, v.id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, q, append(args, limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list videos: %w", err)
	}
	defer rows.Close()

	var out []model.Video
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := r.attachAssociations(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// build assembles the shared WHERE clause + args for list and count queries.
func (f VideoFilter) build() (string, []any) {
	var clauses []string
	var args []any
	clauses = append(clauses, "v.active = 1")

	if q := strings.TrimSpace(f.Query); q != "" {
		clauses = append(clauses, "v.id IN (SELECT rowid FROM videos_fts WHERE videos_fts MATCH ?)")
		args = append(args, ftsPrefixQuery(q))
	}
	for _, pid := range f.PersonIDs {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM video_people vp WHERE vp.video_id = v.id AND vp.person_id = ?)")
		args = append(args, pid)
	}
	for _, tid := range f.TagIDs {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM video_tags vt WHERE vt.video_id = v.id AND vt.tag_id = ?)")
		args = append(args, tid)
	}
	if f.DurationMinSec > 0 {
		clauses = append(clauses, "v.duration_sec >= ?")
		args = append(args, f.DurationMinSec)
	}
	if f.DurationMaxSec > 0 {
		clauses = append(clauses, "v.duration_sec <= ?")
		args = append(args, f.DurationMaxSec)
	}
	if f.WidthMin > 0 {
		clauses = append(clauses, "v.width >= ?")
		args = append(args, f.WidthMin)
	}
	if f.WidthMax > 0 {
		clauses = append(clauses, "v.width < ?")
		args = append(args, f.WidthMax)
	}
	if f.YearMin > 0 {
		clauses = append(clauses, "v.recorded_at >= ?")
		args = append(args, fmt.Sprintf("%04d-01-01T00:00:00Z", f.YearMin))
	}
	if f.YearMax > 0 {
		clauses = append(clauses, "v.recorded_at <= ?")
		args = append(args, fmt.Sprintf("%04d-12-31T23:59:59Z", f.YearMax))
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

// GetVideo returns a single active-or-inactive video with its people, tags, and
// raw metadata, or ErrNotFound.
func (r *Repo) GetVideo(ctx context.Context, id int64) (*model.Video, []model.ExtraMetadata, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, file_path, file_size, title, duration_sec, width, height,
		        recorded_at, indexed_at, file_mtime FROM videos WHERE id = ?`, id)
	v, err := scanVideo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	one := []model.Video{v}
	if err := r.attachAssociations(ctx, one); err != nil {
		return nil, nil, err
	}
	extra, err := r.videoMetadata(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return &one[0], extra, nil
}

// PathByID returns the canonical file path for a video, used by the streaming
// handler so clients never supply paths (ADR-015 security).
func (r *Repo) PathByID(ctx context.Context, id int64) (string, error) {
	var path string
	err := r.db.QueryRowContext(ctx, `SELECT file_path FROM videos WHERE id = ?`, id).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return path, err
}

// attachAssociations fills People and Tags for a page of videos using two
// batched IN queries (avoids an N+1 of two queries per video).
func (r *Repo) attachAssociations(ctx context.Context, videos []model.Video) error {
	if len(videos) == 0 {
		return nil
	}
	ids := make([]int64, len(videos))
	idx := make(map[int64]int, len(videos))
	for i, v := range videos {
		ids[i] = v.ID
		idx[v.ID] = i
	}
	in := "(" + placeholders(len(ids)) + ")"
	args := int64sToAny(ids)

	prows, err := r.db.QueryContext(ctx,
		`SELECT vp.video_id, p.id, p.name FROM people p
		 JOIN video_people vp ON vp.person_id = p.id
		 WHERE vp.video_id IN `+in+` ORDER BY vp.video_id, p.name COLLATE NOCASE`, args...)
	if err != nil {
		return fmt.Errorf("batch people: %w", err)
	}
	defer prows.Close()
	for prows.Next() {
		var vid int64
		var p model.Person
		if err := prows.Scan(&vid, &p.ID, &p.Name); err != nil {
			return err
		}
		videos[idx[vid]].People = append(videos[idx[vid]].People, p)
	}
	if err := prows.Err(); err != nil {
		return err
	}

	trows, err := r.db.QueryContext(ctx,
		`SELECT vt.video_id, t.id, t.name FROM tags t
		 JOIN video_tags vt ON vt.tag_id = t.id
		 WHERE vt.video_id IN `+in+` ORDER BY vt.video_id, t.name COLLATE NOCASE`, args...)
	if err != nil {
		return fmt.Errorf("batch tags: %w", err)
	}
	defer trows.Close()
	for trows.Next() {
		var vid int64
		var t model.Tag
		if err := trows.Scan(&vid, &t.ID, &t.Name); err != nil {
			return err
		}
		videos[idx[vid]].Tags = append(videos[idx[vid]].Tags, t)
	}
	return trows.Err()
}

func (r *Repo) videoMetadata(ctx context.Context, videoID int64) ([]model.ExtraMetadata, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT source_key, value FROM video_metadata WHERE video_id = ? ORDER BY source_key`, videoID)
	if err != nil {
		return nil, fmt.Errorf("video metadata: %w", err)
	}
	defer rows.Close()
	var out []model.ExtraMetadata
	for rows.Next() {
		var m model.ExtraMetadata
		if err := rows.Scan(&m.SourceKey, &m.Value); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// People & Tags navigation (F5, F6)
// ---------------------------------------------------------------------------

// namedRow is the shared shape of a people/tags row joined to its video count.
func namedCountQuery(table, junction, fk string, sortByCount bool) string {
	order := "e.name COLLATE NOCASE ASC"
	if sortByCount {
		order = "cnt DESC, e.name COLLATE NOCASE ASC"
	}
	return fmt.Sprintf(`
		SELECT e.id, e.name, COUNT(j.video_id) AS cnt
		FROM %s e
		JOIN %s j     ON j.%s = e.id
		JOIN videos v ON v.id = j.video_id AND v.active = 1
		GROUP BY e.id, e.name
		ORDER BY %s`, table, junction, fk, order)
}

// ListPeople returns every person with at least one active video, with counts.
// sortByCount orders by video count desc (else name asc).
func (r *Repo) ListPeople(ctx context.Context, sortByCount bool) ([]model.Person, error) {
	rows, err := r.db.QueryContext(ctx, namedCountQuery("people", "video_people", "person_id", sortByCount))
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	defer rows.Close()
	var out []model.Person
	for rows.Next() {
		var p model.Person
		if err := rows.Scan(&p.ID, &p.Name, &p.VideoCount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListTags mirrors ListPeople for tags.
func (r *Repo) ListTags(ctx context.Context, sortByCount bool) ([]model.Tag, error) {
	rows, err := r.db.QueryContext(ctx, namedCountQuery("tags", "video_tags", "tag_id", sortByCount))
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()
	var out []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.VideoCount); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetPerson returns a person by id with video count, or ErrNotFound.
func (r *Repo) GetPerson(ctx context.Context, id int64) (*model.Person, error) {
	var p model.Person
	err := r.db.QueryRowContext(ctx, `
		SELECT p.id, p.name,
		       (SELECT COUNT(*) FROM video_people vp JOIN videos v ON v.id = vp.video_id
		        WHERE vp.person_id = p.id AND v.active = 1)
		FROM people p WHERE p.id = ?`, id).Scan(&p.ID, &p.Name, &p.VideoCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

// GetTag returns a tag by id with video count, or ErrNotFound.
func (r *Repo) GetTag(ctx context.Context, id int64) (*model.Tag, error) {
	var t model.Tag
	err := r.db.QueryRowContext(ctx, `
		SELECT t.id, t.name,
		       (SELECT COUNT(*) FROM video_tags vt JOIN videos v ON v.id = vt.video_id
		        WHERE vt.tag_id = t.id AND v.active = 1)
		FROM tags t WHERE t.id = ?`, id).Scan(&t.ID, &t.Name, &t.VideoCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

// ---------------------------------------------------------------------------
// Global search (ADR-017, F4.10)
// ---------------------------------------------------------------------------

// SearchResult is the mixed-entity payload for the global search box.
type SearchResult struct {
	Videos []model.Video  `json:"videos"`
	People []model.Person `json:"people"`
	Tags   []model.Tag    `json:"tags"`
}

// Search runs a prefix FTS query across videos, people, and tags (limit per
// group).
func (r *Repo) Search(ctx context.Context, query string, limit int) (SearchResult, error) {
	var res SearchResult
	q := strings.TrimSpace(query)
	if q == "" {
		return res, nil
	}
	if limit <= 0 {
		limit = 10
	}
	match := ftsPrefixQuery(q)

	vids, _, err := r.ListVideos(ctx, VideoFilter{Query: q, Limit: limit})
	if err != nil {
		return res, err
	}
	res.Videos = vids

	pr, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.name FROM people_fts f JOIN people p ON p.id = f.rowid
		WHERE people_fts MATCH ? LIMIT ?`, match, limit)
	if err != nil {
		return res, fmt.Errorf("search people: %w", err)
	}
	defer pr.Close()
	for pr.Next() {
		var p model.Person
		if err := pr.Scan(&p.ID, &p.Name); err != nil {
			return res, err
		}
		res.People = append(res.People, p)
	}

	tr, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.name FROM tags_fts f JOIN tags t ON t.id = f.rowid
		WHERE tags_fts MATCH ? LIMIT ?`, match, limit)
	if err != nil {
		return res, fmt.Errorf("search tags: %w", err)
	}
	defer tr.Close()
	for tr.Next() {
		var t model.Tag
		if err := tr.Scan(&t.ID, &t.Name); err != nil {
			return res, err
		}
		res.Tags = append(res.Tags, t)
	}
	return res, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface{ Scan(...any) error }

func scanVideo(s rowScanner) (model.Video, error) {
	var (
		v          model.Video
		recorded   sql.NullString
		indexedStr string
		mtimeStr   string
	)
	if err := s.Scan(&v.ID, &v.FilePath, &v.FileSize, &v.Title, &v.Duration,
		&v.Width, &v.Height, &recorded, &indexedStr, &mtimeStr); err != nil {
		return model.Video{}, err
	}
	if recorded.Valid && recorded.String != "" {
		if t, err := time.Parse(timeLayout, recorded.String); err == nil {
			v.RecordedAt = &t
		}
	}
	v.IndexedAt, _ = time.Parse(timeLayout, indexedStr)
	v.FileMtime, _ = time.Parse(timeLayout, mtimeStr)
	v.Active = true
	return v, nil
}

// ftsPrefixQuery turns free text into a prefix MATCH expression so results
// update as the user types: "the sun" -> `"the"* "sun"*`. Quoting each token
// neutralizes FTS5 operator characters in user input.
func ftsPrefixQuery(s string) string {
	fields := strings.Fields(s)
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ReplaceAll(f, `"`, "")
		if f == "" {
			continue
		}
		parts = append(parts, `"`+f+`"*`)
	}
	if len(parts) == 0 {
		return `""`
	}
	return strings.Join(parts, " ")
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func int64sToAny(ids []int64) []any {
	out := make([]any, len(ids))
	for i, v := range ids {
		out[i] = v
	}
	return out
}
