package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/fieldsource"
	"holodex/internal/repo"
	"holodex/internal/writequeue"
)

// Film-studio cascade (F57, HOLODEX-285, ADR-086): one owner action that sets a new
// manual Studio decision AND enqueues a file writeback across every video attached
// to a film. Owner-gated (mounted alongside mountFilmDecisions).

// mountFilmStudioCascade registers the cascade trigger, called from mountFilms.
func (h *Handlers) mountFilmStudioCascade(r chi.Router) {
	r.Post("/films/{id}/studio/cascade", h.cascadeFilmStudioHandler)
}

// filmStudioCascadeResult is one video's outcome from a cascade run.
type filmStudioCascadeResult struct {
	VideoID  int64                `json:"video_id"`
	Status   string               `json:"status"` // "enqueued" | "collision" | "error"
	Conflict *repo.VideoCollision `json:"conflict,omitempty"`
	Error    string               `json:"error,omitempty"`
}

// cascadeFilmStudioHandler handles POST /films/{id}/studio/cascade. Reuses
// decisionBody's {source, manual_value} shape — no Override field meaning here,
// since cascadeFilmStudio always runs the collision gate (RD4's unconditional
// overwrite applies to a video's prior decision, never to the safety gate).
func (h *Handlers) cascadeFilmStudioHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.GetFilm(r.Context(), id); err != nil {
		h.filmLookupError(w, err)
		return
	}
	var body decisionBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if !fieldsource.Valid(body.Source) {
		writeError(w, http.StatusBadRequest, "source must be 'file', 'manual', or 'provider:<name>'")
		return
	}
	manualValue := ""
	if body.Source == fieldsource.Manual {
		if manualValue = enrich.SanitizeValue(body.ManualValue); manualValue == "" {
			writeError(w, http.StatusBadRequest, "manual_value required for a manual decision")
			return
		}
	}
	if h.writeQueue == nil {
		writeError(w, http.StatusServiceUnavailable, "writeback unavailable")
		return
	}

	batchID, results, err := h.cascadeFilmStudio(r.Context(), id, body.Source, manualValue)
	if err != nil {
		h.fail(w, "cascade film studio", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"batch_id": batchID, "results": results})
}

// cascadeFilmStudio runs decideStudioForVideo once per video attached to the film
// (best-effort — a collision or error excludes only that video, see ADR-086's
// Forces/D2 for why this differs from syncTagWriteback's abort-on-read-failure
// posture), then enqueues one shared-batch writeback job for every video whose
// decision was set successfully. override is always false: RD4's unconditional
// overwrite applies to a video's prior Studio decision, not to the HOLODEX-270/271
// composite-key collision gate, which must stay live per video during the cascade.
//
// A film with zero attached videos, or one where every video collides/errors, is a
// clean no-op: batchID is "" and results may be empty or all non-"enqueued" — never
// an error.
func (h *Handlers) cascadeFilmStudio(ctx context.Context, filmID int64, source, manualValue string) (batchID string, results []filmStudioCascadeResult, err error) {
	field, _ := h.mappings.Current().ByCanonical("studio") // always present — replaceField already guards this shape elsewhere
	videoIDs, err := h.repo.VideoIDsForFilm(ctx, filmID)
	if err != nil {
		return "", nil, err
	}

	jobs := make([]writequeue.BatchJob, 0, len(videoIDs))
	results = make([]filmStudioCascadeResult, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		names, collision, decErr := h.decideStudioForVideo(ctx, videoID, field, source, manualValue, false)
		switch {
		case decErr != nil:
			results = append(results, filmStudioCascadeResult{VideoID: videoID, Status: "error", Error: decErr.Error()})
		case collision != nil:
			results = append(results, filmStudioCascadeResult{VideoID: videoID, Status: "collision", Conflict: collision})
		default:
			results = append(results, filmStudioCascadeResult{VideoID: videoID, Status: "enqueued"})
			jobs = append(jobs, writequeue.BatchJob{
				VideoID: videoID,
				Fields:  []writequeue.JobField{{Field: "studio", Values: names, Source: fieldsource.Manual}},
			})
		}
	}

	if len(jobs) == 0 {
		return "", results, nil // every video collided/errored, or the film has no videos; nothing to enqueue
	}
	batchID = fmt.Sprintf("film-studio-cascade-%d", time.Now().UnixNano())
	if _, err := h.writeQueue.EnqueueMany(ctx, jobs, batchID); err != nil {
		return "", nil, err
	}
	return batchID, results, nil
}
