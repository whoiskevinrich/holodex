package api

import (
	"errors"
	"net/http"
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
	r.Post("/playlists/{id}/freeze", h.freezePlaylist)
}

func (h *Handlers) listPlaylists(w http.ResponseWriter, r *http.Request) {
	isOwner := h.auth.authorized(r)
	items, err := h.repo.ListPlaylists(r.Context(), !isOwner)
	if err != nil {
		h.fail(w, "list playlists", err)
		return
	}
	// A smart playlist has no membership to count; its item_count is the live match
	// count under this reader's posture, 0 while it has stale refs (ADR-121 D3).
	for i := range items {
		if !items[i].Smart() {
			continue
		}
		f, stale, err := h.smartPlaylistFilter(r.Context(), &items[i], isOwner)
		if err != nil {
			h.fail(w, "list playlists", err)
			return
		}
		if len(stale) > 0 {
			continue
		}
		if items[i].ItemCount, err = h.repo.CountVideos(r.Context(), f); err != nil {
			h.fail(w, "list playlists", err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// getPlaylist returns the playlist plus its un-trashed members in the playlist's
// order (spec P0-4), and `ids`, the whole ordered id list a run plays. ?seed=
// parameterizes a 'random' sort so one play-through walks one shuffle (ADR-045);
// absent, a seed is minted per request.
//
// A smart playlist (ADR-121 D3) is evaluated live: visibility first (a visitor on a
// private one gets the unknown-id 404 before anything else is looked at), then the
// stale check, then its query under this reader's posture. Its tiles page with
// ?limit=/&offset= as /media does; `ids` is never paged. With stale refs nothing is
// evaluated: the response carries them and no items.
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
	out := map[string]any{"playlist": p, "stale_refs": []staleRef{}}
	if p.Sort == "random" {
		out["seed"] = seed
	}
	var items []model.Video
	var ids []int64
	if p.Smart() {
		f, stale, err := h.smartPlaylistFilter(r.Context(), p, isOwner)
		if err != nil {
			h.fail(w, "get playlist", err)
			return
		}
		if len(stale) > 0 {
			p.ItemCount = 0
			out["stale_refs"], out["items"], out["ids"], out["total"] = stale, []model.Video{}, []int64{}, 0
			writeJSON(w, http.StatusOK, out)
			return
		}
		f.Seed = seed
		f.Limit = atoiDefault(r.URL.Query().Get("limit"), 50)
		f.Offset = atoiDefault(r.URL.Query().Get("offset"), 0)
		if isOwner {
			h.drainCompleteness(r.Context())
		}
		if items, p.ItemCount, err = h.repo.ListVideos(r.Context(), f); err != nil {
			h.fail(w, "playlist videos", err)
			return
		}
		if ids, err = h.repo.ListVideoIDs(r.Context(), f); err != nil {
			h.fail(w, "playlist videos", err)
			return
		}
		out["limit"], out["offset"] = f.Limit, f.Offset
	} else {
		if items, err = h.repo.PlaylistVideos(r.Context(), id, p.Sort, seed); err != nil {
			h.fail(w, "playlist videos", err)
			return
		}
		ids = make([]int64, len(items))
		for i, v := range items {
			ids[i] = v.ID
		}
		p.ItemCount = len(items)
	}
	if err := h.hydrateTiles(r.Context(), items, isOwner); err != nil {
		h.fail(w, "playlist videos", err)
		return
	}
	out["items"], out["ids"], out["total"] = items, ids, p.ItemCount
	writeJSON(w, http.StatusOK, out)
}

// writePlaylist answers a mutation with the playlist as the owner now sees it.
func (h *Handlers) writePlaylist(w http.ResponseWriter, r *http.Request, id int64) {
	p, err := h.repo.GetPlaylist(r.Context(), id, false)
	if err != nil {
		h.fail(w, "get playlist", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playlist": p})
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
	// Query is a browse filter query string stored as a smart playlist's live query
	// (ADR-121 D1/D2), canonicalised on the way in. On create it makes the playlist
	// smart; on PATCH it is Edit filter's Update, and only a smart playlist takes it.
	Query        *string `json:"query"`
	PlayShuffled *bool   `json:"play_shuffled"`
}

// canonicalBodyQuery canonicalises b.Query in place (ADR-121 D2), writing 400 and
// returning false when it can't be stored. The query's own sort, if any, is returned
// so a create can take the grid's sort from it, as from_query does.
func (h *Handlers) canonicalBodyQuery(w http.ResponseWriter, b *playlistBody) (string, bool) {
	q, err := parseQueryString(*b.Query)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid query")
		return "", false
	}
	canon, err := h.canonicalPlaylistQuery(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid query: "+err.Error())
		return "", false
	}
	b.Query = &canon
	return q.Get("sort"), true
}

// checkSmartPlaylistState refuses, with 400, a playlist state a smart playlist or a
// public one can't hold: `manual` sort on a smart playlist (no position, RD4), and
// owner-only inputs — a completeness sort or a missing_facet query — on a public one
// (ADR-121 D4). Called with the state a create or PATCH would leave behind.
func checkSmartPlaylistState(w http.ResponseWriter, sort, visibility string, query *string) bool {
	if query != nil && sort == model.PlaylistSortManual {
		writeError(w, http.StatusBadRequest, "a smart playlist can't use the manual sort")
		return false
	}
	if visibility == model.PlaylistPublic && usesOwnerOnlyInputs(sort, query) {
		writeError(w, http.StatusBadRequest, "a public playlist can't use owner-only filters or sorts")
		return false
	}
	return true
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
//
// With query instead (ADR-121 D1), the playlist is smart: the canonical query is
// stored, nothing is snapshotted, and the sort is the query's (random included — a
// smart playlist re-shuffles per play-through, RD4) unless the body names one.
func (h *Handlers) createPlaylist(w http.ResponseWriter, r *http.Request) {
	var b playlistBody
	if !decodeJSON(w, r, &b) || !validatePlaylistFields(w, &b) {
		return
	}
	if b.Name == nil {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if b.Query != nil && b.FromQuery != nil {
		writeError(w, http.StatusBadRequest, "send query (smart) or from_query (snapshot), not both")
		return
	}
	sort, visibility := "added_desc", model.PlaylistPrivate
	if b.Visibility != nil {
		visibility = *b.Visibility
	}
	if b.Query != nil {
		querySort, ok := h.canonicalBodyQuery(w, &b)
		if !ok {
			return
		}
		if repo.ValidSort(querySort) {
			sort = querySort
		}
	}
	var ids []int64
	if b.FromQuery != nil {
		q, err := parseQueryString(*b.FromQuery)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from_query")
			return
		}
		f := h.browseFilter(q)
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
	if !checkSmartPlaylistState(w, sort, visibility, b.Query) {
		return
	}
	p, err := h.repo.CreatePlaylist(r.Context(), *b.Name, sort, visibility, b.Query, ids)
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
	if b.Name == nil && b.Sort == nil && b.Visibility == nil && b.Query == nil && b.PlayShuffled == nil {
		writeError(w, http.StatusBadRequest, "nothing to update")
		return
	}
	if b.FromQuery != nil {
		writeError(w, http.StatusBadRequest, "from_query is create-only")
		return
	}
	cur, err := h.repo.GetPlaylist(r.Context(), id, false)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		h.fail(w, "update playlist", err)
		return
	}
	// Check the state the patch would leave behind, so a refusal (D4) lands on
	// whichever of query, sort or visibility arrives second.
	sort, visibility, query := cur.Sort, cur.Visibility, cur.Query
	if b.Query != nil {
		if !cur.Smart() {
			writeError(w, http.StatusBadRequest, "not a smart playlist")
			return
		}
		if _, ok := h.canonicalBodyQuery(w, &b); !ok {
			return
		}
		query = b.Query
	}
	if b.Sort != nil {
		sort = *b.Sort
	}
	if b.Visibility != nil {
		visibility = *b.Visibility
	}
	// A rename or always-shuffle toggle changes none of these, so it doesn't re-judge a
	// state that predates the rule (an F69 public playlist on a completeness sort).
	touchesState := b.Query != nil || b.Sort != nil || b.Visibility != nil
	if touchesState && !checkSmartPlaylistState(w, sort, visibility, query) {
		return
	}
	err = h.repo.UpdatePlaylist(r.Context(), id, repo.PlaylistPatch{
		Name: b.Name, Sort: b.Sort, Visibility: b.Visibility, Query: b.Query, PlayShuffled: b.PlayShuffled,
	})
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		h.fail(w, "update playlist", err)
		return
	}
	h.writePlaylist(w, r, id)
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
	if errors.Is(err, repo.ErrSmartPlaylist) {
		writeError(w, http.StatusBadRequest, "a smart playlist's members come from its query")
		return
	}
	if err != nil {
		h.fail(w, "add playlist video", err)
		return
	}
	h.writePlaylist(w, r, id)
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
	if errors.Is(err, repo.ErrSmartPlaylist) {
		writeError(w, http.StatusBadRequest, "a smart playlist's members come from its query")
		return
	}
	if err != nil {
		h.fail(w, "remove playlist video", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
