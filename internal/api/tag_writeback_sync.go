package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"holodex/internal/fieldsource"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writequeue"
)

// Tag writeback exclusion (HOLODEX-239, ADR-077) — the owner's per-tag Genre
// writeback flag plus the manual sync trigger that pushes an already-set
// decision out to already-written files. Owner-gated (mounted in the
// requireOwner group, like mountTagDenylist/mountTagHierarchy).

// mountTagWritebackSync registers the flag-toggle and manual-sync endpoints,
// single-tag and bulk.
func (h *Handlers) mountTagWritebackSync(r chi.Router) {
	r.Patch("/tags/{id}/writeback", h.setTagWriteback)
	r.Post("/tags/{id}/writeback/sync", h.syncTagWritebackOne)
	r.Patch("/tags/writeback", h.setTagsWriteback)
	r.Post("/tags/writeback/sync", h.syncTagWritebackBulk)
}

// setTagWriteback sets one tag's Genre-writeback participation flag. Never
// enqueues a write (spec P0) — only updates the stored value.
func (h *Handlers) setTagWriteback(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	tag, err := h.repo.SetTagWritebackEnabled(r.Context(), id, body.Enabled)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tag not found")
			return
		}
		h.fail(w, "set tag writeback", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": tag})
}

// setTagsWriteback bulk-sets the writeback flag for every listed tag,
// regardless of each tag's individual prior state. Never enqueues a write.
func (h *Handlers) setTagsWriteback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TagIDs  []int64 `json:"tag_ids"`
		Enabled bool    `json:"enabled"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.TagIDs) == 0 {
		writeError(w, http.StatusBadRequest, "tag_ids is required")
		return
	}
	if err := h.repo.SetTagsWritebackEnabled(r.Context(), body.TagIDs, body.Enabled); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tag not found")
			return
		}
		h.fail(w, "set tags writeback", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// syncTagWritebackOne triggers a manual sync for every video currently
// carrying one tag. A tag attached to zero videos enqueues nothing (still
// 202 — the spec's "trigger is disabled/no-op" is a frontend affordance over
// this same zero-count response, not a distinct API error).
func (h *Handlers) syncTagWritebackOne(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.writeQueue == nil {
		writeError(w, http.StatusServiceUnavailable, "writeback unavailable")
		return
	}
	if err := h.repo.EntityExists(r.Context(), model.EntityTag, id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tag not found")
			return
		}
		h.fail(w, "check tag exists", err)
		return
	}
	videoIDs, err := h.repo.VideoIDsForTag(r.Context(), id)
	if err != nil {
		h.fail(w, "list videos for tag", err)
		return
	}
	h.runTagWritebackSync(w, r, videoIDs)
}

// syncTagWritebackBulk triggers a manual sync across the union of videos
// attached to every listed tag, deduplicated so a video attached to more
// than one selected tag is enqueued once.
func (h *Handlers) syncTagWritebackBulk(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TagIDs []int64 `json:"tag_ids"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.TagIDs) == 0 {
		writeError(w, http.StatusBadRequest, "tag_ids is required")
		return
	}
	if h.writeQueue == nil {
		writeError(w, http.StatusServiceUnavailable, "writeback unavailable")
		return
	}
	if err := h.repo.TagsExist(r.Context(), body.TagIDs); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tag not found")
			return
		}
		h.fail(w, "check tags exist", err)
		return
	}
	videoIDs, err := h.repo.VideoIDsForTags(r.Context(), body.TagIDs)
	if err != nil {
		h.fail(w, "list videos for tags", err)
		return
	}
	h.runTagWritebackSync(w, r, videoIDs)
}

// runTagWritebackSync mints a fresh batchID and enqueues the sync, shared by
// the single- and bulk-tag trigger handlers.
func (h *Handlers) runTagWritebackSync(w http.ResponseWriter, r *http.Request, videoIDs []int64) {
	batchID := fmt.Sprintf("tag-writeback-sync-%d", time.Now().UnixNano())
	enqueued, err := h.syncTagWriteback(r.Context(), videoIDs, batchID)
	if err != nil {
		h.fail(w, "sync tag writeback", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"batch_id": batchID, "enqueued": enqueued})
}

// syncTagWriteback is propagateMerge's structural sibling for the manual sync
// trigger (ADR-077 D2), with one necessary difference: it recomputes
// GenreWritebackValues per video rather than batch-enqueuing a precomputed
// name list. The value being written is the video's full current "genres"
// union (attached tags, ancestor-expanded and writeback-flag-filtered, plus
// the deny-list-filtered raw resolved value) — exactly what a flag toggle
// changed the computation of — so enqueuing anything less would silently
// narrow the file's Genre tag instead of syncing it to the owner's actual
// current decision.
//
// Unlike propagateMerge's best-effort posture (nothing new to lose — the
// merge it propagates already committed), a read failure here aborts the
// whole batch before any enqueue: a sync trigger has committed nothing yet,
// so there is nothing to reconcile a partial batch against.
func (h *Handlers) syncTagWriteback(ctx context.Context, videoIDs []int64, batchID string) (enqueued int, err error) {
	jobs := make([]writequeue.BatchJob, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		values, err := h.GenreWritebackValues(ctx, videoID)
		if err != nil {
			return 0, err
		}
		if len(values) == 0 {
			continue // nothing to write for this video
		}
		jobs = append(jobs, writequeue.BatchJob{
			VideoID: videoID,
			Fields:  []writequeue.JobField{{Field: "genres", Values: values, Source: fieldsource.Manual}},
		})
	}
	if len(jobs) == 0 {
		return 0, nil
	}
	ids, err := h.writeQueue.EnqueueMany(ctx, jobs, batchID)
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}
