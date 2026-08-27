package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/repo"
)

// Film people roles (F56, ADR-085, HOLODEX-281): owner-gated CRUD over
// film_people_roles -- additive, film-only billing/role data (director, billing
// order) distinct from a film's read-only inherited cast (films.go's getFilm
// "cast", the set union over the film's attached videos' own video_people links).
// Mirrors film_videos.go's attach/detach shape: one credited role per person per
// film (repo.ErrFilmPersonAlreadyCredited), addressed by (filmId, personId) so the
// role text itself stays freely editable via PUT rather than living in the URL --
// see repo.ErrFilmPersonAlreadyCredited's doc comment for why.

// mountFilmPeopleRoles registers the owner-gated film-person-role CRUD routes,
// called from mountFilms.
func (h *Handlers) mountFilmPeopleRoles(r chi.Router) {
	r.Post("/films/{filmId}/roles", h.addFilmPersonRole)
	r.Put("/films/{filmId}/roles/{personId}", h.editFilmPersonRole)
	r.Delete("/films/{filmId}/roles/{personId}", h.removeFilmPersonRole)
}

// filmPersonRoleMutationError translates AddFilmPersonRole/EditFilmPersonRole/
// RemoveFilmPersonRole's shared error set into a response. Mirrors
// filmVideoMutationError.
func (h *Handlers) filmPersonRoleMutationError(w http.ResponseWriter, op string, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "film or person not found")
	case errors.Is(err, repo.ErrFilmPersonAlreadyCredited):
		writeError(w, http.StatusConflict, "person already has a credited role on this film")
	default:
		h.fail(w, op, err)
	}
	return true
}

// requireLiveFilmAndPerson validates that both the film and the person exist,
// writing 404 and returning false otherwise. Mirrors requireLiveFilmAndVideo.
func (h *Handlers) requireLiveFilmAndPerson(w http.ResponseWriter, r *http.Request, filmID, personID int64) bool {
	if _, err := h.repo.GetFilm(r.Context(), filmID); err != nil {
		h.filmLookupError(w, err)
		return false
	}
	if _, err := h.repo.GetPerson(r.Context(), personID); err != nil {
		h.personLookupError(w, err)
		return false
	}
	return true
}

// addFilmPersonRole handles POST /films/{filmId}/roles (owner-gated): {person_id,
// role, billing_order}. role is optional (empty = unset, mirrors video_people's
// sentinel); billing_order is optional (nil = unranked). 409 if the person is
// already credited on this film -- use PUT to change their existing role.
func (h *Handlers) addFilmPersonRole(w http.ResponseWriter, r *http.Request) {
	filmID, ok := urlParamID(w, r, "filmId")
	if !ok {
		return
	}
	var body struct {
		PersonID     int64  `json:"person_id"`
		Role         string `json:"role"`
		BillingOrder *int64 `json:"billing_order"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.PersonID <= 0 {
		writeError(w, http.StatusBadRequest, "person_id is required")
		return
	}
	if !h.requireLiveFilmAndPerson(w, r, filmID, body.PersonID) {
		return
	}
	role := strings.TrimSpace(body.Role)
	err := h.repo.AddFilmPersonRole(r.Context(), filmID, body.PersonID, role, body.BillingOrder)
	if h.filmPersonRoleMutationError(w, "add film person role", err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// editFilmPersonRole handles PUT /films/{filmId}/roles/{personId} (owner-gated):
// {role, billing_order}, full-replacing the person's existing credited role. 404 if
// the person has no credited role on this film yet -- use POST to create one.
func (h *Handlers) editFilmPersonRole(w http.ResponseWriter, r *http.Request) {
	filmID, ok := urlParamID(w, r, "filmId")
	if !ok {
		return
	}
	personID, ok := urlParamID(w, r, "personId")
	if !ok {
		return
	}
	var body struct {
		Role         string `json:"role"`
		BillingOrder *int64 `json:"billing_order"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	role := strings.TrimSpace(body.Role)
	err := h.repo.EditFilmPersonRole(r.Context(), filmID, personID, role, body.BillingOrder)
	if h.filmPersonRoleMutationError(w, "edit film person role", err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// removeFilmPersonRole handles DELETE /films/{filmId}/roles/{personId}
// (owner-gated). 404 if the person had no credited role (not idempotent, mirrors
// detachFilmVideo).
func (h *Handlers) removeFilmPersonRole(w http.ResponseWriter, r *http.Request) {
	filmID, ok := urlParamID(w, r, "filmId")
	if !ok {
		return
	}
	personID, ok := urlParamID(w, r, "personId")
	if !ok {
		return
	}
	err := h.repo.RemoveFilmPersonRole(r.Context(), filmID, personID)
	if h.filmPersonRoleMutationError(w, "remove film person role", err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
