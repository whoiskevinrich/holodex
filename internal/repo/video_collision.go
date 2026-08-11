package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	Studio     *string  `json:"studio"`
}

// FindTitleCollision reports whether renaming videoID's title to proposedTitle would
// produce a composite-key collision {title, people, date, studio} against another
// active video. Title is this story's only wired trigger (HOLODEX-270 P0); Studio and
// People triggers reuse this same check once HOLODEX-271/272 land. People and Studio
// are read from videoID's *current* links since only Title changes on this path.
// Comparison is exact-normalized only (lower+trim on title, exact match on
// date/people-set/studio-set) — no fuzzy/near-miss matching, per the spec's "no merge
// verb, no third option" posture.
//
// Title+date matches are rare in a personal media library, so candidates are filtered
// on the cheap indexed columns first; only survivors get their people/studio sets
// compared. This avoids materializing a group_concat key across every video in the
// library just to rule out the common case of zero matches.
func (r *Repo) FindTitleCollision(ctx context.Context, videoID int64, proposedTitle string) (*VideoCollision, error) {
	var recordedAt sql.NullString
	if err := r.db.QueryRowContext(ctx,
		`SELECT recorded_at FROM videos WHERE id = ? AND deleted_at IS NULL`, videoID,
	).Scan(&recordedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
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

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, title FROM videos
		WHERE active = 1 AND deleted_at IS NULL AND id != ?
		  AND lower(trim(title)) = lower(trim(?))
		  AND COALESCE(recorded_at, '') = COALESCE(?, '')`,
		videoID, proposedTitle, recordedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("find title collision: candidates: %w", err)
	}
	type candidate struct {
		id    int64
		title string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.title); err != nil {
			rows.Close()
			return nil, fmt.Errorf("find title collision: candidates: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find title collision: candidates: %w", err)
	}
	rows.Close()

	var c VideoCollision
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
		c.ID, c.Title = cand.id, cand.title
		break
	}
	if c.ID == 0 {
		return nil, nil
	}
	if recordedAt.Valid {
		c.RecordedAt = &recordedAt.String
	}

	people, err := r.PeopleForVideos(ctx, []int64{c.ID})
	if err != nil {
		return nil, fmt.Errorf("find title collision: collision people: %w", err)
	}
	// c.People must marshal as `[]`, never `null` — CollisionOfferCard reads
	// video.people.length unconditionally (HOLODEX-270).
	c.People = []string{}
	for _, p := range people[c.ID] {
		c.People = append(c.People, p.Name)
	}
	studios, err := r.StudiosForVideos(ctx, []int64{c.ID})
	if err != nil {
		return nil, fmt.Errorf("find title collision: collision studios: %w", err)
	}
	if ss := studios[c.ID]; len(ss) > 0 {
		c.Studio = &ss[0].Name
	}
	return &c, nil
}

// linkedIDKey runs a fixed, caller-supplied SELECT of a single int64 column and
// returns its results as a comma-joined key, so two videos' people/studio sets can be
// compared for exact equality by comparing the resulting strings.
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
