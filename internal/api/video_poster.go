package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"holodex/internal/fieldsource"
	"holodex/internal/personimage"
	"holodex/internal/repo"
	"holodex/internal/thumbnail"
)

// Video poster upload (F52, HOLODEX-252) over the F85 poster source slots
// (media-ingest.md "Poster source slots"): an upload is stored in its own slot
// beside the file's cover art and the automatic frame, becomes the poster_url
// decision, and reaches the page through Manager.Present — never by writing the
// served pair directly, so no later re-extract can displace it. Bytes are
// normalized through the same decode/strip/resize gauntlet as a person image
// (personimage.Normalize is entity-agnostic); the size/dimension caps reuse the
// person-image config — same trust boundary (owner upload), no need for a second
// set of knobs.

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
// valid image. serveImageFile (handlers.go) shares serveThumbnail's exact
// posture: public read, id from the route only, no-cache so a rewrite is always
// revalidated.
func (h *Handlers) servePoster(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	h.serveImageFile(w, r, id, "poster",
		thumbnail.PosterPath(h.thumbDir, id), thumbnail.ThumbPath(h.thumbDir, id))
}

// uploadVideoPoster stores the owner's newest upload (P0-4): see ingestOwnerPoster.
func (h *Handlers) uploadVideoPoster(w http.ResponseWriter, r *http.Request) {
	h.ingestOwnerPoster(w, r, fieldsource.LocalUpload, nil)
}

// ingestOwnerPoster reads a multipart `image`, normalizes it, and hands it to the
// thumbnail pipeline's SetOwnerImage: the slot for kind is replaced, poster_url is
// decided local:<kind>, and the served pair is presented from it — all in this
// request, so the next poster GET is the new image (P0-3, P0-4). Nothing is
// written to the media file. 400 on a bad/oversized/undecodable image; 404
// unknown video; 503 when thumbnail storage is unconfigured.
func (h *Handlers) ingestOwnerPoster(w http.ResponseWriter, r *http.Request, kind string, frameAt *float64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.thumbDir == "" || h.thumbs == nil {
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
	// JPEG-only: the bytes land in a {id}-<slot>.jpg source slot (ADR-097), and are
	// what a later writeback embeds in the media file.
	norm, _, _, err := personimage.NormalizeJPEG(raw, h.personImageMaxDim)
	if err != nil {
		h.log.Warn("video poster normalize failed", "video", id, "err", err)
		writeError(w, http.StatusBadRequest, "unsupported or invalid image")
		return
	}
	if err := h.thumbs.SetOwnerImage(r.Context(), id, kind, norm, frameAt); err != nil {
		h.fail(w, "store poster", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{})
}

// deleteVideoPoster removes the owner's upload slot (P0-9). When the upload was
// the decided poster the decision goes with it and the page falls back to the
// file's cover art or the automatic frame; either way the media file is left
// alone. When no other slot exists yet, the same extract/enqueue path
// regenerateThumbnail uses fills one in, so the video isn't left posterless. 404
// unknown video; a missing upload is not an error (the desired end state — no
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
	if h.thumbDir == "" || h.thumbs == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if _, err := h.thumbs.RemoveOwnerImage(r.Context(), id, fieldsource.LocalUpload); err != nil {
		h.fail(w, "remove poster", err)
		return
	}
	if _, err := os.Stat(thumbnail.PosterPath(h.thumbDir, id)); err == nil || !h.thumbs.Enabled() {
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
