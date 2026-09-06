package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/repo"
	"holodex/internal/resolver"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

// WriteBatchFunc is the file-write contract injected into Handlers (F28, ADR-041).
// Production wires internal/writeback.WriteBatch; tests wire a no-op or error stub.
// All fields are written in a single exiftool invocation.
type WriteBatchFunc func(ctx context.Context, path string, fields []writeback.FieldWrite) error

// SetWriteback wires the file-write function (F28, ADR-041). A nil fn disables
// the writeback endpoint (503). Called once at startup before serving.
func (h *Handlers) SetWriteback(fn WriteBatchFunc) { h.writeback = fn }

// SetWriteQueue wires the durable batch-write queue (F30, ADR-048). When set, the
// writeback endpoint enqueues (202) instead of writing synchronously. Nil keeps the
// legacy synchronous behavior (204). Called once at startup before serving.
func (h *Handlers) SetWriteQueue(q *writequeue.Queue) { h.writeQueue = q }

// mountWriteback registers the owner-gated writeback endpoint. Mounted inside
// the requireOwner group set up in Mount.
func (h *Handlers) mountWriteback(r chi.Router) {
	r.Post("/media/{id}/writeback", h.writebackMedia)
	// Rollback (F48.9, ADR-067): revert every field in a completed write batch
	// to its pre-write value. batchID comes from the writeback job's activity
	// history detail line (detailLine, F48.9d) — not scoped under /media/{id}
	// since a future multi-video batch (merge propagation, F48.8) can span
	// more than one video.
	r.Post("/writeback/batches/{batchID}/revert", h.writebackRevert)
	// Job status (HOLODEX-214): the POST below returns 202 the moment the job is
	// enqueued, so the SPA needs a completion signal before it can refetch.
	// Flat like the revert route rather than under /media/{id}: the id alone
	// identifies the job.
	r.Get("/writeback/jobs/{id}", h.writebackJobStatus)
	// Batch status (HOLODEX-239, ADR-077 D3): aggregate progress across every
	// job sharing a batchID, so an N-video tag-sync's dialog polls one
	// endpoint instead of fanning out to N individual job-status calls.
	r.Get("/writeback/batches/{batchID}/status", h.writebackBatchStatus)
	// Retry / Dismiss (ADR-091, HOLODEX-323, spec R3): act on THIS video's failed
	// writeback row(s). Scoped by the {id} path param alone (never a body-supplied
	// id), so one owner session can't retry/dismiss another video's failure.
	r.Post("/media/{id}/writeback/retry", h.retryWriteback)
	r.Post("/media/{id}/writeback/dismiss", h.dismissWriteback)
}

// retryWriteback resets this video's failed writeback job(s) back to pending and
// wakes the queue (spec R3.3). A safe no-op (200, retried:false) when there is
// nothing failed to retry — mirroring GetWritebackJobStatus's "absent row" posture
// rather than 404ing on a state that just means "nothing to do."
func (h *Handlers) retryWriteback(w http.ResponseWriter, r *http.Request) {
	if h.writeQueue == nil {
		writeError(w, http.StatusServiceUnavailable, "writeback unavailable")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	n, err := h.writeQueue.RetryFailed(r.Context(), id)
	if err != nil {
		h.fail(w, "retry writeback", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"retried": n > 0})
}

// dismissWriteback deletes this video's failed writeback row(s) without retrying
// (spec R3.4/RD2). job_runs already holds the permanent audit record for the
// failure, so this only clears the work-queue row — a safe no-op when nothing is
// failed.
func (h *Handlers) dismissWriteback(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	n, err := h.repo.DismissFailedWriteback(r.Context(), id)
	if err != nil {
		h.fail(w, "dismiss writeback", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dismissed": n > 0})
}

// markWriteTargets stamps each field's destination file tag for the video's
// current container (HOLODEX-216), so the writeback dialog can show exactly
// where a value will land and disable a field with no mapping rather than
// offering it and silently dropping it on write. Video-only — like
// markPromoted, the API layer stamps this after resolve since the resolver
// itself is entity-generic and has no container (ADR-052). Delegates to
// writeback.ResolveForContainer — the same mapper the sync write path and the
// durable queue worker use — rather than re-deriving the image-vs-text
// dispatch here, so this preview can never disagree with what actually gets
// written.
func (h *Handlers) markWriteTargets(fields []resolver.ResolvedField, container string) {
	specs := make([]writeback.FieldValues, len(fields))
	for i, f := range fields {
		specs[i] = writeback.FieldValues{Field: f.Canonical, Values: f.Values}
	}
	mapped, _ := writeback.ResolveForContainer(container, specs)
	targets := make(map[string]string, len(mapped))
	for _, m := range mapped {
		targets[m.Field] = m.TagName
	}
	for i := range fields {
		fields[i].WriteTarget = targets[fields[i].Canonical]
	}
}

// writebackBatchStatus reports aggregate counts (pending/running/done/failed)
// across every job enqueued under batchID (ADR-077 D3) — the progress signal
// the tag-scoped manual-sync dialog polls.
func (h *Handlers) writebackBatchStatus(w http.ResponseWriter, r *http.Request) {
	batchID := chi.URLParam(r, "batchID")
	if batchID == "" {
		writeError(w, http.StatusBadRequest, "batch id required")
		return
	}
	pending, running, done, failed, err := h.repo.GetWritebackBatchStatus(r.Context(), batchID)
	if err != nil {
		h.fail(w, "get writeback batch status", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pending": pending, "running": running, "done": done, "failed": failed,
	})
}

// writebackJobStatus reports one queued write's state: "pending" / "running"
// while in flight, "failed" (with the error) when it gave up, "done" once the
// row is gone. Owner-gated with the rest of the writeback surface.
func (h *Handlers) writebackJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID, ok := pathID(w, r)
	if !ok {
		return
	}
	status, errMsg, err := h.repo.GetWritebackJobStatus(r.Context(), jobID)
	if err != nil {
		h.fail(w, "get writeback job", err)
		return
	}
	out := map[string]any{"status": status}
	if errMsg != "" {
		out["error"] = errMsg
	}
	writeJSON(w, http.StatusOK, out)
}

// writebackMedia writes a batch of enriched field values into the media file's
// tags in a single tool pass (exiftool for MP4/mp3/flac; mkvpropedit for
// MKV/WebM — F28, ADR-041). The operator has confirmed the values in the UI.
//
// When queued (the durable path, F30/ADR-048), tag-name resolution happens
// later in the worker, so this returns 202 + job_id with no per-field outcome
// yet. On the legacy synchronous path (writeQueue == nil): if every submitted
// field is unmappable for the file's container, this returns a single 422
// naming them; a batch mixing mapped and unmapped fields instead writes the
// mappable subset and returns 200 with `written`/`skipped` naming exactly
// which is which (HOLODEX-216) — a skipped field must never read as written.
// On success one audit row per written field is inserted.
func (h *Handlers) writebackMedia(w http.ResponseWriter, r *http.Request) {
	if h.writeback == nil && h.writeQueue == nil {
		writeError(w, http.StatusServiceUnavailable, "writeback unavailable")
		return
	}

	id, ok := pathID(w, r)
	if !ok {
		return
	}

	var body struct {
		Fields []struct {
			Field  string   `json:"field"`
			Values []string `json:"values"`
			Source string   `json:"source"`
		} `json:"fields"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.Fields) == 0 {
		writeError(w, http.StatusBadRequest, "fields required")
		return
	}

	v, extra, err := h.repo.GetVideo(r.Context(), id)
	if err != nil {
		h.videoLookupError(w, err)
		return
	}

	// P0-10 (F50, ADR-075 RD9): a "genres" field write is sourced from the union of
	// the video's attached tags (ancestor-expanded) and the deny-list-filtered raw
	// resolved genres value, not whatever the client submitted for this field — so
	// the file's Genre tag deterministically reflects current DB state. Runs before
	// both paths below so neither can bypass it. There is only ever one meaningful
	// "genres" entry, so stop at the first match rather than recomputing per
	// duplicate if a client ever submits more than one. Uses the video already
	// loaded above rather than GenreWritebackValues' own re-fetch.
	for i, f := range body.Fields {
		if f.Field != "genres" {
			continue
		}
		values, gerr := h.genreWritebackValuesForVideo(r.Context(), v, extra)
		if gerr != nil {
			h.fail(w, "compute genre writeback values", gerr)
			return
		}
		if len(values) == 0 {
			// No attached tags and no resolved genre value: nothing to write for
			// this field. Drop the entry outright rather than leaving it with an
			// empty Values — the "each field entry requires field and values"
			// guards below would otherwise reject the whole batch over a
			// legitimately-empty genres union, not just skip that one field.
			body.Fields = append(body.Fields[:i], body.Fields[i+1:]...)
		} else {
			body.Fields[i].Values = values
		}
		break
	}

	// Queued path (F30, ADR-048): when the durable queue is wired, sanitize and
	// enqueue one batch job (202). Tag-name resolution + the actual write happen in
	// the worker so the request returns immediately and writes are throttled.
	if h.writeQueue != nil {
		jobFields := make([]writequeue.JobField, 0, len(body.Fields))
		for _, f := range body.Fields {
			if f.Field == "" || len(f.Values) == 0 {
				writeError(w, http.StatusBadRequest, "each field entry requires field and values")
				return
			}
			cleaned := enrich.SanitizeValues(f.Values)
			if len(cleaned) == 0 {
				continue
			}
			jobFields = append(jobFields, writequeue.JobField{Field: f.Field, Values: cleaned, Source: f.Source})
		}
		if len(jobFields) == 0 {
			writeError(w, http.StatusBadRequest, "no writable fields after sanitization")
			return
		}
		// Spec R3.5: submitting a new write is an implicit acknowledgment of any
		// prior failure for this video, so the Metadata header shows one job's
		// worth of truth rather than a stale "couldn't write" beside a fresh
		// "writing to file". Scoped to this video only — a second video's failed
		// row is untouched. Best-effort: a failure here must not block the actual
		// write the owner is trying to make.
		if _, clearErr := h.repo.DismissFailedWriteback(r.Context(), id); clearErr != nil {
			h.log.Warn("clear prior failed writeback before enqueue", "id", id, "err", clearErr)
		}
		jobID, enqErr := h.writeQueue.Enqueue(r.Context(), id, jobFields)
		if enqErr != nil {
			h.fail(w, "enqueue writeback", enqErr)
			return
		}
		depth, _ := h.writeQueue.Depth(r.Context())
		writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID, "queued": depth})
		return
	}

	// Sanitize, then resolve canonical→tag-name via the shared mapper the queue
	// worker also uses (writeback.ResolveForContainer) so both agree on what is
	// writable for the container. Unmappable fields yield a single 422.
	specs := make([]writeback.FieldValues, 0, len(body.Fields))
	for _, f := range body.Fields {
		if f.Field == "" || len(f.Values) == 0 {
			writeError(w, http.StatusBadRequest, "each field entry requires field and values")
			return
		}
		if cleaned := enrich.SanitizeValues(f.Values); len(cleaned) > 0 {
			specs = append(specs, writeback.FieldValues{Field: f.Field, Values: cleaned, Source: f.Source})
		}
	}
	mapped, unmapped := writeback.ResolveForContainer(v.Container, specs)
	if len(mapped) == 0 {
		// Every field either had no mapping for this container or was empty after
		// sanitization. Name the unmapped fields so the operator can uncheck them.
		if len(unmapped) > 0 {
			writeError(w, http.StatusUnprocessableEntity,
				"fields have no tag mapping for "+v.Container+": "+strings.Join(unmapped, ", "))
		} else {
			writeError(w, http.StatusBadRequest, "no writable fields after sanitization")
		}
		return
	}
	// A mixed batch writes only the mappable subset; unmapped is carried into the
	// response below so a partial batch never reads as a full success (HOLODEX-216).

	// Single tool invocation for all fields (exiftool or mkvpropedit by extension).
	batchFields := make([]writeback.FieldWrite, len(mapped))
	for i, m := range mapped {
		batchFields[i] = writeback.FieldWrite{TagName: m.TagName, Values: m.Values, IsImage: m.IsImage}
	}
	if err := h.writeback(r.Context(), v.FilePath, batchFields); err != nil {
		h.log.Warn("writeback batch failed", "id", id, "fields", len(mapped), "err", err)
		h.fail(w, "write to file", err)
		return
	}

	// Audit rows — one per field, only on success (ADR-041 invariant).
	for _, m := range mapped {
		joined := strings.Join(m.Values, "\n")
		if auditErr := h.repo.InsertWriteback(r.Context(), id, m.Field, m.TagName, joined, m.Source); auditErr != nil {
			h.log.Warn("insert writeback audit row", "id", id, "field", m.Field, "err", auditErr)
		}
	}

	// Re-check for embedded cover art in the (now-modified) file. This is a
	// best-effort pick-up of any image field just written (e.g. poster_url) as
	// well as any pre-existing art now detectable with the current extractor.
	// Errors are non-fatal — the writeback itself succeeded.
	if h.thumbs != nil && h.thumbs.Enabled() {
		if _, err := h.thumbs.ExtractEmbedded(r.Context(), id, v.FilePath); err != nil {
			h.log.Warn("post-writeback thumbnail re-extract", "id", id, "err", err)
		}
	}

	// Same post-write read-back the queued path does (ADR-073): without it this
	// branch writes the file and leaves the stored tags — the baseline `in_sync`
	// compares against — describing the pre-write file (HOLODEX-214).
	if h.refresh != nil {
		if err := h.refresh.ReExtract(r.Context(), id); err != nil {
			h.log.Warn("post-writeback re-extract", "id", id, "err", err)
		}
	}

	written := make([]string, len(mapped))
	for i, m := range mapped {
		written[i] = m.Field
	}
	if unmapped == nil {
		unmapped = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"written": written, "skipped": unmapped})
}

// writebackRevert restores every field snapshotted under batchID to its
// pre-write value (F48.9b) — one inverse writeback job per affected video,
// enqueued through the durable queue so the revert is itself snapshotted
// (F48.9c). Requires the queue; unavailable (503) in legacy synchronous mode,
// matching ADR-067's "revert is a new writeback job; it queues normally."
func (h *Handlers) writebackRevert(w http.ResponseWriter, r *http.Request) {
	if h.writeQueue == nil {
		writeError(w, http.StatusServiceUnavailable, "revert unavailable")
		return
	}
	batchID := chi.URLParam(r, "batchID")
	if batchID == "" {
		writeError(w, http.StatusBadRequest, "batch id required")
		return
	}
	jobIDs, err := h.writeQueue.Revert(r.Context(), batchID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "batch not found")
			return
		}
		h.fail(w, "revert writeback batch", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_ids": jobIDs})
}
