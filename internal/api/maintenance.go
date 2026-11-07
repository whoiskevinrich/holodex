package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"holodex/internal/maintenance"
	"holodex/internal/repo"
)

// Maintenance is the slice of the maintenance worker the API drives (F77): the flip preflight,
// the hold window the confirm dialog quotes, and the wake-up after an enqueue.
type Maintenance interface {
	CheckFlip(ctx context.Context, path string) (maintenance.Preflight, error)
	CheckTrim(ctx context.Context, path string, kept float64, mode, edition, current string) (maintenance.Preflight, error)
	CaptureRoot() string
	Hold() time.Duration
	Kick()
}

// SetMaintenance wires video file maintenance. Nil leaves every maintenance route 503.
func (h *Handlers) SetMaintenance(m Maintenance) { h.maint = m }

// reasonInFlight is a preflight refusal the worker can't see: a maintenance job for this video
// is already queued or running (spec R5).
const reasonInFlight = "in_flight"

// mountMaintenance registers the owner-gated maintenance routes. Mounted inside the
// requireOwner group; every route is scoped by the {id} path param alone, and every path the
// worker touches comes from the videos row, never from the request.
func (h *Handlers) mountMaintenance(r chi.Router) {
	r.Get("/media/{id}/maintenance/preflight", h.maintenancePreflight)
	r.Post("/media/{id}/maintenance", h.enqueueFlip)
	r.Post("/media/{id}/maintenance/trim", h.enqueueTrim)
	r.Post("/media/{id}/maintenance/trim/dismiss", h.dismissTrimResult)
	r.Post("/media/{id}/maintenance/restore", h.enqueueHeld(maintenance.OpRestore))
	r.Post("/media/{id}/maintenance/discard", h.enqueueHeld(maintenance.OpDiscard))
	r.Post("/media/{id}/maintenance/retry", h.retryMaintenance)
	r.Post("/media/{id}/maintenance/dismiss", h.dismissMaintenance)
}

// preflightView is the confirm dialog's payload: the preflight plus the date the held
// original will be deleted (an existing hold keeps its own date: a second flip doesn't
// restart the window).
type preflightView struct {
	maintenance.Preflight
	ExpiresAt time.Time `json:"expires_at"`
}

// laneGate is what every operation's preflight shares: the date a held original would be deleted,
// and whether this video's lane is already busy (the returned view then carries the refusal).
func (h *Handlers) laneGate(ctx context.Context, id int64) (preflightView, bool, error) {
	view := preflightView{ExpiresAt: time.Now().Add(h.maint.Hold()).UTC()}
	held, err := h.repo.GetHeldOriginal(ctx, id)
	if err != nil {
		return preflightView{}, false, err
	}
	if held != nil {
		view.ExpiresAt = held.ExpiresAt
	}
	ms, err := h.repo.GetVideoMaintenanceStatus(ctx, id)
	if err != nil {
		return preflightView{}, false, err
	}
	if ms.Pending {
		view.Reason = reasonInFlight
		return view, true, nil
	}
	return view, false, nil
}

// flipPreflight runs every check, the worker's and the queue's, for video id.
func (h *Handlers) flipPreflight(ctx context.Context, id int64) (preflightView, int, error) {
	path, err := h.repo.PathByID(ctx, id)
	if errors.Is(err, repo.ErrNotFound) {
		return preflightView{}, http.StatusNotFound, err
	} else if err != nil {
		return preflightView{}, http.StatusInternalServerError, err
	}
	view, busy, err := h.laneGate(ctx, id)
	if err != nil {
		return preflightView{}, http.StatusInternalServerError, err
	}
	if busy {
		return view, http.StatusOK, nil
	}
	if view.Preflight, err = h.maint.CheckFlip(ctx, path); err != nil {
		return preflightView{}, http.StatusInternalServerError, err
	}
	return view, http.StatusOK, nil
}

// maintenancePreflight answers the confirm dialog: can this video be flipped, and if not why.
func (h *Handlers) maintenancePreflight(w http.ResponseWriter, r *http.Request) {
	if h.maint == nil {
		writeError(w, http.StatusServiceUnavailable, "maintenance unavailable")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var (
		view   preflightView
		status int
		err    error
	)
	switch r.URL.Query().Get("op") {
	case string(maintenance.OpHFlip):
		view, status, err = h.flipPreflight(r.Context(), id)
	case string(maintenance.OpTrim):
		q := r.URL.Query()
		in, _ := strconv.ParseFloat(q.Get("in"), 64)
		out, _ := strconv.ParseFloat(q.Get("out"), 64)
		mode := q.Get("mode")
		if !validTrimMode(mode) {
			writeError(w, http.StatusBadRequest, "unknown trim mode")
			return
		}
		view, status, err = h.trimPreflight(r.Context(), id, in, out, mode, q.Get("edition"))
	default:
		writeError(w, http.StatusBadRequest, "unknown maintenance operation")
		return
	}
	if err != nil {
		if status == http.StatusNotFound {
			writeError(w, status, "media not found")
			return
		}
		h.fail(w, "maintenance preflight", err)
		return
	}
	writeJSON(w, status, view)
}

// enqueueFlip queues a flip. The preflight runs again here, so a dialog opened before the
// library went read-only (or before another flip was queued) can't get past it.
func (h *Handlers) enqueueFlip(w http.ResponseWriter, r *http.Request) {
	if h.maint == nil {
		writeError(w, http.StatusServiceUnavailable, "maintenance unavailable")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body maintenance.Payload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Op != string(maintenance.OpHFlip) {
		writeError(w, http.StatusBadRequest, "unknown maintenance operation")
		return
	}
	view, status, err := h.flipPreflight(r.Context(), id)
	if err != nil {
		if status == http.StatusNotFound {
			writeError(w, status, "media not found")
			return
		}
		h.fail(w, "maintenance preflight", err)
		return
	}
	if !view.OK {
		writeJSON(w, http.StatusConflict, view)
		return
	}
	h.enqueueOp(w, r, id, body.Op)
}

// enqueueHeld queues a restore or a discard of the video's standing hold.
func (h *Handlers) enqueueHeld(op string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.maint == nil {
			writeError(w, http.StatusServiceUnavailable, "maintenance unavailable")
			return
		}
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		held, err := h.repo.GetHeldOriginal(r.Context(), id)
		if err != nil {
			h.fail(w, "held original", err)
			return
		}
		if held == nil || held.State != repo.HeldHeld {
			writeError(w, http.StatusConflict, "no original is being held for this video")
			return
		}
		h.enqueueOp(w, r, id, op)
	}
}

func (h *Handlers) enqueueOp(w http.ResponseWriter, r *http.Request, id int64, op string) {
	payload, _ := json.Marshal(maintenance.Payload{Op: op})
	jobID, err := h.repo.EnqueueMaintenance(r.Context(), id, string(payload))
	if errors.Is(err, repo.ErrMaintenanceInFlight) {
		writeJSON(w, http.StatusConflict, preflightView{Preflight: maintenance.Preflight{Reason: reasonInFlight}})
		return
	} else if err != nil {
		h.fail(w, "enqueue maintenance", err)
		return
	}
	h.maint.Kick()
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

func (h *Handlers) retryMaintenance(w http.ResponseWriter, r *http.Request) {
	if h.maint == nil {
		writeError(w, http.StatusServiceUnavailable, "maintenance unavailable")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	n, err := h.repo.RetryFailedMaintenance(r.Context(), id)
	if err != nil {
		h.fail(w, "retry maintenance", err)
		return
	}
	if n > 0 {
		h.maint.Kick()
	}
	writeJSON(w, http.StatusOK, map[string]any{"retried": n > 0})
}

func (h *Handlers) dismissMaintenance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	// A failed trim's captures wait for Retry, so dismissing it is what lets them go.
	ms, err := h.repo.GetVideoMaintenanceStatus(r.Context(), id)
	if err != nil {
		h.fail(w, "dismiss maintenance", err)
		return
	}
	n, err := h.repo.DismissFailedMaintenance(r.Context(), id)
	if err != nil {
		h.fail(w, "dismiss maintenance", err)
		return
	}
	if n > 0 && h.maint != nil {
		h.removeTrimCaptures(ms.Payload)
	}
	writeJSON(w, http.StatusOK, map[string]any{"dismissed": n > 0})
}

// maintenanceView is the media payload's owner-only maintenance block: the lane's state, the
// operation it describes, and the standing hold. A visitor's payload carries no such key at
// all (spec R1): the state, the error (absolute paths) and the held path never leave the
// server for them.
type maintenanceView struct {
	Pending   bool       `json:"pending"`
	Running   bool       `json:"running"` // claimed by the worker; pending and not running = queued
	StartedAt *time.Time `json:"started_at,omitempty"`
	Failed    bool       `json:"failed"`
	Error     string     `json:"error,omitempty"`
	Op        string     `json:"op,omitempty"`
	Held      *heldView  `json:"held"`
	// The last trim's outcome until dismissed, and an edition's source (F88).
	Trim        *trimResultView  `json:"trim,omitempty"`
	TrimmedFrom *trimmedFromView `json:"trimmed_from,omitempty"`
}

type heldView struct {
	Op        string    `json:"op"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type trimResultView struct {
	Mode           string                     `json:"mode"`
	EditionName    string                     `json:"edition_name,omitempty"`
	EditionVideoID *int64                     `json:"edition_video_id,omitempty"`
	Skipped        []maintenance.SkippedCheck `json:"skipped"`
	CreatedAt      time.Time                  `json:"created_at"`
}

type trimmedFromView struct {
	VideoID int64  `json:"video_id"`
	Title   string `json:"title"`
}

func (h *Handlers) maintenanceFor(ctx context.Context, id int64) (*maintenanceView, error) {
	ms, err := h.repo.GetVideoMaintenanceStatus(ctx, id)
	if err != nil {
		return nil, err
	}
	v := &maintenanceView{Pending: ms.Pending, Running: ms.Running, Failed: ms.Failed, Error: ms.Error}
	if ms.Running && !ms.StartedAt.IsZero() {
		v.StartedAt = &ms.StartedAt
	}
	var p maintenance.Payload
	if json.Unmarshal([]byte(ms.Payload), &p) == nil {
		v.Op = p.Op
	}
	held, err := h.repo.GetHeldOriginal(ctx, id)
	if err != nil {
		return nil, err
	}
	if held != nil && held.State == repo.HeldHeld {
		v.Held = &heldView{Op: held.Op, CreatedAt: held.CreatedAt, ExpiresAt: held.ExpiresAt}
	}
	tr, err := h.repo.GetTrimResult(ctx, id)
	if err != nil {
		return nil, err
	}
	if tr != nil {
		view := &trimResultView{Mode: tr.Mode, EditionName: tr.EditionName, EditionVideoID: tr.EditionVideoID,
			Skipped: []maintenance.SkippedCheck{}, CreatedAt: tr.CreatedAt}
		if err := json.Unmarshal([]byte(tr.Skipped), &view.Skipped); err != nil {
			return nil, err
		}
		v.Trim = view
	}
	from, err := h.repo.GetTrimmedFrom(ctx, id)
	if err != nil {
		return nil, err
	}
	if from != nil {
		v.TrimmedFrom = &trimmedFromView{VideoID: from.VideoID, Title: from.Title}
	}
	return v, nil
}
