package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"

	"holodex/internal/fieldsource"
	"holodex/internal/personimage"
	"holodex/internal/repo"
	"holodex/internal/resolver"
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
	r.Post("/media/{id}/poster/frame", h.pickVideoFrame)
	// The chooser's tile images (F85): each source slot, owner-only like the
	// chooser itself — a visitor sees only the presented poster.
	r.Get("/media/{id}/poster/sources/{kind}", h.servePosterSource)
}

// pickVideoFrame stores the owner's newest picked frame (P0-3): the frame picker
// captures it in the browser and posts it as the multipart `image`, with the picked
// time in seconds as the `at` query parameter (optional; shown on the tile, P1-1).
// See ingestOwnerPoster.
func (h *Handlers) pickVideoFrame(w http.ResponseWriter, r *http.Request) {
	var at *float64
	if raw := r.URL.Query().Get("at"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			writeError(w, http.StatusBadRequest, "at must be a non-negative number of seconds")
			return
		}
		at = &v
	}
	h.ingestOwnerPoster(w, r, fieldsource.LocalFrame, at)
}

// servePosterSource serves one poster source slot's image for the chooser's tile:
// kind is `file` or a local kind, mapped through the closed slot set so no request
// value ever names a file. 404 when the slot is empty or the kind is unknown.
func (h *Handlers) servePosterSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	slot, ok := thumbnail.SlotForKind(chi.URLParam(r, "kind"))
	if !ok || h.thumbDir == "" {
		writeError(w, http.StatusNotFound, "poster source not found")
		return
	}
	h.serveImageFile(w, r, id, "poster source", thumbnail.SlotPath(h.thumbDir, id, slot))
}

// posterSlotHeld reports whether a video's slot file is on disk.
func (h *Handlers) posterSlotHeld(id int64, slot string) bool {
	return h.thumbDir != "" && fileExists(thumbnail.SlotPath(h.thumbDir, id, slot))
}

// localPosters loads the video's held local slots for resolver.Options (F85). A
// row whose image is missing from storage (a partial restore, say) is left out, so
// its tile disappears and a decision naming it falls back exactly as Present does.
// A load failure degrades to no local tiles.
func (h *Handlers) localPosters(ctx context.Context, id int64) map[string]resolver.LocalPoster {
	rows, err := h.repo.PosterSources(ctx, id)
	if err != nil {
		h.log.Warn("poster sources for detail", "id", id, "err", err)
		return nil
	}
	out := make(map[string]resolver.LocalPoster, len(rows))
	for kind, ps := range rows {
		if slot, ok := thumbnail.SlotForKind(kind); ok && h.posterSlotHeld(id, slot) {
			out[kind] = resolver.LocalPoster{Hash: ps.Hash, FrameAt: ps.FrameAt}
		}
	}
	return out
}

// stampPosterTileImages sets image_url on the poster_url candidates whose value is
// not an image URL: the file's cover art (when its slot is held) and each local
// slot. The ?v= is the slot's content (hash, or the file slot's mtime), so a new
// image busts the tile's cache. Owner read only.
func (h *Handlers) stampPosterTileImages(fields []resolver.ResolvedField, id int64, locals map[string]resolver.LocalPoster) {
	for i := range fields {
		if resolver.NormKey(fields[i].Canonical) != fieldsource.LocalField {
			continue
		}
		for j := range fields[i].Candidates {
			c := &fields[i].Candidates[j]
			base := fmt.Sprintf("/api/v1/media/%d/poster/sources/", id)
			switch kind := fieldsource.Local(c.Source); {
			case kind != "":
				c.ImageURL = base + kind + "?v=" + locals[kind].Hash
			case c.Source == fieldsource.File && h.posterSlotHeld(id, thumbnail.SlotFile):
				c.ImageURL = fmt.Sprintf("%sfile?v=%d", base, imageVersion(thumbnail.SlotPath(h.thumbDir, id, thumbnail.SlotFile), 0))
			}
		}
	}
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
