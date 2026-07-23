package api

import (
	"net/http"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/thumbnail"
)

// activityResponse is the aggregated "under the hood" read-model (F21.1,
// ADR-028). A typed struct (not a map) keeps the contract explicit and avoids
// per-request boxing/allocation on this ~3s-polled endpoint.
type activityResponse struct {
	Scan       model.ScanStatus     `json:"scan"`
	Thumbnails thumbnail.QueueStats `json:"thumbnails"`
	Library    repo.LibraryCounts   `json:"library"`
	System     activitySystem       `json:"system"`
}

// activitySystem carries non-sensitive runtime info. No-secrets invariant
// (ADR-028): MediaPathPresent is a boolean, never the path; no env or tokens.
type activitySystem struct {
	Ready            bool   `json:"ready"`
	Version          string `json:"version"`
	MediaPathPresent bool   `json:"media_path_present"`
	// ControlsUnauthenticated is the F21.7 fail-loud signal: true when the admin
	// surface is reachable beyond loopback with no ADMIN_TOKEN set (ADR-030).
	ControlsUnauthenticated bool  `json:"controls_unauthenticated"`
	UptimeSeconds           int64 `json:"uptime_seconds,omitempty"`
}

// adminActivity serves the live activity read-model (F21.1).
func (h *Handlers) adminActivity(w http.ResponseWriter, r *http.Request) {
	resp := activityResponse{Scan: model.ScanStatus{State: "idle"}}
	if h.scanStatus != nil {
		resp.Scan = h.scanStatus.Status()
	}
	if h.thumbs != nil {
		resp.Thumbnails = h.thumbs.QueueStats()
	}

	counts, err := h.repo.LibraryCounts(r.Context())
	if err != nil {
		h.fail(w, "library counts", err)
		return
	}
	resp.Library = counts

	resp.System = activitySystem{
		Ready:                   h.health != nil && h.health.Ready(),
		Version:                 h.version,
		MediaPathPresent:        h.mediaPathPresent,
		ControlsUnauthenticated: h.controlsUnauthenticated(),
	}
	if !h.startedAt.IsZero() {
		resp.System.UptimeSeconds = int64(time.Since(h.startedAt).Seconds())
	}

	writeJSON(w, http.StatusOK, resp)
}

// adminActivityHistory serves the persisted 30-day job-run history newest-first
// (F21.3). ?days= is clamped to the retention window by the repo.
func (h *Handlers) adminActivityHistory(w http.ResponseWriter, r *http.Request) {
	days := atoiDefault(r.URL.Query().Get("days"), 30)
	runs, err := h.repo.ListJobRuns(r.Context(), days)
	if err != nil {
		h.fail(w, "job history", err)
		return
	}
	if runs == nil {
		runs = []model.JobRun{}
	}
	writeJSON(w, http.StatusOK, struct {
		Runs []model.JobRun `json:"runs"`
	}{Runs: runs})
}

// adminActivityDigest serves the per-kind activity digest (ADR-071 D3): a bounded
// summary answering "is each job still running, and did anything fail in the
// window" whose response size is the number of job kinds plus the capped failure
// list — independent of how many runs the window holds. ?days= is clamped to the
// retention window by the repo.
func (h *Handlers) adminActivityDigest(w http.ResponseWriter, r *http.Request) {
	digest, err := h.repo.JobRunDigest(r.Context(), atoiDefault(r.URL.Query().Get("days"), 30))
	if err != nil {
		h.fail(w, "job digest", err)
		return
	}
	writeJSON(w, http.StatusOK, digest)
}
