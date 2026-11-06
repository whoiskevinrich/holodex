package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/rename"
	"holodex/internal/repo"
)

// mountRenameApply registers the Manual-mode routes inside the owner group.
func (h *Handlers) mountRenameApply(r chi.Router) {
	r.Post("/rename/apply", h.renameApply)
	r.Get("/media/{id}/rename/status", h.renameStatus)
	r.Post("/media/{id}/rename/undo", h.renameUndo)
	r.Post("/media/{id}/rename/retry", h.renameRetry)
	r.Post("/media/{id}/rename/dismiss", h.renameDismiss)
}

// applyMaxBody bounds an apply request before anything is queued (A9).
const applyMaxBody = 1 << 20

// renameApplyBody is an apply: video ids only. A path, folder or name in the body is ignored; the
// folder always comes from the video's row (security-perimeter.md, A2).
type renameApplyBody struct {
	VideoIDs []int64 `json:"video_ids"`
}

// RenamePlanRow is the worker's PlanFunc: one video's plan row, with run counting as moving.
func (h *Handlers) RenamePlanRow(ctx context.Context, videoID int64, run []int64) (rename.Row, error) {
	// Collisions are settled per folder, so only run members in this video's folder are resolved.
	path, err := h.repo.PathByID(ctx, videoID)
	if err != nil {
		return rename.Row{}, err
	}
	only := map[int64]bool{videoID: true}
	for _, id := range run {
		if p, err := h.repo.PathByID(ctx, id); err == nil && filepath.Dir(p) == filepath.Dir(path) {
			only[id] = true
		}
	}
	st, err := h.loadRenameState(ctx, only)
	if err != nil {
		return rename.Row{}, err
	}
	_, list := h.renameTemplates()
	for _, row := range rename.Plan(list, h.renameDelimiter(), st.inputs, st.disk, func(id int64) bool { return only[id] }) {
		if row.VideoID == videoID {
			return row, nil
		}
	}
	return rename.Row{}, errors.New("the video is not in the library")
}

// renameReady answers 503/409 unless renaming can touch files: wired, and the mode at Manual or
// above (P0-7: Off and Plan never change anything on disk).
func (h *Handlers) renameReady(w http.ResponseWriter) bool {
	if h.settings == nil || h.mappings == nil || h.renamer == nil {
		writeError(w, http.StatusServiceUnavailable, "renaming not configured")
		return false
	}
	if !rename.ModeAtLeast(h.settings.Get(rename.KeyMode), rename.ModeManual) {
		writeError(w, http.StatusConflict, "renaming is not in Manual mode")
		return false
	}
	return true
}

func (h *Handlers) renameApply(w http.ResponseWriter, r *http.Request) {
	if !h.renameReady(w) {
		return
	}
	var body renameApplyBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, applyMaxBody)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	only := map[int64]bool{}
	for _, id := range body.VideoIDs {
		if id > 0 {
			only[id] = true
		}
	}
	if len(only) == 0 {
		writeError(w, http.StatusBadRequest, "no videos selected")
		return
	}
	ctx := r.Context()
	st, err := h.loadRenameState(ctx, only)
	if err != nil {
		h.fail(w, "rename apply", err)
		return
	}
	texts, list := h.renameTemplates()
	rows := rename.Plan(list, h.renameDelimiter(), st.inputs, st.disk, nil)

	batchID := "rename-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var jobs []repo.RenameEnqueue
	refused := map[string]int{}
	for _, row := range rows {
		if !rename.Renames(row.Reason) {
			refused[row.Reason]++
			continue
		}
		payload, _ := json.Marshal(rename.Payload{Name: row.Name, Template: texts[row.Template], Written: row.Written})
		jobs = append(jobs, repo.RenameEnqueue{VideoID: row.VideoID, Payload: string(payload)})
	}
	queued, err := h.repo.EnqueueRenames(ctx, jobs, batchID)
	if err != nil {
		h.fail(w, "rename apply", err)
		return
	}
	h.recordRenameApply(ctx, batchID, queued, refused)
	if queued > 0 {
		h.renamer.Kick()
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"batch_id": batchID, "queued": queued, "not_renamed": refused})
}

// recordRenameApply writes the batch's one System activity summary (P0-15): Seen is the queued
// count, Skipped the refused total and Detail the refused count per reason (locked and held among
// them), never a name. The queue fills in renamed and failed when the batch's last job finishes
// (repo.FinishRenameBatch).
func (h *Handlers) recordRenameApply(ctx context.Context, batchID string, queued int, refused map[string]int) {
	reasons := make([]string, 0, len(refused))
	total := 0
	for k, n := range refused {
		reasons = append(reasons, k)
		total += n
	}
	sort.Strings(reasons)
	parts := make([]string, 0, len(reasons))
	for _, k := range reasons {
		parts = append(parts, fmt.Sprintf("%s %d", k, refused[k]))
	}
	now := time.Now()
	if err := h.repo.RecordJobRun(ctx, model.JobRun{Kind: model.JobKindRename, Trigger: model.TriggerManual,
		Status: model.JobStatusOK, StartedAt: now, FinishedAt: now, Seen: queued, Skipped: total,
		Detail: strings.Join(parts, " · "), BatchID: batchID}); err != nil {
		h.log.Warn("record rename apply", "err", err)
	}
}

// renameStatusView is a video's rename job state. Owner-only: the error may carry paths.
type renameStatusView struct {
	Queued bool   `json:"queued"`
	Failed bool   `json:"failed"`
	Error  string `json:"error,omitempty"`
}

func (h *Handlers) renameStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	queued, err := h.repo.HasRenameJob(r.Context(), id)
	if err != nil {
		h.fail(w, "rename status", err)
		return
	}
	msg, failed, err := h.repo.FailedRename(r.Context(), id)
	if err != nil {
		h.fail(w, "rename status", err)
		return
	}
	writeJSON(w, http.StatusOK, renameStatusView{Queued: queued, Failed: failed, Error: msg})
}

// renameUndo queues an undo of the video's latest rename (P0-14). It is refused while the file no
// longer has the name Holodex wrote, or when the previous name is taken by any file; it never
// disambiguates. The worker checks all of it again.
func (h *Handlers) renameUndo(w http.ResponseWriter, r *http.Request) {
	if !h.renameReady(w) {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	path, err := h.repo.PathByID(ctx, id)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "video not found")
		return
	} else if err != nil {
		h.fail(w, "rename undo", err)
		return
	}
	last, err := h.repo.LatestDoneRename(ctx, id)
	if err != nil {
		h.fail(w, "rename undo", err)
		return
	}
	if last == nil || last.NewName != filepath.Base(path) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "nothing to undo", "reason": "not-holodex-name"})
		return
	}
	if held, err := h.repo.GetHeldOriginal(ctx, id); err != nil {
		h.fail(w, "rename undo", err)
		return
	} else if held != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "the video has a held original", "reason": "held"})
		return
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(path), last.OldName)); err == nil && rename.FoldKey(last.OldName) != rename.FoldKey(last.NewName) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "the previous name is taken", "reason": "taken", "name": last.OldName})
		return
	}
	payload, _ := json.Marshal(rename.Payload{Name: last.OldName, UndoOf: last.ID})
	queued, err := h.repo.EnqueueRenames(ctx, []repo.RenameEnqueue{{VideoID: id, Payload: string(payload)}}, "")
	if err != nil {
		h.fail(w, "rename undo", err)
		return
	}
	if queued == 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "a rename is already queued", "reason": "in-flight"})
		return
	}
	h.renamer.Kick()
	writeJSON(w, http.StatusAccepted, map[string]int{"queued": queued})
}

func (h *Handlers) renameRetry(w http.ResponseWriter, r *http.Request) {
	if !h.renameReady(w) {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	n, err := h.repo.RetryFailedRename(r.Context(), id)
	if err != nil {
		h.fail(w, "rename retry", err)
		return
	}
	if n > 0 {
		h.renamer.Kick()
	}
	writeJSON(w, http.StatusOK, map[string]int64{"retried": n})
}

func (h *Handlers) renameDismiss(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	n, err := h.repo.DismissFailedRename(r.Context(), id)
	if err != nil {
		h.fail(w, "rename dismiss", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"dismissed": n})
}
