package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// mountEnrich registers the owner-gated enrichment endpoints (F22.5/F22.9a). They
// are mounted inside the requireOwner group set up in Mount.
func (h *Handlers) mountEnrich(r chi.Router) {
	r.Get("/enrich/sources", h.enrichSources)
	r.Post("/people/{id}/enrich/resolve", h.enrichResolve)
	r.Post("/people/{id}/enrich", h.enrichApply)
	r.Delete("/people/{id}/enrich/{provider}", h.enrichClear)
}

func (h *Handlers) enrichSources(w http.ResponseWriter, _ *http.Request) {
	if h.enrich == nil {
		writeJSON(w, http.StatusOK, map[string]any{"sources": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": h.enrich.Sources()})
}

// enrichResolve runs provider name-search for a person and returns candidates for
// the owner to confirm (F22.5b). Nothing is applied here.
func (h *Handlers) enrichResolve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	var body struct {
		Provider string `json:"provider"`
		Query    string `json:"query"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if _, err := h.repo.GetPerson(r.Context(), id); err != nil {
		h.personLookupError(w, err)
		return
	}
	cands, err := h.enrich.Resolve(r.Context(), body.Provider, model.EnrichEntityPerson, enrich.Hint{Query: body.Query})
	if err != nil {
		h.log.Warn("enrich resolve failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "provider lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": cands})
}

// enrichApply fetches the chosen record and stores it in the shadow layer,
// returning the person's resolved fields with provenance (F22.5/F22.7).
func (h *Handlers) enrichApply(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	var body struct {
		Provider   string `json:"provider"`
		ExternalID string `json:"external_id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.ExternalID == "" {
		writeError(w, http.StatusBadRequest, "external_id required")
		return
	}
	if _, err := h.repo.GetPerson(r.Context(), id); err != nil {
		h.personLookupError(w, err)
		return
	}
	fields, err := h.enrich.Enrich(r.Context(), model.EnrichEntityPerson, id, body.Provider, body.ExternalID)
	if err != nil {
		h.log.Warn("enrich apply failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "enrichment failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enriched": fields})
}

// enrichClear removes a provider's contribution for a person (F22.7b).
func (h *Handlers) enrichClear(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	provider := chi.URLParam(r, "provider")
	if err := h.enrich.Clear(r.Context(), model.EnrichEntityPerson, id, provider); err != nil {
		h.fail(w, "clear enrichment", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// personEnrichment reads a person's stored enrichment for the (ungated) person
// detail view — enriched values are public metadata; only the controls are gated.
// Returns nil when no enrichment service is wired.
func (h *Handlers) personEnrichment(r *http.Request, id int64) []model.EnrichedField {
	if h.enrich == nil {
		return nil
	}
	fields, err := h.enrich.Fields(r.Context(), model.EnrichEntityPerson, id)
	if err != nil {
		h.log.Warn("read person enrichment", "id", id, "err", err)
		return nil
	}
	return fields
}

func (h *Handlers) personLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "person not found")
		return
	}
	h.fail(w, "get person", err)
}

// decodeJSON reads a small JSON request body, writing a 400 on malformed input.
func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	if err := dec.Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
