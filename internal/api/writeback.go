package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
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
}

// writebackMedia writes a batch of enriched field values into the media file's
// tags in a single tool pass (exiftool for MP4/mp3/flac; mkvpropedit for
// MKV/WebM — F28, ADR-041). The operator has confirmed the values in the UI.
//
// All canonical→tag-name mappings are validated before any write; if any field
// has no mapping for the file's container a 422 is returned listing the
// unmappable fields. On success one audit row per field is inserted.
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

	v, _, err := h.repo.GetVideo(r.Context(), id)
	if err != nil {
		h.videoLookupError(w, err)
		return
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
			cleaned := make([]string, 0, len(f.Values))
			for _, val := range f.Values {
				if s := enrich.SanitizeValue(val); s != "" {
					cleaned = append(cleaned, s)
				}
			}
			if len(cleaned) == 0 {
				continue
			}
			jobFields = append(jobFields, writequeue.JobField{Field: f.Field, Values: cleaned, Source: f.Source})
		}
		if len(jobFields) == 0 {
			writeError(w, http.StatusBadRequest, "no writable fields after sanitization")
			return
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

	// Sanitize and resolve canonical→tag-name for every field before touching
	// the file. Collect all unmappable fields and return a single 422.
	type resolved struct {
		field   string
		tagName string
		cleaned []string
		source  string
		isImage bool
	}
	items := make([]resolved, 0, len(body.Fields))
	var unmapped []string

	for _, f := range body.Fields {
		if f.Field == "" || len(f.Values) == 0 {
			writeError(w, http.StatusBadRequest, "each field entry requires field and values")
			return
		}
		cleaned := make([]string, 0, len(f.Values))
		for _, val := range f.Values {
			if s := enrich.SanitizeValue(val); s != "" {
				cleaned = append(cleaned, s)
			}
		}
		if len(cleaned) == 0 {
			continue // skip fully-empty fields silently
		}
		// Image fields (e.g. poster_url) embed binary cover art — check before text tags.
		if imgTag, ok := writeback.ImageTagForField(f.Field, v.Container); ok {
			items = append(items, resolved{f.Field, imgTag, cleaned, f.Source, true})
			continue
		}
		tagName, supported := writeback.TagForField(f.Field, v.Container)
		if !supported {
			unmapped = append(unmapped, f.Field)
			continue
		}
		items = append(items, resolved{f.Field, tagName, cleaned, f.Source, false})
	}

	if len(items) == 0 {
		// Every checked field either had no mapping for this container or was
		// empty after sanitization. Tell the client which fields lacked a mapping
		// so the operator knows they need to uncheck them.
		if len(unmapped) > 0 {
			writeError(w, http.StatusUnprocessableEntity,
				"fields have no tag mapping for "+v.Container+": "+strings.Join(unmapped, ", "))
		} else {
			writeError(w, http.StatusBadRequest, "no writable fields after sanitization")
		}
		return
	}
	// Unmapped fields are silently skipped — only the mappable subset is written.
	// The audit rows below record exactly what was written.

	// Single tool invocation for all fields (exiftool or mkvpropedit by extension).
	batchFields := make([]writeback.FieldWrite, len(items))
	for i, it := range items {
		batchFields[i] = writeback.FieldWrite{TagName: it.tagName, Values: it.cleaned, IsImage: it.isImage}
	}
	if err := h.writeback(r.Context(), v.FilePath, batchFields); err != nil {
		h.log.Warn("writeback batch failed", "id", id, "fields", len(items), "err", err)
		h.fail(w, "write to file", err)
		return
	}

	// Audit rows — one per field, only on success (ADR-041 invariant).
	for _, it := range items {
		joined := strings.Join(it.cleaned, "\n")
		if auditErr := h.repo.InsertWriteback(r.Context(), id, it.field, it.tagName, joined, it.source); auditErr != nil {
			h.log.Warn("insert writeback audit row", "id", id, "field", it.field, "err", auditErr)
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

	w.WriteHeader(http.StatusNoContent)
}
