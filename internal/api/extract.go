package api

import (
	"errors"
	"net/http"

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
func (h *Handlers) adminExtractAll(w http.ResponseWriter, _ *http.Request) {
	if h.extractBatch == nil {
		writeError(w, http.StatusServiceUnavailable, "extraction unavailable")
		return
	}
	started := h.extractBatch.TriggerAll()
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "started": started})
}
