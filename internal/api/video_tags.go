package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/repo"
)

// Video↔tag attach/detach API (F50, ADR-075 P0-7) — the owner's media-page add/
// remove tag chips. Named "/media/{id}/tags" to match this codebase's existing
// video-resource noun (getMedia, refreshMedia, extractMedia, …), not the spec's
// "/videos/{id}/tags" shorthand. Owner-gated (mounted in the requireOwner group).

// mountVideoTags registers the attach/detach routes.
func (h *Handlers) mountVideoTags(r chi.Router) {
	r.Post("/media/{id}/tags", h.attachVideoTag)
	r.Delete("/media/{id}/tags/{tagID}", h.detachVideoTag)
}

// attachVideoTag resolves-or-creates a tag by name and links it to the video,
// source='manual'. 422 on a denied term, 400 on an oversized name, 409 on a
// name that collides with an existing category (ADR-077 D3) -- all three from
// the shared resolveOrCreateByName choke point -- 404 if the video doesn't exist.
func (h *Handlers) attachVideoTag(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	tag, err := h.repo.AttachTagToVideo(r.Context(), id, name)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "video not found")
		return
	case errors.Is(err, repo.ErrTagDenied):
		writeError(w, http.StatusUnprocessableEntity, "term is on the deny-list")
		return
	case errors.Is(err, repo.ErrTagNameTooLong):
		writeError(w, http.StatusBadRequest, "name is too long")
		return
	case errors.Is(err, repo.ErrTagNameCollidesWithCategory):
		writeError(w, http.StatusConflict, "that name already belongs to a category")
		return
	case err != nil:
		h.fail(w, "attach video tag", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": tag})
}

// detachVideoTag unlinks a tag from a video. 404 if the tag wasn't attached (or
// either id is unknown — both collapse to the same DELETE-affected-zero-rows case).
func (h *Handlers) detachVideoTag(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	tagID, err := strconv.ParseInt(chi.URLParam(r, "tagID"), 10, 64)
	if err != nil || tagID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid tag id")
		return
	}
	switch err := h.repo.DetachTagFromVideo(r.Context(), id, tagID); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "tag not attached to video")
		return
	case err != nil:
		h.fail(w, "detach video tag", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
