package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/personimage"
	"holodex/internal/repo"
	"holodex/internal/thumbnail"
)

// Video poster upload (F52, HOLODEX-252). The poster IS the ADR-009 thumbnail —
// this adds a highest-precedence "uploaded" tier to that existing single-slot
// pipeline rather than a new asset store, reusing thumbnail.ThumbPath/
// serveThumbnail/cache-busting end to end. Bytes are normalized through the same
// decode/strip/resize gauntlet as a person image (personimage.Normalize is
// entity-agnostic); the size/dimension caps reuse the person-image config —
// same trust boundary (owner upload), no need for a second set of knobs.

// mountVideoPoster registers the owner-gated poster upload/remove mutations.
// Mounted inside the requireOwner group in Mount. The public GET on this same
// path is servePoster, registered separately in Mount (outside the owner
// group, alongside serveThumbnail).
func (h *Handlers) mountVideoPoster(r chi.Router) {
	r.Post("/media/{id}/poster", h.uploadVideoPoster)
	r.Delete("/media/{id}/poster", h.deleteVideoPoster)
}

// servePoster serves the larger detail-page poster tier for a video (P0-6,
// F53/HOLODEX-253): {id}-poster.jpg when present, falling back to the
// existing {id}.jpg thumbnail bytes when the poster-tier derivative hasn't
// been generated yet (RD6 lazy backfill) — the route always resolves to a
// valid image, quality improves silently once a natural trigger (scan,
// re-enrich/writeback, poster upload, or Regenerate) produces the poster
// file. serveImageFile (handlers.go) shares serveThumbnail's exact posture:
// public read, id from the route only, no-cache so a rewrite is always
// revalidated.
func (h *Handlers) servePoster(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	h.serveImageFile(w, r, id, "poster",
		thumbnail.PosterPath(h.thumbDir, id), thumbnail.ThumbPath(h.thumbDir, id))
}

// uploadVideoPoster ingests a multipart upload (`image` file), normalizes it, and
// writes it to both the thumbnail and poster tiers (F53/HOLODEX-253 — the same
// bytes serve both; an owner upload is a one-off, not a bulk pass, so there's no
// need to scale two sizes) with state "uploaded" — never auto-replaced by the
// startup sweep (repo.go's NULL/failed-only query) or a rescan. Writing both
// tiers matters because servePoster prefers the poster-tier file when present:
// without this, an upload on a video whose poster tier was already extracted
// would be silently shadowed by the stale auto-generated poster. 400 on a
// bad/oversized/undecodable image; 404 unknown video; 503 when thumbnail
// storage is unconfigured.
func (h *Handlers) uploadVideoPoster(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.thumbDir == "" {
		writeError(w, http.StatusServiceUnavailable, "poster storage unavailable")
		return
	}
	if _, _, err := h.repo.GetVideo(r.Context(), id); err != nil {
		h.videoLookupError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.personImageMaxBytes)
	if err := r.ParseMultipartForm(h.personImageMaxBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid or oversized upload")
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image file is required")
		return
	}
	defer file.Close()
	raw, err := readAllLimited(file, h.personImageMaxBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read image")
		return
	}
	norm, _, _, err := personimage.Normalize(raw, h.personImageMaxDim)
	if err != nil {
		h.log.Warn("video poster normalize failed", "video", id, "err", err)
		writeError(w, http.StatusBadRequest, "unsupported or invalid image")
		return
	}
	if err := os.MkdirAll(h.thumbDir, 0o755); err != nil {
		h.fail(w, "prepare poster storage", err)
		return
	}
	if err := os.WriteFile(thumbnail.ThumbPath(h.thumbDir, id), norm, 0o644); err != nil {
		h.fail(w, "store poster", err)
		return
	}
	if err := os.WriteFile(thumbnail.PosterPath(h.thumbDir, id), norm, 0o644); err != nil {
		h.fail(w, "store poster", err)
		return
	}
	if err := h.repo.SetThumbnailState(r.Context(), id, model.ThumbnailUploaded); err != nil {
		h.fail(w, "set poster state", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{})
}

// deleteVideoPoster removes an uploaded poster (both tiers — F53/HOLODEX-253)
// and reverts to the file-derived one — the same extract/enqueue path
// regenerateThumbnail already uses, so the video isn't left posterless.
// ExtractEmbedded below regenerates both tiers together when it finds art;
// otherwise EnqueueHigh's background frame-grab does the same. 404 unknown
// video; a missing uploaded file is not an error (the desired end state — no
// upload — is already true).
func (h *Handlers) deleteVideoPoster(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, _, err := h.repo.GetVideo(r.Context(), id); err != nil {
		h.videoLookupError(w, err)
		return
	}
	if h.thumbDir != "" {
		for _, path := range []string{thumbnail.ThumbPath(h.thumbDir, id), thumbnail.PosterPath(h.thumbDir, id)} {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				h.fail(w, "remove poster", err)
				return
			}
		}
	}
	if err := h.repo.ResetThumbnailState(r.Context(), id); err != nil {
		h.fail(w, "reset poster state", err)
		return
	}
	if h.thumbs == nil || !h.thumbs.Enabled() {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	path, err := h.repo.PathByID(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	} else if err != nil {
		h.fail(w, "resolve media path", err)
		return
	}
	if extracted, err := h.thumbs.ExtractEmbedded(r.Context(), id, path); err != nil {
		h.log.Warn("re-extract after poster remove", "video", id, "err", err)
	} else if !extracted {
		h.thumbs.EnqueueHigh([]int64{id})
	}
	w.WriteHeader(http.StatusNoContent)
}
