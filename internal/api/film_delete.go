package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/repo"
)

// Film delete/restore (F78, HOLODEX-562) — the film counterpart of delete.go's media
// routes, mounted with the other owner-gated film mutations (so films_enabled=false
// unregisters them too). Trashed films are listed by the shared GET /admin/trash.
func (h *Handlers) mountFilmDelete(r chi.Router) {
	r.Delete("/films/{id}", h.deleteFilm)
	r.Post("/films/{id}/restore", h.restoreFilm)
}

// deleteFilm moves a film to Trash (P0-2), or deletes it permanently now when
// ?purge=true (P0-3) — the latter works on a film already in Trash. 204 on success,
// 404 for an unknown id.
func (h *Handlers) deleteFilm(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if h.purger == nil {
			writeError(w, http.StatusServiceUnavailable, "purge is not available")
			return
		}
		switch err := h.purger.PurgeFilmNow(r.Context(), id); {
		case errors.Is(err, repo.ErrNotFound):
			writeError(w, http.StatusNotFound, "film not found")
		case err != nil:
			// Left in Trash for retry; never echo a filesystem path.
			h.log.Warn("film purge-now failed", "id", id, "err", err)
			writeError(w, http.StatusInternalServerError,
				"could not remove the film's images; it is still in Trash and will be retried")
		default:
			w.WriteHeader(http.StatusNoContent)
		}
		return
	}
	switch err := h.repo.SoftDeleteFilm(r.Context(), id); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "film not found")
	case err != nil:
		h.fail(w, "soft delete film", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// restoreFilm takes a film out of Trash (P0-6) and returns it as GET /films/{id}
// would. 404 when it isn't in Trash.
func (h *Handlers) restoreFilm(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	switch err := h.repo.RestoreFilm(r.Context(), id); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "film not in Trash")
		return
	case err != nil:
		h.fail(w, "restore film", err)
		return
	}
	f, err := h.repo.GetFilm(r.Context(), id)
	if err != nil {
		h.fail(w, "load restored film", err)
		return
	}
	setFilmImageURLs(f)
	writeJSON(w, http.StatusOK, map[string]any{"film": f})
}

// writeTrashedConflict answers a create/rename that landed on a trashed film's key
// with the restore offer (RD6): 409 with the film named under "trashed". Reports
// false, writing nothing, when filmID isn't in Trash — the caller's ordinary conflict
// applies.
func (h *Handlers) writeTrashedConflict(w http.ResponseWriter, r *http.Request, filmID int64) bool {
	c, trashed, err := h.repo.TrashedFilm(r.Context(), filmID)
	if err != nil {
		h.fail(w, "load trashed film", err)
		return true
	}
	if !trashed {
		return false
	}
	writeJSON(w, http.StatusConflict, map[string]any{
		"error":   repo.FilmLabel(c.FilmName, c.Year) + " is in Trash",
		"trashed": c,
	})
	return true
}
