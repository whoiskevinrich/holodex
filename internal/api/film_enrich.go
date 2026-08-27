package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/model"
)

// Film enrichment (F56/ADR-086): entity_type "film" gets its own resolve/apply/clear
// trio, mirroring enrichStudioResolve/Apply/Clear (enrich.go) exactly — a film's
// enrichment lifecycle is independent of any attached video (ADR-086 §1), so it is
// wired the same way Studio was (ADR-079), not derived from video enrichment.

// filmEnrichResolve searches a provider for film candidates matching a film.
// Mirrors enrichStudioResolve; nothing is applied here.
func (h *Handlers) filmEnrichResolve(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.repo.GetFilm(r.Context(), id); err != nil {
		h.filmLookupError(w, err)
		return
	}
	if !h.enrichDismissedCheck(w, r, model.EnrichEntityFilm, id, body.Provider) {
		return
	}
	cands, err := h.enrich.Resolve(r.Context(), body.Provider, model.EnrichEntityFilm, enrich.Hint{Query: body.Query})
	if err != nil {
		h.log.Warn("film enrich resolve failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "provider lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": cands})
}

// filmEnrichApply fetches and stores provider enrichment for a film. Unlike the video
// path there is no relink: a film-entity enrich changes only the film's own resolved
// fields (description/release_date/poster), never a video → film link (film_videos is
// an owner-asserted attachment, ADR-085 §2, with no reconciler at all).
func (h *Handlers) filmEnrichApply(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.repo.GetFilm(r.Context(), id); err != nil {
		h.filmLookupError(w, err)
		return
	}
	fields, err := h.enrich.Enrich(r.Context(), model.EnrichEntityFilm, id, body.Provider, body.ExternalID, h.auth.authorized(r))
	if err != nil {
		h.log.Warn("film enrich apply failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "enrichment failed")
		return
	}
	// A provider's poster arrives as an image asset and is already stored by Enrich's
	// entity-generic downloadAssets (F51/ADR-079, widened to Film by ADR-086) — no
	// separate relink step, same as studio's logo.
	writeJSON(w, http.StatusOK, map[string]any{"enriched": fields})
}

// filmEnrichClear removes a provider's contribution for a film.
func (h *Handlers) filmEnrichClear(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	provider := chi.URLParam(r, "provider")
	if err := h.enrich.Clear(r.Context(), model.EnrichEntityFilm, id, provider); err != nil {
		h.fail(w, "clear film enrichment", err)
		return
	}
	// Clearing a provider's shadow fields does not touch already-stored film_images
	// rows — mirrors studio enrich Clear, which likewise never deletes downloaded
	// images. The owner removes an image explicitly via the image endpoints.
	w.WriteHeader(http.StatusNoContent)
}
