package api

import (
	"context"
	"fmt"

	"holodex/internal/fieldsource"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// List-wide entity completeness (F55, ADR-081 D4). Browse-page completeness
// sort, the "missing facet" filter, and the remediation queue all read the
// same three functions below — the one backend predicate the design handoff
// (§9/§1) requires. Each fetches the full per-type entity set (bypassing SQL
// LIMIT/OFFSET, per D4), resolves every entity's fields exactly as its detail
// handler does, and scores it via resolver.Complete — leaving sort, filter,
// and pagination to the caller in Go, not here (D4 draws that seam at "return
// a scored slice," not at any particular consumer's shape).
//
// Every entity in the returned set is resolved and scored — the ADR's cost
// envelope is personal-library scale (hundreds to low thousands), the same
// assumption ExtractionQueue's full-table read already leans on.

// VideoCompleteness pairs one video with its computed completeness.
type VideoCompleteness struct {
	Video        model.Video           `json:"video"`
	Completeness resolver.Completeness `json:"completeness"`
}

// PersonCompleteness pairs one person with its computed completeness.
type PersonCompleteness struct {
	Person       model.Person          `json:"person"`
	Completeness resolver.Completeness `json:"completeness"`
}

// StudioCompleteness pairs one studio with its computed completeness.
type StudioCompleteness struct {
	Studio       model.Studio          `json:"studio"`
	Completeness resolver.Completeness `json:"completeness"`
}

// injectAssetFacet appends a synthetic field/resolved-row pair for a scored facet
// that is delivered as an image asset rather than a field value (person `photo`,
// studio `branding_image` — both registry.go doc comments say so explicitly) and
// therefore has no row from the normal resolve pipeline by construction. Mirrors
// Derive's computed-row shape (the pattern the D3 flightplan entry calls for
// branding_image specifically). present is stamped as a "manual:" winning source
// so classifyTier scores it at the curated tier: an asset is a binary present/
// absent state with no unapplied-candidate concept for the actionability signal
// to target, unlike a text field's provider/curated split — curated is the only
// tier that makes sense once the asset exists. Absent stays unrepresented in
// resolved (Complete's own missing-row convention), not a placeholder row.
func injectAssetFacet(fields []mapping.Field, resolved []resolver.ResolvedField, canonical, label string, present bool) ([]mapping.Field, []resolver.ResolvedField) {
	fields = append(fields, mapping.Field{Canonical: canonical, Label: label})
	if present {
		resolved = append(resolved, resolver.ResolvedField{
			Canonical:     canonical,
			Label:         label,
			WinningSource: fieldsource.Manual + ":" + canonical,
		})
	}
	return fields, resolved
}

// completenessForVideos resolves and scores every active video (ADR-081 D4).
// Mirrors getMedia's resolve pipeline per video, batch-loading each input
// instead of the detail handler's per-entity queries. Critically, unlike
// applyBrowseTitles, it loads ExtraMetadataForVideos: studio/actors (both
// critical facets) resolve from file tags that live only in ExtraMetadata,
// not on model.Video, so skipping it would misreport them as missing.
func (h *Handlers) completenessForVideos(ctx context.Context) ([]VideoCompleteness, error) {
	if h.mappings == nil {
		return nil, nil
	}
	videos, err := h.repo.ListAllVideos(ctx, repo.VideoFilter{})
	if err != nil {
		return nil, fmt.Errorf("list all videos: %w", err)
	}
	if len(videos) == 0 {
		return nil, nil
	}
	ids := make([]int64, len(videos))
	for i, v := range videos {
		ids[i] = v.ID
	}

	extraByVideo, err := h.repo.ExtraMetadataForVideos(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("extra metadata for videos: %w", err)
	}
	enrByVideo, err := h.repo.EnrichmentForVideos(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("enrichment for videos: %w", err)
	}
	curByVideo, err := h.repo.CurationForVideos(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("curation for videos: %w", err)
	}
	decByVideo, err := h.repo.DecisionsForVideos(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("decisions for videos: %w", err)
	}
	notApplicableByVideo, err := h.repo.FacetsNotApplicableForEntities(ctx, model.EnrichEntityVideo, ids)
	if err != nil {
		return nil, fmt.Errorf("facets not applicable for videos: %w", err)
	}

	m := h.mappings.Current()
	baseFields := m.Fields()
	out := make([]VideoCompleteness, len(videos))
	for i, v := range videos {
		rows := enrByVideo[v.ID]
		cur := curationFromRows(curByVideo[v.ID])
		dec := decisionsFromRows(decByVideo[v.ID])

		fields, promoted := h.mergePromotions(ctx, model.EnrichEntityVideo, baseFields, rows)
		fields = h.mergeClaims(ctx, model.EnrichEntityVideo, fields)
		resolved := resolver.Resolve(&v, extraByVideo[v.ID], enrichmentFromRows(rows), cur, fields, h.resolveOptions(dec))
		h.markPromoted(resolved, promoted)
		resolved = h.appendAutoRegistered(ctx, rows, fields, resolved)

		out[i] = VideoCompleteness{
			Video:        v,
			Completeness: resolver.Complete(fields, resolved, notApplicableByVideo[v.ID]),
		}
	}
	return out, nil
}

// completenessForPeople resolves and scores every person with at least one
// active video (ADR-081 D4). Mirrors personResolved's pipeline per person,
// batch-loading each input instead of personResolved's per-entity queries.
func (h *Handlers) completenessForPeople(ctx context.Context) ([]PersonCompleteness, error) {
	people, err := h.repo.ListPeople(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	if len(people) == 0 {
		return nil, nil
	}
	ids := make([]int64, len(people))
	for i, p := range people {
		ids[i] = p.ID
	}

	enrByPerson, err := h.repo.EnrichmentForEntities(ctx, model.EnrichEntityPerson, ids)
	if err != nil {
		return nil, fmt.Errorf("enrichment for people: %w", err)
	}
	curByPerson, err := h.repo.CurationForEntities(ctx, model.EnrichEntityPerson, ids)
	if err != nil {
		return nil, fmt.Errorf("curation for people: %w", err)
	}
	decByPerson, err := h.repo.DecisionsForEntities(ctx, model.EnrichEntityPerson, ids)
	if err != nil {
		return nil, fmt.Errorf("decisions for people: %w", err)
	}
	notApplicableByPerson, err := h.repo.FacetsNotApplicableForEntities(ctx, model.EnrichEntityPerson, ids)
	if err != nil {
		return nil, fmt.Errorf("facets not applicable for people: %w", err)
	}

	photoLabel := registry.Lookup("photo").Label
	out := make([]PersonCompleteness, len(people))
	for i, p := range people {
		rows := enrByPerson[p.ID]
		cur := curationFromRows(curByPerson[p.ID])
		dec := decisionsFromRows(decByPerson[p.ID])

		fields := personFields(h.personProviders(rows))
		fields, promoted := h.mergePromotions(ctx, model.EnrichEntityPerson, fields, rows)
		fields = h.mergeClaims(ctx, model.EnrichEntityPerson, fields)
		resolved := resolver.ResolveFields(resolver.NewPersonBaseline(&p), enrichmentFromRows(rows), cur, fields, h.resolveOptions(dec))
		h.markPromoted(resolved, promoted)
		resolved = h.appendAutoRegistered(ctx, rows, fields, personizeResolved(resolved))
		resolved = resolver.Derive(resolved, h.clock())
		// photo is delivered as an asset (person_images), never a field value
		// (personFields deliberately excludes it) — score it off HeadshotVersion,
		// the default-avatar role the singular "Portrait image" facet maps to.
		fields, resolved = injectAssetFacet(fields, resolved, "photo", photoLabel, p.HeadshotVersion != 0)

		out[i] = PersonCompleteness{
			Person:       p,
			Completeness: resolver.Complete(fields, resolved, notApplicableByPerson[p.ID]),
		}
	}
	return out, nil
}

// completenessForStudios resolves and scores every studio with at least one
// active video (ADR-081 D4). Mirrors resolveStudio's pipeline per studio,
// batch-loading each input instead of resolveStudio's per-entity queries.
func (h *Handlers) completenessForStudios(ctx context.Context) ([]StudioCompleteness, error) {
	studios, err := h.repo.ListStudios(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("list studios: %w", err)
	}
	if len(studios) == 0 {
		return nil, nil
	}
	ids := make([]int64, len(studios))
	for i, s := range studios {
		ids[i] = s.ID
	}

	enrByStudio, err := h.repo.EnrichmentForEntities(ctx, model.EnrichEntityStudio, ids)
	if err != nil {
		return nil, fmt.Errorf("enrichment for studios: %w", err)
	}
	curByStudio, err := h.repo.CurationForEntities(ctx, model.EnrichEntityStudio, ids)
	if err != nil {
		return nil, fmt.Errorf("curation for studios: %w", err)
	}
	decByStudio, err := h.repo.DecisionsForEntities(ctx, model.EnrichEntityStudio, ids)
	if err != nil {
		return nil, fmt.Errorf("decisions for studios: %w", err)
	}
	notApplicableByStudio, err := h.repo.FacetsNotApplicableForEntities(ctx, model.EnrichEntityStudio, ids)
	if err != nil {
		return nil, fmt.Errorf("facets not applicable for studios: %w", err)
	}

	brandingLabel := registry.Lookup("branding_image").Label
	out := make([]StudioCompleteness, len(studios))
	for i, s := range studios {
		rows := enrByStudio[s.ID]
		cur := curationFromRows(curByStudio[s.ID])
		dec := decisionsFromRows(decByStudio[s.ID])

		fields := studioFields(h.studioProviders(rows))
		fields, promoted := h.mergePromotions(ctx, model.EnrichEntityStudio, fields, rows)
		fields = h.mergeClaims(ctx, model.EnrichEntityStudio, fields)
		resolved := resolver.ResolveFields(resolver.NewStudioBaseline(&s), enrichmentFromRows(rows), cur, fields, h.resolveOptions(dec))
		h.markPromoted(resolved, promoted)
		resolved = h.appendAutoRegistered(ctx, rows, fields, recordizeResolved(resolved))
		// branding_image is delivered as an asset (studio_images), never a field
		// value — resolved if any of the icon/logo/poster roles is set (spec
		// F55.13), which ListStudios already batches onto s.ImageVersions.
		fields, resolved = injectAssetFacet(fields, resolved, "branding_image", brandingLabel, len(s.ImageVersions) > 0)

		out[i] = StudioCompleteness{
			Studio:       s,
			Completeness: resolver.Complete(fields, resolved, notApplicableByStudio[s.ID]),
		}
	}
	return out, nil
}
