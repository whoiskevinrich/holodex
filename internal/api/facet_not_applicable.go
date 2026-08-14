package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/registry"
)

// mountFacetNotApplicable registers the owner-gated not-applicable endpoints
// for video facets (F55, ADR-081 D2). v1's only UI target is
// external_provider_id (spec § Out of scope), but the mutation validates
// against the full registry so widening the affordance to more facets later
// (F55.16) needs no backend change.
func (h *Handlers) mountFacetNotApplicable(r chi.Router) {
	r.Put("/media/{id}/fields/{canonical}/not-applicable", h.setFacetNotApplicable)
	r.Delete("/media/{id}/fields/{canonical}/not-applicable", h.clearFacetNotApplicable)
}

// setFacetNotApplicable marks a video facet not-applicable (F55.10) —
// excluded from that video's completeness score and the remediation queue on
// the next read, independent of any standing field_source_decisions row for
// the same field (ADR-081 D2).
func (h *Handlers) setFacetNotApplicable(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	canonical := chi.URLParam(r, "canonical")
	if !registry.IsKnown(canonical) {
		writeError(w, http.StatusNotFound, "unknown field")
		return
	}
	if !h.decisionTargetLive(w, r, id) {
		return
	}
	if err := h.repo.SetFacetNotApplicable(r.Context(), model.EnrichEntityVideo, id, canonical); err != nil {
		h.fail(w, "set facet not applicable", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clearFacetNotApplicable removes the not-applicable exclusion, restoring the
// facet to normal scoring. Clearing an unmarked facet is an idempotent no-op
// success.
func (h *Handlers) clearFacetNotApplicable(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	canonical := chi.URLParam(r, "canonical")
	if !registry.IsKnown(canonical) {
		writeError(w, http.StatusNotFound, "unknown field")
		return
	}
	if !h.decisionTargetLive(w, r, id) {
		return
	}
	if _, err := h.repo.ClearFacetNotApplicable(r.Context(), model.EnrichEntityVideo, id, canonical); err != nil {
		h.fail(w, "clear facet not applicable", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
