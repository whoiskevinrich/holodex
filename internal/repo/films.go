package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"holodex/internal/model"
)

// Films (F56, ADR-085): the first entity whose video membership (film_videos) is an
// owner ASSERTION, not a value derived from resolved fields -- see FilmAttachment's
// doc comment below. Name+year is the identity key (UNIQUE(name, year), migration
// 0043); films are deliberately NOT part of the shared alias/merge identity spine
// (F43) that person/studio/tag ride -- a title collision across different
// releases/years is the common, legitimate case here, not a scanner-driven duplicate
// to fold away.

// FilmAttachment is one video's link to a film (F56, ADR-085 §4) -- read to inject
// synthetic "film:<id>" resolver-source candidates at the getMedia call site. Unlike
// StudiosForVideos/PeopleForVideos, film_videos is an asserted owner link with no
// reconciler, so this is a plain read with no relink semantics.
type FilmAttachment struct {
	FilmID     int64  `json:"film_id"`
	FilmName   string `json:"film_name"`
	IsFullFilm bool   `json:"is_full_film"`
	// SceneNumber is nil for an unnumbered scene (or a full-film attachment, which
	// carries no scene number) -- added for the media detail page's Films section
	// pill badge (design handoff §3a), which needs "#6" or "Full film", not just the
	// is_full_film flag.
	SceneNumber *int64 `json:"scene_number"`
}

// FilmsForVideo returns the films a single video is attached to (F56, ADR-085) -- a
// video may belong to zero to many films. Ordered by film name for deterministic
// output. A thin wrapper over FilmsForVideos so the two share one scan.
func (r *Repo) FilmsForVideo(ctx context.Context, videoID int64) ([]FilmAttachment, error) {
	byVideo, err := r.FilmsForVideos(ctx, []int64{videoID})
	if err != nil {
		return nil, err
	}
	return byVideo[videoID], nil
}

// FilmsForVideos returns each of the given videos' film attachments, keyed by
// video id -- a batched form of FilmsForVideo (avoids an N+1 when the film→video
// candidates picker needs to flag "already attached elsewhere" across a page of
// results). Videos with no film link are absent from the map.
func (r *Repo) FilmsForVideos(ctx context.Context, ids []int64) (map[int64][]FilmAttachment, error) {
	if len(ids) == 0 {
		return map[int64][]FilmAttachment{}, nil
	}
	q := `SELECT fv.video_id, f.id, f.name, fv.is_full_film, fv.scene_number
	      FROM film_videos fv JOIN films f ON f.id = fv.film_id
	      WHERE fv.video_id IN (` + placeholders(len(ids)) + `)
	      ORDER BY fv.video_id, f.name COLLATE NOCASE`
	rows, err := r.db.QueryContext(ctx, q, toAnySlice(ids)...)
	if err != nil {
		return nil, fmt.Errorf("films for videos: %w", err)
	}
	defer rows.Close()
	out := make(map[int64][]FilmAttachment, len(ids))
	for rows.Next() {
		var vid int64
		var fa FilmAttachment
		var isFull int
		if err := rows.Scan(&vid, &fa.FilmID, &fa.FilmName, &isFull, &fa.SceneNumber); err != nil {
			return nil, err
		}
		fa.IsFullFilm = isFull != 0
		out[vid] = append(out[vid], fa)
	}
	return out, rows.Err()
}

// ErrFilmExists is returned by CreateFilm when name+year already names a film --
// get-or-create, not a hard failure (the returned id is the existing film's), so the
// video→film picker's "create new" action is idempotent against a duplicate submit.
var ErrFilmExists = errors.New("film already exists")

// ErrSceneNumberTaken is returned by AttachFilmVideo/BulkAttachFilmVideos when the
// requested scene number is already occupied within the film (spec: reject naming
// the current occupant, never a silent swap or auto-bump renumber).
var ErrSceneNumberTaken = errors.New("scene number already taken")

// ErrFilmVideoAlreadyAttached is returned when the (film, video) pair is already
// linked. Attach is deliberately not an idempotent upsert: a second attach call with
// a different scene number/is_full_film would otherwise silently change the existing
// owner assertion.
var ErrFilmVideoAlreadyAttached = errors.New("video already attached to this film")

// FilmSceneCollision names the video already occupying a scene number, letting the
// API layer's 409 name the occupant instead of just rejecting blind.
type FilmSceneCollision struct {
	VideoID    int64  `json:"video_id"`
	VideoTitle string `json:"video_title"`
}

// nullableYear turns a zero-or-negative "no year" sentinel into SQL NULL --
// films.year is nullable (migration 0043); 0 is not a valid release year.
func nullableYear(year int) any {
	if year <= 0 {
		return nil
	}
	return year
}

// scanFilm reads one films row plus its active-video count from a *sql.Row-shaped
// scanner, shared by GetFilm/ListFilms/SearchFilms.
func scanFilm(scan func(...any) error) (model.Film, error) {
	var f model.Film
	var year sql.NullInt64
	if err := scan(&f.ID, &f.Name, &year, &f.VideoCount); err != nil {
		return model.Film{}, err
	}
	if year.Valid {
		f.Year = int(year.Int64)
	}
	return f, nil
}

// scanFilms drains a filmSelectCols result set. Non-nil so a zero-row result
// marshals as `[]`, never `null` (HOLODEX-275).
func scanFilms(rows *sql.Rows) ([]model.Film, error) {
	out := []model.Film{}
	for rows.Next() {
		f, err := scanFilm(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// video_count excludes full-film rows — it's rendered client-side as a scene count
// (the films list's "N scenes" label), and a full-film-only release with no
// separate scene rips would otherwise be mislabeled "1 scene".
const filmSelectCols = `f.id, f.name, f.year,
	(SELECT COUNT(*) FROM film_videos fv JOIN videos v ON v.id = fv.video_id
	 WHERE fv.film_id = f.id AND fv.is_full_film = 0 AND v.active = 1 AND v.deleted_at IS NULL)`

// CreateFilm inserts a new film. A pre-existing (name, year COLLATE NOCASE) match
// returns that film's id with ErrFilmExists rather than creating a duplicate --
// name+year is films' whole identity key, so this is CreateFilm's equivalent of the
// identity-spine's resolve-or-create, without the alias/merge machinery films don't
// use (see the package doc comment above).
func (r *Repo) CreateFilm(ctx context.Context, name string, year int) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("create film: name is required")
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	var existing int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM films WHERE name = ? COLLATE NOCASE AND year IS ?`,
		name, nullableYear(year)).Scan(&existing)
	switch {
	case err == nil:
		return existing, ErrFilmExists
	case !errors.Is(err, sql.ErrNoRows):
		return 0, fmt.Errorf("check film exists: %w", err)
	}

	res, err := r.db.ExecContext(ctx, `INSERT INTO films (name, year) VALUES (?, ?)`, name, nullableYear(year))
	if err != nil {
		return 0, fmt.Errorf("create film: %w", err)
	}
	return res.LastInsertId()
}

// ListFilms returns every film, name-sorted, with its active-video count. Unlike
// ListStudios, a zero-count film is NOT excluded: film_videos has no prune-on-empty
// reconciler (it's an assertion, not a derived index), so an empty film is a
// legitimate, persistent record -- e.g. just created and not yet attached to
// anything, or every scene since detached.
func (r *Repo) ListFilms(ctx context.Context) ([]model.Film, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+filmSelectCols+` FROM films f ORDER BY f.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list films: %w", err)
	}
	defer rows.Close()
	films, err := scanFilms(rows)
	if err != nil {
		return nil, err
	}
	if err := r.attachFilmImages(ctx, films); err != nil {
		return nil, err
	}
	return films, nil
}

// ListFilmsForEntity returns every film whose video union (film_videos joined
// through video_people/video_studios/video_tags) includes the given
// person/studio/tag, name-sorted -- the films row on person/studio/tag detail
// pages (F56). Zero-valued ids are ignored; a call with every id zero behaves
// like ListFilms. Multiple non-zero ids AND together (mirrors VideoFilter).
func (r *Repo) ListFilmsForEntity(ctx context.Context, personID, studioID, tagID int64) ([]model.Film, error) {
	where := "1=1"
	var args []any
	for _, dim := range []struct {
		id           int64
		table, alias string
	}{
		{personID, "video_people", "vp.person_id"},
		{studioID, "video_studios", "vs.studio_id"},
		{tagID, "video_tags", "vt.tag_id"},
	} {
		if dim.id <= 0 {
			continue
		}
		joinAlias := strings.SplitN(dim.alias, ".", 2)[0]
		where += ` AND EXISTS (SELECT 1 FROM film_videos fv JOIN ` + dim.table + ` ` + joinAlias + ` ON ` + joinAlias + `.video_id = fv.video_id
			WHERE fv.film_id = f.id AND ` + dim.alias + ` = ?)`
		args = append(args, dim.id)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+filmSelectCols+` FROM films f WHERE `+where+` ORDER BY f.name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, fmt.Errorf("list films for entity: %w", err)
	}
	defer rows.Close()
	films, err := scanFilms(rows)
	if err != nil {
		return nil, err
	}
	if err := r.attachFilmImages(ctx, films); err != nil {
		return nil, err
	}
	return films, nil
}

// GetFilm returns a film by id with its active-video count, or ErrNotFound.
func (r *Repo) GetFilm(ctx context.Context, id int64) (*model.Film, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+filmSelectCols+` FROM films f WHERE f.id = ?`, id)
	f, err := scanFilm(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	versions, err := r.filmImageVersions(ctx, []int64{id})
	if err != nil {
		return nil, err
	}
	f.ImageVersions = versions[id]
	return &f, nil
}

// SearchFilms returns films whose name FTS-matches query (films_fts, migration
// 0043), name-sorted -- the video→film picker's small-scale (low hundreds) name
// search (spec: "results by film name, poster, and year"). An empty query matches
// nothing (mirrors the other _fts searches' MATCH "" behavior); limit defaults/caps
// to 25/100.
func (r *Repo) SearchFilms(ctx context.Context, query string, limit int) ([]model.Film, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	match := ftsPrefixQuery(query)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+filmSelectCols+`
		FROM films_fts ft JOIN films f ON f.id = ft.rowid
		WHERE films_fts MATCH ?
		ORDER BY f.name COLLATE NOCASE LIMIT ?`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("search films: %w", err)
	}
	defer rows.Close()
	films, err := scanFilms(rows)
	if err != nil {
		return nil, err
	}
	if err := r.attachFilmImages(ctx, films); err != nil {
		return nil, err
	}
	return films, nil
}

// FilmVideo is one scene/full-film row on a film's detail page: the video plus its
// film_videos attachment attributes.
type FilmVideo struct {
	Video       model.Video `json:"video"`
	SceneNumber *int64      `json:"scene_number"`
	IsFullFilm  bool        `json:"is_full_film"`
}

// FilmVideos returns every video attached to a film, scenes first (ordered by scene
// number, unnumbered last) then full-film files, each ordered by title. The API
// layer splits this into the film detail page's two regions (spec: scenes list vs.
// full-film section) by IsFullFilm.
func (r *Repo) FilmVideos(ctx context.Context, filmID int64) ([]FilmVideo, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT v.id, v.file_path, v.title, v.duration_sec, v.width, v.height,
		       v.file_mtime, fv.scene_number, fv.is_full_film
		FROM film_videos fv JOIN videos v ON v.id = fv.video_id
		WHERE fv.film_id = ? AND v.active = 1 AND v.deleted_at IS NULL
		ORDER BY fv.is_full_film, fv.scene_number IS NULL, fv.scene_number, v.title COLLATE NOCASE`, filmID)
	if err != nil {
		return nil, fmt.Errorf("film videos: %w", err)
	}
	defer rows.Close()
	var out []FilmVideo
	for rows.Next() {
		var fv FilmVideo
		var scene sql.NullInt64
		var isFull int
		var mtimeStr string
		if err := rows.Scan(&fv.Video.ID, &fv.Video.FilePath, &fv.Video.Title, &fv.Video.Duration,
			&fv.Video.Width, &fv.Video.Height, &mtimeStr, &scene, &isFull); err != nil {
			return nil, err
		}
		fv.Video.FileMtime, _ = time.Parse(timeLayout, mtimeStr)
		fv.IsFullFilm = isFull != 0
		if scene.Valid {
			fv.SceneNumber = &scene.Int64
		}
		out = append(out, fv)
	}
	return out, rows.Err()
}

// filmSceneOccupant looks up the video currently holding a scene number within a
// film, for the ErrSceneNumberTaken collision payload. Runs inside the caller's tx.
func filmSceneOccupant(ctx context.Context, q queryRower, filmID, sceneNumber int64) (*FilmSceneCollision, error) {
	var c FilmSceneCollision
	err := q.QueryRowContext(ctx,
		`SELECT v.id, v.title FROM film_videos fv JOIN videos v ON v.id = fv.video_id
		 WHERE fv.film_id = ? AND fv.scene_number = ?`, filmID, sceneNumber).Scan(&c.VideoID, &c.VideoTitle)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// insertFilmVideo performs one attach inside the caller's tx: reject an already
// attached pair, reject an occupied scene number (returning the occupant alongside
// ErrSceneNumberTaken), then insert. Shared by AttachFilmVideo and
// BulkAttachFilmVideos so the "no silent swap, no auto-bump renumber" invariant is
// enforced in exactly one place. A nil sceneNumber is unnumbered and never collides.
func insertFilmVideo(ctx context.Context, tx *sql.Tx, filmID, videoID int64, sceneNumber *int64, isFullFilm bool, now string) (*FilmSceneCollision, error) {
	var already int
	if err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM film_videos WHERE film_id = ? AND video_id = ?`, filmID, videoID).Scan(&already); err == nil {
		return nil, ErrFilmVideoAlreadyAttached
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("check existing attachment: %w", err)
	}

	if sceneNumber != nil {
		occ, err := filmSceneOccupant(ctx, tx, filmID, *sceneNumber)
		if err != nil {
			return nil, fmt.Errorf("check scene collision: %w", err)
		}
		if occ != nil {
			return occ, ErrSceneNumberTaken
		}
	}

	full := 0
	if isFullFilm {
		full = 1
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO film_videos (film_id, video_id, scene_number, is_full_film, created_at) VALUES (?, ?, ?, ?, ?)`,
		filmID, videoID, sceneNumber, full, now); err != nil {
		return nil, fmt.Errorf("attach film video: %w", err)
	}
	return nil, nil
}

// AttachFilmVideo links a video to a film as an owner assertion (F56, ADR-085): no
// reconciler ever calls this or its sibling mutators (see the zero-relink-
// participation regression test, film_links_test.go) -- it is only ever invoked by
// an explicit attach/bulk-attach request. sceneNumber nil means unnumbered (legal,
// non-colliding -- migration 0043's NULL-distinct UNIQUE). Returns
// *FilmSceneCollision wrapped in ErrSceneNumberTaken when sceneNumber is already
// occupied, or ErrFilmVideoAlreadyAttached when the pair already exists.
func (r *Repo) AttachFilmVideo(ctx context.Context, filmID, videoID int64, sceneNumber *int64, isFullFilm bool) (*FilmSceneCollision, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if occ, err := insertFilmVideo(ctx, tx, filmID, videoID, sceneNumber, isFullFilm,
		time.Now().UTC().Format(time.RFC3339)); err != nil {
		return occ, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit attach: %w", err)
	}
	return nil, nil
}

// DetachFilmVideo removes a video's link to a film. ErrNotFound if the pair wasn't
// attached (idempotent-detach is the caller's choice, not this method's -- mirrors
// DeleteStudioImage's non-idempotent counterpart DeleteFilmVideo would otherwise
// mask a stale UI's double-click as silent success).
func (r *Repo) DetachFilmVideo(ctx context.Context, filmID, videoID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `DELETE FROM film_videos WHERE film_id = ? AND video_id = ?`, filmID, videoID)
	if err != nil {
		return fmt.Errorf("detach film video: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// BulkAttachFilmVideos attaches many videos to a film in one call, auto-numbering
// them sequentially from startScene (the film→video picker's multi-select attach,
// spec: "sequential auto-numbering from a user-supplied starting scene number").
// All-or-nothing: every number is checked for a collision before any row is
// inserted, so a mid-batch collision never leaves a partial attach behind -- keeping
// the same "no silent swap/auto-bump" invariant AttachFilmVideo enforces for a
// single video. Already-attached videos in the batch are also a hard failure (same
// ErrFilmVideoAlreadyAttached AttachFilmVideo returns), not a silent skip, so the
// caller sees exactly which video is the problem rather than a batch that quietly
// attached fewer videos than requested.
func (r *Repo) BulkAttachFilmVideos(ctx context.Context, filmID int64, videoIDs []int64, startScene int64) (*FilmSceneCollision, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	now := time.Now().UTC().Format(time.RFC3339)
	for i, videoID := range videoIDs {
		scene := startScene + int64(i)
		occ, err := insertFilmVideo(ctx, tx, filmID, videoID, &scene, false, now)
		if err != nil {
			return occ, fmt.Errorf("video %d: %w", videoID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit bulk attach: %w", err)
	}
	return nil, nil
}

// scanFilmUnionRows drains a (id, name) result set from one of FilmCast/FilmTags/
// FilmStudios' identical set-union queries into T, sharing their scan loop.
func scanFilmUnionRows[T any](rows *sql.Rows, build func(id int64, name string) T) ([]T, error) {
	out := []T{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out = append(out, build(id, name))
	}
	return out, rows.Err()
}

// FilmCast returns the read-only set union of people across every video attached to
// a film (spec: "Films should inherit tags and people from its videos/scenes") --
// NOT film_people_roles (film-only additive billing/role data, see
// FilmPeopleRoles in film_people_roles.go). Name-sorted, deduped by person.
func (r *Repo) FilmCast(ctx context.Context, filmID int64) ([]model.Person, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT p.id, p.name
		FROM film_videos fv JOIN video_people vp ON vp.video_id = fv.video_id
		JOIN people p ON p.id = vp.person_id
		WHERE fv.film_id = ?
		ORDER BY p.name COLLATE NOCASE`, filmID)
	if err != nil {
		return nil, fmt.Errorf("film cast: %w", err)
	}
	defer rows.Close()
	return scanFilmUnionRows(rows, func(id int64, name string) model.Person {
		return model.Person{ID: id, Name: name}
	})
}

// FilmTags returns the read-only set union of tags across every video attached to a
// film. Name-sorted, deduped by tag.
func (r *Repo) FilmTags(ctx context.Context, filmID int64) ([]model.Tag, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT t.id, t.name
		FROM film_videos fv JOIN video_tags vt ON vt.video_id = fv.video_id
		JOIN tags t ON t.id = vt.tag_id
		WHERE fv.film_id = ?
		ORDER BY t.name COLLATE NOCASE`, filmID)
	if err != nil {
		return nil, fmt.Errorf("film tags: %w", err)
	}
	defer rows.Close()
	return scanFilmUnionRows(rows, func(id int64, name string) model.Tag {
		return model.Tag{ID: id, Name: name}
	})
}

// FilmStudios returns the read-only set union of studios across every video
// attached to a film. Name-sorted, deduped by studio.
func (r *Repo) FilmStudios(ctx context.Context, filmID int64) ([]model.Studio, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT s.id, s.name
		FROM film_videos fv JOIN video_studios vs ON vs.video_id = fv.video_id
		JOIN studios s ON s.id = vs.studio_id
		WHERE fv.film_id = ?
		ORDER BY s.name COLLATE NOCASE`, filmID)
	if err != nil {
		return nil, fmt.Errorf("film studios: %w", err)
	}
	defer rows.Close()
	return scanFilmUnionRows(rows, func(id int64, name string) model.Studio {
		return model.Studio{ID: id, Name: name}
	})
}

// VideoIDsForFilm returns the active/non-deleted video ids attached to a film via
// film_videos — the Studio cascade's per-video scope (ADR-087 D2), mirroring
// VideoIDsForTag's shape (tag_hierarchy.go).
func (r *Repo) VideoIDsForFilm(ctx context.Context, filmID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT v.id FROM film_videos fv JOIN videos v ON v.id = fv.video_id
		WHERE fv.film_id = ? AND v.active = 1 AND v.deleted_at IS NULL
		ORDER BY v.id`, filmID)
	if err != nil {
		return nil, fmt.Errorf("video ids for film: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var videoID int64
		if err := rows.Scan(&videoID); err != nil {
			return nil, err
		}
		out = append(out, videoID)
	}
	return out, rows.Err()
}
