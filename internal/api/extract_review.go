package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/extract"
	"holodex/internal/repo"
	"holodex/internal/writequeue"
)

// mountExtractionReview registers the extraction review-queue surface
// (F48.6/F48.7, ADR-067): list, resolve, dismiss. Mounted inside the
// requireOwner group set up in Mount, alongside the Phase 4 triggers
// (extract.go).
func (h *Handlers) mountExtractionReview(r chi.Router) {
	r.Get("/owner/extraction-queue", h.extractionQueue)
	r.Post("/owner/extraction-review/{id}/resolve", h.resolveExtractionReview)
	r.Post("/owner/extraction-review/{id}/dismiss", h.dismissExtractionReview)
}

// extractionQueue lists every pending extraction-review row, video-joined,
// for the owner's Extraction tab (F48.6) — a pure DB read, no write, mirrors
// enrichQueue's zero-cost-load contract.
func (h *Handlers) extractionQueue(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repo.ExtractionQueue(r.Context())
	if err != nil {
		h.fail(w, "extraction queue", err)
		return
	}
	if rows == nil {
		rows = []repo.ExtractionQueueRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

// resolveExtractionReview applies the owner's choice for one pending field
// (F48.6c) — action is "filename" | "tag" | "manual" — then marks the row
// resolved. "tag" writes nothing (the file's own tag value is already what's
// kept); "filename"/"manual" enqueue a write through the durable queue, the
// same path any other curated value takes. A row that is no longer pending
// (resolved or dismissed already, e.g. a second tab) is treated as
// already-handled per the design handoff's stale-row convention — 204, no
// error, no write.
func (h *Handlers) resolveExtractionReview(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	row, err := h.repo.GetExtractionReview(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		h.fail(w, "load extraction review", err)
		return
	}
	if row.Status != repo.ExtractionReviewPending {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var body struct {
		Action string `json:"action"`
		Value  string `json:"value"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	// The value-decision logic — what to write, if anything, for each action
	// — lives in internal/extract alongside Process's automatic routing
	// (F48.3/F48.4), so both the automatic and owner-resolved paths share one
	// place that knows how a field's values+source are decided.
	write, err := extract.ResolveReviewAction(extract.ReviewAction(body.Action), row.FilenameValue, row.TagValue, body.Value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(write.Values) > 0 {
		if h.writeQueue == nil {
			writeError(w, http.StatusServiceUnavailable, "writeback unavailable")
			return
		}
		if _, err := h.writeQueue.Enqueue(r.Context(), row.VideoID, []writequeue.JobField{
			{Field: row.FieldKey, Values: write.Values, Source: write.Source},
		}); err != nil {
			h.fail(w, "enqueue extraction resolve write", err)
			return
		}
	}

	if err := h.repo.ResolveExtractionReview(r.Context(), id); err != nil {
		h.fail(w, "resolve extraction review", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// dismissExtractionReview marks a pending row dismissed (F48.6d) — durable
// until the owner re-triggers extraction for the video, which opens a fresh
// pending row for the same field. Idempotent.
func (h *Handlers) dismissExtractionReview(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.repo.DismissExtractionReview(r.Context(), id); err != nil {
		h.fail(w, "dismiss extraction review", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
