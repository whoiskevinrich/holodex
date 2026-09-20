package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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
	entityTypes := []string{model.EnrichEntityPerson, model.EnrichEntityStudio, model.EnrichEntityVideo}
	// Film rows are suppressed entirely when films_enabled is off (F56, ADR-085),
	// mirroring every other film surface — even if a provider is misconfigured with
	// entity_types including "film" while this instance has films disabled.
	if h.filmsEnabled {
		entityTypes = append(entityTypes, model.EnrichEntityFilm)
	}
	for _, et := range entityTypes {
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
	h.applyPartsToEnrichQueue(r.Context(), rows)
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
// path's provider. verb labels the error context on failure and picks the response:
// a dismiss answers 200 with the written_back flag (HOLODEX-370) since the owner
// may be walking away from values the file still carries; an undismiss is 204.
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
		if verb == "dismiss" {
			writeJSON(w, http.StatusOK, map[string]any{"written_back": h.providerWrittenBack(r, entityType, id, provider)})
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
			h.providerError(w, "enrich refresh failed", provider, err, "refresh failed")
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
	Status   string                `json:"status"` // refreshed | auto_applied | needs_review | no_candidates | rate_limited
	Enriched []model.EnrichedField `json:"enriched,omitempty"`
	// RetryAfter (seconds) accompanies rate_limited: the provider's bucket is paused
	// (ADR-103 D4) and this row failed fast rather than waiting; the other providers'
	// rows are unaffected, which is why the fan-out reports it per row, not as a 503.
	RetryAfter int `json:"retry_after,omitempty"`
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
		hintFor, ok := h.enrichQueryHint(w, r, entityType, id)
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
		linked := make(map[string]*enrich.Match, len(matches))
		for i := range matches {
			linked[matches[i].Provider] = &matches[i]
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
				// Per-provider hint, built inside the goroutine (ADR-095 D8): each
				// provider gets its own ADR-080 render and its own opted-in keys.
				res, skip := h.refreshOneProvider(r, entityType, id, name, hintFor(name), linked[name])
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

// refreshOneProvider renders one provider's RefreshPair outcome (F66 RD1: the step
// itself lives in enrich.Service so the sweep runs the identical routing). skip=true
// means the provider is left out of the response entirely (a dismissed, unlinked
// provider — RD4's block on re-resolving it). A failed call is logged and reported
// as no_candidates, the pre-existing rendering. A click always forces (ADR-103 D7).
func (h *Handlers) refreshOneProvider(r *http.Request, entityType string, id int64, provider string, hint enrich.Hint, link *enrich.Match) (result refreshAllResult, skip bool) {
	out := h.enrich.RefreshPair(r.Context(), entityType, id, provider, hint, link, enrich.RefreshOpts{
		Force:            true,
		BypassGalleryCap: h.auth.authorized(r),
	})
	switch out.Status {
	case enrich.PairDismissed:
		return refreshAllResult{}, true
	case enrich.PairFailed:
		var paused *enrich.ErrProviderPaused
		if errors.As(out.Err, &paused) {
			return refreshAllResult{Provider: provider, Status: "rate_limited", RetryAfter: paused.RetryAfterSeconds()}, false
		}
		h.log.Warn("refresh-all provider step failed", "provider", provider, "err", out.Err)
		return refreshAllResult{Provider: provider, Status: string(enrich.PairNoCandidates)}, false
	}
	return refreshAllResult{Provider: provider, Status: string(out.Status), Enriched: out.Enriched}, false
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

// rateLimitedLine is the owner-facing sentence for a paused provider (F66 RD8): the
// same words the refresh-all result row and the sweep's status line use.
func rateLimitedLine(provider string, secs int) string {
	return fmt.Sprintf("%s is rate-limiting — try again in %d s", provider, secs)
}

// providerError answers an interactive provider call's failure. A paused bucket
// (ADR-103 D4) is 503 + Retry-After, the body naming the provider and the seconds so
// the SPA's inline status line can say "tmdb is rate-limiting — try again in 42 s";
// every other error stays the generic 502 the caller used, logged with the raw
// error (which may carry a base_url — never echoed to the client).
func (h *Handlers) providerError(w http.ResponseWriter, op, provider string, err error, msg string) {
	var paused *enrich.ErrProviderPaused
	if errors.As(err, &paused) {
		secs := paused.RetryAfterSeconds()
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error":       rateLimitedLine(provider, secs),
			"provider":    provider,
			"retry_after": secs,
		})
		return
	}
	h.log.Warn(op, "provider", provider, "err", err)
	writeError(w, http.StatusBadGateway, msg)
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
// name-identity spine doesn't cover it), so it goes through GetVideo as before. Film
// (F56/ADR-086) has no canonicalTable entry either — films are purely owner-asserted,
// deliberately outside the alias/merge name-identity spine (ADR-085) — so it goes
// through GetFilm the same way video goes through GetVideo.
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
	case model.EnrichEntityFilm:
		if _, err := h.repo.GetFilm(r.Context(), id); err != nil {
			h.filmLookupError(w, err)
			return false
		}
		return true
	default:
		writeError(w, http.StatusBadRequest, "unknown entity type")
		return false
	}
}

// enrichQueryHint resolves an entity (404/409 on failure) and returns the builder of
// the /resolve hint refresh-all sends an unlinked provider, keyed by provider name.
// Person/studio/film hints are the entity's own name, the same for every provider.
// A video's hint is built PER PROVIDER (ADR-095 D8): the provider's own ADR-080
// render of the video's resolved fields (providerQuery — the same chain the
// interactive picker is seeded from, closing ADR-080 AI5's shared-title scope trim),
// query_source "pattern" (nobody typed it), any embedded IMDb id, and the structured
// keys videoHint always builds — the batch path is the one with no owner present to
// retype the query, so it is where a better-aimed hint matters most. The resolve
// happens once, here; only the render is per provider.
func (h *Handlers) enrichQueryHint(w http.ResponseWriter, r *http.Request, entityType string, id int64) (hintFor func(provider string) enrich.Hint, ok bool) {
	same := func(hint enrich.Hint) func(string) enrich.Hint {
		return func(string) enrich.Hint { return hint }
	}
	switch entityType {
	case model.EnrichEntityPerson:
		p, err := h.repo.GetPerson(r.Context(), id)
		if err != nil {
			h.personLookupError(w, err)
			return nil, false
		}
		return same(enrich.Hint{Query: p.Name}), true
	case model.EnrichEntityStudio:
		s, err := h.repo.GetStudio(r.Context(), id)
		if err != nil {
			h.studioLookupError(w, err)
			return nil, false
		}
		return same(enrich.Hint{Query: s.Name}), true
	case model.EnrichEntityVideo:
		v, resolved, err := h.videoResolveInputs(r.Context(), id)
		if err != nil {
			h.videoLookupError(w, err)
			return nil, false
		}
		fields := queryFieldsFrom(v, resolved)
		return func(provider string) enrich.Hint {
			src, _ := h.enrich.Store().Current().ByName(provider)
			return videoHint(v, resolved, h.providerQuery(src, fields), enrich.QuerySourcePattern)
		}, true
	case model.EnrichEntityFilm:
		f, err := h.repo.GetFilm(r.Context(), id)
		if err != nil {
			h.filmLookupError(w, err)
			return nil, false
		}
		return same(enrich.Hint{Query: f.Name}), true
	default:
		writeError(w, http.StatusBadRequest, "unknown entity type")
		return nil, false
	}
}
