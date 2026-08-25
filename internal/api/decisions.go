package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
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
	// video blocks here unless the owner already saw and overrode it. The check and
	// the write happen as one writeMu-locked operation (SetDecisionChecked) so two
	// concurrent title edits can't both pass the check before either commits.
	if field.Canonical == "title" && body.Source == fieldsource.Manual {
		var check func() (*repo.VideoCollision, error)
		if !body.Override {
			check = func() (*repo.VideoCollision, error) { return h.repo.FindTitleCollision(r.Context(), id, manualValue) }
		}
		collision, err := h.repo.SetDecisionChecked(r.Context(), model.EnrichEntityVideo, id, field.Canonical, body.Source, manualValue, check)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				writeError(w, http.StatusNotFound, "media not found")
				return
			}
			h.fail(w, "check title collision", err)
			return
		}
		if collision != nil {
			writeCollisionConflict(w, collision)
			return
		}
		h.relinkIfEntity(r.Context(), id, field.Canonical)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Studio composite-key collision gate (HOLODEX-271, reusing HOLODEX-270's
	// mechanism): see decideStudioForVideo's doc comment for the full TOCTOU
	// rationale. This is a thin wrapper — override is honored from the request body,
	// matching the single-video path's existing behavior (ADR-086 D1 extraction).
	if field.Canonical == "studio" {
		_, collision, err := h.decideStudioForVideo(r.Context(), id, field, body.Source, manualValue, body.Override)
		if err != nil {
			h.fail(w, "set studio decision", err)
			return
		}
		if collision != nil {
			writeCollisionConflict(w, collision)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.repo.SetDecision(r.Context(), model.EnrichEntityVideo, id, field.Canonical, body.Source, manualValue); err != nil {
		h.fail(w, "set decision", err)
		return
	}
	h.relinkIfEntity(r.Context(), id, field.Canonical)
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

// resolveProposedStudioNames resolves the studio names that a proposed Studio
// decision (source/manualValue) would produce — mirroring relinkVideoStudios' own
// resolve (resolveStudioNames), without persisting or collision-checking anything
// (HOLODEX-271). An empty resolved set (e.g. picking the baseline chip when the file
// has no studio) is a legitimate proposal, not an error — the caller still runs it
// through FindStudioCollision, since two videos that both resolve to no studio,
// matching on every other axis, is a real collision. Also returns the fetched
// relinkContext so the no-collision path can reconcile video_studios directly
// (relinkStudiosWithContext) instead of re-fetching and re-resolving from scratch.
func (h *Handlers) resolveProposedStudioNames(ctx context.Context, videoID int64, field mapping.Field, source, manualValue string) (*relinkContext, []string, error) {
	rc, err := h.loadRelinkContext(ctx, videoID)
	if err != nil || rc == nil {
		return rc, nil, err
	}
	decisions := decisionsFromRows(rc.decRows)
	if decisions == nil {
		decisions = resolver.Decisions{}
	}
	decisions["studio"] = resolver.Decision{Source: source, ManualValue: manualValue}
	return rc, h.resolveStudioNames(rc, field, decisions), nil
}

// decideStudioForVideo runs the Studio composite-key collision gate (HOLODEX-271)
// and SetDecisionChecked for one video, then relinks video_studios on success.
// Shared by setFieldDecision's Studio branch (single video, override honored) and
// the film-studio cascade (film_studio_cascade.go, override always false — RD4's
// unconditional overwrite applies to a video's prior decision, not to this safety
// gate). Extracted verbatim from setFieldDecision's former Studio branch (ADR-086
// D1) — no behavior change for the single-video path.
//
// Unlike Title, this isn't manual-only — a known-candidate chip pick changes the
// composite key exactly as much as a searched/created value does, so every
// non-override Studio source pick is checked, including one that resolves to no
// studio at all (two videos that both drop their studio, matching on every other
// axis, is still a real composite-key collision). The proposed names are resolved
// once, unlocked, and checked immediately so the common blocking case (and every
// override write) never touches the write lock at all; SetDecisionChecked then
// re-runs the same cheap FindStudioCollision query inside one writeMu-locked
// operation right before the write, closing the TOCTOU gap two concurrent Studio
// decisions could otherwise race through — without holding the lock across the
// earlier fetch+resolve pass the way a single locked closure would. (Title's own
// FindTitleCollision-then-SetDecision path has the same unlocked race and isn't
// fixed here — pre-existing, out of scope.)
func (h *Handlers) decideStudioForVideo(ctx context.Context, videoID int64, field mapping.Field, source, manualValue string, override bool) (names []string, collision *repo.VideoCollision, err error) {
	rc, names, err := h.resolveProposedStudioNames(ctx, videoID, field, source, manualValue)
	if err != nil {
		return nil, nil, err
	}
	check := func() (*repo.VideoCollision, error) { return h.repo.FindStudioCollision(ctx, videoID, names) }
	if override {
		check = nil
	} else if collision, err = check(); err != nil {
		return nil, nil, err
	} else if collision != nil {
		return names, collision, nil
	}
	if collision, err = h.repo.SetDecisionChecked(ctx, model.EnrichEntityVideo, videoID, field.Canonical, source, manualValue, check); err != nil {
		return nil, nil, err
	}
	if collision != nil {
		return names, collision, nil
	}
	// Studio reuses the fetch+resolve the collision gate above already ran (same
	// video, same pending decision) instead of paying for a second
	// loadRelinkContext + resolver.Resolve pass here.
	h.relinkStudiosWithContext(ctx, videoID, rc, names)
	return names, nil, nil
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
//
// A film source (F56, ADR-085 §4) is a video-only exception: injectFilmSources
// synthesizes its "film:<id>" candidates from film_videos at read time and never
// persists them to entity_enrichment, so the row scan below can never match one —
// a real attachment in film_videos is the equivalent "currently offered" check.
func (h *Handlers) providerMatched(r *http.Request, entityType string, id int64, provider string) bool {
	if entityType == model.EnrichEntityVideo && strings.HasPrefix(provider, "film:") {
		return h.filmAttachedToVideo(r, id, provider)
	}
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

// filmAttachedToVideo reports whether the video actually has a film_videos row for
// the "film:<id>" provider namespace — see providerMatched.
func (h *Handlers) filmAttachedToVideo(r *http.Request, videoID int64, provider string) bool {
	filmID, err := strconv.ParseInt(strings.TrimPrefix(provider, "film:"), 10, 64)
	if err != nil {
		return false
	}
	films, err := h.repo.FilmsForVideo(r.Context(), videoID)
	if err != nil {
		h.log.Warn("films lookup for decision", "id", videoID, "err", err)
		return false
	}
	for _, f := range films {
		if f.FilmID == filmID {
			return true
		}
	}
	return false
}
