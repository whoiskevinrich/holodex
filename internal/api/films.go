package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Film entity endpoints (F56, ADR-085). Reads are public, gated on films_enabled at
// the Mount call site (unregistered entirely when off, per spec); mutations are
// owner-gated below, mirroring studios.go/mountStudioImages' split.

// mountFilms registers the owner-gated film mutations (create + the attach/detach/
// bulk-attach endpoints in film_videos.go). The public list/get/search reads are
// mounted ungated, alongside listStudios/getStudio.
func (h *Handlers) mountFilms(r chi.Router) {
	r.Post("/films", h.createFilm)
	h.mountFilmVideos(r)
	h.mountFilmDecisions(r)
	h.mountFilmStudioCascade(r)
	h.mountFilmPeopleRoles(r)
}

// listFilms handles GET /films (F56): name-sorted films with active-video counts.
// Public, mirroring listStudios' unfiltered posture. Unlike studios, empty films
// are never excluded -- film_videos has no prune-on-empty (see films.go's package doc).
// ?person_id=/?studio_id=/?tag_id= filter to films whose video union includes that
// entity (the films row on person/studio/tag detail pages) -- mutually exclusive
// with ?q in practice, but if both are given, ?q wins (name search, no entity join).
func (h *Handlers) listFilms(w http.ResponseWriter, r *http.Request) {
	var (
		films []model.Film
		err   error
	)
	q := r.URL.Query()
	op := "list films"
	switch {
	case strings.TrimSpace(q.Get("q")) != "":
		op = "search films"
		films, err = h.repo.SearchFilms(r.Context(), q.Get("q"), 25)
	case q.Get("person_id") != "" || q.Get("studio_id") != "" || q.Get("tag_id") != "":
		op = "list films for entity"
		films, err = h.repo.ListFilmsForEntity(r.Context(),
			int64(atoiDefault(q.Get("person_id"), 0)),
			int64(atoiDefault(q.Get("studio_id"), 0)),
			int64(atoiDefault(q.Get("tag_id"), 0)))
	default:
		films, err = h.repo.ListFilms(r.Context())
	}
	if err != nil {
		h.fail(w, op, err)
		return
	}
	for i := range films {
		setFilmImageURLs(&films[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": films})
}

// getFilm handles GET /films/{id} (F56): the film, its resolved[] fields (record
// vocabulary), its scenes and full-film files (the two-region detail page, spec),
// the read-only inherited cast/tags/studios (set union over its videos), and the
// owner-entered credited_roles (HOLODEX-281, film_people_roles) -- kept as a
// separate field from cast so the response clearly distinguishes "appears in a
// scene" from "credited on the film itself". Public.
func (h *Handlers) getFilm(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	f, err := h.repo.GetFilm(r.Context(), id)
	if err != nil {
		h.filmLookupError(w, err)
		return
	}
	setFilmImageURLs(f)
	resolved := h.resolveFilm(r.Context(), id, f)

	fvs, err := h.repo.FilmVideos(r.Context(), id)
	if err != nil {
		h.fail(w, "film videos", err)
		return
	}
	authorized := h.auth.authorized(r)
	scenes := []repo.FilmVideo{}
	fullFilms := []repo.FilmVideo{}
	for _, fv := range fvs {
		setThumbnailURL(&fv.Video)
		redactFileMetadataForVisitor(&fv.Video, authorized)
		if fv.IsFullFilm {
			fullFilms = append(fullFilms, fv)
		} else {
			scenes = append(scenes, fv)
		}
	}

	cast, err := h.repo.FilmCast(r.Context(), id)
	if err != nil {
		h.fail(w, "film cast", err)
		return
	}
	tags, err := h.repo.FilmTags(r.Context(), id)
	if err != nil {
		h.fail(w, "film tags", err)
		return
	}
	studios, err := h.repo.FilmStudios(r.Context(), id)
	if err != nil {
		h.fail(w, "film studios", err)
		return
	}
	credited, err := h.repo.FilmPeopleRoles(r.Context(), id)
	if err != nil {
		h.fail(w, "film people roles", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"film":           f,
		"resolved":       resolved,
		"scenes":         scenes,
		"full_films":     fullFilms,
		"cast":           cast,
		"tags":           tags,
		"studios":        studios,
		"credited_roles": credited,
	})
}

// createFilm handles POST /films (owner-gated): {name, year}. name is required;
// year is optional (0 = undated). A name+year collision is get-or-create, not an
// error -- returns 200 with the existing film rather than 409, since the video→film
// picker's "create new" action should be idempotent against a duplicate submit, not
// force the caller to branch on a conflict it can't usefully resolve differently.
func (h *Handlers) createFilm(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Year int    `json:"year"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	id, err := h.repo.CreateFilm(r.Context(), name, body.Year)
	status := http.StatusCreated
	switch {
	case errors.Is(err, repo.ErrFilmExists):
		status = http.StatusOK
	case err != nil:
		h.fail(w, "create film", err)
		return
	}
	f, err := h.repo.GetFilm(r.Context(), id)
	if err != nil {
		h.fail(w, "get created film", err)
		return
	}
	setFilmImageURLs(f)
	writeJSON(w, status, map[string]any{"film": f})
}
