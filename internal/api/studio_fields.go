package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/fieldsource"
	"holodex/internal/mapping"
	"holodex/internal/model"
)

// mountStudioDecisions registers the owner-gated studio source-of-truth surface
// (F38, RD5): per-field decisions and value-level curation mirroring the person
// endpoints. Mounted inside the requireOwner group. All DB-only — a studio has no
// file, so there is no writeback and no rename (RD4: studio names are derived
// identity, corrected by editing the underlying video field, not renamed here).
// After a decision/curation change the affected studio's video links are unchanged;
// it is the *video's* studio field that drives links, relinked on the media path.
func (h *Handlers) mountStudioDecisions(r chi.Router) {
	r.Put("/studios/{id}/fields/{canonical}/decision", h.setStudioFieldDecision)
	r.Delete("/studios/{id}/fields/{canonical}/decision", h.clearStudioFieldDecision)
	r.Post("/studios/{id}/curation", h.setStudioCuration)
	r.Post("/studios/{id}/curation/clear", h.clearStudioCuration)
}

// setStudioFieldDecision records a standing decision pinning a studio replace field
// to a source (F38). Payload vocabulary is record | manual | provider:<name> (RD5 —
// "record" stores the internal "file" token); name is rejected (read-only identity);
// a provider pick must be currently matched. DB-only.
func (h *Handlers) setStudioFieldDecision(w http.ResponseWriter, r *http.Request) {
	id, field, ok := h.studioDecisionTarget(w, r)
	if !ok {
		return
	}
	var body decisionBody
	if !decodeJSON(w, r, &body) {
		return
	}
	source, ok := recordDecisionSource(body.Source)
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
	if p := fieldsource.Provider(source); p != "" && !h.providerMatched(r.Context(), model.EnrichEntityStudio, id, p) {
		writeError(w, http.StatusBadRequest, "provider is not matched to this studio")
		return
	}

	if err := h.repo.SetDecision(r.Context(), model.EnrichEntityStudio, id, field.Canonical, source, manualValue); err != nil {
		h.fail(w, "set studio decision", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clearStudioFieldDecision removes a studio field's standing decision, reverting it
// to the record-first default. Clearing an undecided field is an idempotent no-op.
func (h *Handlers) clearStudioFieldDecision(w http.ResponseWriter, r *http.Request) {
	id, field, ok := h.studioDecisionTarget(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.ClearDecision(r.Context(), model.EnrichEntityStudio, id, field.Canonical); err != nil {
		h.fail(w, "clear studio decision", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// studioDecisionTarget validates the {id}/{canonical} pair: the studio must exist
// (404) and the canonical must name a decidable replace field (404 unknown, 400 name).
func (h *Handlers) studioDecisionTarget(w http.ResponseWriter, r *http.Request) (id int64, field mapping.Field, ok bool) {
	id, ok = pathID(w, r)
	if !ok {
		return 0, mapping.Field{}, false
	}
	if _, err := h.repo.GetStudio(r.Context(), id); err != nil {
		h.studioLookupError(w, err)
		return 0, mapping.Field{}, false
	}
	field, ok = h.studioReplaceField(w, chi.URLParam(r, "canonical"))
	return id, field, ok
}

// studioReplaceField resolves a canonical name against the synthesized studio schema
// and confirms a decision may target it: unknown → 404, name → 400 (read-only
// identity, RD5), merge → 400 (no merge fields in v1, but guarded for parity).
func (h *Handlers) studioReplaceField(w http.ResponseWriter, canonical string) (mapping.Field, bool) {
	f, ok := studioFieldByCanonical(canonical)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown field")
		return mapping.Field{}, false
	}
	if f.Canonical == "name" {
		writeError(w, http.StatusBadRequest, "studio name is read-only; edit the studio field on its videos instead")
		return mapping.Field{}, false
	}
	if f.Multi {
		writeError(w, http.StatusBadRequest, "source decisions apply to replace fields only")
		return mapping.Field{}, false
	}
	return f, true
}

// setStudioCuration records one value-level decision for a studio field (F38).
// Mirrors the person handler; nowrite is accepted-but-moot (a studio has no writeback).
func (h *Handlers) setStudioCuration(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.repo.GetStudio(r.Context(), id); err != nil {
		h.studioLookupError(w, err)
		return
	}
	if err := h.repo.SetCuration(r.Context(), model.EnrichEntityStudio, id, body.Field, value, body.Action); err != nil {
		h.fail(w, "set studio curation", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clearStudioCuration removes one value-level decision (idempotent no-op if absent).
func (h *Handlers) clearStudioCuration(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.repo.GetStudio(r.Context(), id); err != nil {
		h.studioLookupError(w, err)
		return
	}
	if _, err := h.repo.ClearCuration(r.Context(), model.EnrichEntityStudio, id, body.Field, body.Value, body.Action); err != nil {
		h.fail(w, "clear studio curation", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
