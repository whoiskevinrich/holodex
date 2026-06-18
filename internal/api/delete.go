package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"holodex/internal/repo"
)

// purger hard-deletes a single item now, bypassing the grace period (F24.5). Wired
// from main; nil leaves purge-now disabled (the soft-delete + Trash paths still work).
type purger interface {
	PurgeNow(ctx context.Context, id int64) error
}

// SetDelete wires the soft-delete/purge surface (F24, ADR-037): the purge-now
// executor and the grace period used to compute each Trash item's purge_at. Called
// once at startup before serving. A zero grace means auto-purge is disabled, so the
// Trash view reports no purge_at.
func (h *Handlers) SetDelete(p purger, grace time.Duration) {
	h.purger = p
	h.deleteGrace = grace
}

// mountDelete registers the owner-gated delete/restore/trash routes (F24, ADR-037).
// Mounted inside the requireOwner group; the reads they hide from are public but
// blind to soft-deleted rows by construction (the repo visibility seam).
func (h *Handlers) mountDelete(r chi.Router) {
	r.Delete("/media/{id}", h.deleteMedia)
	r.Post("/media/{id}/restore", h.restoreMedia)
	r.Get("/admin/trash", h.listTrash)
}

// deleteMedia soft-deletes a media item (F24.1), or hard-deletes it now when
// ?purge=true (F24.5). Soft-delete is idempotent (204); an unknown id is 404.
func (h *Handlers) deleteMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}

	if r.URL.Query().Get("purge") == "true" {
		if h.purger == nil {
			writeError(w, http.StatusServiceUnavailable, "purge is not available")
			return
		}
		switch err := h.purger.PurgeNow(r.Context(), id); {
		case errors.Is(err, repo.ErrNotFound):
			writeError(w, http.StatusNotFound, "media not found")
		case err != nil:
			// The row is left soft-deleted (in Trash) for retry; surface a generic
			// reason — never the filesystem path (no-secrets invariant).
			h.log.Warn("purge-now failed", "id", id, "err", err)
			writeError(w, http.StatusInternalServerError,
				"could not remove the file from disk; the item is still in Trash and will be retried")
		default:
			w.WriteHeader(http.StatusNoContent)
		}
		return
	}

	switch err := h.repo.SoftDelete(r.Context(), id); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "media not found")
	case err != nil:
		h.fail(w, "soft delete", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// restoreMedia clears a soft-delete so the item returns to every view (F24.6).
// 404 when the id isn't currently soft-deleted (nothing to restore).
func (h *Handlers) restoreMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	switch err := h.repo.Restore(r.Context(), id); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "media not in Trash")
		return
	case err != nil:
		h.fail(w, "restore", err)
		return
	}
	v, extra, err := h.repo.GetVideo(r.Context(), id)
	if err != nil {
		h.fail(w, "load restored", err)
		return
	}
	setThumbnailURL(v)
	writeJSON(w, http.StatusOK, map[string]any{"video": v, "metadata": extra})
}

// trashItem is one soft-deleted item with its computed purge_at (F24.7). purge_at
// is nil when auto-purge is disabled (grace == 0) — the item stays until purged.
type trashItem struct {
	ID        int64      `json:"id"`
	Title     string     `json:"title"`
	Path      string     `json:"path"` // shown in the owner-only permanent-delete confirm
	DeletedAt time.Time  `json:"deleted_at"`
	PurgeAt   *time.Time `json:"purge_at"`
}

// listTrash returns the soft-deleted items for the owner's Trash view (F24.7),
// each with deleted_at and the computed purge_at.
func (h *Handlers) listTrash(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.Trash(r.Context())
	if err != nil {
		h.fail(w, "list trash", err)
		return
	}
	out := make([]trashItem, 0, len(items))
	for _, it := range items {
		t := trashItem{ID: it.ID, Title: it.Title, Path: it.FilePath, DeletedAt: it.DeletedAt}
		if h.deleteGrace > 0 {
			pa := it.DeletedAt.Add(h.deleteGrace)
			t.PurgeAt = &pa
		}
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": len(out)})
}
