package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// The owner's watch history (F89 P0-7, P0-9; watch-history.md). Both routes mount in the
// requireOwner group (SEC-9, SEC-10).

// recordPlay handles POST /media/{id}/plays: one counted play. The client posts once, when
// a play crosses the counting point, never as a heartbeat.
func (h *Handlers) recordPlay(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Progress *float64 `json:"progress"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Progress == nil || *body.Progress < 0 || *body.Progress > 1 {
		writeError(w, http.StatusBadRequest, "progress is a fraction from 0 to 1")
		return
	}
	h.watchWritten(w, h.repo.RecordPlay(r.Context(), id, *body.Progress), "record play")
}

// setWatched handles PUT /media/{id}/watched: the owner's mark. It adds or removes no plays.
func (h *Handlers) setWatched(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Watched *bool `json:"watched"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Watched == nil {
		writeError(w, http.StatusBadRequest, "watched is true or false")
		return
	}
	h.watchWritten(w, h.repo.SetWatchedMark(r.Context(), id, *body.Watched), "set watched")
}

func (h *Handlers) watchWritten(w http.ResponseWriter, err error, what string) {
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "media not found")
	case err != nil:
		h.fail(w, what, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// attachWatched stamps the watched mark onto videos. Only ever called for the owner, so a
// visitor's payload never carries it (SEC-10).
func (h *Handlers) attachWatched(ctx context.Context, items []model.Video) error {
	ids := make([]int64, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	watched, err := h.repo.WatchedVideos(ctx, ids)
	if err != nil {
		return err
	}
	for i := range items {
		items[i].Watched = watched[items[i].ID]
	}
	return nil
}
