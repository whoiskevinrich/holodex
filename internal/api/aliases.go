package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/repo"
)

// maxAliasLen bounds a stored alias (F23.1) — generous for any real name while
// keeping the row and the FTS term sane.
const maxAliasLen = 200

// mountAliases registers the owner-gated person-alias mutations (F23, ADR-036).
// Mounted inside the requireOwner group in Mount; the alias list itself is served
// (ungated) as part of GET /people/{id}.
func (h *Handlers) mountAliases(r chi.Router) {
	r.Post("/people/{id}/aliases", h.addAlias)
	r.Delete("/people/{id}/aliases/{aliasId}", h.deleteAlias)
	r.Post("/people/{id}/merge", h.mergePerson)
}

// addAlias adds an alias to a person and returns the person's full alias list
// (F23.2). Idempotent: re-adding an existing alias is a no-op success.
func (h *Handlers) addAlias(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Alias string `json:"alias"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	alias := strings.TrimSpace(body.Alias)
	if alias == "" {
		writeError(w, http.StatusBadRequest, "alias is required")
		return
	}
	if len([]rune(alias)) > maxAliasLen {
		writeError(w, http.StatusBadRequest, "alias is too long")
		return
	}
	if err := h.repo.PersonExists(r.Context(), id); err != nil {
		h.personLookupError(w, err)
		return
	}
	// Never silently merge same-named people (homonyms exist). If the alias already
	// names another person, surface that person so the owner can decide: merge or
	// keep separate (F23, ADR-036).
	conflict, err := h.repo.PersonConflict(r.Context(), id, alias)
	if err != nil {
		h.fail(w, "alias conflict check", err)
		return
	}
	if conflict != nil {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":    "that name already belongs to another person",
			"conflict": conflict,
		})
		return
	}
	if _, err := h.repo.AddPersonAlias(r.Context(), id, alias); err != nil {
		h.fail(w, "add alias", err)
		return
	}
	aliases, err := h.repo.AliasesForPerson(r.Context(), id)
	if err != nil {
		h.fail(w, "list aliases", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"aliases": aliases})
}

// deleteAlias removes one alias from a person (F23.3). 404 when the alias does not
// belong to the person.
func (h *Handlers) deleteAlias(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	aliasID, err := strconv.ParseInt(chi.URLParam(r, "aliasId"), 10, 64)
	if err != nil || aliasID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid alias id")
		return
	}
	switch err := h.repo.DeletePersonAlias(r.Context(), id, aliasID); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "alias not found")
		return
	case err != nil:
		h.fail(w, "delete alias", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// mergePerson folds another person into this one (F23, ADR-036). The path person is
// the canonical (surviving) record; body.from_id is absorbed: its videos move here,
// its name becomes an alias here, and it is deleted. Owner-gated; an explicit,
// owner-confirmed action. Returns the updated canonical person.
func (h *Handlers) mergePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		FromID int64 `json:"from_id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.FromID <= 0 {
		writeError(w, http.StatusBadRequest, "from_id required")
		return
	}
	if body.FromID == id {
		writeError(w, http.StatusBadRequest, "cannot merge a person into itself")
		return
	}
	switch err := h.repo.MergePersons(r.Context(), id, body.FromID); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "person not found")
		return
	case err != nil:
		h.fail(w, "merge person", err)
		return
	}
	p, err := h.repo.GetPerson(r.Context(), id)
	if err != nil {
		h.fail(w, "get merged person", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"person": p})
}
