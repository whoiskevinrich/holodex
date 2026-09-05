package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/filmimage"
	"holodex/internal/imagesink"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// Film images (F56/HOLODEX-280, ADR-086): owner-editable poster/thumb roles, mirroring
// studio_images.go (F51, ADR-079). Public reads serve the on-disk JPEG for a filled
// role, or 404 for an empty one (the SPA renders its own monogram/empty state — no
// placeholder route, unlike Person). Owner-gated mutations upload/replace/delete and
// are always scoped to model.FilmImageSourceUpload — a provider-sourced row is never
// written or removed through this file. Public reads (setFilmImageURLs, serveFilmImage)
// resolve the owner's uploaded image if one exists, else the provider's, via
// repo.GetFilmImageDisplayed/filmImageVersions (unlike Studio, film_images can hold a
// provider-sourced row alongside an uploaded one per role — film_images' UNIQUE is
// (film_id, role, source), migration 0043). A role from a request is always validated
// against the enum (model.ValidFilmImageRole) — a filesystem path is only ever built
// from the server-assigned integer id, never a request value. The provider-sourced
// writer lives in internal/imagesink.storeFilmAsset (HOLODEX-284/ADR-086), reached only
// through enrichment — never through this file.

// mountFilmImages registers the owner-gated image mutations. The public serve read is
// mounted ungated (but films-enabled-gated) in Mount; only the controls are gated.
func (h *Handlers) mountFilmImages(r chi.Router) {
	r.Post("/films/{id}/images/{role}", h.uploadFilmImage)
	r.Delete("/films/{id}/images/{role}", h.deleteFilmImage)
}

// setFilmImageURLs fills PosterURL/BannerURL from ImageVersions, pointing at the served
// route on our own origin. A role absent from ImageVersions stays empty (the SPA
// renders its fallback). Mirrors setStudioImageURLs.
func setFilmImageURLs(f *model.Film) {
	if f == nil {
		return
	}
	for role, v := range f.ImageVersions {
		url := fmt.Sprintf("/api/v1/films/%d/images/%s?v=%d", f.ID, role, v)
		switch role {
		case model.FilmImagePoster:
			f.PosterURL = url
		case model.FilmImageBanner:
			f.BannerURL = url
		}
	}
}

// filmImageRole validates the {role} path param against the enum, writing 400 and
// returning ok=false on an unknown value.
func filmImageRole(w http.ResponseWriter, r *http.Request) (string, bool) {
	role := chi.URLParam(r, "role")
	if !model.ValidFilmImageRole(role) {
		writeError(w, http.StatusBadRequest, "invalid image role")
		return "", false
	}
	return role, true
}

// serveFilmImage streams a film's on-disk displayed image JPEG for a role (the
// owner's upload if set, else a provider's — see GetFilmImageDisplayed) with a long
// immutable cache. The ?v={id} the model emits changes when the image is replaced, so
// a stale image is never pinned. No placeholder: an absent role is 404 and the SPA
// falls back to its own empty state.
func (h *Handlers) serveFilmImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	role, ok := filmImageRole(w, r)
	if !ok {
		return
	}
	if h.filmImageDir == "" {
		writeError(w, http.StatusNotFound, "image not available")
		return
	}
	img, err := h.repo.GetFilmImageDisplayed(r.Context(), id, role)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no image")
			return
		}
		h.fail(w, "get film image", err)
		return
	}
	serveEntityImageFile(w, r, filmimage.ImagePath(h.filmImageDir, id, img.ID))
}

// uploadFilmImage ingests a multipart upload (`image` file) for one role, normalizes
// the bytes (metadata strip + bomb guard), stores them on disk, and replaces the
// source='upload' row. 400 on a bad role/missing field/undecodable image; 201 with
// {id, version} on success.
func (h *Handlers) uploadFilmImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	role, ok := filmImageRole(w, r)
	if !ok {
		return
	}
	if h.filmImageDir == "" {
		writeError(w, http.StatusServiceUnavailable, "image storage unavailable")
		return
	}
	if _, err := h.repo.GetFilm(r.Context(), id); err != nil {
		h.filmLookupError(w, err)
		return
	}

	norm, iw, ih, ok := h.parseImageUpload(w, r, h.filmImageMaxBytes, h.filmImageMaxDim, "film", id, role)
	if !ok {
		return
	}
	// The replace/store/cleanup sequence is shared with a future enrichment asset path
	// (internal/imagesink.ReplaceFilmImageFile) — only Source differs.
	imgID, err := imagesink.ReplaceFilmImageFile(r.Context(), h.repo, h.filmImageDir,
		repo.FilmImageInsert{FilmID: id, Role: role, Source: model.FilmImageSourceUpload}, norm, iw, ih)
	if err != nil {
		h.fail(w, "replace film image", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": imgID, "version": imgID})
}

// deleteFilmImage removes a film's uploaded image for one role: the row, then the
// file (best-effort — a left-behind file is harmless, the index is the source of
// truth). Idempotent — deleting an already-empty slot is 204, not 404.
func (h *Handlers) deleteFilmImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	role, ok := filmImageRole(w, r)
	if !ok {
		return
	}
	existing, err := h.repo.GetFilmImage(r.Context(), id, role, model.FilmImageSourceUpload)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		h.fail(w, "get film image", err)
		return
	}
	if err := h.repo.DeleteFilmImage(r.Context(), id, role, model.FilmImageSourceUpload); err != nil {
		h.fail(w, "delete film image", err)
		return
	}
	if h.filmImageDir != "" && existing.ID != 0 {
		_ = filmimage.Remove(h.filmImageDir, id, existing.ID)
	}
	w.WriteHeader(http.StatusNoContent)
}
