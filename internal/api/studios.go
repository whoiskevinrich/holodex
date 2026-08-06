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
// vocabulary. Mirrors personResolved; degraded reads log and resolve without the
// failing layer.
func (h *Handlers) studioResolved(r *http.Request, id int64, s *model.Studio) []resolver.ResolvedField {
	return h.resolveStudio(r.Context(), id, s)
}

// resolveStudio is the ctx-based core of studioResolved, so it is callable off the
// request path. Degraded reads log and resolve without the failing layer.
func (h *Handlers) resolveStudio(ctx context.Context, id int64, s *model.Studio) []resolver.ResolvedField {
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
	return h.appendAutoRegistered(ctx, rows, fields, recordizeResolved(resolved))
}

// listStudios handles GET /studios (F38): name-sorted (or count-sorted) studios with
// active-video counts. Public, mirroring people/tags. Empty studios never appear.
func (h *Handlers) listStudios(w http.ResponseWriter, r *http.Request) {
	studios, err := h.repo.ListStudios(r.Context(), r.URL.Query().Get("sort") == "count")
	if err != nil {
		h.fail(w, "list studios", err)
		return
	}
	for i := range studios {
		setStudioImageURLs(&studios[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": studios})
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
	items, total, err := h.repo.ListVideos(r.Context(), repo.VideoFilter{StudioIDs: []int64{id}, Limit: 500})
	if err != nil {
		h.fail(w, "studio videos", err)
		return
	}
	redactFileMetadataForVisitors(items, h.auth.authorized(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"studio": s, "items": items, "total": total,
		"resolved": h.studioResolved(r, id, s),
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
	resolved := resolver.Resolve(rc.video, rc.extra, enrichmentFromRows(rc.enrRows), curationFromRows(rc.curRows),
		[]mapping.Field{studioField}, h.resolveOptions(decisionsFromRows(rc.decRows)))
	var names []string
	for _, rf := range resolved {
		if strings.EqualFold(rf.Canonical, "studio") {
			names = append(names, rf.Values...)
		}
	}
	return h.repo.ReconcileVideoStudios(ctx, videoID, names, studioExternalIDsFromRows(rc.enrRows))
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
// the unambiguous remainder). Keyed by trimmed name so it survives the resolver's
// reordering/curation; a name with no entry resolves by name only. Returns nil when
// no ids are present.
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
			out[name] = extID
		}
	}
	return out
}
