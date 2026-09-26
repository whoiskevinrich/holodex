package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/imagesink"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/studioimage"
)

// Studio images (F51, ADR-079): three owner-editable core roles (icon/logo/poster),
// generalizing the single self-hosted studio logo (HOLODEX-130, ADR-057). Public reads
// serve the on-disk JPEG for a filled role, or 404 for an empty one (the SPA renders
// its own monogram/empty state — no placeholder route, unlike Person). Owner-gated
// mutations upload/replace/delete. A role from a request is always validated against
// the enum (model.ValidStudioImageRole) — a filesystem path is only ever built from
// the server-assigned integer id, never a request value.

// mountStudioImages registers the owner-gated image mutations. The public serve read
// is mounted ungated in Mount; only the controls are gated.
func (h *Handlers) mountStudioImages(r chi.Router) {
	r.Post("/studios/{id}/images/{role}", h.uploadStudioImage)
	r.Delete("/studios/{id}/images/{role}", h.deleteStudioImage)
	r.Put("/studios/{id}/images/{role}/halo", h.setStudioImageHalo)
}

// setStudioImageHalo turns the halo on or off for one image role in one palette mode
// (HOLODEX-463, ADR-109). Body {mode: "dark"|"light", on: bool}. It is a display choice
// only: it touches no image bytes and no provenance, and it does not need the role's
// slot to be filled, so a choice made before a replace still stands after it. 400 on a
// bad role or mode, 404 on an unknown studio, 204 on success (idempotent).
func (h *Handlers) setStudioImageHalo(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	role, ok := studioImageRole(w, r)
	if !ok {
		return
	}
	var body struct {
		Mode string `json:"mode"`
		On   bool   `json:"on"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !model.ValidHaloMode(body.Mode) {
		writeError(w, http.StatusBadRequest, "invalid halo mode")
		return
	}
	if _, err := h.repo.GetStudio(r.Context(), id); err != nil {
		h.studioLookupError(w, err)
		return
	}
	if err := h.repo.SetStudioImageHalo(r.Context(), id, role, body.Mode, body.On); err != nil {
		h.fail(w, "set studio image halo", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setStudioImageURLs fills IconURL/LogoURL/PosterURL from ImageVersions, pointing at
// the served route on our own origin. A role absent from ImageVersions stays empty
// (the SPA renders its fallback). Mirrors setThumbnailURL / the pre-F51 setStudioLogoURL.
func setStudioImageURLs(s *model.Studio) {
	if s == nil {
		return
	}
	for role, v := range s.ImageVersions {
		url := fmt.Sprintf("/api/v1/studios/%d/images/%s?v=%d", s.ID, role, v)
		switch role {
		case model.StudioImageIcon:
			s.IconURL = url
		case model.StudioImageLogo:
			s.LogoURL = url
		case model.StudioImagePoster:
			s.PosterURL = url
		}
	}
}

// studioImageRole validates the {role} path param against the enum, writing 400 and
// returning ok=false on an unknown value.
func studioImageRole(w http.ResponseWriter, r *http.Request) (string, bool) {
	role := chi.URLParam(r, "role")
	if !model.ValidStudioImageRole(role) {
		writeError(w, http.StatusBadRequest, "invalid image role")
		return "", false
	}
	return role, true
}

// serveStudioImage streams a studio's on-disk image for a role with a long
// immutable cache. The ?v={id} the model emits changes when the image is replaced, so
// a stale image is never pinned. No placeholder: an absent role is 404 and the SPA
// falls back to its own empty state. Public read, like every other studio read.
func (h *Handlers) serveStudioImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	role, ok := studioImageRole(w, r)
	if !ok {
		return
	}
	if h.studioImageDir == "" {
		writeError(w, http.StatusNotFound, "image not available")
		return
	}
	img, err := h.repo.GetStudioImage(r.Context(), id, role)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no image")
			return
		}
		h.fail(w, "get studio image", err)
		return
	}
	path, err := studioimage.Find(h.studioImageDir, id, img.ID)
	serveEntityImageFile(w, r, path, err)
}

// uploadStudioImage ingests a multipart upload (`image` file) for one role, normalizes
// the bytes (metadata strip + bomb guard), stores them on disk, and replaces the row
// with source='upload' — which then provenance-locks the slot against enrichment
// (F51, the ADR-049 pattern). 400 on a bad role/missing field/undecodable image; 201
// with {id, version} on success.
func (h *Handlers) uploadStudioImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	role, ok := studioImageRole(w, r)
	if !ok {
		return
	}
	if h.studioImageDir == "" {
		writeError(w, http.StatusServiceUnavailable, "image storage unavailable")
		return
	}
	if _, err := h.repo.GetStudio(r.Context(), id); err != nil {
		h.studioLookupError(w, err)
		return
	}

	norm, iw, ih, ok := h.parseImageUpload(w, r, h.studioImageMaxBytes, h.studioImageMaxDim, "studio", id, role)
	if !ok {
		return
	}
	// The replace/store/cleanup sequence is shared with the enrichment asset path
	// (internal/imagesink.ReplaceStudioImageFile) — only Source differs.
	imgID, err := imagesink.ReplaceStudioImageFile(r.Context(), h.repo, h.studioImageDir,
		repo.StudioImageInsert{StudioID: id, Role: role, Source: model.StudioImageSourceUpload}, norm, iw, ih)
	if err != nil {
		h.fail(w, "replace studio image", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": imgID, "version": imgID})
}

// deleteStudioImage removes a studio's image for one role: the row, then the file
// (best-effort — a left-behind file is harmless, the index is the source of truth).
// Unlocks the slot for the next enrich to refill (the ADR-049 "delete unlocks" rule).
// Idempotent — deleting an already-empty slot is 204, not 404 (mirrors DeleteStudioLogo).
func (h *Handlers) deleteStudioImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	role, ok := studioImageRole(w, r)
	if !ok {
		return
	}
	existing, err := h.repo.GetStudioImage(r.Context(), id, role)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		h.fail(w, "get studio image", err)
		return
	}
	if err := h.repo.DeleteStudioImage(r.Context(), id, role); err != nil {
		h.fail(w, "delete studio image", err)
		return
	}
	if h.studioImageDir != "" && existing.ID != 0 {
		_ = studioimage.Remove(h.studioImageDir, id, existing.ID)
	}
	w.WriteHeader(http.StatusNoContent)
}
