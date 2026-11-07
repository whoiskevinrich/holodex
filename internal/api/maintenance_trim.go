package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/jpeg"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"holodex/internal/maintenance"
	"holodex/internal/repo"
)

// A trim's captures are the frames the owner saw, for the screen check (F88 R6). The check scales
// them far down, so this is generous for a 4K frame and still small.
const trimCaptureMaxBytes = 8 << 20

func validTrimMode(mode string) bool {
	return mode == maintenance.TrimReplace || mode == maintenance.TrimEdition
}

// trimPreflight runs every check for a trim of video id: the worker's, and the queue's. It answers
// the review step and guards the enqueue, so a dialog left open can't get past it.
func (h *Handlers) trimPreflight(ctx context.Context, id int64, in, out float64, mode, edition string) (preflightView, int, error) {
	v, _, err := h.repo.GetVideo(ctx, id)
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
	kept := 1.0
	if v.Duration > 0 && out > in {
		kept = math.Min((out-in)/float64(v.Duration), 1)
	}
	current := ""
	if mode == maintenance.TrimEdition {
		current = h.videoEdition(ctx, id)
	}
	if view.Preflight, err = h.maint.CheckTrim(ctx, v.FilePath, kept, mode, edition, current); err != nil {
		return preflightView{}, http.StatusInternalServerError, err
	}
	return view, http.StatusOK, nil
}

// enqueueTrim queues a trim: one multipart request carrying the range, the save mode and both
// captures, so a job can't exist without them or they without a job. The folder name is generated
// here. Nothing from the request reaches a path except through the worker's name sanitizer.
func (h *Handlers) enqueueTrim(w http.ResponseWriter, r *http.Request) {
	if h.maint == nil {
		writeError(w, http.StatusServiceUnavailable, "maintenance unavailable")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2*trimCaptureMaxBytes+1<<16)
	if err := r.ParseMultipartForm(2*trimCaptureMaxBytes + 1<<16); err != nil {
		writeError(w, http.StatusBadRequest, "invalid trim request")
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck // best effort, the OS temp dir

	in, err1 := strconv.ParseFloat(r.FormValue("in_time"), 64)
	out, err2 := strconv.ParseFloat(r.FormValue("out_time"), 64)
	mode, edition := r.FormValue("mode"), r.FormValue("edition")
	if err1 != nil || err2 != nil || math.IsNaN(in) || math.IsNaN(out) || math.IsInf(in, 0) || math.IsInf(out, 0) || in < 0 || out <= in {
		writeError(w, http.StatusBadRequest, "the first frame kept must come before the last")
		return
	}
	if !validTrimMode(mode) {
		writeError(w, http.StatusBadRequest, "unknown trim mode")
		return
	}
	captureIn, err1 := readTrimCapture(r, "capture_in")
	captureOut, err2 := readTrimCapture(r, "capture_out")
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, "both frame captures must be JPEG images")
		return
	}

	if v, _, err := h.repo.GetVideo(r.Context(), id); err == nil && v.Duration > 0 && in > float64(v.Duration)+1 {
		writeError(w, http.StatusBadRequest, "the range starts past the end of the file")
		return
	}
	view, status, err := h.trimPreflight(r.Context(), id, in, out, mode, edition)
	if err != nil {
		if status == http.StatusNotFound {
			writeError(w, status, "media not found")
			return
		}
		h.fail(w, "trim preflight", err)
		return
	}
	if !view.OK {
		writeJSON(w, http.StatusConflict, view)
		return
	}

	captures, err := h.storeTrimCaptures(captureIn, captureOut)
	if err != nil {
		h.fail(w, "store trim captures", err)
		return
	}
	payload, _ := json.Marshal(maintenance.Payload{Op: string(maintenance.OpTrim), Trim: &maintenance.TrimParams{
		InTime: in, OutTime: out, Mode: mode, Edition: edition, Captures: captures,
	}})
	jobID, err := h.repo.EnqueueMaintenance(r.Context(), id, string(payload))
	if err != nil {
		os.RemoveAll(filepath.Join(h.maint.CaptureRoot(), captures))
		if errors.Is(err, repo.ErrMaintenanceInFlight) {
			writeJSON(w, http.StatusConflict, preflightView{Preflight: maintenance.Preflight{Reason: reasonInFlight}})
			return
		}
		h.fail(w, "enqueue trim", err)
		return
	}
	h.maint.Kick()
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID})
}

// readTrimCapture reads one capture and proves it decodes as a JPEG, by content, not by its name or
// declared type. ffmpeg later opens it through a forced JPEG demuxer only.
func readTrimCapture(r *http.Request, field string) ([]byte, error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, trimCaptureMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > trimCaptureMaxBytes {
		return nil, errors.New("capture too large")
	}
	if _, err := jpeg.DecodeConfig(bytes.NewReader(data)); err != nil {
		return nil, err
	}
	return data, nil
}

// storeTrimCaptures saves a job's two captures in a new folder and returns the folder's name.
func (h *Handlers) storeTrimCaptures(in, out []byte) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	name := hex.EncodeToString(b[:])
	dir := filepath.Join(h.maint.CaptureRoot(), name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	for file, data := range map[string][]byte{"in.jpg": in, "out.jpg": out} {
		if err := os.WriteFile(filepath.Join(dir, file), data, 0o600); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
	}
	return name, nil
}

// removeTrimCaptures deletes the captures a failed trim's payload names. The name was generated by
// this server, and is read back from the queue row, never from a request.
func (h *Handlers) removeTrimCaptures(rawPayload string) {
	var p maintenance.Payload
	if json.Unmarshal([]byte(rawPayload), &p) != nil || p.Trim == nil || p.Trim.Captures == "" {
		return
	}
	os.RemoveAll(filepath.Join(h.maint.CaptureRoot(), p.Trim.Captures))
}

// dismissTrimResult clears the outcome notice on the video the trim ran from.
func (h *Handlers) dismissTrimResult(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	dismissed, err := h.repo.DismissTrimResult(r.Context(), id)
	if err != nil {
		h.fail(w, "dismiss trim result", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dismissed": dismissed})
}
