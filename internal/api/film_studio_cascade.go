package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"holodex/internal/fieldsource"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writequeue"
)

// Film-studio cascade (F57, HOLODEX-285, ADR-087): one owner action that sets a new
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
// decisionBody's {source, manual_value} shape — Override is rejected rather than
// silently ignored: cascadeFilmStudio always runs the per-video collision gate
// (RD4's unconditional overwrite applies to a video's prior decision, never to the
// safety gate), so a caller that sends override=true would be misled into thinking
// it bypassed a check that in fact still ran.
func (h *Handlers) cascadeFilmStudioHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.GetFilm(r.Context(), id); err != nil {
		h.filmLookupError(w, err)
		return
	}
	body, manualValue, ok := decodeDecisionBody(w, r)
	if !ok {
		return
	}
	if body.Override {
		writeError(w, http.StatusBadRequest, "override is not supported for the film-studio cascade")
		return
	}
	field, ok := h.replaceField(w, "studio")
	if !ok {
		return
	}
	if h.writeQueue == nil {
		writeError(w, http.StatusServiceUnavailable, "writeback unavailable")
		return
	}

	batchID, results, err := h.cascadeFilmStudio(r.Context(), id, field, body.Source, manualValue)
	if err != nil {
		if results == nil {
			h.fail(w, "cascade film studio", err)
			return
		}
		// Decisions were already committed per-video before EnqueueMany failed —
		// surface the partial results instead of hiding them behind a 5xx (D3).
		h.log.Warn("cascade film studio: writeback enqueue failed after decisions committed", "film_id", id, "err", err)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"batch_id": batchID, "results": results})
}

// cascadeFilmStudio runs decideStudioForVideo once per video attached to the film
// (best-effort — a collision or error excludes only that video, see ADR-087's
// Forces/D2 for why this differs from syncTagWriteback's abort-on-read-failure
// posture), then enqueues one shared-batch writeback job for every video whose
// decision was set successfully. override is always false: RD4's unconditional
// overwrite applies to a video's prior Studio decision, not to the HOLODEX-270/271
// composite-key collision gate, which must stay live per video during the cascade.
//
// A film with zero attached videos, or one where every video collides/errors, is a
// clean no-op: batchID is "" and results may be empty or all non-"enqueued" — never
// an error. field is the caller's already-resolved+guarded "studio" mapping.Field
// (cascadeFilmStudioHandler resolves it via replaceField, mirroring setFieldDecision).
func (h *Handlers) cascadeFilmStudio(ctx context.Context, filmID int64, field mapping.Field, source, manualValue string) (batchID string, results []filmStudioCascadeResult, err error) {
	videoIDs, err := h.repo.VideoIDsForFilm(ctx, filmID)
	if err != nil {
		return "", nil, err
	}

	provider := fieldsource.Provider(source)
	jobs := make([]writequeue.BatchJob, 0, len(videoIDs))
	results = make([]filmStudioCascadeResult, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		errResult := func(msg string) filmStudioCascadeResult {
			return filmStudioCascadeResult{VideoID: videoID, Status: "error", Error: msg}
		}
		// Liveness re-check: VideoIDsForFilm's snapshot can race a concurrent
		// soft-delete/removal between that fetch and this video's turn in the
		// loop — skip it as a clean per-video error rather than letting
		// decideStudioForVideo accumulate a decision against a trashed item.
		if _, err := h.repo.RefreshTarget(ctx, videoID); err != nil {
			msg := "video no longer exists"
			if errors.Is(err, repo.ErrDeleted) {
				msg = "video is deleted"
			}
			results = append(results, errResult(msg))
			continue
		}
		if provider != "" && !h.providerMatched(ctx, model.EnrichEntityVideo, videoID, provider) {
			results = append(results, errResult("provider is not matched to this item"))
			continue
		}
		names, collision, decErr := h.decideStudioForVideo(ctx, videoID, field, source, manualValue, false)
		switch {
		case decErr != nil:
			results = append(results, errResult(decErr.Error()))
		case collision != nil:
			results = append(results, filmStudioCascadeResult{VideoID: videoID, Status: "collision", Conflict: collision})
		default:
			results = append(results, filmStudioCascadeResult{VideoID: videoID, Status: "enqueued"})
			jobs = append(jobs, writequeue.BatchJob{
				VideoID: videoID,
				Fields:  []writequeue.JobField{{Field: "studio", Values: names, Source: source}},
			})
		}
	}

	if len(jobs) == 0 {
		return "", results, nil // every video collided/errored, or the film has no videos; nothing to enqueue
	}
	batchID = fmt.Sprintf("film-studio-cascade-%d", time.Now().UnixNano())
	if _, err := h.writeQueue.EnqueueMany(ctx, jobs, batchID); err != nil {
		// Decisions above are already committed per-video (SetDecisionChecked
		// writes as each video is processed) — only the batch enqueue failed, so
		// results must survive this error for the caller to see what landed (D3).
		return "", results, err
	}
	return batchID, results, nil
}
