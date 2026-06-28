package api

import (
	"errors"
	"net/http"

	"holodex/internal/repo"
)

// refreshMedia re-extracts one media file's embedded metadata (forced, bypassing
// the scanner's (size, mtime) change-detection) and persists the file layer
// (F31, ADR-047). Owner-gated; per-item path mirrors /media/{id}/enrich and
// /writeback. This slice covers the file layer only; re-enrich of linked
// providers follows.
func (h *Handlers) refreshMedia(w http.ResponseWriter, r *http.Request) {
	if h.refresh == nil {
		writeError(w, http.StatusServiceUnavailable, "refresh unavailable")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}

	report, err := h.refresh.Refresh(r.Context(), id)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
		return
	case errors.Is(err, repo.ErrDeleted):
		// The row exists but is in Trash — refuse rather than reactivate it
		// (ADR-037 #26 guard). 409 is truer than 404: the item is real.
		writeError(w, http.StatusConflict, "item is deleted")
		return
	case err != nil:
		// Includes a missing/unreadable file: nothing was persisted, the row is
		// untouched. Surface a generic error (no raw paths in the body).
		h.log.Warn("refresh failed", "id", id, "err", err)
		writeError(w, http.StatusBadGateway, "refresh failed")
		return
	}
	writeJSON(w, http.StatusAccepted, report)
}
