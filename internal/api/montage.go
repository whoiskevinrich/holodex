package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"holodex/internal/model"
	"holodex/internal/montage"
	"holodex/internal/repo"
)

// montager is the hover-montage pipeline (F83, HOLODEX-578). Nil when it is not
// wired. It is a seam of its own, never folded into thumbnailer (F83 RD5).
type montager interface {
	Mode() montage.Mode
	Request(id int64) error
	EnqueueVisible(ctx context.Context, ids []int64)
	Status(ctx context.Context, id int64) (montage.Status, error)
	QueueStats() montage.Stats
}

// SetMontages wires the montage pipeline and the directory its files live in.
func (h *Handlers) SetMontages(m montager, dir string) {
	h.montages = m
	h.montageDir = dir
}

// setMontageURL sets MontageURL when the montage file exists, versioned by its
// own mtime like the still's URL.
func (h *Handlers) setMontageURL(v *model.Video) {
	if h.montageDir == "" {
		return
	}
	info, err := os.Stat(montage.Path(h.montageDir, v.ID))
	if err != nil || info.IsDir() {
		return
	}
	v.MontageURL = fmt.Sprintf("/api/v1/media/%d/montage?v=%d", v.ID, info.ModTime().UnixNano())
}

// prepareMontages sets each video's MontageURL and, in automatic mode only,
// bumps the ones with no file into the montage queue. Manual and off mode never
// reach the manager from a browse (F83 RD6).
func (h *Handlers) prepareMontages(ctx context.Context, videos []model.Video) {
	var missing []int64
	for i := range videos {
		h.setMontageURL(&videos[i])
		if videos[i].MontageURL == "" {
			missing = append(missing, videos[i].ID)
		}
	}
	if h.montages != nil && h.montages.Mode() == montage.ModeAutomatic && len(missing) > 0 {
		h.montages.EnqueueVisible(ctx, missing)
	}
}

// serveMontage serves a video's montage. Public, like the still; http.ServeContent
// answers the Range requests a <video> element makes.
func (h *Handlers) serveMontage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.montageDir == "" {
		writeError(w, http.StatusNotFound, "montage not ready")
		return
	}
	h.serveImageFile(w, r, id, "montage", montage.Path(h.montageDir, id))
}

// requestMontage queues one video's montage (spec P0-15). Owner-only: it starts
// ffmpeg work.
func (h *Handlers) requestMontage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.montages == nil {
		writeError(w, http.StatusServiceUnavailable, "montages unavailable")
		return
	}
	if _, err := h.repo.PathByID(r.Context(), id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "media not found")
			return
		}
		h.fail(w, "request montage", err)
		return
	}
	if err := h.montages.Request(id); err != nil {
		if errors.Is(err, montage.ErrDisabled) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, montage.ErrUnavailable) {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		h.fail(w, "request montage", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

// montageStatus reports where one video's montage stands, plus the mode, for the
// owner's status line. Owner-only.
func (h *Handlers) montageStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.montages == nil {
		writeJSON(w, http.StatusOK, map[string]any{"mode": montage.ModeOff, "state": "none"})
		return
	}
	st, err := h.montages.Status(r.Context(), id)
	if err != nil {
		h.fail(w, "montage status", err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Mode montage.Mode `json:"mode"`
		montage.Status
	}{h.montages.Mode(), st})
}
