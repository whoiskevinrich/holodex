package api

import (
	"context"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// F47 (ADR-066): the enrichment review workflow's backend surface — a generalized
// review queue, a durable "not matched" verdict, and a refresh bypass that skips the
// picker for an already-linked provider. Entity-generic across person/studio/video,
// mirroring enrich.go's resolve/apply/clear route shape. Mounted inside mountEnrich,
// itself inside the requireOwner group (handlers.go Mount).

// enrichQueue lists every Person/Studio/Media entity missing at least one supporting
// provider's data (RD2/P0-1) — a pure DB read, never a provider call.
func (h *Handlers) enrichQueue(w http.ResponseWriter, r *http.Request) {
	if h.enrich == nil {
		writeJSON(w, http.StatusOK, map[string]any{"rows": []any{}})
		return
	}
	srcs := h.enrich.Sources()
	providersByType := map[string][]string{}
	for _, et := range []string{model.EnrichEntityPerson, model.EnrichEntityStudio, model.EnrichEntityVideo} {
		for _, src := range srcs {
			if src.Supports(et) {
				providersByType[et] = append(providersByType[et], src.Name)
			}
		}
	}
	rows, err := h.repo.EnrichQueue(r.Context(), providersByType)
	if err != nil {
		h.fail(w, "enrich queue", err)
		return
	}
	if rows == nil {
		rows = []repo.EnrichQueueRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

// enrichDismiss records a durable "not matched" verdict for (entity, provider) — the
// owner's "None of these match" action (RD4). Idempotent.
func (h *Handlers) enrichDismiss(entityType string) http.HandlerFunc {
	return h.enrichDismissalAction(entityType, "dismiss", h.repo.DismissEnrichment)
}

// enrichUndismiss clears a dismissal — "Try again" (RD4) — so a future /resolve for
// the pair is no longer blocked. Idempotent; clearing a non-existent dismissal is a
// no-op success.
func (h *Handlers) enrichUndismiss(entityType string) http.HandlerFunc {
	return h.enrichDismissalAction(entityType, "undismiss", h.repo.UndismissEnrichment)
}

// enrichDismissalAction is the shared dismiss/undismiss handler shape (RD4): resolve
// the entity, then run action (DismissEnrichment or UndismissEnrichment) against the
// path's provider. verb only labels the error context on failure.
func (h *Handlers) enrichDismissalAction(entityType, verb string, action func(ctx context.Context, entityType string, id int64, provider string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if !h.enrichEntityLookup(w, r, entityType, id) {
			return
		}
		provider := chi.URLParam(r, "provider")
		if err := action(r.Context(), entityType, id, provider); err != nil {
			h.fail(w, verb+" enrichment", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// enrichRefresh re-fetches a provider's data using the stored external_id — no
// /resolve call, no picker (RD7). 400 if the provider isn't linked yet.
func (h *Handlers) enrichRefresh(entityType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if h.enrich == nil {
			writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
			return
		}
		if !h.enrichEntityLookup(w, r, entityType, id) {
			return
		}
		provider := chi.URLParam(r, "provider")
		externalID, linked, err := h.enrich.ExistingMatch(r.Context(), entityType, id, provider)
		if err != nil {
			h.fail(w, "refresh lookup", err)
			return
		}
		if !linked {
			writeError(w, http.StatusBadRequest, "provider is not linked")
			return
		}
		fields, err := h.enrich.Enrich(r.Context(), entityType, id, provider, externalID, h.auth.authorized(r))
		if err != nil {
			h.log.Warn("enrich refresh failed", "provider", provider, "entity_type", entityType, "err", err)
			writeError(w, http.StatusBadGateway, "refresh failed")
			return
		}
		h.afterEnrichApply(r, entityType, id)
		writeJSON(w, http.StatusOK, map[string]any{"enriched": fields})
	}
}

// refreshAllResult is one provider's outcome in a POST .../enrich/refresh-all response
// (RD8/P1-2).
type refreshAllResult struct {
	Provider string                `json:"provider"`
	Status   string                `json:"status"` // refreshed | auto_applied | needs_review | no_candidates
	Enriched []model.EnrichedField `json:"enriched,omitempty"`
}

// enrichRefreshAll fans out over an entity's configured providers (RD8): a linked
// provider refreshes directly; an unlinked provider resolves and auto-applies a single
// strong match (ADR-066 D1) or surfaces inline for review — never silently dropped. A
// dismissed, not-yet-linked provider is left out of the response entirely (RD4's block
// on re-resolving it stands even inside a bulk fan-out).
func (h *Handlers) enrichRefreshAll(entityType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if h.enrich == nil {
			writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
			return
		}
		hint, ok := h.enrichQueryHint(w, r, entityType, id)
		if !ok {
			return
		}
		// One batched lookup for every already-linked provider (enrich.ProviderMatches),
		// instead of a per-provider ExistingMatch query in the fan-out below.
		matches, err := h.enrich.ProviderMatches(r.Context(), entityType, id)
		if err != nil {
			h.fail(w, "refresh-all match lookup", err)
			return
		}
		linked := make(map[string]string, len(matches))
		for _, m := range matches {
			linked[m.Provider] = m.ExternalID
		}

		var supported []enrich.SourceInfo
		for _, src := range h.enrich.Sources() {
			if src.Supports(entityType) {
				supported = append(supported, src)
			}
		}
		// Each provider is an independent HTTP round-trip to its own sidecar, so the
		// fan-out runs concurrently rather than paying N sequential round-trips
		// (writes still serialize under repo.writeMu, which is cheap next to the I/O).
		type outcome struct {
			res     refreshAllResult
			skip    bool
			changed bool
		}
		outcomes := make([]outcome, len(supported))
		var wg sync.WaitGroup
		for i, src := range supported {
			wg.Add(1)
			go func(i int, name string) {
				defer wg.Done()
				externalID, isLinked := linked[name]
				res, skip := h.refreshOneProvider(r, entityType, id, name, hint, externalID, isLinked)
				changed := !skip && (res.Status == "refreshed" || res.Status == "auto_applied")
				outcomes[i] = outcome{res: res, skip: skip, changed: changed}
			}(i, src.Name)
		}
		wg.Wait()

		results := make([]refreshAllResult, 0, len(supported))
		changed := false
		for _, o := range outcomes {
			if o.skip {
				continue
			}
			results = append(results, o.res)
			changed = changed || o.changed
		}
		// Run the shared post-apply side effects (studio relink / logo relink) once for
		// the whole fan-out rather than once per provider that changed something — the
		// relink only cares about the entity's final resolved state.
		if changed {
			h.afterEnrichApply(r, entityType, id)
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": results})
	}
}

// refreshOneProvider is refresh-all's per-provider step (RD8): a linked provider
// (externalID/isLinked from the caller's one batched ProviderMatches lookup) refreshes
// directly; an unlinked one resolves and auto-applies a single strong match or leaves
// itself for the owner. skip=true means the provider is left out of the response
// entirely (only for a dismissed, unlinked provider — RD4's block on re-resolving it).
// Runs concurrently with its sibling providers (see enrichRefreshAll); it does not call
// afterEnrichApply itself — the caller runs that once, after the whole fan-out settles.
func (h *Handlers) refreshOneProvider(r *http.Request, entityType string, id int64, provider string, hint enrich.Hint, externalID string, isLinked bool) (result refreshAllResult, skip bool) {
	ctx := r.Context()
	// noCandidates logs and reports the shared "this provider produced nothing usable"
	// outcome — every failure path below except a dismissed-skip reduces to it.
	noCandidates := func(msg string, err error) (refreshAllResult, bool) {
		h.log.Warn(msg, "provider", provider, "err", err)
		return refreshAllResult{Provider: provider, Status: "no_candidates"}, false
	}

	if isLinked {
		fields, err := h.enrich.Enrich(ctx, entityType, id, provider, externalID, h.auth.authorized(r))
		if err != nil {
			return noCandidates("refresh-all refresh failed", err)
		}
		return refreshAllResult{Provider: provider, Status: "refreshed", Enriched: fields}, false
	}

	dismissed, err := h.repo.EnrichmentDismissed(ctx, entityType, id, provider)
	if err != nil {
		h.log.Warn("refresh-all dismissal lookup failed", "provider", provider, "err", err)
		return refreshAllResult{}, true
	}
	if dismissed {
		return refreshAllResult{}, true // RD4: never re-resolved until an explicit "Try again"
	}

	cands, err := h.enrich.Resolve(ctx, provider, entityType, hint)
	if err != nil {
		return noCandidates("refresh-all resolve failed", err)
	}
	if strong, ok := enrich.SingleStrongMatch(cands); ok {
		fields, err := h.enrich.Enrich(ctx, entityType, id, provider, strong.ExternalID, h.auth.authorized(r))
		if err != nil {
			return noCandidates("refresh-all auto-apply failed", err)
		}
		return refreshAllResult{Provider: provider, Status: "auto_applied", Enriched: fields}, false
	}
	if len(cands) == 0 {
		return refreshAllResult{Provider: provider, Status: "no_candidates"}, false
	}
	return refreshAllResult{Provider: provider, Status: "needs_review"}, false
}

// afterEnrichApply runs the same post-apply side effects the existing per-entity apply
// handler already runs (enrichVideoApply) — a new studio or person-typed (actors/
// director) candidate can move a video's resolved value (F38/F40 relink). Shared so
// Refresh/Refresh-all stay in lockstep with a manual apply instead of silently
// skipping this. A studio's image assets (F51, ADR-079) need no equivalent post-step
// here — Enrich's entity-generic downloadAssets already stored them before this runs.
func (h *Handlers) afterEnrichApply(r *http.Request, entityType string, id int64) {
	if entityType == model.EnrichEntityVideo {
		h.relinkStudios(r.Context(), id)
		h.relinkPeople(r.Context(), id)
		h.materializeTags(r.Context(), id) // F50 P0-9, ADR-075 D4
	}
}

// enrichDismissedCheck writes 409 and returns false when (entityType, id, provider)
// carries a durable "not matched" verdict (RD4) — blocks /resolve from re-asking a
// provider the owner already rejected, until an explicit undismiss ("Try again").
func (h *Handlers) enrichDismissedCheck(w http.ResponseWriter, r *http.Request, entityType string, id int64, provider string) bool {
	dismissed, err := h.repo.EnrichmentDismissed(r.Context(), entityType, id, provider)
	if err != nil {
		h.fail(w, "check enrichment dismissal", err)
		return false
	}
	if dismissed {
		writeError(w, http.StatusConflict, "provider dismissed for this entity — undismiss to try again")
		return false
	}
	return true
}

// enrichEntityLookup validates that an entity exists (writing 404/409 otherwise)
// before a dismiss/undismiss/refresh action proceeds — repo.EntityExists' cheap
// existence-only check for person/studio (no need for the full get-with-aliases
// enrichQueryHint does for refresh-all); video has no canonicalTable entry (F43's
// name-identity spine doesn't cover it), so it goes through GetVideo as before.
func (h *Handlers) enrichEntityLookup(w http.ResponseWriter, r *http.Request, entityType string, id int64) bool {
	switch entityType {
	case model.EnrichEntityPerson:
		if err := h.repo.EntityExists(r.Context(), entityType, id); err != nil {
			h.personLookupError(w, err)
			return false
		}
		return true
	case model.EnrichEntityStudio:
		if err := h.repo.EntityExists(r.Context(), entityType, id); err != nil {
			h.studioLookupError(w, err)
			return false
		}
		return true
	case model.EnrichEntityVideo:
		if _, _, err := h.repo.GetVideo(r.Context(), id); err != nil {
			h.videoLookupError(w, err)
			return false
		}
		return true
	default:
		writeError(w, http.StatusBadRequest, "unknown entity type")
		return false
	}
}

// enrichQueryHint resolves an entity (404/409 on failure) and builds the /resolve hint
// refresh-all uses for an unlinked provider — the entity's own name, plus (video only)
// any embedded IMDb id (videoHint, shared with enrichVideoResolve).
func (h *Handlers) enrichQueryHint(w http.ResponseWriter, r *http.Request, entityType string, id int64) (enrich.Hint, bool) {
	switch entityType {
	case model.EnrichEntityPerson:
		p, err := h.repo.GetPerson(r.Context(), id)
		if err != nil {
			h.personLookupError(w, err)
			return enrich.Hint{}, false
		}
		return enrich.Hint{Query: p.Name}, true
	case model.EnrichEntityStudio:
		s, err := h.repo.GetStudio(r.Context(), id)
		if err != nil {
			h.studioLookupError(w, err)
			return enrich.Hint{}, false
		}
		return enrich.Hint{Query: s.Name}, true
	case model.EnrichEntityVideo:
		v, _, err := h.repo.GetVideo(r.Context(), id)
		if err != nil {
			h.videoLookupError(w, err)
			return enrich.Hint{}, false
		}
		return videoHint(v, v.Title), true
	default:
		writeError(w, http.StatusBadRequest, "unknown entity type")
		return enrich.Hint{}, false
	}
}
