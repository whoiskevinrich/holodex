package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Video playlists API (F69, ADR-104). Reads are ungated at the router and
// visibility-filtered by the request's owner-ness (D5): a visitor lists and opens
// public playlists only, and a private id answers 404 exactly like an unknown
// one. Every mutation mounts inside the requireOwner group. A playlist is a
// container, not an entity (D1): nothing here touches the identity spine.

// maxPlaylistName bounds the free-text name (spec P0-3).
const maxPlaylistName = 200

// mountPlaylists registers the public, visibility-filtered reads.
func (h *Handlers) mountPlaylists(r chi.Router) {
	r.Get("/playlists", h.listPlaylists)
	r.Get("/playlists/{id}", h.getPlaylist)
}

// mountPlaylistMutations registers the owner-gated writes. Mounted inside the
// requireOwner group in Mount.
func (h *Handlers) mountPlaylistMutations(r chi.Router) {
	r.Post("/playlists", h.createPlaylist)
	r.Patch("/playlists/{id}", h.patchPlaylist)
	r.Delete("/playlists/{id}", h.deletePlaylist)
	r.Put("/playlists/{id}/videos/{videoId}", h.addPlaylistVideo)
	r.Delete("/playlists/{id}/videos/{videoId}", h.removePlaylistVideo)
}

func (h *Handlers) listPlaylists(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListPlaylists(r.Context(), !h.auth.authorized(r))
	if err != nil {
		h.fail(w, "list playlists", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// getPlaylist returns the playlist plus its un-trashed members in the playlist's
// order (spec P0-4). ?seed= parameterizes a 'random' sort so one play-through
// walks one shuffle (ADR-045); absent, a seed is minted per request.
func (h *Handlers) getPlaylist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	isOwner := h.auth.authorized(r)
	p, err := h.repo.GetPlaylist(r.Context(), id, !isOwner)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		h.fail(w, "get playlist", err)
		return
	}
	var seed int64
	if p.Sort == "random" {
		seed = parseSeedOrRandom(r.URL.Query().Get("seed"))
	}
	items, err := h.repo.PlaylistVideos(r.Context(), id, p.Sort, seed)
	if err != nil {
		h.fail(w, "playlist videos", err)
		return
	}
	// Same hydration as the browse list so the tile renders identically.
	h.prepareThumbnails(items)
	if h.mappings != nil {
		h.applyBrowseTitles(r.Context(), items, h.mappings.Current().Fields())
	}
	h.applyPartsTo(r.Context(), items)
	if isOwner {
		if err := h.attachVideoCompleteness(r.Context(), items); err != nil {
			h.fail(w, "playlist videos", err)
			return
		}
	}
	redactFileMetadataForVisitors(items, isOwner)
	out := map[string]any{"playlist": p, "items": items, "total": len(items)}
	if p.Sort == "random" {
		out["seed"] = seed
	}
	writeJSON(w, http.StatusOK, out)
}

// playlistBody is the create/patch body. Pointer fields distinguish "absent"
// from "empty" for PATCH; create reads the same shape.
type playlistBody struct {
	Name       *string `json:"name"`
	Sort       *string `json:"sort"`
	Visibility *string `json:"visibility"`
	// FromQuery is a browse filter query string (the F4.7 shareable form) whose
	// whole result set becomes the playlist's membership — the snapshot producer
	// (ADR-104 D3). Create only.
	FromQuery *string `json:"from_query"`
}

// validPlaylistSort accepts every browse sort key plus 'manual' (ADR-104 D2).
func validPlaylistSort(s string) bool {
	return s == model.PlaylistSortManual || repo.ValidSort(s)
}

func validVisibility(s string) bool {
	return s == model.PlaylistPrivate || s == model.PlaylistPublic
}

// validatePlaylistFields checks the optional fields a body carries, writing 400
// and returning false on the first problem. name is trimmed in place.
func validatePlaylistFields(w http.ResponseWriter, b *playlistBody) bool {
	if b.Name != nil {
		n := strings.TrimSpace(*b.Name)
		if n == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return false
		}
		if len(n) > maxPlaylistName {
			writeError(w, http.StatusBadRequest, "name is too long")
			return false
		}
		b.Name = &n
	}
	if b.Sort != nil && !validPlaylistSort(*b.Sort) {
		writeError(w, http.StatusBadRequest, "unknown sort")
		return false
	}
	if b.Visibility != nil && !validVisibility(*b.Visibility) {
		writeError(w, http.StatusBadRequest, "unknown visibility")
		return false
	}
	return true
}

// createPlaylist: POST /playlists {name, sort?, visibility?, from_query?}. With
// from_query the membership is the whole browse result for that filter, in the
// filter's order, and the stored sort is the filter's — except 'random', which
// stores 'manual' with the seeded order of this request, because "save this
// shuffle" means the one on screen (ADR-104 D3). An explicit body sort wins.
func (h *Handlers) createPlaylist(w http.ResponseWriter, r *http.Request) {
	var b playlistBody
	if !decodeJSON(w, r, &b) || !validatePlaylistFields(w, &b) {
		return
	}
	if b.Name == nil {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	sort, visibility := "added_desc", model.PlaylistPrivate
	if b.Visibility != nil {
		visibility = *b.Visibility
	}
	var ids []int64
	if b.FromQuery != nil {
		q, err := url.ParseQuery(strings.TrimPrefix(*b.FromQuery, "?"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from_query")
			return
		}
		f := h.videoFilterFromQuery(q)
		f.HideFullFilmVideos = h.filmsEnabled
		f.MissingFacets = q["missing_facet"]
		f.Limit, f.Offset = 0, 0 // the set, not a page
		if wantsCompleteness(f.Sort, f.MissingFacets) {
			h.drainCompleteness(r.Context()) // score before ranking, as listMedia does
		}
		ids, err = h.repo.ListVideoIDs(r.Context(), f)
		if err != nil {
			h.fail(w, "snapshot playlist", err)
			return
		}
		switch {
		case f.Sort == "random":
			sort = model.PlaylistSortManual
		case repo.ValidSort(f.Sort):
			sort = f.Sort
		}
	}
	if b.Sort != nil {
		sort = *b.Sort
	}
	p, err := h.repo.CreatePlaylist(r.Context(), *b.Name, sort, visibility, ids)
	if err != nil {
		h.fail(w, "create playlist", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"playlist": p})
}

func (h *Handlers) patchPlaylist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b playlistBody
	if !decodeJSON(w, r, &b) || !validatePlaylistFields(w, &b) {
		return
	}
	if b.Name == nil && b.Sort == nil && b.Visibility == nil {
		writeError(w, http.StatusBadRequest, "nothing to update")
		return
	}
	err := h.repo.UpdatePlaylist(r.Context(), id, repo.PlaylistPatch{Name: b.Name, Sort: b.Sort, Visibility: b.Visibility})
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		h.fail(w, "update playlist", err)
		return
	}
	p, err := h.repo.GetPlaylist(r.Context(), id, false)
	if err != nil {
		h.fail(w, "update playlist", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playlist": p})
}

func (h *Handlers) deletePlaylist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	err := h.repo.DeletePlaylist(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		h.fail(w, "delete playlist", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// addPlaylistVideo: PUT /playlists/{id}/videos/{videoId} — append, idempotent
// (spec RD9). Answers with the playlist so the caller's count is current.
func (h *Handlers) addPlaylistVideo(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	videoID, ok := urlParamID(w, r, "videoId")
	if !ok {
		return
	}
	err := h.repo.AddPlaylistVideo(r.Context(), id, videoID)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "playlist or video not found")
		return
	}
	if err != nil {
		h.fail(w, "add playlist video", err)
		return
	}
	p, err := h.repo.GetPlaylist(r.Context(), id, false)
	if err != nil {
		h.fail(w, "add playlist video", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playlist": p})
}

func (h *Handlers) removePlaylistVideo(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	videoID, ok := urlParamID(w, r, "videoId")
	if !ok {
		return
	}
	err := h.repo.RemovePlaylistVideo(r.Context(), id, videoID)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not a member")
		return
	}
	if err != nil {
		h.fail(w, "remove playlist video", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
