package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// VideoCollision is the other active video that exactly matches a proposed composite
// key {title, people, date, studio} (HOLODEX-270). Populated with resolved display
// values (not raw ids) so the API response — and the frontend verdict card it feeds —
// needs no follow-up fetch.
type VideoCollision struct {
	ID         int64    `json:"id"`
	Title      string   `json:"title"`
	People     []string `json:"people"`
	RecordedAt *string  `json:"recorded_at"`
	Studios    []string `json:"studios"`
}

// compositeKeyCandidate is one other active video sharing a proposed effective-title+
// date pair — the cheap first filter shared by every composite-key collision check,
// before the more expensive people/studio set comparison narrows it further. Title+
// date matches are rare in a personal media library, so filtering on these indexed
// columns first avoids materializing a group_concat key across every video just to
// rule out the common case of zero matches. title is the *effective* title (a
// candidate's standing manual decision, if any, else its file title) so a candidate
// whose displayed title differs from its raw file title is still matched/displayed
// correctly.
type compositeKeyCandidate struct {
	id    int64
	title string
}

// FindTitleCollision reports whether renaming videoID's title to proposedTitle would
// produce a composite-key collision {title, people, date, studio} against another
// active video. People and Studio are read from videoID's *current* links since only
// Title changes on this path — the inverse of FindStudioCollision (studio varies,
// title fixed). Comparison is exact-normalized only (lower+trim on title, exact match
// on date/people-set/studio-set) — no fuzzy/near-miss matching, per the spec's "no
// merge verb, no third option" posture.
func (r *Repo) FindTitleCollision(ctx context.Context, videoID int64, proposedTitle string) (*VideoCollision, error) {
	recordedAt, err := r.recordedAtOf(ctx, videoID)
	if err != nil {
		return nil, fmt.Errorf("find title collision: source video: %w", err)
	}
	peopleKey, err := r.linkedIDKey(ctx,
		`SELECT person_id FROM video_people WHERE video_id = ? ORDER BY person_id`, videoID)
	if err != nil {
		return nil, fmt.Errorf("find title collision: people: %w", err)
	}
	studioKey, err := r.linkedIDKey(ctx,
		`SELECT studio_id FROM video_studios WHERE video_id = ? ORDER BY studio_id`, videoID)
	if err != nil {
		return nil, fmt.Errorf("find title collision: studios: %w", err)
	}

	candidates, err := r.compositeKeyCandidates(ctx, videoID, proposedTitle, recordedAt)
	if err != nil {
		return nil, fmt.Errorf("find title collision: %w", err)
	}

	for _, cand := range candidates {
		ck, err := r.linkedIDKey(ctx,
			`SELECT person_id FROM video_people WHERE video_id = ? ORDER BY person_id`, cand.id)
		if err != nil {
			return nil, fmt.Errorf("find title collision: candidate people: %w", err)
		}
		if ck != peopleKey {
			continue
		}
		sk, err := r.linkedIDKey(ctx,
			`SELECT studio_id FROM video_studios WHERE video_id = ? ORDER BY studio_id`, cand.id)
		if err != nil {
			return nil, fmt.Errorf("find title collision: candidate studios: %w", err)
		}
		if sk != studioKey {
			continue
		}
		return r.hydrateCollision(ctx, cand.id, cand.title, recordedAt)
	}
	return nil, nil
}

// FindStudioCollision reports whether reassigning videoID's studio to
// proposedStudioNames would produce a composite-key collision {title, people, date,
// studio} against another active video. Title, date, and people are read from
// videoID's *current* row/links since only Studio changes on this path (HOLODEX-271).
// Comparison is by normalized studio *name*, not id, so a proposed studio that
// doesn't exist yet — the picker's create-new path — still collides correctly
// against an existing video's linked studio names.
func (r *Repo) FindStudioCollision(ctx context.Context, videoID int64, proposedStudioNames []string) (*VideoCollision, error) {
	var title string
	var recordedAt sql.NullString
	if err := r.db.QueryRowContext(ctx,
		`SELECT title, recorded_at FROM videos WHERE id = ? AND deleted_at IS NULL`, videoID,
	).Scan(&title, &recordedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find studio collision: source video: %w", err)
	}
	peopleKey, err := r.linkedIDKey(ctx,
		`SELECT person_id FROM video_people WHERE video_id = ? ORDER BY person_id`, videoID)
	if err != nil {
		return nil, fmt.Errorf("find studio collision: people: %w", err)
	}
	studioKey := normalizedNameKey(proposedStudioNames)

	candidates, err := r.compositeKeyCandidates(ctx, videoID, title, recordedAt)
	if err != nil {
		return nil, fmt.Errorf("find studio collision: %w", err)
	}

	for _, cand := range candidates {
		ck, err := r.linkedIDKey(ctx,
			`SELECT person_id FROM video_people WHERE video_id = ? ORDER BY person_id`, cand.id)
		if err != nil {
			return nil, fmt.Errorf("find studio collision: candidate people: %w", err)
		}
		if ck != peopleKey {
			continue
		}
		sk, err := r.linkedNameKey(ctx,
			`SELECT s.name FROM video_studios vs JOIN studios s ON s.id = vs.studio_id WHERE vs.video_id = ?`, cand.id)
		if err != nil {
			return nil, fmt.Errorf("find studio collision: candidate studios: %w", err)
		}
		if sk != studioKey {
			continue
		}
		return r.hydrateCollision(ctx, cand.id, cand.title, recordedAt)
	}
	return nil, nil
}

// recordedAtOf reads an active video's recorded_at, shared by every composite-key
// collision check that needs the *current* row (Title's check needs it as the fixed
// half of the candidate filter; Studio's check reads it directly instead since it
// also needs the current title in the same query).
func (r *Repo) recordedAtOf(ctx context.Context, videoID int64) (sql.NullString, error) {
	var recordedAt sql.NullString
	if err := r.db.QueryRowContext(ctx,
		`SELECT recorded_at FROM videos WHERE id = ? AND deleted_at IS NULL`, videoID,
	).Scan(&recordedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return recordedAt, ErrNotFound
		}
		return recordedAt, err
	}
	return recordedAt, nil
}

// compositeKeyCandidates finds other active videos sharing a normalized effective-
// title+date pair with videoID — the cheap indexed-column filter every composite-key
// collision check runs first, before the more expensive people/studio set comparison
// narrows survivors down. Title+date matches are rare in a personal media library, so
// this avoids materializing a group_concat key across every video in the table just to
// rule out the common case of zero matches. A candidate's *effective* title (its
// standing manual decision, if any, else its file title) is used for both the match
// and the returned display value, so a candidate whose displayed title diverges from
// its raw file title is neither missed nor misreported.
func (r *Repo) compositeKeyCandidates(ctx context.Context, videoID int64, title string, recordedAt sql.NullString) ([]compositeKeyCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT v.id, COALESCE(fsd.manual_value, v.title) AS effective_title FROM videos v
		LEFT JOIN field_source_decisions fsd
		  ON fsd.entity_type = 'video' AND fsd.entity_id = v.id
		  AND fsd.field_key = 'title' AND fsd.source = 'manual'
		WHERE v.active = 1 AND v.deleted_at IS NULL AND v.id != ?
		  AND lower(trim(COALESCE(fsd.manual_value, v.title))) = lower(trim(?))
		  AND v.recorded_at IS ?`,
		videoID, title, recordedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("candidates: %w", err)
	}
	defer rows.Close()
	var candidates []compositeKeyCandidate
	for rows.Next() {
		var c compositeKeyCandidate
		if err := rows.Scan(&c.id, &c.title); err != nil {
			return nil, fmt.Errorf("candidates: %w", err)
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

// hydrateCollision fills in a winning candidate's people/studio/recorded_at display
// values, shared by both Find*Collision functions once they've settled on a match.
func (r *Repo) hydrateCollision(ctx context.Context, id int64, title string, recordedAt sql.NullString) (*VideoCollision, error) {
	c := VideoCollision{ID: id, Title: title}
	if recordedAt.Valid {
		c.RecordedAt = &recordedAt.String
	}

	people, err := r.PeopleForVideos(ctx, []int64{id})
	if err != nil {
		return nil, fmt.Errorf("collision people: %w", err)
	}
	// c.People must marshal as `[]`, never `null` — CollisionOfferCard reads
	// video.people.length unconditionally (HOLODEX-270).
	c.People = []string{}
	for _, p := range people[id] {
		c.People = append(c.People, p.Name)
	}
	studios, err := r.StudiosForVideos(ctx, []int64{id})
	if err != nil {
		return nil, fmt.Errorf("collision studios: %w", err)
	}
	// c.Studios must marshal as `[]`, never `null`, mirroring c.People above.
	c.Studios = []string{}
	for _, s := range studios[id] {
		c.Studios = append(c.Studios, s.Name)
	}
	return &c, nil
}

// linkedIDKey runs a fixed, caller-supplied SELECT of a single int64 column and
// returns its results as a comma-joined key, so two videos' people/studio id sets can
// be compared for exact equality by comparing the resulting strings.
func (r *Repo) linkedIDKey(ctx context.Context, query string, videoID int64) (string, error) {
	rows, err := r.db.QueryContext(ctx, query, videoID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ","), rows.Err()
}

// linkedNameKey runs a fixed, caller-supplied SELECT of a single name column and
// returns a normalizedNameKey over the results — the name-based sibling of
// linkedIDKey, used for Studio (HOLODEX-271) since a proposed studio may not have an
// id yet (the picker's create-new path).
func (r *Repo) linkedNameKey(ctx context.Context, query string, videoID int64) (string, error) {
	rows, err := r.db.QueryContext(ctx, query, videoID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return "", err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return normalizedNameKey(names), nil
}

// normalizedNameKey folds (foldNameKey) and sorts names into a comma-joined key so
// two name sets compare equal regardless of case or input order — mirrors
// linkedIDKey's role for id sets.
func normalizedNameKey(names []string) string {
	norm := make([]string, 0, len(names))
	for _, n := range names {
		if n = foldNameKey(n); n != "" {
			norm = append(norm, n)
		}
	}
	sort.Strings(norm)
	return strings.Join(norm, ",")
}
