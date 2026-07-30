package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/repo"
)

// Tag hierarchy API (F50, ADR-075 D1) — the owner's `/tags` pill-menu
// "set parent" action. Owner-gated (mounted in the requireOwner group, like
// mountTagDenylist); the hierarchy itself is visible to everyone via the
// ungated GET /tags (Tag.ParentTagID).

// mountTagHierarchy registers the parent-set/clear endpoint.
func (h *Handlers) mountTagHierarchy(r chi.Router) {
	r.Post("/tags/{id}/parent", h.setTagParent)
}

// setTagParent sets or clears (parent_id: null) a tag's parent. 400 with
// {"cycle": true} when the proposed parent is the tag itself or one of its
// own descendants (ADR-075 D1's cycle guard).
func (h *Handlers) setTagParent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		ParentID *int64 `json:"parent_id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.ParentID != nil && *body.ParentID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid parent_id")
		return
	}
	tag, err := h.repo.SetTagParent(r.Context(), id, body.ParentID)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "tag not found")
		return
	case errors.Is(err, repo.ErrTagCycle):
		writeJSON(w, http.StatusBadRequest, map[string]any{"cycle": true})
		return
	case err != nil:
		h.fail(w, "set tag parent", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": tag})
}
