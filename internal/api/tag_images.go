package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/tagimage"
)

// Tag images (F82, list-views spec P0-10): the owner's one poster image per tag, shown
// on the Tags page's Posters view. Same shape as a studio image role: a public read
// serves the on-disk image or 404 (the SPA draws its own placeholder), and owner-gated
// mutations upload/replace/delete. Uploads pass the shared image gate
// (parseImageUpload → personimage.Normalize) with the studio image bounds. A path is
// only ever built from server-assigned integer ids.

// mountTagImages registers the owner-gated mutations; the serve read is mounted ungated.
func (h *Handlers) mountTagImages(r chi.Router) {
	r.Post("/tags/{id}/image", h.uploadTagImage)
	r.Delete("/tags/{id}/image", h.deleteTagImage)
}

// setTagImageURL points ImageURL at the served route on our own origin, with the row
// id as the ?v= cache-buster. A tag without an image keeps an empty ImageURL.
func setTagImageURL(t *model.Tag) {
	if t != nil && t.ImageVersion != 0 {
		t.ImageURL = fmt.Sprintf("/api/v1/tags/%d/image?v=%d", t.ID, t.ImageVersion)
	}
}

// serveTagImage streams a tag's image with a long immutable cache; the ?v= changes on a
// replace, so a stale image is never pinned. 404 when the tag has none. Public read.
func (h *Handlers) serveTagImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.tagImageDir == "" {
		writeError(w, http.StatusNotFound, "image not available")
		return
	}
	img, err := h.repo.GetTagImage(r.Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no image")
			return
		}
		h.fail(w, "get tag image", err)
		return
	}
	path, err := tagimage.Find(h.tagImageDir, id, img.ID)
	serveEntityImageFile(w, r, path, err)
}

// uploadTagImage ingests a multipart upload (`image` file), normalizes it, stores it,
// and replaces the tag's row; the superseded file is removed best-effort, and a failed
// store puts the previous row back. 404 on an unknown tag, 400 on a bad upload, 201 with
// {id, version} on success.
func (h *Handlers) uploadTagImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.tagImageDir == "" {
		writeError(w, http.StatusServiceUnavailable, "image storage unavailable")
		return
	}
	if _, err := h.repo.GetTag(r.Context(), id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tag not found")
			return
		}
		h.fail(w, "get tag", err)
		return
	}
	norm, iw, ih, ok := h.parseImageUpload(w, r, h.tagImageMaxBytes, h.tagImageMaxDim, "tag", id, "poster")
	if !ok {
		return
	}
	existing, existErr := h.repo.GetTagImage(r.Context(), id)
	imgID, err := h.repo.ReplaceTagImage(r.Context(), id, iw, ih, len(norm))
	if err != nil {
		h.fail(w, "replace tag image", err)
		return
	}
	if err := tagimage.Store(h.tagImageDir, id, imgID, norm); err != nil {
		// Undo the row swap: the tag keeps its previous image (whose file is untouched),
		// or goes back to having none.
		if existErr == nil && existing.ID != 0 {
			if rerr := h.repo.RestoreTagImage(r.Context(), existing); rerr != nil {
				h.log.Warn("restore tag image after failed store", "tag", id, "err", rerr)
			}
		} else {
			_ = h.repo.DeleteTagImage(r.Context(), id)
		}
		h.fail(w, "store tag image", err)
		return
	}
	if existErr == nil && existing.ID != 0 {
		_ = tagimage.Remove(h.tagImageDir, id, existing.ID) // best-effort; a left-behind file is harmless
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": imgID, "version": imgID})
}

// deleteTagImage removes the tag's image: the row, then the file (best-effort — the row
// is the source of truth). Idempotent: an empty slot is 204, not 404.
func (h *Handlers) deleteTagImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	existing, err := h.repo.GetTagImage(r.Context(), id)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		h.fail(w, "get tag image", err)
		return
	}
	if err := h.repo.DeleteTagImage(r.Context(), id); err != nil {
		h.fail(w, "delete tag image", err)
		return
	}
	if h.tagImageDir != "" && existing.ID != 0 {
		_ = tagimage.Remove(h.tagImageDir, id, existing.ID)
	}
	w.WriteHeader(http.StatusNoContent)
}

// settleMergedTagImage brings the files in line with a tag merge that just committed.
// The merge hands the loser's image row to the survivor only when the survivor had none
// (the tag_images idMove); lostID is the loser's image id read before the merge. If the
// survivor now holds that row, its file moves across; otherwise the row went with the
// loser and its file is removed. Then the loser's directory goes. Best-effort, like
// every image-file cleanup: the rows are the source of truth.
func (h *Handlers) settleMergedTagImage(ctx context.Context, survivorID, mergedID, lostID int64) {
	if h.tagImageDir == "" {
		return
	}
	if lostID != 0 {
		if kept, err := h.repo.GetTagImage(ctx, survivorID); err == nil && kept.ID == lostID {
			if err := tagimage.Move(h.tagImageDir, mergedID, survivorID, lostID); err != nil {
				// Leave the loser's directory: it still holds the survivor's file.
				h.log.Warn("merge: move tag image", "tag", survivorID, "from", mergedID, "err", err)
				return
			}
		}
	}
	_ = tagimage.RemoveAll(h.tagImageDir, mergedID)
}
