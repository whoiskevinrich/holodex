package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// mountCuration registers the owner-gated value-level curation endpoints (F30,
// ADR-048). Mounted inside the requireOwner group in Mount.
func (h *Handlers) mountCuration(r chi.Router) {
	r.Post("/media/{id}/curation", h.setCuration)
	r.Post("/media/{id}/curation/clear", h.clearCuration)
}

// curationBody is the shared request shape for set/clear.
type curationBody struct {
	Field  string `json:"field"`
	Value  string `json:"value"`
	Action string `json:"action"` // add | suppress | nowrite
}

func validCurationAction(a string) bool {
	switch a {
	case repo.CurationAdd, repo.CurationSuppress, repo.CurationNoWrite:
		return true
	}
	return false
}

// setCuration records one value-level decision for a field of a video (F30.2/F30.5a).
// The manual value is sanitized as untrusted input (security condition C4/F30.6b).
func (h *Handlers) setCuration(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body curationBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Field == "" || !validCurationAction(body.Action) {
		writeError(w, http.StatusBadRequest, "field and a valid action (add|suppress|nowrite) are required")
		return
	}
	value := enrich.SanitizeValue(body.Value)
	if value == "" {
		writeError(w, http.StatusBadRequest, "value required")
		return
	}
	// Confirm the video exists so curation can't accumulate against unknown ids.
	if _, _, err := h.repo.GetVideo(r.Context(), id); err != nil {
		h.videoLookupError(w, err)
		return
	}
	if err := h.repo.SetCuration(r.Context(), model.EnrichEntityVideo, id, body.Field, value, body.Action); err != nil {
		h.fail(w, "set curation", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clearCuration removes one decision so the underlying source value is restored
// (F30.2e). A clear of a non-existent decision is a no-op success (idempotent).
func (h *Handlers) clearCuration(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body curationBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Field == "" || !validCurationAction(body.Action) {
		writeError(w, http.StatusBadRequest, "field and a valid action (add|suppress|nowrite) are required")
		return
	}
	if _, err := h.repo.ClearCuration(r.Context(), model.EnrichEntityVideo, id, body.Field, body.Value, body.Action); err != nil {
		h.fail(w, "clear curation", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
