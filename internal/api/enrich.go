package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// imdbPathRe extracts an IMDb ID from a Plex/Jellyfin-style path component like
// "{imdb-tt1160419}" so it can be forwarded as an external-ID hint to providers.
var imdbPathRe = regexp.MustCompile(`\{imdb-(tt\d+)\}`)

// videoHint builds a /resolve hint for a video: the given query plus, if the video's
// path carries an embedded IMDb id, that id as a deterministic external-id hint —
// and ADR-095's structured hints, built unconditionally here: the resolved values of
// the five search fields (D2, as-is), the media basename (D3, verbatim — never a
// directory component), and querySource (D4). Whether any of those actually reach
// the provider is enrich.Service.Resolve's call, against the provider's manifest
// opt-in and the operator's deny; this builder does not know or care. Shared by
// enrichVideoResolve and refresh-all's enrichQueryHint (enrich_review.go).
func videoHint(v *model.Video, resolved []resolver.ResolvedField, query, querySource string) enrich.Hint {
	hint := enrich.Hint{
		Query:       query,
		Fields:      searchFieldValues(v, resolved),
		Filename:    filepath.Base(v.FilePath),
		QuerySource: querySource,
	}
	if m := imdbPathRe.FindStringSubmatch(v.FilePath); m != nil {
		hint.ExternalIDs = []string{"imdb:" + m[1]}
	}
	return hint
}

// searchFieldValues projects a video's resolved fields onto hint.fields' vocabulary
// (enrich.SearchFieldKeys): every surviving value of each, /enrich-shaped. The title
// falls back to the raw video title when no mapping resolved one — the same
// baseline queryFieldsFrom renders the {title} token from.
func searchFieldValues(v *model.Video, resolved []resolver.ResolvedField) map[string][]string {
	out := map[string][]string{}
	for _, key := range enrich.SearchFieldKeys {
		if vals := resolvedValues(resolved, key); len(vals) > 0 {
			out[key] = vals
		}
	}
	if len(out["title"]) == 0 && v.Title != "" {
		out["title"] = []string{v.Title}
	}
	return out
}

// videoResolveInputs fetches a video and resolves the five search fields
// (enrich.SearchFieldKeys) the /resolve hint and the ADR-080 query render both read —
// the same mapped-field resolve the link derivation (relinkVideoPeople) runs, so the
// values sent to a provider are exactly the ones the owner's decisions and curation
// produced. A missing/soft-deleted video is repo.ErrNotFound (videoLookupError → 404);
// no mapping configured resolves nothing, and callers fall back to the raw title.
func (h *Handlers) videoResolveInputs(ctx context.Context, id int64) (*model.Video, []resolver.ResolvedField, error) {
	rc, err := h.loadRelinkContext(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if rc == nil {
		return nil, nil, repo.ErrNotFound
	}
	if h.mappings == nil {
		return rc.video, nil, nil
	}
	m := h.mappings.Current()
	var fields []mapping.Field
	for _, key := range enrich.SearchFieldKeys {
		if f, ok := m.ByCanonical(key); ok {
			fields = append(fields, f)
		}
	}
	resolved := resolver.Resolve(rc.video, rc.extra, enrichmentFromRows(rc.enrRows), curationFromRows(rc.curRows),
		fields, h.resolveOptions(decisionsFromRows(rc.decRows)))
	return rc.video, resolved, nil
}

// queryFieldsFrom translates a video's resolved fields into BuildQuery's input
// (ADR-080 D3): top-precedence studio/title/release_date, the full uncapped
// actors-then-director list as performers. The raw video title stands in when no
// mapping resolved one.
func queryFieldsFrom(v *model.Video, resolved []resolver.ResolvedField) enrich.QueryFields {
	title := firstResolvedValue(resolved, "title")
	if title == "" {
		title = v.Title
	}
	return enrich.QueryFields{
		Studio:      firstResolvedValue(resolved, "studio"),
		Title:       title,
		Performers:  append(resolvedValues(resolved, "actors"), resolvedValues(resolved, "director")...),
		ReleaseDate: firstResolvedValue(resolved, "release_date"),
	}
}

// providerQuery renders one provider's /resolve search query from fields through
// its ADR-080 D2 precedence chain (operator search_pattern → cached /describe
// preference → fleet default → sanitized-title floor). The one render every path
// shares: the picker seed (buildVideoQueries), the query_source comparison
// (enrichVideoResolve) and the batch hint (enrichQueryHint) — so "pattern" vs "user"
// is decided against the same string the picker was seeded with.
func (h *Handlers) providerQuery(src enrich.Source, fields enrich.QueryFields) string {
	preferred, _ := h.enrich.PreferredSearchPattern(src.Name)
	return src.BuildQuery(fields, preferred, h.enrich.Store().Current().DefaultSearchPattern())
}

// buildVideoQueries computes, per enabled video-capable provider, the seeded
// /resolve search query the owner's Enrich picker should default to (ADR-080 D5,
// FR5) — the provider's own precedence chain (its operator-configured
// SearchPattern, then its cached /describe preference, then the operator's
// fleet-wide default, then the sanitized-title floor) rendered from the video's
// already-resolved fields. Always non-empty per provider as long as the video has
// any title at all (BuildQuery's sanitized floor never fails otherwise). A nil
// h.enrich (enrichment disabled) or nil resolved (no mapping configured — falls
// back to the raw video title) both degrade gracefully rather than omitting the map.
func (h *Handlers) buildVideoQueries(v *model.Video, resolved []resolver.ResolvedField) map[string]string {
	if h.enrich == nil {
		return nil
	}
	fields := queryFieldsFrom(v, resolved)
	out := map[string]string{}
	for _, src := range h.enrich.Store().Current().Enabled() {
		if !src.Supports(model.EnrichEntityVideo) {
			continue
		}
		out[src.Name] = h.providerQuery(src, fields)
	}
	return out
}

// firstResolvedValue returns a scalar field's top-precedence resolved value, or ""
// when the field is absent or has no surviving values.
func firstResolvedValue(fields []resolver.ResolvedField, canonical string) string {
	rf, ok := resolvedByCanonical(fields, canonical)
	if !ok || len(rf.Values) == 0 {
		return ""
	}
	return rf.Values[0]
}

// resolvedValues returns every surviving value of a multi-valued field (e.g.
// actors/director), or nil when the field is absent.
func resolvedValues(fields []resolver.ResolvedField, canonical string) []string {
	rf, ok := resolvedByCanonical(fields, canonical)
	if !ok {
		return nil
	}
	return rf.Values
}

// enrichRoute pairs a route path prefix with the entity type it enriches — the F47
// dismiss/undismiss/refresh/refresh-all route loop below iterates a slice of these.
type enrichRoute struct {
	path       string
	entityType string
}

// mountEnrich registers the owner-gated enrichment endpoints (F22.5/F22.9a). They
// are mounted inside the requireOwner group set up in Mount.
func (h *Handlers) mountEnrich(r chi.Router) {
	r.Get("/enrich/sources", h.enrichSources)
	r.Post("/people/{id}/enrich/resolve", h.enrichResolve)
	r.Post("/people/{id}/enrich", h.enrichApply)
	r.Delete("/people/{id}/enrich/{provider}", h.enrichClear)
	r.Post("/media/{id}/enrich/resolve", h.enrichVideoResolve)
	r.Post("/media/{id}/enrich", h.enrichVideoApply)
	r.Delete("/media/{id}/enrich/{provider}", h.enrichVideoClear)
	r.Post("/studios/{id}/enrich/resolve", h.enrichStudioResolve)
	r.Post("/studios/{id}/enrich", h.enrichStudioApply)
	r.Delete("/studios/{id}/enrich/{provider}", h.enrichStudioClear)
	// Film enrichment routes (F56/ADR-086) are unregistered entirely when
	// films_enabled is off, mirroring every other film route in Mount.
	entityTypes := []enrichRoute{
		{"/people", model.EnrichEntityPerson},
		{"/studios", model.EnrichEntityStudio},
		{"/media", model.EnrichEntityVideo},
	}
	if h.filmsEnabled {
		r.Post("/films/{id}/enrich/resolve", h.filmEnrichResolve)
		r.Post("/films/{id}/enrich", h.filmEnrichApply)
		r.Delete("/films/{id}/enrich/{provider}", h.filmEnrichClear)
		entityTypes = append(entityTypes, enrichRoute{"/films", model.EnrichEntityFilm})
	}

	// F47 (ADR-066): the review queue, and the dismiss/undismiss/refresh/refresh-all
	// mutations — entity-generic across all four, mirroring the route shape above.
	r.Get("/owner/enrich-queue", h.enrichQueue)
	for _, et := range entityTypes {
		r.Post(et.path+"/{id}/enrich/{provider}/dismiss", h.enrichDismiss(et.entityType))
		r.Delete(et.path+"/{id}/enrich/{provider}/dismiss", h.enrichUndismiss(et.entityType))
		r.Post(et.path+"/{id}/enrich/{provider}/refresh", h.enrichRefresh(et.entityType))
		r.Post(et.path+"/{id}/enrich/refresh-all", h.enrichRefreshAll(et.entityType))
	}
}

func (h *Handlers) enrichSources(w http.ResponseWriter, r *http.Request) {
	if h.enrich == nil {
		writeJSON(w, http.StatusOK, map[string]any{"sources": []any{}})
		return
	}
	// Same shape the public /providers directory returns, so the owner enrich controls
	// get the provider icon URL (ADR-059) without a second lookup. Extra icon_url field
	// is ignored by any older SPA consuming just name/entity_types.
	writeJSON(w, http.StatusOK, map[string]any{"sources": h.providerInfos(r.Context())})
}

// enrichResolve runs provider name-search for a person and returns candidates for
// the owner to confirm (F22.5b). Nothing is applied here.
func (h *Handlers) enrichResolve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	var body struct {
		Provider string `json:"provider"`
		Query    string `json:"query"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if _, err := h.repo.GetPerson(r.Context(), id); err != nil {
		h.personLookupError(w, err)
		return
	}
	if !h.enrichDismissedCheck(w, r, model.EnrichEntityPerson, id, body.Provider) {
		return
	}
	res, err := h.enrich.Resolve(r.Context(), body.Provider, model.EnrichEntityPerson, enrich.Hint{Query: body.Query})
	if err != nil {
		h.log.Warn("enrich resolve failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "provider lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": res.Candidates})
}

// enrichApply fetches the chosen record and stores it in the shadow layer,
// returning the person's resolved fields with provenance (F22.5/F22.7).
func (h *Handlers) enrichApply(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	var body struct {
		Provider   string `json:"provider"`
		ExternalID string `json:"external_id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.ExternalID == "" {
		writeError(w, http.StatusBadRequest, "external_id required")
		return
	}
	if _, err := h.repo.GetPerson(r.Context(), id); err != nil {
		h.personLookupError(w, err)
		return
	}
	// Bypass the gallery auto-fill cap for an owner/admin caller (HOLODEX-174).
	// Every /enrich/* route already sits behind requireOwner, so this is redundant
	// with route-level gating today — but deriving it from the actual auth check
	// (the single choke point documented on Auth, auth.go) rather than asserting a
	// literal true keeps this correct if that mounting ever changes.
	fields, err := h.enrich.Enrich(r.Context(), model.EnrichEntityPerson, id, body.Provider, body.ExternalID, h.auth.authorized(r))
	if err != nil {
		h.log.Warn("enrich apply failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "enrichment failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enriched": fields})
}

// enrichClear removes a provider's contribution for a person (F22.7b).
func (h *Handlers) enrichClear(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	provider := chi.URLParam(r, "provider")
	if err := h.enrich.Clear(r.Context(), model.EnrichEntityPerson, id, provider); err != nil {
		h.fail(w, "clear enrichment", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) personLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "person not found")
		return
	}
	h.fail(w, "get person", err)
}

// enrichVideoResolve searches a provider for film candidates matching a video (F26).
func (h *Handlers) enrichVideoResolve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	// The body carries no query_source: ADR-095 D4 derives it server-side below and
	// a client-supplied value is ignored by construction (it is never decoded).
	var body struct {
		Provider string `json:"provider"`
		Query    string `json:"query"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	v, resolved, err := h.videoResolveInputs(r.Context(), id)
	if err != nil {
		h.videoLookupError(w, err)
		return
	}
	if !h.enrichDismissedCheck(w, r, model.EnrichEntityVideo, id, body.Provider) {
		return
	}
	// query_source (ADR-095 D4): re-render this provider's query from the same
	// resolved slice, in this same call, and compare — equal means the owner searched
	// the seeded string untouched (the sanitized-title floor included; it is a render
	// too), anything else is the owner's own text. An unknown provider renders from a
	// zero Source and then fails in Resolve as it always has.
	src, _ := h.enrich.Store().Current().ByName(body.Provider)
	querySource := enrich.QuerySourceUser
	if body.Query == h.providerQuery(src, queryFieldsFrom(v, resolved)) {
		querySource = enrich.QuerySourcePattern
	}
	res, err := h.enrich.Resolve(r.Context(), body.Provider, model.EnrichEntityVideo, videoHint(v, resolved, body.Query, querySource))
	if err != nil {
		h.log.Warn("video enrich resolve failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "provider lookup failed")
		return
	}
	// ResolveResult marshals as {"candidates": […], "searched": […]} — searched
	// omitted when the provider sent none, so the picker renders nothing (HOLODEX-369).
	writeJSON(w, http.StatusOK, res)
}

// enrichVideoApply fetches and stores film enrichment for a video (F26).
func (h *Handlers) enrichVideoApply(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	var body struct {
		Provider   string `json:"provider"`
		ExternalID string `json:"external_id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.ExternalID == "" {
		writeError(w, http.StatusBadRequest, "external_id required")
		return
	}
	if _, _, err := h.repo.GetVideo(r.Context(), id); err != nil {
		h.videoLookupError(w, err)
		return
	}
	fields, err := h.enrich.Enrich(r.Context(), model.EnrichEntityVideo, id, body.Provider, body.ExternalID, h.auth.authorized(r))
	if err != nil {
		h.log.Warn("video enrich apply failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "enrichment failed")
		return
	}
	// Shared post-apply side effects (F38 studio relink, F50 P0-9 tag materialization)
	// — the same dispatcher Refresh/Refresh-all use, so manual apply doesn't skip them.
	h.afterEnrichApply(r, model.EnrichEntityVideo, id)
	writeJSON(w, http.StatusOK, map[string]any{"enriched": fields})
}

// enrichVideoClear removes a provider's contribution for a video (F26).
func (h *Handlers) enrichVideoClear(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	provider := chi.URLParam(r, "provider")
	if err := h.enrich.Clear(r.Context(), model.EnrichEntityVideo, id, provider); err != nil {
		h.fail(w, "clear video enrichment", err)
		return
	}
	// Clearing a provider can change the resolved studio/genres value just as much
	// as applying one — same shared dispatcher enrichVideoApply/Refresh use, so a
	// clear doesn't skip studio relink (F38) or tag materialization (F50 P0-9).
	h.afterEnrichApply(r, model.EnrichEntityVideo, id)
	writeJSON(w, http.StatusOK, map[string]any{"written_back": h.providerWrittenBack(r, model.EnrichEntityVideo, id, provider)})
}

// providerWrittenBack reports whether the provider's values were ever written into
// the entity's file (HOLODEX-370). Clear and Dismiss drop the provider from the DB
// but never touch the file, so re-extract leaves its values as the file-layer
// baseline and the page keeps showing them; the flag lets the UI say so and point
// at the batch Revert in Job history. Only videos have a file — every other entity
// type is false. A lookup failure is logged, not surfaced: the clear/dismiss
// itself succeeded, and a missing notice is the lesser harm.
func (h *Handlers) providerWrittenBack(r *http.Request, entityType string, id int64, provider string) bool {
	if entityType != model.EnrichEntityVideo {
		return false
	}
	found, err := h.repo.HasWritebackFromProvider(r.Context(), id, provider)
	if err != nil {
		h.log.Warn("writeback attribution lookup failed", "video", id, "provider", provider, "err", err)
		return false
	}
	return found
}

// enrichStudioResolve searches a provider for company candidates matching a studio
// (F38 S3). Mirrors enrichResolve; nothing is applied here.
func (h *Handlers) enrichStudioResolve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	var body struct {
		Provider string `json:"provider"`
		Query    string `json:"query"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if _, err := h.repo.GetStudio(r.Context(), id); err != nil {
		h.studioLookupError(w, err)
		return
	}
	if !h.enrichDismissedCheck(w, r, model.EnrichEntityStudio, id, body.Provider) {
		return
	}
	res, err := h.enrich.Resolve(r.Context(), body.Provider, model.EnrichEntityStudio, enrich.Hint{Query: body.Query})
	if err != nil {
		h.log.Warn("studio enrich resolve failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "provider lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": res.Candidates})
}

// enrichStudioApply fetches and stores company enrichment for a studio (F38 S3).
// Unlike the video path there is no relink: a studio-entity enrich changes the
// studio's own resolved fields (description/country/website/logo), never the
// video → studio links (those derive from the video's studio field, RD1).
func (h *Handlers) enrichStudioApply(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	var body struct {
		Provider   string `json:"provider"`
		ExternalID string `json:"external_id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.ExternalID == "" {
		writeError(w, http.StatusBadRequest, "external_id required")
		return
	}
	if _, err := h.repo.GetStudio(r.Context(), id); err != nil {
		h.studioLookupError(w, err)
		return
	}
	fields, err := h.enrich.Enrich(r.Context(), model.EnrichEntityStudio, id, body.Provider, body.ExternalID, h.auth.authorized(r))
	if err != nil {
		h.log.Warn("studio enrich apply failed", "provider", body.Provider, "err", err)
		writeError(w, http.StatusBadGateway, "enrichment failed")
		return
	}
	// A provider's logo (and, once a provider supplies them, icon/poster) arrives as
	// an image asset and is already stored by Enrich's entity-generic downloadAssets
	// (F51, ADR-079) — no separate relink step, unlike the pre-F51 field-derived cache.
	writeJSON(w, http.StatusOK, map[string]any{"enriched": fields})
}

// enrichStudioClear removes a provider's contribution for a studio (F38 S3).
func (h *Handlers) enrichStudioClear(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.enrich == nil {
		writeError(w, http.StatusServiceUnavailable, "enrichment unavailable")
		return
	}
	provider := chi.URLParam(r, "provider")
	if err := h.enrich.Clear(r.Context(), model.EnrichEntityStudio, id, provider); err != nil {
		h.fail(w, "clear studio enrichment", err)
		return
	}
	// Clearing a provider's shadow fields does not touch already-stored studio_images
	// rows (F51, ADR-079) — mirrors person enrich Clear, which likewise never deletes
	// downloaded images. The owner removes an image explicitly via the image endpoints.
	w.WriteHeader(http.StatusNoContent)
}

// videoEnrichment reads a video's stored enrichment for the detail view.
// Returns nil when no enrichment service is wired or no rows exist.
func (h *Handlers) videoEnrichment(r *http.Request, id int64) []model.EnrichedField {
	if h.enrich == nil {
		return nil
	}
	fields, err := h.enrich.Fields(r.Context(), model.EnrichEntityVideo, id)
	if err != nil {
		h.log.Warn("read video enrichment", "id", id, "err", err)
		return nil
	}
	return fields
}

func (h *Handlers) videoLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "media not found")
		return
	}
	h.fail(w, "get media", err)
}

// decodeJSON reads a small JSON request body, writing a 400 on malformed input.
func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	if err := dec.Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
