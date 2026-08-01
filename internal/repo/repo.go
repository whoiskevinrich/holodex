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
	"sync/atomic"
	"time"

	"holodex/internal/fieldsource"
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
	// galleryCap overrides the per-person 'extra' gallery cap (PERSON_GALLERY_MAX,
	// F25). Zero means "unset" — GalleryCapValue then falls back to GalleryCap so a
	// bare New(db) (tests, MCP stdio) keeps the built-in default.
	galleryCap int
	// promotions caches field_promotions rows keyed by entity type behind an atomic
	// pointer (F44 follow-up, mirrors enrich.Service.fieldHints) — lazily loaded and
	// invalidated on SetPromotion/ClearPromotion, so the per-detail-resolve visitor
	// read path never queries the table.
	promotions atomic.Pointer[map[string][]PromotionRow]
	// claims caches field_claims rows keyed by entity type behind the same atomic
	// pointer idiom as promotions (F49, ADR-074) — lazily loaded and invalidated on
	// SetClaim/ClearClaim, so the per-detail-resolve visitor read path never queries
	// the table.
	claims atomic.Pointer[map[string][]ClaimRow]
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

// SetGalleryCap overrides the per-person 'extra' gallery cap (F25). A value < 1 is
// ignored, leaving the built-in default. Called once at startup from config.
func (r *Repo) SetGalleryCap(n int) {
	if n >= 1 {
		r.galleryCap = n
	}
}

// GalleryCapValue is the effective per-person gallery cap: the configured override
// or the built-in GalleryCap default. Exposed so the API can advertise it to the SPA.
func (r *Repo) GalleryCapValue() int {
	if r.galleryCap >= 1 {
		return r.galleryCap
	}
	return GalleryCap
}

// timeLayout is the storage format for timestamps (ISO-8601, UTC).
const timeLayout = time.RFC3339

// ---------------------------------------------------------------------------
// Write path (scanner)
// ---------------------------------------------------------------------------

// VideoStat is the minimal row the scanner reads to decide whether a file needs
// re-extraction (ADR-018). Active lets the change-detection fast-path notice a row
// that was deactivated (e.g. by a transient empty walk) so it can be reactivated
// without re-extraction when the file reappears unchanged (issue #26).
type VideoStat struct {
	ID     int64
	Size   int64
	Mtime  time.Time
	Active bool
	// Deleted is true when the owner soft-deleted the row (F24, ADR-037). The
	// scanner short-circuits a soft-deleted row before the change-detection
	// fast-path so a delete is never undone by a re-scan of a still-present file.
	Deleted bool
}

// StatByPath returns the stored (id, size, mtime, active, deleted) for a canonical
// path, or ok=false if the file has never been indexed.
func (r *Repo) StatByPath(ctx context.Context, path string) (VideoStat, bool, error) {
	var (
		st       VideoStat
		mtimeStr string
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, file_size, file_mtime, active, deleted_at IS NOT NULL
		   FROM videos WHERE file_path = ?`, path,
	).Scan(&st.ID, &st.Size, &mtimeStr, &st.Active, &st.Deleted)
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
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO videos (file_path, file_size, title, duration_sec, width, height,
		                    video_codec, audio_codec, bitrate_kbps, container,
		                    recorded_at, indexed_at, file_mtime, active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(file_path) DO UPDATE SET
			file_size    = excluded.file_size,
			title        = excluded.title,
			duration_sec = excluded.duration_sec,
			width        = excluded.width,
			height       = excluded.height,
			video_codec  = excluded.video_codec,
			audio_codec  = excluded.audio_codec,
			bitrate_kbps = excluded.bitrate_kbps,
			container    = excluded.container,
			recorded_at  = excluded.recorded_at,
			indexed_at   = excluded.indexed_at,
			file_mtime   = excluded.file_mtime,
			active       = 1`,
		// NB: deleted_at is deliberately NOT in the UPDATE set — a soft-deleted row
		// whose file changes on disk stays soft-deleted (F24/ADR-037). The scanner
		// also short-circuits soft-deleted rows before reaching here.
		v.FilePath, v.FileSize, v.Title, v.Duration, v.Width, v.Height,
		v.VideoCodec, v.AudioCodec, v.BitrateKbps, v.Container,
		recorded, now, v.FileMtime.UTC().Format(timeLayout),
	); err != nil {
		return 0, fmt.Errorf("upsert video: %w", err)
	}

	// Resolve the id by the unique file_path rather than LastInsertId(): on the
	// ON CONFLICT update path the latter reflects the connection's last INSERT —
	// which may be an unrelated row (e.g. an enrichment write between scans) — so
	// it can return the wrong id. The unique-key lookup is always correct.
	var id int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM videos WHERE file_path = ?`, v.FilePath).Scan(&id); err != nil {
		return 0, fmt.Errorf("resolve video id: %w", err)
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
	// Scoped to source=fieldsource.File (ADR-075 D3): a manually-attached or
	// enrichment-materialized tag must survive every future rescan, since the
	// file on disk has no way to re-supply it if this delete ever widened back
	// to unconditional.
	tagDelete := fmt.Sprintf(`DELETE FROM video_tags WHERE video_id = ? AND source = '%s'`, fieldsource.File)
	tagInsert := fmt.Sprintf(`INSERT OR IGNORE INTO video_tags (video_id, tag_id, source) VALUES (?, ?, '%s')`, fieldsource.File)

	for _, stmt := range []string{
		`DELETE FROM video_people   WHERE video_id = ?`,
		tagDelete,
		`DELETE FROM video_metadata WHERE video_id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, videoID); err != nil {
			return fmt.Errorf("clear associations: %w", err)
		}
	}

	// People and tags both resolve through the shared name-identity spine (F43,
	// ADR-061) so case/whitespace variants converge and a merged-away name routes to
	// the canonical entity — the merge survives re-scans.
	for _, p := range people {
		pid, err := resolveOrCreatePerson(ctx, tx, p.Name)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO video_people (video_id, person_id) VALUES (?, ?)`, videoID, pid); err != nil {
			return fmt.Errorf("link person: %w", err)
		}
	}
	for _, t := range tags {
		// A denied, oversized, or category-colliding term is skipped silently
		// (ADR-075 D2/item 11; ADR-078 D3): the scanner has no owner present to
		// surface a rejection to, unlike the manual-attach endpoint (422/400/409).
		tid, err := resolveOrCreateByName(ctx, tx, model.EntityTag, t.Name, "")
		if errors.Is(err, ErrTagDenied) || errors.Is(err, ErrTagNameTooLong) || errors.Is(err, ErrTagNameCollidesWithCategory) {
			continue
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, tagInsert, videoID, tid); err != nil {
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
	res, err := r.db.ExecContext(ctx, q, toAnySlice(seenIDs)...)
	if err != nil {
		return 0, fmt.Errorf("deactivate missing: %w", err)
	}
	return res.RowsAffected()
}

// Reactivate flips active=1 for a single row whose file has reappeared unchanged
// (issue #26). It is the cheap counterpart to a full re-index: the metadata is
// already current (size + mtime matched), so the scanner only needs to undo a
// prior deactivation, no ffprobe/exiftool re-extraction.
func (r *Repo) Reactivate(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if _, err := r.db.ExecContext(ctx,
		`UPDATE videos SET active = 1 WHERE id = ?`, id,
	); err != nil {
		return fmt.Errorf("reactivate: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Read path (API)
// ---------------------------------------------------------------------------

// VideoFilter expresses the browse/search query (F4). Zero-valued fields are
// ignored. People/Tags use AND semantics: a video must match every selected id.
type VideoFilter struct {
	Query                          string
	PersonIDs                      []int64
	// PersonIDsAny matches videos credited to ANY of these people (OR), unlike
	// PersonIDs which ANDs. Used by global search to fold a matched person's media
	// (incl. alias matches) into the results (F23, ADR-036).
	PersonIDsAny []int64
	TagIDs       []int64
	// StudioIDs matches videos linked to ALL of these studios (AND), like TagIDs —
	// the entity-backed browse facet filter ?studio_id (F38, ADR-053).
	StudioIDs []int64
	// CategoryIDs matches videos tagged with ANY member tag of ALL of these
	// categories (AND across categories, OR within one category's member
	// tags) — the browse-page "Categories" facet (HOLODEX-240, ADR-078 D2).
	// Expands to member tag ids at query time via category_tags; no new
	// filtering primitive beyond the TagIDs EXISTS(...) clause shape below.
	CategoryIDs []int64
	DurationMinSec, DurationMaxSec int
	WidthMin, WidthMax             int
	YearMin, YearMax               int
	// DateFrom/DateTo are inclusive ISO dates (YYYY-MM-DD) matched against
	// recorded_at — finer-grained than Year*, used by the MCP search tool (F10.2).
	DateFrom, DateTo string
	// MappedFilters constrain by configurable mapped fields (F20.5); each must
	// match (AND), like People/Tags.
	MappedFilters []MappedFilter
	Limit, Offset int
	// Sort is a canonical sort key (F12.1); empty/unknown falls back to
	// newest-indexed-first. See orderBy for the allowed set.
	Sort string
	// Seed parameterizes the "random" sort's deterministic shuffle (ADR-045): a
	// fixed seed makes holo_shuffle(id, seed) a stable order, so LIMIT/OFFSET
	// pages tile without duplicate or skipped rows. Ignored unless Sort=="random".
	Seed int64
}

// MappedFilter matches videos carrying a metadata row whose source_key is one of
// SourceKeys (case-insensitive) and whose value equals Value (F20.5).
type MappedFilter struct {
	SourceKeys []string
	Value      string
}

// orderBy maps the sort key to a safe ORDER BY clause (whitelist — the key is
// never interpolated) plus any bound args the clause needs. The id tiebreaker
// keeps pagination stable. "Resolution" sorts by width, consistent with the
// width-based resolution buckets (ADR-012). "random" orders by the deterministic
// holo_shuffle(id, seed) function (ADR-045) with the seed as a bound parameter, so
// one seed yields a single shuffle that tiles across LIMIT/OFFSET pages.
func (f VideoFilter) orderBy() (string, []any) {
	switch f.Sort {
	case "title_asc":
		return "v.title COLLATE NOCASE ASC, v.id ASC", nil
	case "title_desc":
		return "v.title COLLATE NOCASE DESC, v.id DESC", nil
	case "added_asc":
		return "v.indexed_at ASC, v.id ASC", nil
	case "duration_desc":
		return "v.duration_sec DESC, v.id DESC", nil
	case "duration_asc":
		return "v.duration_sec ASC, v.id ASC", nil
	case "resolution_desc":
		return "v.width DESC, v.height DESC, v.id DESC", nil
	case "resolution_asc":
		return "v.width ASC, v.height ASC, v.id ASC", nil
	case "random":
		return "holo_shuffle(v.id, ?), v.id ASC", []any{f.Seed}
	default: // "added_desc" and anything unrecognized
		return "v.indexed_at DESC, v.id DESC", nil
	}
}

// ListVideos returns a page of active videos matching filter plus the total
// match count (for pagination). Ordering follows filter.Sort, defaulting to
// newest-indexed first (F3.4 / F12.1).
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
	orderClause, orderArgs := f.orderBy()
	q := `SELECT v.id, v.file_path, v.file_size, v.title, v.duration_sec, v.width,
	             v.height, v.video_codec, v.audio_codec, v.bitrate_kbps, v.container,
	             v.recorded_at, v.indexed_at, v.file_mtime, v.thumbnail_state
	      FROM videos v ` + where +
		` ORDER BY ` + orderClause + ` LIMIT ? OFFSET ?`
	// Order args (the random seed) sit between the WHERE args and LIMIT/OFFSET,
	// matching the clause position. Build a fresh slice so args isn't mutated.
	qArgs := make([]any, 0, len(args)+len(orderArgs)+2)
	qArgs = append(qArgs, args...)
	qArgs = append(qArgs, orderArgs...)
	qArgs = append(qArgs, limit, f.Offset)
	rows, err := r.db.QueryContext(ctx, q, qArgs...)
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
	// Library visibility: a row is visible only when on disk (active, ADR-018) AND
	// not owner-soft-deleted (deleted_at NULL, F24/ADR-037). This is the central
	// read-path seam every list/count/search surface flows through; the by-id reads
	// (GetVideo/PathByID/Related subject) carry the same `deleted_at IS NULL` guard.
	clauses = append(clauses, "v.active = 1 AND v.deleted_at IS NULL")

	if q := strings.TrimSpace(f.Query); q != "" {
		clauses = append(clauses, "v.id IN (SELECT rowid FROM videos_fts WHERE videos_fts MATCH ?)")
		args = append(args, ftsPrefixQuery(q))
	}
	for _, pid := range f.PersonIDs {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM video_people vp WHERE vp.video_id = v.id AND vp.person_id = ?)")
		args = append(args, pid)
	}
	if len(f.PersonIDsAny) > 0 {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM video_people vp WHERE vp.video_id = v.id AND vp.person_id IN ("+placeholders(len(f.PersonIDsAny))+"))")
		args = append(args, toAnySlice(f.PersonIDsAny)...)
	}
	for _, tid := range f.TagIDs {
		// Descendant-inclusive (F50, ADR-075 D1/P0-6): a video tagged only with a
		// descendant of tid still matches — tagSubtreeQuery expands tid to itself
		// plus its full subtree at query time.
		clauses = append(clauses, "EXISTS (SELECT 1 FROM video_tags vt WHERE vt.video_id = v.id AND vt.tag_id IN ("+tagSubtreeQuery+"))")
		args = append(args, tid)
	}
	for _, sid := range f.StudioIDs {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM video_studios vs WHERE vs.video_id = v.id AND vs.studio_id = ?)")
		args = append(args, sid)
	}
	for _, cid := range f.CategoryIDs {
		// Category → member-tag-id expansion (ADR-078 D2/Consequences): the same
		// EXISTS(...) clause shape as TagIDs above, with the tag-id set sourced from
		// category_tags instead of the recursive subtree query (categories are flat).
		clauses = append(clauses, "EXISTS (SELECT 1 FROM video_tags vt WHERE vt.video_id = v.id AND vt.tag_id IN (SELECT tag_id FROM category_tags WHERE category_id = ?))")
		args = append(args, cid)
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
	// recorded_at is stored RFC3339; a bare date sorts lexicographically before
	// that day's timestamps, so >= DateFrom and <= DateTo+end-of-day are inclusive.
	if f.DateFrom != "" {
		clauses = append(clauses, "v.recorded_at >= ?")
		args = append(args, f.DateFrom)
	}
	if f.DateTo != "" {
		clauses = append(clauses, "v.recorded_at <= ?")
		args = append(args, f.DateTo+"T23:59:59Z")
	}
	for _, mf := range f.MappedFilters {
		if mf.Value == "" || len(mf.SourceKeys) == 0 {
			continue
		}
		clauses = append(clauses,
			"EXISTS (SELECT 1 FROM video_metadata m WHERE m.video_id = v.id"+
				" AND m.source_key COLLATE NOCASE IN ("+placeholders(len(mf.SourceKeys))+") AND m.value = ?)")
		for _, k := range mf.SourceKeys {
			args = append(args, k)
		}
		args = append(args, mf.Value)
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

// PeopleForVideos returns the people linked to each of the given videos, keyed by
// video id (mirrors StudiosForVideos) — used by merge-writeback propagation (F48.8)
// to look up each affected video's full, post-merge People tag value in one query
// rather than one GetVideo call per video. Videos with no person link are absent
// from the map.
func (r *Repo) PeopleForVideos(ctx context.Context, ids []int64) (map[int64][]model.Person, error) {
	if len(ids) == 0 {
		return map[int64][]model.Person{}, nil
	}
	q := `SELECT vp.video_id, p.id, p.name
	      FROM video_people vp JOIN people p ON p.id = vp.person_id
	      WHERE vp.video_id IN (` + placeholders(len(ids)) + `)
	      ORDER BY p.name COLLATE NOCASE`
	rows, err := r.db.QueryContext(ctx, q, toAnySlice(ids)...)
	if err != nil {
		return nil, fmt.Errorf("people for videos: %w", err)
	}
	defer rows.Close()
	out := make(map[int64][]model.Person, len(ids))
	for rows.Next() {
		var vid int64
		var p model.Person
		if err := rows.Scan(&vid, &p.ID, &p.Name); err != nil {
			return nil, err
		}
		out[vid] = append(out[vid], p)
	}
	return out, rows.Err()
}

// GetVideo returns a single (active-or-inactive) non-soft-deleted video with its
// people, tags, and raw metadata, or ErrNotFound. A soft-deleted row 404s here
// just as it is absent from every list surface (F24.2/ADR-037 §4).
func (r *Repo) GetVideo(ctx context.Context, id int64) (*model.Video, []model.ExtraMetadata, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, file_path, file_size, title, duration_sec, width, height,
		        video_codec, audio_codec, bitrate_kbps, container,
		        recorded_at, indexed_at, file_mtime, thumbnail_state
		   FROM videos WHERE id = ? AND deleted_at IS NULL`, id)
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
	// Soft-deleted rows 404 from streaming too — the bytes are hidden during the
	// grace window (F24/ADR-037 §4). The purge job reads the path via PurgePath,
	// which deliberately ignores deleted_at.
	err := r.db.QueryRowContext(ctx,
		`SELECT file_path FROM videos WHERE id = ? AND deleted_at IS NULL`, id).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return path, err
}

// ---------------------------------------------------------------------------
// Thumbnails (ADR-009)
// ---------------------------------------------------------------------------

// ThumbnailCandidate is the minimal row a generation worker needs to produce a
// thumbnail for one video.
type ThumbnailCandidate struct {
	ID          int64
	FilePath    string
	DurationSec int
}

// SetThumbnailState records the per-video thumbnail pipeline state (ADR-009).
func (r *Repo) SetThumbnailState(ctx context.Context, id int64, state string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx,
		`UPDATE videos SET thumbnail_state = ? WHERE id = ?`, state, id); err != nil {
		return fmt.Errorf("set thumbnail state: %w", err)
	}
	return nil
}

// ResetThumbnailState clears the state (NULL) so the pipeline regenerates the
// image on the next sweep / enqueue (used by the regenerate endpoint).
func (r *Repo) ResetThumbnailState(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx,
		`UPDATE videos SET thumbnail_state = NULL WHERE id = ?`, id); err != nil {
		return fmt.Errorf("reset thumbnail state: %w", err)
	}
	return nil
}

// ThumbnailBackfillCandidates returns active videos still needing a generated
// thumbnail — never attempted (NULL) or previously failed (retried on restart) —
// newest first so a fresh library fills the most-recently-added items sooner.
func (r *Repo) ThumbnailBackfillCandidates(ctx context.Context, limit int) ([]ThumbnailCandidate, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, file_path, duration_sec FROM videos
		WHERE active = 1 AND deleted_at IS NULL AND (thumbnail_state IS NULL OR thumbnail_state = ?)
		ORDER BY indexed_at DESC, id DESC LIMIT ?`, model.ThumbnailFailed, limit)
	if err != nil {
		return nil, fmt.Errorf("thumbnail backfill candidates: %w", err)
	}
	defer rows.Close()
	var out []ThumbnailCandidate
	for rows.Next() {
		var c ThumbnailCandidate
		if err := rows.Scan(&c.ID, &c.FilePath, &c.DurationSec); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ThumbnailCandidateByID resolves one candidate row, ok=false if the id is gone
// (e.g. the file was removed between enqueue and processing).
func (r *Repo) ThumbnailCandidateByID(ctx context.Context, id int64) (ThumbnailCandidate, bool, error) {
	var c ThumbnailCandidate
	err := r.db.QueryRowContext(ctx,
		`SELECT id, file_path, duration_sec FROM videos WHERE id = ?`, id).
		Scan(&c.ID, &c.FilePath, &c.DurationSec)
	if errors.Is(err, sql.ErrNoRows) {
		return ThumbnailCandidate{}, false, nil
	}
	if err != nil {
		return ThumbnailCandidate{}, false, fmt.Errorf("thumbnail candidate: %w", err)
	}
	return c, true, nil
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
	args := toAnySlice(ids)

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
		`SELECT vt.video_id, t.id, t.name, vt.source FROM tags t
		 JOIN video_tags vt ON vt.tag_id = t.id
		 WHERE vt.video_id IN `+in+` ORDER BY vt.video_id, t.name COLLATE NOCASE`, args...)
	if err != nil {
		return fmt.Errorf("batch tags: %w", err)
	}
	defer trows.Close()
	for trows.Next() {
		var vid int64
		var t model.Tag
		if err := trows.Scan(&vid, &t.ID, &t.Name, &t.Source); err != nil {
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
// Configurable metadata fields (F20, ADR-013)
// ---------------------------------------------------------------------------

// FacetValue is one distinct mapped-field value with its video count.
type FacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// FacetValues returns the distinct values (with counts) for a mapped field across
// active videos — the union of metadata rows whose source_key is in sourceKeys
// (case-insensitive). Drives the filter facet value list (F20.4).
func (r *Repo) FacetValues(ctx context.Context, sourceKeys []string) ([]FacetValue, error) {
	if len(sourceKeys) == 0 {
		return nil, nil
	}
	q := `SELECT m.value, COUNT(DISTINCT m.video_id) AS cnt
	      FROM video_metadata m
	      JOIN videos v ON v.id = m.video_id AND v.active = 1 AND v.deleted_at IS NULL
	      WHERE m.source_key COLLATE NOCASE IN (` + placeholders(len(sourceKeys)) + `)
	      GROUP BY m.value ORDER BY cnt DESC, m.value COLLATE NOCASE`
	rows, err := r.db.QueryContext(ctx, q, toAnySlice(sourceKeys)...)
	if err != nil {
		return nil, fmt.Errorf("facet values: %w", err)
	}
	defer rows.Close()
	var out []FacetValue
	for rows.Next() {
		var fv FacetValue
		if err := rows.Scan(&fv.Value, &fv.Count); err != nil {
			return nil, err
		}
		out = append(out, fv)
	}
	return out, rows.Err()
}

// MetadataKey is a distinct raw source key with its occurrence count and a few
// sample values — the library-wide mapping-authoring aid (F20.9).
type MetadataKey struct {
	SourceKey string   `json:"source_key"`
	Count     int      `json:"count"`
	Samples   []string `json:"samples"`
}

// MetadataKeys enumerates all distinct source keys across active videos, most
// frequent first, each with up to sampleLimit sample values (F20.9).
func (r *Repo) MetadataKeys(ctx context.Context, sampleLimit int) ([]MetadataKey, error) {
	if sampleLimit <= 0 {
		sampleLimit = 3
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.source_key, COUNT(DISTINCT m.video_id) AS cnt
		FROM video_metadata m
		JOIN videos v ON v.id = m.video_id AND v.active = 1 AND v.deleted_at IS NULL
		GROUP BY m.source_key COLLATE NOCASE
		ORDER BY cnt DESC, m.source_key COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("metadata keys: %w", err)
	}
	defer rows.Close()
	var keys []MetadataKey
	for rows.Next() {
		var k MetadataKey
		if err := rows.Scan(&k.SourceKey, &k.Count); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Distinct-key cardinality is small (tens), so a sample query per key on this
	// cold discovery view is fine.
	for i := range keys {
		srows, err := r.db.QueryContext(ctx,
			`SELECT DISTINCT value FROM video_metadata WHERE source_key COLLATE NOCASE = ? LIMIT ?`,
			keys[i].SourceKey, sampleLimit)
		if err != nil {
			return nil, fmt.Errorf("metadata key samples: %w", err)
		}
		for srows.Next() {
			var v string
			if err := srows.Scan(&v); err != nil {
				srows.Close()
				return nil, err
			}
			keys[i].Samples = append(keys[i].Samples, v)
		}
		srows.Close()
	}
	return keys, nil
}

// ---------------------------------------------------------------------------
// People & Tags navigation (F5, F6)
// ---------------------------------------------------------------------------

// PersonIDByName resolves a person id by exact, case-insensitive name; ok=false
// if absent. Used by the MCP search tool to filter by names rather than ids.
func (r *Repo) PersonIDByName(ctx context.Context, name string) (int64, bool, error) {
	return idByName(ctx, r.db, "people", name)
}

// TagIDByName mirrors PersonIDByName for tags.
func (r *Repo) TagIDByName(ctx context.Context, name string) (int64, bool, error) {
	return idByName(ctx, r.db, "tags", name)
}

// idByName looks up a row id by unique name (case-insensitive). table is a
// trusted literal ("people" | "tags"), never user input.
func idByName(ctx context.Context, db *sql.DB, table, name string) (int64, bool, error) {
	var id int64
	err := db.QueryRowContext(ctx,
		"SELECT id FROM "+table+" WHERE name = ? COLLATE NOCASE", strings.TrimSpace(name)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("%s id by name: %w", table, err)
	}
	return id, true, nil
}

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
		JOIN videos v ON v.id = j.video_id AND v.active = 1 AND v.deleted_at IS NULL
		GROUP BY e.id, e.name
		ORDER BY %s`, table, junction, fk, order)
}

// ListPeople returns every person with at least one active video, with counts and the
// headshot image id (the list avatar's ?v= cache-buster, so it refreshes when the
// headshot changes — F25.29). sortByCount orders by video count desc (else name asc).
func (r *Repo) ListPeople(ctx context.Context, sortByCount bool) ([]model.Person, error) {
	order := "e.name COLLATE NOCASE ASC"
	if sortByCount {
		order = "cnt DESC, e.name COLLATE NOCASE ASC"
	}
	// People-specific (not namedCountQuery, which is shared with tags): the correlated
	// subquery pulls the current headshot image id so the list avatar URL can carry a
	// version that busts the browser cache after enrichment fills/replaces the headshot.
	q := fmt.Sprintf(`
		SELECT e.id, e.name, COUNT(j.video_id) AS cnt,
		       (SELECT id FROM person_images WHERE person_id = e.id AND role = 'headshot') AS headshot_id
		FROM people e
		JOIN video_people j ON j.person_id = e.id
		JOIN videos v       ON v.id = j.video_id AND v.active = 1 AND v.deleted_at IS NULL
		GROUP BY e.id, e.name
		ORDER BY %s`, order)
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	defer rows.Close()
	var out []model.Person
	for rows.Next() {
		var (
			p          model.Person
			headshotID sql.NullInt64
		)
		if err := rows.Scan(&p.ID, &p.Name, &p.VideoCount, &headshotID); err != nil {
			return nil, err
		}
		p.HeadshotVersion = headshotID.Int64 // 0 when no headshot row (placeholder)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Attach owner-curated aliases (F43, ADR-061) in one batch — tags have no detail
	// page, so the list is where the owner sees and manages them (RD7).
	ids := make([]int64, len(out))
	for i, t := range out {
		ids[i] = t.ID
	}
	byTag, err := r.AliasesForEntities(ctx, model.EntityTag, ids)
	if err != nil {
		return nil, err
	}
	// parent_tag_id isn't part of namedCountQuery (shared with ListStudios, which
	// has no such column) -- attach it in its own batch, same pattern as aliases.
	parents, err := r.tagParents(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Aliases = byTag[out[i].ID]
		out[i].ParentTagID = parents[out[i].ID]
	}
	return out, nil
}

// tagParents returns each id's parent_tag_id, keyed by id — absent from the
// map when the tag is a root (nil parent).
func (r *Repo) tagParents(ctx context.Context, ids []int64) (map[int64]*int64, error) {
	out := make(map[int64]*int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, parent_tag_id FROM tags WHERE id IN (`+placeholders(len(ids))+`)`,
		toAnySlice(ids)...)
	if err != nil {
		return nil, fmt.Errorf("tag parents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var parentID sql.NullInt64
		if err := rows.Scan(&id, &parentID); err != nil {
			return nil, err
		}
		if parentID.Valid {
			out[id] = &parentID.Int64
		}
	}
	return out, rows.Err()
}

// GetPerson returns a person by id with video count, or ErrNotFound.
func (r *Repo) GetPerson(ctx context.Context, id int64) (*model.Person, error) {
	var p model.Person
	err := r.db.QueryRowContext(ctx, `
		SELECT p.id, p.name,
		       (SELECT COUNT(*) FROM video_people vp JOIN videos v ON v.id = vp.video_id
		        WHERE vp.person_id = p.id AND v.active = 1 AND v.deleted_at IS NULL)
		FROM people p WHERE p.id = ?`, id).Scan(&p.ID, &p.Name, &p.VideoCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// Attach owner-curated aliases (F23, ADR-036) for the detail view.
	if p.Aliases, err = r.AliasesForPerson(ctx, id); err != nil {
		return nil, err
	}
	return &p, nil
}

// PersonExists reports a person id is present (ErrNotFound otherwise), skipping
// the video-count and alias fetches GetPerson does — for cheap existence checks
// on the write path (e.g. before adding an alias).
func (r *Repo) PersonExists(ctx context.Context, id int64) error {
	var x int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM people WHERE id = ?`, id).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// GetTag returns a tag by id with video count, or ErrNotFound.
func (r *Repo) GetTag(ctx context.Context, id int64) (*model.Tag, error) {
	var t model.Tag
	var parentID sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT t.id, t.name, t.parent_tag_id,
		       (SELECT COUNT(*) FROM video_tags vt JOIN videos v ON v.id = vt.video_id
		        WHERE vt.tag_id = t.id AND v.active = 1 AND v.deleted_at IS NULL)
		FROM tags t WHERE t.id = ?`, id).Scan(&t.ID, &t.Name, &parentID, &t.VideoCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		t.ParentTagID = &parentID.Int64
	}
	if t.Aliases, err = r.AliasesForEntity(ctx, model.EntityTag, id); err != nil {
		return nil, err
	}
	if t.Ancestors, err = r.AncestorNamesForTag(ctx, id); err != nil {
		return nil, err
	}
	return &t, nil
}

// ---------------------------------------------------------------------------
// Related media — "More with …" shelves (ADR-031, QW2/QW3)
// ---------------------------------------------------------------------------

// RelatedShelf is one "More with <entity>" set: the chosen person or tag and up to
// N random sibling videos, excluding the source item. Items is always non-nil (an
// empty shelf is valid — the entity exists on the item but has no other siblings).
type RelatedShelf struct {
	ID    int64         `json:"id"`
	Name  string        `json:"name"`
	Items []model.Video `json:"items"`
}

// RelatedMedia carries the person- and tag-keyed shelves for a media item. Either
// field is nil when the item has no people / no tags (ADR-031).
type RelatedMedia struct {
	Person *RelatedShelf `json:"person"`
	Tag    *RelatedShelf `json:"tag"`
}

// Related builds the two "More with …" shelves for a media item (ADR-031): one keyed
// to its most-connected person, one to its most distinctive tag, each filled with up
// to `limit` random sibling videos (excluding the item). Returns ErrNotFound if the
// item is missing or inactive.
func (r *Repo) Related(ctx context.Context, videoID int64, limit int) (*RelatedMedia, error) {
	if limit <= 0 {
		limit = 5
	}
	// The subject item must itself be visible (on disk, not soft-deleted) — a
	// soft-deleted item has no "more with" shelves, consistent with its 404 detail.
	var active int
	switch err := r.db.QueryRowContext(ctx,
		`SELECT active FROM videos WHERE id = ? AND deleted_at IS NULL`, videoID).Scan(&active); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("related: load item: %w", err)
	case active == 0:
		return nil, ErrNotFound
	}

	out := &RelatedMedia{}
	var err error

	// Person shelf — the item's person with the highest global active-video count
	// (most-connected → most likely to populate the shelf); tie-break lowest id.
	if out.Person, err = r.relatedShelf(ctx, videoID, limit, "person", "video_people", "person_id", `
		SELECT p.id, p.name
		FROM people p
		JOIN video_people vp ON vp.person_id = p.id
		WHERE vp.video_id = ?
		ORDER BY (SELECT COUNT(*) FROM video_people vp2 JOIN videos v ON v.id = vp2.video_id
		          WHERE vp2.person_id = p.id AND v.active = 1 AND v.deleted_at IS NULL) DESC,
		         p.id ASC
		LIMIT 1`); err != nil {
		return nil, err
	}

	// Tag shelf — the item's most *distinctive* tag: maximize c·(1 − c/N), where c is
	// the tag's global active-video count and N is the total active videos. Rewards
	// shared tags but demotes near-universal ones (ADR-031); tie-break higher c, lowest id.
	if out.Tag, err = r.relatedShelf(ctx, videoID, limit, "tag", "video_tags", "tag_id", `
		SELECT id, name FROM (
			SELECT t.id AS id, t.name AS name,
			       (SELECT COUNT(*) FROM video_tags vt2 JOIN videos v ON v.id = vt2.video_id
			        WHERE vt2.tag_id = t.id AND v.active = 1 AND v.deleted_at IS NULL) AS cnt
			FROM tags t
			JOIN video_tags vt ON vt.tag_id = t.id
			WHERE vt.video_id = ?
		)
		CROSS JOIN (SELECT COUNT(*) AS total FROM videos WHERE active = 1 AND deleted_at IS NULL) AS n
		ORDER BY (cnt * (1.0 - cnt * 1.0 / n.total)) DESC, cnt DESC, id ASC
		LIMIT 1`); err != nil {
		return nil, err
	}

	return out, nil
}

// relatedShelf picks one keyed entity for a media item via pickQuery (which scans
// id, name and takes the video id as its only arg), then fills the shelf with random
// siblings sharing that entity through junction/fk. Returns (nil, nil) when the item
// has no such entity — a present-but-empty shelf still comes back with id/name.
func (r *Repo) relatedShelf(ctx context.Context, videoID int64, limit int, label, junction, fk, pickQuery string) (*RelatedShelf, error) {
	var id int64
	var name string
	switch err := r.db.QueryRowContext(ctx, pickQuery, videoID).Scan(&id, &name); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("related: pick %s: %w", label, err)
	}
	items, err := r.randomSiblings(ctx, junction, fk, id, videoID, limit)
	if err != nil {
		return nil, err
	}
	return &RelatedShelf{ID: id, Name: name, Items: items}, nil
}

// randomSiblings returns up to `limit` active videos that share entity keyID via the
// junction table (video_people/person_id or video_tags/tag_id), excluding excludeID,
// in random order. junction and fk are caller-controlled constants, never user input.
// This is the project's only ORDER BY RANDOM() (ADR-031) — kept here rather than in
// VideoFilter so the general list path stays deterministically ordered.
func (r *Repo) randomSiblings(ctx context.Context, junction, fk string, keyID, excludeID int64, limit int) ([]model.Video, error) {
	q := `SELECT v.id, v.file_path, v.file_size, v.title, v.duration_sec, v.width, v.height,
	             v.video_codec, v.audio_codec, v.bitrate_kbps, v.container,
	             v.recorded_at, v.indexed_at, v.file_mtime, v.thumbnail_state
	      FROM videos v
	      WHERE v.active = 1 AND v.deleted_at IS NULL AND v.id != ?
	        AND EXISTS (SELECT 1 FROM ` + junction + ` j WHERE j.video_id = v.id AND j.` + fk + ` = ?)
	      ORDER BY RANDOM() LIMIT ?`
	rows, err := r.db.QueryContext(ctx, q, excludeID, keyID, limit)
	if err != nil {
		return nil, fmt.Errorf("related siblings: %w", err)
	}
	defer rows.Close()
	out := make([]model.Video, 0, limit) // non-nil → empty shelf serializes as []
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

// ---------------------------------------------------------------------------
// Global search (ADR-017, F4.10)
// ---------------------------------------------------------------------------

// SearchResult is the mixed-entity payload for the global search box.
type SearchResult struct {
	Videos  []model.Video  `json:"videos"`
	People  []model.Person `json:"people"`
	Tags    []model.Tag    `json:"tags"`
	Studios []model.Studio `json:"studios"`
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

	// People first (so their media can be folded into the video results below):
	// canonical-name matches, then alias-only matches (F23, ADR-036), deduped by id
	// so a person matching both its name and an alias appears once.
	pr, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.name FROM people_fts f JOIN people p ON p.id = f.rowid
		WHERE people_fts MATCH ? LIMIT ?`, match, limit)
	if err != nil {
		return res, fmt.Errorf("search people: %w", err)
	}
	defer pr.Close()
	seen := make(map[int64]struct{})
	for pr.Next() {
		var p model.Person
		if err := pr.Scan(&p.ID, &p.Name); err != nil {
			return res, err
		}
		seen[p.ID] = struct{}{}
		res.People = append(res.People, p)
	}
	if err := pr.Err(); err != nil {
		return res, err
	}
	if remaining := limit - len(res.People); remaining > 0 {
		aliasHits, err := r.searchPeopleByAlias(ctx, match, remaining)
		if err != nil {
			return res, err
		}
		for _, p := range aliasHits {
			if _, dup := seen[p.ID]; dup {
				continue
			}
			seen[p.ID] = struct{}{}
			res.People = append(res.People, p)
			if len(res.People) >= limit {
				break
			}
		}
	}

	// Videos: title matches first, then the media of any matched person (incl. alias
	// matches) so searching a person's name OR alias returns their library — the merge
	// promise (F23, ADR-036). Deduped by id, capped at limit.
	titleVids, _, err := r.ListVideos(ctx, VideoFilter{Query: q, Limit: limit})
	if err != nil {
		return res, err
	}
	res.Videos = titleVids
	if len(res.Videos) < limit && len(res.People) > 0 {
		peopleIDs := make([]int64, len(res.People))
		for i, p := range res.People {
			peopleIDs[i] = p.ID
		}
		pvids, _, err := r.ListVideos(ctx, VideoFilter{PersonIDsAny: peopleIDs, Limit: limit})
		if err != nil {
			return res, err
		}
		seenV := make(map[int64]struct{}, len(res.Videos))
		for _, v := range res.Videos {
			seenV[v.ID] = struct{}{}
		}
		for _, v := range pvids {
			if _, dup := seenV[v.ID]; dup {
				continue
			}
			res.Videos = append(res.Videos, v)
			if len(res.Videos) >= limit {
				break
			}
		}
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
	if err := tr.Err(); err != nil {
		return res, err
	}

	// Studios: FTS name matches, a new entity group (F38, ADR-053).
	sr, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.name FROM studios_fts f JOIN studios s ON s.id = f.rowid
		WHERE studios_fts MATCH ? LIMIT ?`, match, limit)
	if err != nil {
		return res, fmt.Errorf("search studios: %w", err)
	}
	defer sr.Close()
	for sr.Next() {
		var s model.Studio
		if err := sr.Scan(&s.ID, &s.Name); err != nil {
			return res, err
		}
		res.Studios = append(res.Studios, s)
	}
	return res, sr.Err()
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
		thumbState sql.NullString
	)
	if err := s.Scan(&v.ID, &v.FilePath, &v.FileSize, &v.Title, &v.Duration,
		&v.Width, &v.Height, &v.VideoCodec, &v.AudioCodec, &v.BitrateKbps, &v.Container,
		&recorded, &indexedStr, &mtimeStr, &thumbState); err != nil {
		return model.Video{}, err
	}
	v.ThumbnailState = thumbState.String
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

// toAnySlice boxes a typed slice into []any for variadic query args.
func toAnySlice[T any](in []T) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}
