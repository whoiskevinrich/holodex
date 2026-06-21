package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/writeback"
)

// WriteFunc is the file-write contract injected into Handlers (F28, ADR-041).
// Production wires internal/writeback.Write; tests wire a no-op or error stub.
type WriteFunc func(ctx context.Context, path, tagName string, values []string) error

// SetWriteback wires the file-write function (F28, ADR-041). A nil fn disables
// the writeback endpoint (503). Called once at startup before serving.
func (h *Handlers) SetWriteback(fn WriteFunc) { h.writeback = fn }

// mountWriteback registers the owner-gated writeback endpoint. Mounted inside
// the requireOwner group set up in Mount.
func (h *Handlers) mountWriteback(r chi.Router) {
	r.Post("/media/{id}/writeback", h.writebackMedia)
}

// writebackMedia writes one enriched field value into the media file's tags
// (F28, ADR-041). The operator has already seen and confirmed the value in the
// UI before this call. On success the handler inserts one audit row; on any
// failure the original file is untouched and no audit row is written.
func (h *Handlers) writebackMedia(w http.ResponseWriter, r *http.Request) {
	if h.writeback == nil {
		writeError(w, http.StatusServiceUnavailable, "writeback unavailable")
		return
	}

	id, ok := pathID(w, r)
	if !ok {
		return
	}

	var body struct {
		Field  string   `json:"field"`
		Values []string `json:"values"`
		Source string   `json:"source"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Field == "" {
		writeError(w, http.StatusBadRequest, "field required")
		return
	}
	if len(body.Values) == 0 {
		writeError(w, http.StatusBadRequest, "values required")
		return
	}

	cleaned := make([]string, 0, len(body.Values))
	for _, v := range body.Values {
		if s := enrich.SanitizeValue(v); s != "" {
			cleaned = append(cleaned, s)
		}
	}
	if len(cleaned) == 0 {
		writeError(w, http.StatusBadRequest, "values empty after sanitization")
		return
	}

	v, _, err := h.repo.GetVideo(r.Context(), id)
	if err != nil {
		h.videoLookupError(w, err)
		return
	}

	tagName, supported := writeback.TagForField(body.Field, v.Container)
	if !supported {
		writeError(w, http.StatusUnprocessableEntity,
			"field "+body.Field+" has no tag mapping for container "+v.Container)
		return
	}

	if err := h.writeback(r.Context(), v.FilePath, tagName, cleaned); err != nil {
		h.log.Warn("writeback failed", "id", id, "field", body.Field, "err", err)
		h.fail(w, "write to file", err)
		return
	}

	// Audit row — only on success (ADR-041 invariant: failed writes are silent here).
	joined := strings.Join(cleaned, "\n")
	if auditErr := h.repo.InsertWriteback(r.Context(), id, body.Field, tagName, joined, body.Source); auditErr != nil {
		h.log.Warn("insert writeback audit row", "id", id, "err", auditErr)
	}

	w.WriteHeader(http.StatusNoContent)
}
