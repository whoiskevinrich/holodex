package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/rename"
	"holodex/internal/settings"
)

// SetSettings wires the owner-settable settings allowlist. Nil leaves the settings routes 503.
func (h *Handlers) SetSettings(s *settings.Store) { h.settings = s }

// mountSettings registers the generic settings routes inside the owner group. Writes go through
// settings.Store.Put, so every key's validator runs on this path as on any other.
func (h *Handlers) mountSettings(r chi.Router) {
	r.Get("/admin/settings", h.getSettings)
	r.Put("/admin/settings/{key}", h.putSetting)
}

func (h *Handlers) getSettings(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		writeError(w, http.StatusServiceUnavailable, "settings not configured")
		return
	}
	writeJSON(w, http.StatusOK, h.settings.All())
}

// settingWrite is the body of a settings write: the value is always a string, as stored.
type settingWrite struct {
	Value string `json:"value"`
}

func (h *Handlers) putSetting(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		writeError(w, http.StatusServiceUnavailable, "settings not configured")
		return
	}
	var body settingWrite
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	key := chi.URLParam(r, "key")
	err := h.settings.Put(r.Context(), key, body.Value)
	var tle *rename.TemplateListError
	var cte *rename.CharacterTableError
	var inv *settings.InvalidError
	switch {
	case errors.Is(err, settings.ErrUnknownKey):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.As(err, &tle):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": tle.Error(), "rows": tle.Rows})
	case errors.As(err, &cte):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": cte.Error(), "characters": cte.Rows})
	case errors.As(err, &inv):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case err != nil:
		h.fail(w, "put setting", err)
	default:
		writeJSON(w, http.StatusOK, map[string]string{"key": key, "value": h.settings.Get(key)})
	}
}
