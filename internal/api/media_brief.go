package api

import (
	"net/http"

	"holodex/internal/repo"
)

// maxBriefIDs caps one POST /media/brief: the client asks for a long run in chunks of
// this size, which keeps each read inside ListVideos' page cap.
const maxBriefIDs = 1000

// mediaBrief is one row of the run strip's jump list (spec RD14 / P0-13): just enough to
// recognise an item. No file metadata, so there's nothing for a visitor to redact.
type mediaBrief struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	Duration     int    `json:"duration_sec"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

// mediaBriefs handles POST /media/brief {ids}: the title, length and thumbnail of each
// listed video, in request order, for the jump list over a whole run. A POST because a
// run's ids don't fit a URL. Gated like GET /media/{id}: an id that isn't an active,
// undeleted video is left out, so the list can't reveal one that page would 404. Titles
// resolve as browse tiles do, so a row reads the same as the card it came from.
func (h *Handlers) mediaBriefs(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.IDs) > maxBriefIDs {
		writeError(w, http.StatusBadRequest, "too many ids")
		return
	}
	items := []mediaBrief{}
	if len(body.IDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	videos, _, err := h.repo.ListVideos(r.Context(), repo.VideoFilter{IDs: body.IDs, Limit: len(body.IDs)})
	if err != nil {
		h.fail(w, "media brief", err)
		return
	}
	if h.mappings != nil {
		h.applyBrowseTitles(r.Context(), videos, h.mappings.Current().Fields())
	}
	byID := make(map[int64]mediaBrief, len(videos))
	for i := range videos {
		v := &videos[i]
		// setThumbnailURL, not prepareThumbnails: a jump list over a whole run mustn't
		// queue a cover for every item in it. Browse queues the ones it shows.
		h.setThumbnailURL(v)
		byID[v.ID] = mediaBrief{ID: v.ID, Title: v.Title, Duration: v.Duration, ThumbnailURL: v.ThumbnailURL}
	}
	for _, id := range body.IDs {
		if b, ok := byID[id]; ok {
			items = append(items, b)
			delete(byID, id) // a repeated id is listed once
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
