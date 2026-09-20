package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// extractMedia runs filename extraction (F48.1-F48.4) for one video
// on-demand (F48.5a) and reflects the result immediately — no queue, no
// preview: the same synchronous shape as refreshMedia.
func (h *Handlers) extractMedia(w http.ResponseWriter, r *http.Request) {
	if h.extract == nil {
		writeError(w, http.StatusServiceUnavailable, "extraction unavailable")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}

	res, err := h.extract.ExtractVideo(r.Context(), id)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
		return
	case err != nil:
		h.log.Warn("extract from filename failed", "id", id, "err", err)
		h.fail(w, "extract from filename", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// adminExtractAll triggers a library-wide filename-extraction pass ("Extract
// all", F48.5b) and returns 202 Accepted immediately; the pass runs in the
// background and its progress is observable via System Activity
// (kind=extraction). "started":false means a pass was already in progress,
// which already satisfies the request — mirrors adminRescan.
// sweepKinds maps the route's plural segment to the entity type the sweep runs
// over (spec P0-3: `sweep.kind` is the entity type, never the route plural).
var sweepKinds = map[string]string{
	"people":  model.EnrichEntityPerson,
	"studios": model.EnrichEntityStudio,
}

// adminEnrichSweep triggers one background refresh sweep over every person or
// studio (F66, ADR-103 D8): POST /admin/enrich/sweep/{people|studios} with an
// optional {force:true} body to ignore the 24 h staleness window. Answers 202 with
// started:false when a sweep of either kind is already running — informational,
// never an error (RD4). Progress rides the `sweep` block on /admin/activity.
func (h *Handlers) adminEnrichSweep(w http.ResponseWriter, r *http.Request) {
	if h.sweep == nil {
		writeError(w, http.StatusServiceUnavailable, "refresh sweep unavailable")
		return
	}
	kind, ok := sweepKinds[chi.URLParam(r, "kind")]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown sweep kind")
		return
	}
	var body struct {
		Force bool `json:"force"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	started, err := h.sweep.Trigger(kind, body.Force)
	if err != nil {
		h.fail(w, "enrich sweep trigger", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "started": started})
}

func (h *Handlers) adminExtractAll(w http.ResponseWriter, _ *http.Request) {
	if h.extractBatch == nil {
		writeError(w, http.StatusServiceUnavailable, "extraction unavailable")
		return
	}
	started := h.extractBatch.TriggerAll()
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "started": started})
}
