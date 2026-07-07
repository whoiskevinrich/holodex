package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Near-miss review queue API (F43 S5, ADR-061 D4) — the owner's Duplicates tab. Reads
// the queue seeded at boot + flagged at scan time; dismissing a pair records a durable
// keep-separate marker so the detector never re-proposes it. Resolving a pair uses the
// per-entity merge endpoint (S2), not this surface. Owner-gated (mounted in the
// requireOwner group).

// mountDuplicates registers the review-queue routes + the editor near-miss lookup
// (studio + tag; person keeps its own F23/F37 flows).
func (h *Handlers) mountDuplicates(r chi.Router) {
	r.Get("/owner/duplicates", h.listDuplicates)
	r.Post("/owner/duplicates/dismiss", h.dismissDuplicate)
	r.Get("/studios/{id}/near-miss", h.entityNearMiss(model.EnrichEntityStudio))
	r.Get("/tags/{id}/near-miss", h.entityNearMiss(model.EntityTag))
}

// entityNearMiss backs the editor's non-blocking soft-warning (F43 P1-5): given a
// candidate ?name=, return the fuzzy near-miss entity (loose-key match, not an exact
// collision, not kept-separate) or null. Owner-gated; a read with no side effect.
func (h *Handlers) entityNearMiss(entityType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			writeJSON(w, http.StatusOK, map[string]any{"near_miss": nil})
			return
		}
		ref, err := h.repo.NearMiss(r.Context(), entityType, id, name)
		if err != nil {
			h.fail(w, entityType+" near-miss", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"near_miss": ref})
	}
}

// listDuplicates returns every flagged possible-duplicate pair, grouped tags-first,
// each with both entities' names + active-video counts + the variation kind.
func (h *Handlers) listDuplicates(w http.ResponseWriter, r *http.Request) {
	pairs, err := h.repo.ListReviewPairs(r.Context())
	if err != nil {
		h.fail(w, "list duplicates", err)
		return
	}
	if pairs == nil {
		pairs = []repo.ReviewPair{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"pairs": pairs})
}

// dismissDuplicate records that a pair is deliberately distinct (keep-separate, RD5)
// and removes it from the queue — the owner's "these are not the same" verdict.
func (h *Handlers) dismissDuplicate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EntityType string `json:"entity_type"`
		IDA        int64  `json:"id_a"`
		IDB        int64  `json:"id_b"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !validEntityType(body.EntityType) {
		writeError(w, http.StatusBadRequest, "entity_type must be person, studio, or tag")
		return
	}
	if body.IDA <= 0 || body.IDB <= 0 || body.IDA == body.IDB {
		writeError(w, http.StatusBadRequest, "id_a and id_b must be two distinct ids")
		return
	}
	if err := h.repo.DismissReviewPair(r.Context(), body.EntityType, body.IDA, body.IDB); err != nil {
		h.fail(w, "dismiss duplicate", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validEntityType guards the untrusted entity_type against the three named entities —
// the same set canonicalTable accepts, so a bad value can't reach a repo query.
func validEntityType(t string) bool {
	switch t {
	case model.EnrichEntityPerson, model.EnrichEntityStudio, model.EntityTag:
		return true
	default:
		return false
	}
}
