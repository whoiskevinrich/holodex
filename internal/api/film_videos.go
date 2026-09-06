package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// filmVideoCandidate is one row in the film→video attach picker's result list
// (design handoff §4): the video plus, when it's already linked to a film
// other than the one being edited, which film(s) so the picker can show an
// "Also in: X" badge (spec: "must also indicate if a candidate video is
// already attached to another film").
type filmVideoCandidate struct {
	Video           model.Video           `json:"video"`
	AlreadyAttached []repo.FilmAttachment `json:"already_attached"`
}

// Film↔video attach/detach (F56, ADR-085 §2/§6): film_videos is an owner
// ASSERTION, never a value RelinkVideoEntity derives -- see
// internal/repo/films.go's AttachFilmVideo doc comment and the zero-relink-
// participation regression test (film_links_test.go). These are the ONLY writers
// of film_videos; no relink trigger may ever call them.

// mountFilmVideos registers the owner-gated attach/detach/bulk-attach routes,
// called from mountFilms.
func (h *Handlers) mountFilmVideos(r chi.Router) {
	r.Get("/films/{filmId}/video-candidates", h.filmVideoCandidates)
	r.Post("/films/{filmId}/videos", h.attachFilmVideo)
	r.Post("/films/{filmId}/videos/bulk", h.bulkAttachFilmVideos)
	r.Patch("/films/{filmId}/videos/{videoId}", h.updateFilmVideoScene)
	r.Delete("/films/{filmId}/videos/{videoId}", h.detachFilmVideo)
}

// filmVideoCandidates handles GET /films/{filmId}/video-candidates (owner-gated):
// the film→video picker's search (design handoff §4). Reuses repo.VideoFilter's
// q/studio_id/person filtering (same struct GET /media uses) plus two new
// dimensions: always excludes videos already attached to this film, and --
// unless ?unattached=false -- excludes videos attached to ANY film (the
// picker's default-unattached scope). Rows already attached to a different
// film carry that film in already_attached so the picker can flag it.
func (h *Handlers) filmVideoCandidates(w http.ResponseWriter, r *http.Request) {
	filmID, ok := urlParamID(w, r, "filmId")
	if !ok {
		return
	}
	if _, err := h.repo.GetFilm(r.Context(), filmID); err != nil {
		h.filmLookupError(w, err)
		return
	}

	q := r.URL.Query()
	f := h.videoFilterFromQuery(q)
	f.UnattachedToAnyFilm = q.Get("unattached") != "false"
	// Only meaningful (and only applied) when the default unattached-only scope is
	// widened -- when UnattachedToAnyFilm is set, its blanket "no film_videos row at
	// all" check already excludes this film's own attachments, making a second,
	// this-film-specific NOT EXISTS redundant on the common-case query.
	if !f.UnattachedToAnyFilm {
		f.ExcludeAttachedToFilmID = filmID
	}

	videos, total, err := h.repo.ListVideos(r.Context(), f)
	if err != nil {
		h.fail(w, "film video candidates", err)
		return
	}
	ids := make([]int64, len(videos))
	for i, v := range videos {
		ids[i] = v.ID
	}
	attachedByVideo, err := h.repo.FilmsForVideos(r.Context(), ids)
	if err != nil {
		h.fail(w, "film video candidates attachments", err)
		return
	}
	items := make([]filmVideoCandidate, len(videos))
	for i, v := range videos {
		attached := attachedByVideo[v.ID]
		if attached == nil {
			attached = []repo.FilmAttachment{}
		}
		items[i] = filmVideoCandidate{Video: v, AlreadyAttached: attached}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "limit": f.Limit, "offset": f.Offset,
	})
}

// writeSceneCollisionConflict writes the 409 envelope for a scene-number collision
// (spec: "reject with an inline error naming the current occupant, no silent swap,
// no auto-bump renumbering"), mirroring writeCollisionConflict's shape.
func writeSceneCollisionConflict(w http.ResponseWriter, occupant *repo.FilmSceneCollision) {
	writeJSON(w, http.StatusConflict, map[string]any{
		"error":    "scene number already taken",
		"conflict": occupant,
	})
}

// filmVideoMutationError translates AttachFilmVideo/BulkAttachFilmVideos/
// DetachFilmVideo's shared error set into a response, reporting whether it wrote
// one. occupant is the *repo.FilmSceneCollision an attach call returned alongside
// ErrSceneNumberTaken (nil for detach's error set, which has no collision).
func (h *Handlers) filmVideoMutationError(w http.ResponseWriter, op string, err error, occupant *repo.FilmSceneCollision) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "film or video not found")
	case errors.Is(err, repo.ErrFilmVideoAlreadyAttached):
		writeError(w, http.StatusConflict, "video already attached to this film")
	case errors.Is(err, repo.ErrSceneNumberTaken):
		writeSceneCollisionConflict(w, occupant)
	default:
		h.fail(w, op, err)
	}
	return true
}

// requireLiveFilmAndVideo validates that both the film and the video exist (and the
// video is live -- not soft-deleted), writing 404/409 and returning false
// otherwise. Attach must never silently create a link to a trashed video.
func (h *Handlers) requireLiveFilmAndVideo(w http.ResponseWriter, r *http.Request, filmID, videoID int64) bool {
	if _, err := h.repo.GetFilm(r.Context(), filmID); err != nil {
		h.filmLookupError(w, err)
		return false
	}
	return h.requireLiveVideo(w, r, videoID)
}

// requireLiveVideo validates that a video exists and is live (not soft-deleted),
// writing 404/409 and returning false otherwise.
func (h *Handlers) requireLiveVideo(w http.ResponseWriter, r *http.Request, videoID int64) bool {
	switch _, err := h.repo.RefreshTarget(r.Context(), videoID); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "video not found")
		return false
	case errors.Is(err, repo.ErrDeleted):
		writeError(w, http.StatusConflict, "video is deleted")
		return false
	case err != nil:
		h.fail(w, "get video", err)
		return false
	}
	return true
}

// requireLiveVideos applies requireLiveVideo to every id, stopping at the first
// offender -- the bulk-attach counterpart to attach's single-video check, so a
// trashed or nonexistent video can't be silently linked via the bulk path.
func (h *Handlers) requireLiveVideos(w http.ResponseWriter, r *http.Request, ids []int64) bool {
	for _, id := range ids {
		if !h.requireLiveVideo(w, r, id) {
			return false
		}
	}
	return true
}

// attachFilmVideo handles POST /films/{filmId}/videos (owner-gated): {video_id,
// scene_number, is_full_film}. scene_number is optional (nil = unnumbered, legal
// and non-colliding). 409 on an already-attached pair or a taken scene number
// (naming the occupant); 404/409 if the film/video isn't live.
func (h *Handlers) attachFilmVideo(w http.ResponseWriter, r *http.Request) {
	filmID, ok := urlParamID(w, r, "filmId")
	if !ok {
		return
	}
	var body struct {
		VideoID     int64  `json:"video_id"`
		SceneNumber *int64 `json:"scene_number"`
		IsFullFilm  bool   `json:"is_full_film"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.VideoID <= 0 {
		writeError(w, http.StatusBadRequest, "video_id is required")
		return
	}
	if !h.requireLiveFilmAndVideo(w, r, filmID, body.VideoID) {
		return
	}
	occupant, err := h.repo.AttachFilmVideo(r.Context(), filmID, body.VideoID, body.SceneNumber, body.IsFullFilm)
	if h.filmVideoMutationError(w, "attach film video", err, occupant) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// bulkAttachFilmVideos handles POST /films/{filmId}/videos/bulk (owner-gated):
// {video_ids, starting_scene_number} -- the film→video picker's multi-select
// attach, sequentially auto-numbered from starting_scene_number. A nil/omitted
// starting_scene_number attaches every video unnumbered instead (design handoff
// §4c). All-or-nothing: see BulkAttachFilmVideos. Always scene files, never
// is_full_film (a full-film attach is always the single-video attachFilmVideo path).
func (h *Handlers) bulkAttachFilmVideos(w http.ResponseWriter, r *http.Request) {
	filmID, ok := urlParamID(w, r, "filmId")
	if !ok {
		return
	}
	var body struct {
		VideoIDs            []int64 `json:"video_ids"`
		StartingSceneNumber *int64  `json:"starting_scene_number"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.VideoIDs) == 0 {
		writeError(w, http.StatusBadRequest, "video_ids is required")
		return
	}
	if body.StartingSceneNumber != nil && *body.StartingSceneNumber <= 0 {
		writeError(w, http.StatusBadRequest, "starting_scene_number must be positive")
		return
	}
	if _, err := h.repo.GetFilm(r.Context(), filmID); err != nil {
		h.filmLookupError(w, err)
		return
	}
	if !h.requireLiveVideos(w, r, body.VideoIDs) {
		return
	}
	occupant, err := h.repo.BulkAttachFilmVideos(r.Context(), filmID, body.VideoIDs, body.StartingSceneNumber)
	if h.filmVideoMutationError(w, "bulk attach film videos", err, occupant) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// updateFilmVideoScene handles PATCH /films/{filmId}/videos/{videoId} (owner-gated):
// {scene_number} (optional, nil = unnumbered) -- corrects an already-attached
// video's scene number in place, without a detach+reattach round trip. Same 409
// collision semantics as attachFilmVideo (naming the occupant, no auto-bump);
// re-setting the video's own current number is a no-op. 404 if the pair isn't
// attached.
func (h *Handlers) updateFilmVideoScene(w http.ResponseWriter, r *http.Request) {
	filmID, ok := urlParamID(w, r, "filmId")
	if !ok {
		return
	}
	videoID, ok := urlParamID(w, r, "videoId")
	if !ok {
		return
	}
	var body struct {
		SceneNumber *int64 `json:"scene_number"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	occupant, err := h.repo.UpdateFilmVideoScene(r.Context(), filmID, videoID, body.SceneNumber)
	if h.filmVideoMutationError(w, "update film video scene", err, occupant) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// detachFilmVideo handles DELETE /films/{filmId}/videos/{videoId} (owner-gated).
// 404 if the pair wasn't attached (not idempotent -- mirrors DetachFilmVideo).
func (h *Handlers) detachFilmVideo(w http.ResponseWriter, r *http.Request) {
	filmID, ok := urlParamID(w, r, "filmId")
	if !ok {
		return
	}
	videoID, ok := urlParamID(w, r, "videoId")
	if !ok {
		return
	}
	if err := h.repo.DetachFilmVideo(r.Context(), filmID, videoID); h.filmVideoMutationError(w, "detach film video", err, nil) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
