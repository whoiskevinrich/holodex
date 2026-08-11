package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/fieldsource"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// mountDecisions registers the owner-gated per-field source-of-truth endpoints
// (F36, ADR-051 §7). Mounted inside the requireOwner group in Mount. Both routes are
// DB-only — they never touch the file (RD5); file tags change solely via the
// explicit "Write decisions to file" writeback action.
func (h *Handlers) mountDecisions(r chi.Router) {
	r.Put("/media/{id}/fields/{canonical}/decision", h.setFieldDecision)
	r.Delete("/media/{id}/fields/{canonical}/decision", h.clearFieldDecision)
}

// decisionBody is the PUT request shape: the pinned source and, for a manual pick,
// the frozen literal value. Override bypasses the title composite-key collision
// check (HOLODEX-270) — set only on a resubmit after the owner has already seen and
// dismissed a collision verdict for this exact edit.
type decisionBody struct {
	Source      string `json:"source"`
	ManualValue string `json:"manual_value"`
	Override    bool   `json:"override"`
}

// setFieldDecision records a standing decision pinning a replace field to a source
// (F36). It validates the source shape, the item's live status (404/409), that the
// canonical names a known replace field (404/400), and — for a provider pick — that
// the provider is currently matched (400). The manual literal is sanitized as
// untrusted input on the same path as an F30 manual add. Persisting the decision is
// DB-only; no file is written.
func (h *Handlers) setFieldDecision(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	canonical := chi.URLParam(r, "canonical")

	var body decisionBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if !fieldsource.Valid(body.Source) {
		writeError(w, http.StatusBadRequest, "source must be 'file', 'manual', or 'provider:<name>'")
		return
	}
	manualValue := ""
	if body.Source == fieldsource.Manual {
		if manualValue = enrich.SanitizeValue(body.ManualValue); manualValue == "" {
			writeError(w, http.StatusBadRequest, "manual_value required for a manual decision")
			return
		}
	}

	// Live-status gate: 404 unknown vs 409 soft-deleted (a decision must not
	// accumulate against a trashed item), before any field/provider validation.
	if !h.decisionTargetLive(w, r, id) {
		return
	}
	field, ok := h.replaceField(w, canonical)
	if !ok {
		return
	}
	if p := fieldsource.Provider(body.Source); p != "" && !h.providerMatched(r, model.EnrichEntityVideo, id, p) {
		writeError(w, http.StatusBadRequest, "provider is not matched to this item")
		return
	}

	// Title composite-key collision gate (HOLODEX-270): a manual title edit that
	// would produce a {title, people, date, studio} match against another active
	// video blocks here unless the owner already saw and overrode it.
	if field.Canonical == "title" && body.Source == fieldsource.Manual && !body.Override {
		collision, err := h.repo.FindTitleCollision(r.Context(), id, manualValue)
		if err != nil {
			h.fail(w, "check title collision", err)
			return
		}
		if collision != nil {
			writeCollisionConflict(w, collision)
			return
		}
	}

	// Studio composite-key collision gate (HOLODEX-271, reusing HOLODEX-270's
	// mechanism): unlike Title, this isn't manual-only — a known-candidate chip
	// pick changes the composite key exactly as much as a searched/created value
	// does, so every Studio source pick is checked.
	var studioRC *relinkContext
	var studioNames []string
	if field.Canonical == "studio" && !body.Override {
		var collision *repo.VideoCollision
		var err error
		collision, studioRC, studioNames, err = h.studioCollision(r.Context(), id, field, body.Source, manualValue)
		if err != nil {
			h.fail(w, "check studio collision", err)
			return
		}
		if collision != nil {
			writeCollisionConflict(w, collision)
			return
		}
	}

	if err := h.repo.SetDecision(r.Context(), model.EnrichEntityVideo, id, field.Canonical, body.Source, manualValue); err != nil {
		h.fail(w, "set decision", err)
		return
	}
	// An entity-typed field decision (studio, actors, director) moves the resolved
	// value → re-derive links (F38/F40). Studio reuses the fetch+resolve the
	// collision gate above already ran (same video, same pending decision) instead
	// of paying for a second loadRelinkContext + resolver.Resolve pass here.
	if field.Canonical == "studio" && studioRC != nil {
		h.relinkStudiosWithContext(r.Context(), id, studioRC, studioNames)
	} else {
		h.relinkIfEntity(r.Context(), id, field.Canonical)
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeCollisionConflict writes the shared 409 envelope for a composite-key
// collision hit, used by both the Title (HOLODEX-270) and Studio (HOLODEX-271) gates.
func writeCollisionConflict(w http.ResponseWriter, collision *repo.VideoCollision) {
	writeJSON(w, http.StatusConflict, map[string]any{
		"error":    "another video already matches this title, people, date, and studio",
		"conflict": collision,
	})
}

// studioCollision resolves the studio names that a proposed Studio decision
// (source/manualValue) would produce — mirroring relinkVideoStudios' own resolve,
// without persisting anything — then checks that proposed set for a composite-key
// collision (HOLODEX-271). Runs the same resolver.Resolve pass relinkVideoStudios
// runs after a decision commits, just with the pending decision substituted in first,
// so the collision check sees exactly what would end up linked. Also returns the
// fetched relinkContext and resolved names so the no-collision path can reconcile
// video_studios directly (relinkStudiosWithContext) instead of re-fetching and
// re-resolving from scratch.
func (h *Handlers) studioCollision(ctx context.Context, videoID int64, field mapping.Field, source, manualValue string) (*repo.VideoCollision, *relinkContext, []string, error) {
	rc, err := h.loadRelinkContext(ctx, videoID)
	if err != nil || rc == nil {
		return nil, rc, nil, err
	}
	decisions := decisionsFromRows(rc.decRows)
	if decisions == nil {
		decisions = resolver.Decisions{}
	}
	decisions["studio"] = resolver.Decision{Source: source, ManualValue: manualValue}
	resolved := resolver.Resolve(rc.video, rc.extra, enrichmentFromRows(rc.enrRows), curationFromRows(rc.curRows),
		[]mapping.Field{field}, h.resolveOptions(decisions))
	var names []string
	for _, rf := range resolved {
		if strings.EqualFold(rf.Canonical, "studio") {
			names = append(names, rf.Values...)
		}
	}
	if len(names) == 0 {
		return nil, rc, names, nil
	}
	collision, err := h.repo.FindStudioCollision(ctx, videoID, names)
	return collision, rc, names, err
}

// clearFieldDecision removes a field's standing decision, reverting it to the
// file-first default (F36). Clearing an undecided field is an idempotent no-op
// success. DB-only; no file is written.
func (h *Handlers) clearFieldDecision(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	canonical := chi.URLParam(r, "canonical")

	if !h.decisionTargetLive(w, r, id) {
		return
	}
	field, ok := h.replaceField(w, canonical)
	if !ok {
		return
	}
	if _, err := h.repo.ClearDecision(r.Context(), model.EnrichEntityVideo, id, field.Canonical); err != nil {
		h.fail(w, "clear decision", err)
		return
	}
	h.relinkIfEntity(r.Context(), id, field.Canonical)
	w.WriteHeader(http.StatusNoContent)
}

// decisionTargetLive reports whether the video is live, writing 404 (unknown) or
// 409 (soft-deleted) and returning false otherwise. It reuses RefreshTarget's
// missing-vs-soft-deleted distinction (the resolved path is unused here) so a
// decision, like a refresh, never accumulates against a trashed item.
func (h *Handlers) decisionTargetLive(w http.ResponseWriter, r *http.Request, id int64) bool {
	switch _, err := h.repo.RefreshTarget(r.Context(), id); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "media not found")
		return false
	case errors.Is(err, repo.ErrDeleted):
		writeError(w, http.StatusConflict, "item is deleted")
		return false
	case err != nil:
		h.fail(w, "get media", err)
		return false
	}
	return true
}

// replaceField resolves a canonical field name to its configured mapping.Field and
// confirms it is a replace (scalar) field — source decisions are replace-only (RD1).
// Writes 503 when no mappings are configured, 404 for an unknown field, and 400 for
// a merge field, returning ok=false in each case.
func (h *Handlers) replaceField(w http.ResponseWriter, canonical string) (mapping.Field, bool) {
	if h.mappings == nil {
		writeError(w, http.StatusServiceUnavailable, "field mapping unavailable")
		return mapping.Field{}, false
	}
	f, ok := h.mappings.Current().ByCanonical(canonical)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown field")
		return mapping.Field{}, false
	}
	if f.Multi || f.Merge {
		writeError(w, http.StatusBadRequest, "source decisions apply to replace fields only")
		return mapping.Field{}, false
	}
	return f, true
}

// providerMatched reports whether a provider currently has enrichment rows for the
// entity — the "matched provider" precondition for a provider decision (ADR-051 §1).
// Entity-generic: the video and person decision paths share it (F37).
func (h *Handlers) providerMatched(r *http.Request, entityType string, id int64, provider string) bool {
	rows, err := h.repo.EnrichmentForEntity(r.Context(), entityType, id)
	if err != nil {
		h.log.Warn("enrichment lookup for decision", "id", id, "err", err)
		return false
	}
	for _, row := range rows {
		if row.Provider == provider {
			return true
		}
	}
	return false
}
