package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// imdbPathRe extracts an IMDb ID from a Plex/Jellyfin-style path component like
// "{imdb-tt1160419}" so it can be forwarded as an external-ID hint to providers.
var imdbPathRe = regexp.MustCompile(`\{imdb-(tt\d+)\}`)

// mountEnrich registers the owner-gated enrichment endpoints (F22.5/F22.9a). They
// are mounted inside the requireOwner group set up in Mount.
func (h *Handlers) mountEnrich(r chi.Router) {
	r.Get("/enrich/sources", h.enrichSources)
	r.Post("/people/{id}/enrich/resolve", h.enrichResolve)
	r.Post("/people/{id}/enrich", h.enrichApply)
	r.Delete("/people/{id}/enrich/{provider}", h.enrichClear)
	r.Post("/media/{id}/enrich/resolve", h.enrichVideoResolve)
	r.Post("/media/{id}/enrich", h.enrichVideoApply)
	r.Delete("/media/{id}/enrich/{provider}", h.enrichVideoClear)
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

func (h *Handlers) personLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "person not found")
		return
	}
	h.fail(w, "get person", err)
}

// enrichVideoResolve searches a provider for film candidates matching a video (F26).
func (h *Handlers) enrichVideoResolve(w http.ResponseWriter, r *http.Request) {
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
	v, _, err := h.repo.GetVideo(r.Context(), id)
	if err != nil {
		h.videoLookupError(w, err)
		return
	}
	hint := enrich.Hint{Query: body.Query}
	if m := imdbPathRe.FindStringSubmatch(v.FilePath); m != nil {
		hint.ExternalIDs = []string{"imdb:" + m[1]}
	}
	cands, err := h.enrich.Resolve(r.Context(), body.Provider, model.EnrichEntityVideo, hint)
	if err != nil {
		h.log.Warn("video enrich resolve failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "provider lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": cands})
}

// enrichVideoApply fetches and stores film enrichment for a video (F26).
func (h *Handlers) enrichVideoApply(w http.ResponseWriter, r *http.Request) {
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
	if _, _, err := h.repo.GetVideo(r.Context(), id); err != nil {
		h.videoLookupError(w, err)
		return
	}
	fields, err := h.enrich.Enrich(r.Context(), model.EnrichEntityVideo, id, body.Provider, body.ExternalID)
	if err != nil {
		h.log.Warn("video enrich apply failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "enrichment failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enriched": fields})
}

// enrichVideoClear removes a provider's contribution for a video (F26).
func (h *Handlers) enrichVideoClear(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	provider := chi.URLParam(r, "provider")
	if err := h.enrich.Clear(r.Context(), model.EnrichEntityVideo, id, provider); err != nil {
		h.fail(w, "clear video enrichment", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// videoEnrichment reads a video's stored enrichment for the detail view.
// Returns nil when no enrichment service is wired or no rows exist.
func (h *Handlers) videoEnrichment(r *http.Request, id int64) []model.EnrichedField {
	if h.enrich == nil {
		return nil
	}
	fields, err := h.enrich.Fields(r.Context(), model.EnrichEntityVideo, id)
	if err != nil {
		h.log.Warn("read video enrichment", "id", id, "err", err)
		return nil
	}
	return fields
}

func (h *Handlers) videoLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "media not found")
		return
	}
	h.fail(w, "get media", err)
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
