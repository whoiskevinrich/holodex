package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/repo"
)

// Tag deny-list API (F50, ADR-075 D2) — the owner's /owner/tags "Deny-list" tab.
// A denied term is blocked from becoming a tag from any origin (scanner,
// manual attach, materialization), enforced once inside resolveOrCreateByName.
// Owner-gated (mounted in the requireOwner group, like mountDuplicates).

// mountTagDenylist registers the deny-list management routes: list, add, and
// remove-by-term. term is a ?term= query param, not a path segment — unlike
// serveProviderIcon's {name} (drawn from a small config-declared allowlist),
// a denied term is arbitrary owner-entered text that could contain characters
// (e.g. "/") a raw path segment can't round-trip.
func (h *Handlers) mountTagDenylist(r chi.Router) {
	r.Get("/owner/tags/denylist", h.listDeniedTags)
	r.Post("/owner/tags/denylist", h.denyTag)
	r.Delete("/owner/tags/denylist", h.removeDeniedTag)
}

// listDeniedTags returns every denied term, newest first.
func (h *Handlers) listDeniedTags(w http.ResponseWriter, r *http.Request) {
	terms, err := h.repo.ListDeniedTags(r.Context())
	if err != nil {
		h.fail(w, "list denied tags", err)
		return
	}
	if terms == nil {
		terms = []repo.DeniedTag{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"terms": terms})
}

// denyTag adds a term to the deny-list (idempotent: re-denying is a no-op).
func (h *Handlers) denyTag(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Term string `json:"term"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	term := strings.TrimSpace(body.Term)
	if term == "" {
		writeError(w, http.StatusBadRequest, "term is required")
		return
	}
	if len([]rune(term)) > maxAliasLen {
		writeError(w, http.StatusBadRequest, "term is too long")
		return
	}
	if err := h.repo.DenyTag(r.Context(), term); err != nil {
		h.fail(w, "deny tag", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// removeDeniedTag removes a term from the deny-list. 404 when the term isn't
// currently denied.
func (h *Handlers) removeDeniedTag(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(r.URL.Query().Get("term"))
	if term == "" {
		writeError(w, http.StatusBadRequest, "term is required")
		return
	}
	switch err := h.repo.RemoveDeniedTag(r.Context(), term); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "term not denied")
		return
	case err != nil:
		h.fail(w, "remove denied tag", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
