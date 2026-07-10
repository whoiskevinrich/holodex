package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/fieldsource"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/repo"
)

// mountPersonDecisions registers the owner-gated person source-of-truth
// surface (F37, RD7): per-field decisions and value-level curation mirroring
// the media endpoints, plus the identity rename (RD1). Mounted inside the
// requireOwner group in Mount. Decisions/curation are DB-only; rename is the
// one identity mutation — it feeds search FTS and scan routing via the F23
// alias, never a decision row.
func (h *Handlers) mountPersonDecisions(r chi.Router) {
	r.Put("/people/{id}/fields/{canonical}/decision", h.setPersonFieldDecision)
	r.Delete("/people/{id}/fields/{canonical}/decision", h.clearPersonFieldDecision)
	r.Post("/people/{id}/curation", h.setPersonCuration)
	r.Post("/people/{id}/curation/clear", h.clearPersonCuration)
	r.Post("/people/{id}/rename", h.renamePerson)
}

// setPersonFieldDecision records a standing decision pinning a person replace
// field to a source (F37 P0-3). The payload vocabulary is record | manual |
// provider:<name> (RD4 — "record" stores the internal "file" token); name is
// rejected (RD1 — identity materializes via rename, it never pins); a provider
// pick must be currently matched. DB-only, like the media path.
func (h *Handlers) setPersonFieldDecision(w http.ResponseWriter, r *http.Request) {
	id, field, ok := h.personDecisionTarget(w, r)
	if !ok {
		return
	}
	var body decisionBody
	if !decodeJSON(w, r, &body) {
		return
	}
	source, ok := personDecisionSource(body.Source)
	if !ok {
		writeError(w, http.StatusBadRequest, "source must be 'record', 'manual', or 'provider:<name>'")
		return
	}
	manualValue := ""
	if source == fieldsource.Manual {
		if manualValue = enrich.SanitizeValue(body.ManualValue); manualValue == "" {
			writeError(w, http.StatusBadRequest, "manual_value required for a manual decision")
			return
		}
	}
	if p := fieldsource.Provider(source); p != "" && !h.providerMatched(r, model.EnrichEntityPerson, id, p) {
		writeError(w, http.StatusBadRequest, "provider is not matched to this person")
		return
	}

	if err := h.repo.SetDecision(r.Context(), model.EnrichEntityPerson, id, field.Canonical, source, manualValue); err != nil {
		h.fail(w, "set person decision", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clearPersonFieldDecision removes a person field's standing decision,
// reverting it to the record-first default. Clearing an undecided field is an
// idempotent no-op success, as on the media path.
func (h *Handlers) clearPersonFieldDecision(w http.ResponseWriter, r *http.Request) {
	id, field, ok := h.personDecisionTarget(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.ClearDecision(r.Context(), model.EnrichEntityPerson, id, field.Canonical); err != nil {
		h.fail(w, "clear person decision", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// personDecisionTarget validates the {id}/{canonical} pair shared by the
// decision handlers: the person must exist (404) and the canonical must name a
// decidable replace field (404 unknown, 400 name/merge).
func (h *Handlers) personDecisionTarget(w http.ResponseWriter, r *http.Request) (id int64, field mapping.Field, ok bool) {
	id, ok = pathID(w, r)
	if !ok {
		return 0, mapping.Field{}, false
	}
	if err := h.repo.PersonExists(r.Context(), id); err != nil {
		h.personLookupError(w, err)
		return 0, mapping.Field{}, false
	}
	field, ok = h.personReplaceField(w, chi.URLParam(r, "canonical"))
	return id, field, ok
}

// personReplaceField resolves a canonical name against the synthesized person
// schema and confirms a decision may target it: computed → 400 (F45, ADR-063 §D3 —
// a derived field is never adoptable), unknown → 404, name → 400 (RD1 — no decision
// row ever exists for name), merge (aliases) → 400.
func (h *Handlers) personReplaceField(w http.ResponseWriter, canonical string) (mapping.Field, bool) {
	// A computed field is source-less and read-only: reject a pin explicitly rather
	// than relying on it being absent from the synthesized schema (structural guard).
	if registry.Lookup(canonical).Computed {
		writeError(w, http.StatusBadRequest, "computed fields are read-only and cannot be pinned")
		return mapping.Field{}, false
	}
	f, ok := personFieldByCanonical(canonical)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown field")
		return mapping.Field{}, false
	}
	if f.Canonical == "name" {
		writeError(w, http.StatusBadRequest, "name has no source decision; rename the person instead")
		return mapping.Field{}, false
	}
	if f.Multi {
		writeError(w, http.StatusBadRequest, "source decisions apply to replace fields only")
		return mapping.Field{}, false
	}
	return f, true
}

// setPersonCuration records one value-level decision for a person field (F37
// P0-4 — the aliases merge field's add/suppress/nowrite semantics). Mirrors
// the media handler; nowrite is accepted-but-moot (a person has no writeback).
func (h *Handlers) setPersonCuration(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body curationBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if msg := validateCurationBody(body); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	value := enrich.SanitizeValue(body.Value)
	if value == "" {
		writeError(w, http.StatusBadRequest, "value required")
		return
	}
	// Confirm the person exists so curation can't accumulate against unknown ids.
	if err := h.repo.PersonExists(r.Context(), id); err != nil {
		h.personLookupError(w, err)
		return
	}
	if err := h.repo.SetCuration(r.Context(), model.EnrichEntityPerson, id, body.Field, value, body.Action); err != nil {
		h.fail(w, "set person curation", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clearPersonCuration removes one decision so the underlying source value is
// restored. A clear of a non-existent decision is a no-op success (idempotent).
func (h *Handlers) clearPersonCuration(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body curationBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if msg := validateCurationBody(body); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if err := h.repo.PersonExists(r.Context(), id); err != nil {
		h.personLookupError(w, err)
		return
	}
	if _, err := h.repo.ClearCuration(r.Context(), model.EnrichEntityPerson, id, body.Field, body.Value, body.Action); err != nil {
		h.fail(w, "clear person curation", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// renamePerson is the RD1 identity mutation (F37 P0-5): one transaction sets
// people.name and keeps the old name as an F23 alias, so display, search, and
// scan routing remain one identity. A collision with another person's name is
// a 409 carrying that person (the F23 conflict shape) with no mutation — the
// owner routes through the explicit merge flow, never an auto-merge. Renaming
// to the current name is a no-op success.
func (h *Handlers) renamePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	// The old name becomes an alias, so it must satisfy the alias bound too.
	if len([]rune(name)) > maxAliasLen {
		writeError(w, http.StatusBadRequest, "name is too long")
		return
	}
	conflictID, err := h.repo.RenamePerson(r.Context(), id, name)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "person not found")
		return
	case errors.Is(err, repo.ErrNameTaken):
		conflict, cerr := h.repo.GetPerson(r.Context(), conflictID)
		if cerr != nil {
			h.fail(w, "load rename conflict", cerr)
			return
		}
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":    "that name already belongs to another person",
			"conflict": conflict,
		})
		return
	case err != nil:
		h.fail(w, "rename person", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
