package repo

import (
	"context"
	"fmt"
)

// FilmAttachment is one video's link to a film (F56, ADR-085 §4) -- read to inject
// synthetic "film:<id>" resolver-source candidates at the getMedia call site. Unlike
// StudiosForVideos/PeopleForVideos, film_videos is an asserted owner link with no
// reconciler, so this is a plain read with no relink semantics.
type FilmAttachment struct {
	FilmID     int64
	FilmName   string
	IsFullFilm bool
}

// FilmsForVideo returns the films a single video is attached to (F56, ADR-085) -- a
// video may belong to zero to many films. Ordered by film name for deterministic output.
func (r *Repo) FilmsForVideo(ctx context.Context, videoID int64) ([]FilmAttachment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT f.id, f.name, fv.is_full_film
		FROM film_videos fv JOIN films f ON f.id = fv.film_id
		WHERE fv.video_id = ?
		ORDER BY f.name COLLATE NOCASE`, videoID)
	if err != nil {
		return nil, fmt.Errorf("films for video: %w", err)
	}
	defer rows.Close()
	var out []FilmAttachment
	for rows.Next() {
		var fa FilmAttachment
		var isFull int
		if err := rows.Scan(&fa.FilmID, &fa.FilmName, &isFull); err != nil {
			return nil, err
		}
		fa.IsFullFilm = isFull != 0
		out = append(out, fa)
	}
	return out, rows.Err()
}
