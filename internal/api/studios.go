package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// Studio entity endpoints (F38, ADR-053). Studio rides the same unified resolver +
// decision/curation model as person (F37); its baseline is the DB record (name),
// presented in the shared "record" vocabulary. video_studios links are derived from
// the resolved `studio` video field by RelinkVideoStudios (RD1), not authored.

// studioScalarFields are the provider-backed replace studio fields, in registry
// documentation order. name is synthesized separately (the only baseline-backed
// field, read-only — no rename in v1, RD4/RD5). These are the fields the TMDB
// company enrichment slice (S3) populates; before enrichment they resolve empty and
// the detail page hides the Details section. logo is NOT here (F51, ADR-079): it
// moved off the resolved-field model onto the studio_images asset-slot model,
// delivered like a person photo rather than a decidable image_url field.
var studioScalarFields = []string{"description", "country", "website"}

// studioFields synthesizes the []mapping.Field for studio resolution: name (record
// baseline only — no provider name candidates, studio has no rename) then the scalar
// registry fields (record baseline + one candidate per provider). No merge field.
func studioFields(providers []string) []mapping.Field {
	fields := make([]mapping.Field, 0, len(studioScalarFields)+1)
	fields = append(fields, studioField("name", []mapping.Source{{Namespace: "file", Key: "name"}}))
	for _, canonical := range studioScalarFields {
		fields = append(fields, studioField(canonical, providerSources(providers, canonical)))
	}
	return fields
}

// studioField builds one synthesized replace field; raw Sources mirror ParsedSources
// so the field round-trips like a parsed YAML one.
func studioField(canonical string, sources []mapping.Source) mapping.Field {
	return mapping.Field{
		Canonical:     canonical,
		Label:         registry.Lookup(canonical).Label,
		Sources:       rawSources(sources),
		ParsedSources: sources,
	}
}

// studioSchema is the provider-independent synthesized schema used for field
// validation (the provider list only widens sources), built once.
var studioSchema = studioFields(nil)

// studioFieldByCanonical resolves a canonical name against the synthesized studio schema.
func studioFieldByCanonical(canonical string) (mapping.Field, bool) {
	canonical = strings.ToLower(strings.TrimSpace(canonical))
	for _, f := range studioSchema {
		if f.Canonical == canonical {
			return f, true
		}
	}
	return mapping.Field{}, false
}

// studioProviders lists the provider namespaces the synthesized studio fields
// consult: the registry's studio-capable providers plus any provider already matched
// to this studio — so stored shadow values keep rendering even if their registry
// entry is later disabled. Mirrors personProviders.
func (h *Handlers) studioProviders(rows []repo.EnrichmentRow) []string {
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	if h.enrich != nil {
		for _, s := range h.enrich.Sources() {
			if slices.Contains(s.EntityTypes, model.EnrichEntityStudio) {
				add(s.Name)
			}
		}
	}
	for _, row := range rows {
		add(row.Provider)
	}
	return out
}

// studioResolved resolves a studio's fields through the unified resolver (F38): the
// record baseline + shadow enrichment + curation + standing decisions, in the record
// vocabulary. Mirrors personResolve; degraded reads log and resolve without the
// failing layer. Also returns the synthesized field list, so callers (e.g. the F55
// completeness breakdown panel) can score the same resolve pass instead of
// re-resolving from scratch.
func (h *Handlers) studioResolved(r *http.Request, id int64, s *model.Studio) ([]resolver.ResolvedField, []mapping.Field) {
	return h.resolveStudio(r.Context(), id, s)
}

// resolveStudio is the ctx-based core of studioResolved, so it is callable off the
// request path. Degraded reads log and resolve without the failing layer.
func (h *Handlers) resolveStudio(ctx context.Context, id int64, s *model.Studio) ([]resolver.ResolvedField, []mapping.Field) {
	rows, err := h.repo.EnrichmentForEntity(ctx, model.EnrichEntityStudio, id)
	if err != nil {
		h.log.Warn("enrichment for studio detail", "id", id, "err", err)
		rows = nil
	}
	var cur resolver.Curation
	if curRows, curErr := h.repo.CurationForEntity(ctx, model.EnrichEntityStudio, id); curErr != nil {
		h.log.Warn("curation for studio detail", "id", id, "err", curErr)
	} else {
		cur = curationFromRows(curRows)
	}
	var dec resolver.Decisions
	if decRows, decErr := h.repo.DecisionsForEntity(ctx, model.EnrichEntityStudio, id); decErr != nil {
		h.log.Warn("decisions for studio detail", "id", id, "err", decErr)
	} else {
		dec = decisionsFromRows(decRows)
	}
	fields := studioFields(h.studioProviders(rows))
	fields, promoted := h.mergePromotions(ctx, model.EnrichEntityStudio, fields, rows)
	fields = h.mergeClaims(ctx, model.EnrichEntityStudio, fields)
	resolved := resolver.ResolveFields(resolver.NewStudioBaseline(s), enrichmentFromRows(rows), cur, fields, h.resolveOptions(dec))
	h.markPromoted(resolved, promoted)
	return h.appendAutoRegistered(ctx, rows, fields, recordizeResolved(resolved)), fields
}

// listStudios handles GET /studios (F38): name-sorted (or count-sorted, or
// completeness-sorted/filtered) studios with active-video counts. Public,
// mirroring people/tags, except sort=completeness_asc|completeness_desc and
// the repeatable missing_facet param, which are owner-only (F55.5/F55.6,
// ADR-081 D4), same posture as listMedia/listPeople. Empty studios never appear.
func (h *Handlers) listStudios(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sort := q.Get("sort")
	missingFacets := q["missing_facet"]
	if wantsCompleteness(sort, missingFacets) {
		if !h.requireOwnerInline(w, r) {
			return
		}
		h.listStudiosByCompleteness(w, r, sort == sortCompletenessDesc, missingFacets)
		return
	}
	studios, err := h.repo.ListStudios(r.Context(), sort == "count")
	if err != nil {
		h.fail(w, "list studios", err)
		return
	}
	for i := range studios {
		setStudioImageURLs(&studios[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": studios})
}

// listStudiosByCompleteness serves GET /studios once listStudios has
// determined the request is completeness-sorted or missing-facet-filtered
// (F55.5/F55.6). No other browse filter or pagination to preserve, like
// people. Caller has already checked owner auth.
func (h *Handlers) listStudiosByCompleteness(w http.ResponseWriter, r *http.Request, desc bool, missingFacets []string) {
	scored, err := h.completenessForStudios(r.Context())
	if err != nil {
		h.fail(w, "list studios by completeness", err)
		return
	}
	writeCompletenessList(w, scored, missingFacets, desc,
		func(sc StudioCompleteness) resolver.Completeness { return sc.Completeness },
		func(sc StudioCompleteness) model.Studio { return sc.Studio },
	)
}

// getStudio handles GET /studios/{id} (F38): the studio, its resolved[] fields (record
// vocabulary, no in_sync), and its videos. Public.
func (h *Handlers) getStudio(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s, err := h.repo.GetStudio(r.Context(), id)
	if err != nil {
		h.studioLookupError(w, err)
		return
	}
	setStudioImageURLs(s)
	items, total, err := h.repo.ListVideos(r.Context(), repo.VideoFilter{StudioIDs: []int64{id}, Limit: 500, HideFullFilmVideos: h.filmsEnabled})
	if err != nil {
		h.fail(w, "studio videos", err)
		return
	}
	authorized := h.auth.authorized(r)
	redactFileMetadataForVisitors(items, authorized)
	resolved, fields := h.studioResolved(r, id, s)
	var completeness *resolver.Completeness
	if authorized {
		na, naErr := h.repo.FacetsNotApplicableForEntity(r.Context(), model.EnrichEntityStudio, id)
		if naErr != nil {
			h.log.Warn("facets not applicable for studio detail", "id", id, "err", naErr)
			na = map[string]bool{}
		}
		// branding_image is delivered as an asset (studio_images), never a field
		// value — resolved if any of the icon/logo/poster roles is set (F55.13),
		// same signal completenessForStudios uses.
		cFields, cResolved := injectSyntheticFacet(fields, resolved, "branding_image", registry.Lookup("branding_image").Label,
			len(s.ImageVersions) > 0)
		// Same spine-backed facet the person path injects (F58/ADR-088 D7); GetStudio
		// already loaded s.Aliases for the panel.
		cFields, cResolved = injectSyntheticFacet(cFields, cResolved, "alternate_names",
			registry.Lookup("alternate_names").Label, len(s.Aliases) > 0)
		c := resolver.Complete(cFields, cResolved, na)
		completeness = &c
	}
	// HOLODEX-266 (ADR-083): the provider-link badge projection — best-effort, a
	// lookup failure logs and serves the page with no badges rather than failing it.
	links, linksErr := h.externalLinksForEntity(r.Context(), model.EnrichEntityStudio, id)
	if linksErr != nil {
		h.log.Warn("external links for studio detail", "id", id, "err", linksErr)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"studio": s, "items": items, "total": total,
		"resolved":       resolved,
		"completeness":   completeness,
		"external_links": links,
	})
}

func (h *Handlers) studioLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "studio not found")
		return
	}
	h.fail(w, "get studio", err)
}

// relinkStudios re-derives a video's studio links, best-effort: a relink failure is
// logged, never failing the user action that triggered it (the decision/curation/
// enrichment write already committed; links self-heal on the next trigger or the
// startup backfill).
func (h *Handlers) relinkStudios(ctx context.Context, videoID int64) {
	if err := h.RelinkVideoStudios(ctx, videoID); err != nil {
		h.log.Warn("relink studios", "video", videoID, "err", err)
	}
}

// relinkStudiosWithContext reconciles video_studios directly from a relinkContext and
// resolved names a caller already has in hand (HOLODEX-271's studioCollision, which
// resolves the pending decision to check for a composite-key collision before it
// commits) — skipping the fetch-and-resolve relinkStudios/RelinkVideoStudios would
// otherwise repeat immediately afterward for the same video and the same decision.
// Falls back to the normal fetch-on-call path if rc is nil (loadRelinkContext found
// no live video), best-effort like relinkStudios.
func (h *Handlers) relinkStudiosWithContext(ctx context.Context, videoID int64, rc *relinkContext, names []string) {
	if rc == nil {
		h.relinkStudios(ctx, videoID)
		return
	}
	if err := h.repo.ReconcileVideoStudios(ctx, videoID, names, studioExternalIDsFromRows(rc.enrRows)); err != nil {
		h.log.Warn("relink studios", "video", videoID, "err", err)
	}
}

// RelinkVideoStudios re-derives a video's studio links from its RESOLVED `studio`
// field and reconciles video_studios (ADR-053 RD1). It is the single resolution
// entry point behind every relink trigger (scan/enrich/decision/curation) — the repo
// reconcile is the sole writer of the table. A missing/soft-deleted video resolves
// to no names → all links removed + prune-on-empty. Safe to call redundantly;
// idempotent.
func (h *Handlers) RelinkVideoStudios(ctx context.Context, videoID int64) error {
	return h.relinkVideoStudios(ctx, videoID, nil)
}

// relinkVideoStudios is RelinkVideoStudios' implementation, taking an optional
// pre-fetched relinkContext (nil fetches it here) so RelinkVideoEntity can share
// one fetch across both entity kinds — see loadRelinkContext in person_links.go.
func (h *Handlers) relinkVideoStudios(ctx context.Context, videoID int64, rc *relinkContext) error {
	if h.mappings == nil {
		return nil
	}
	studioField, ok := h.mappings.Current().ByCanonical("studio")
	if !ok {
		// No studio field is configured. This is NOT the same as "resolved to zero
		// studios" — it means metadata-mappings.yaml doesn't map studio (yet), so
		// this reconcile has no opinion at all. Leave any existing links untouched
		// rather than treating "unconfigured" as an affirmative empty result and
		// wiping them: the same conflation PR #216 fixed for people (HOLODEX-256).
		// Studio has it worse than person did — ReconcileVideoStudios prunes
		// immediately with no orphan grace (ADR-053 §2.4), so this path used to
		// permanently delete every studio link and every studio entity the moment
		// an instance's config omitted `studio`.
		return nil
	}
	if rc == nil {
		var err error
		rc, err = h.loadRelinkContext(ctx, videoID)
		if err != nil {
			return err
		}
	}
	if rc == nil {
		return h.repo.ReconcileVideoStudios(ctx, videoID, nil, nil)
	}
	names := h.resolveStudioNames(rc, studioField, decisionsFromRows(rc.decRows))
	return h.repo.ReconcileVideoStudios(ctx, videoID, names, studioExternalIDsFromRows(rc.enrRows))
}

// resolveStudioNames resolves the studio field against rc under decisions and
// extracts the canonical `studio` values — the single resolve+extract shared by
// relinkVideoStudios (the committed state) and decisions.go's studioCollision (a
// pending decision's proposed state, before it commits), which independently
// duplicated this same loop (HOLODEX-271 review fix).
func (h *Handlers) resolveStudioNames(rc *relinkContext, studioField mapping.Field, decisions resolver.Decisions) []string {
	resolved := resolver.Resolve(rc.video, rc.extra, enrichmentFromRows(rc.enrRows), curationFromRows(rc.curRows),
		[]mapping.Field{studioField}, h.resolveOptions(decisions))
	var names []string
	for _, rf := range resolved {
		if strings.EqualFold(rf.Canonical, "studio") {
			names = append(names, rf.Values...)
		}
	}
	return names
}

// studioExternalIDsFromRows builds a resolved-name → provider external-id side-map
// from a video's internal _studio_external_ids sidecar rows (ADR-054). See
// externalIDsFromRows for the shared parse.
func studioExternalIDsFromRows(rows []repo.EnrichmentRow) map[string]string {
	return externalIDsFromRows(rows, model.StudioExternalIDsField)
}

// externalIDsFromRows builds a resolved-name → provider external-id side-map from an
// internal sidecar field's rows — the shared parse behind studioExternalIDsFromRows
// (ADR-054) and personExternalIDsFromRows (F32, ADR-055, person_links.go). Each
// sidecar value is "<external_id> <name>" (the id token has no space, so the name is
// the unambiguous remainder). Keyed by the raw trimmed provider name (see repo's
// extIDFor for the casing-transform-drift fallback this implies for callers); a name
// with no entry resolves by name only. Returns nil when no ids are present.
//
// Two distinct entities sharing an exact display name collide on this key —
// first-write-wins, deterministic rather than depending on row order. This is a
// pre-existing limit of name-keyed linking (the resolved field can only carry one
// display entry per name, so at most one of the two could ever be linked here
// regardless of map shape), not something this map could fix alone.
func externalIDsFromRows(rows []repo.EnrichmentRow, fieldKey string) map[string]string {
	var out map[string]string
	for _, row := range rows {
		if row.FieldKey != fieldKey {
			continue
		}
		for _, v := range row.Values {
			sep := strings.IndexByte(v, ' ')
			if sep <= 0 {
				continue
			}
			extID := strings.TrimSpace(v[:sep])
			name := strings.TrimSpace(v[sep+1:])
			if extID == "" || name == "" {
				continue
			}
			if out == nil {
				out = make(map[string]string)
			}
			if _, collision := out[name]; collision {
				continue
			}
			out[name] = extID
		}
	}
	return out
}
