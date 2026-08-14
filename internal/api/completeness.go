package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"

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

// completenessForVideos resolves and scores every active video matching f
// (ADR-081 D4). Mirrors getMedia's resolve pipeline per video, batch-loading
// each input instead of the detail handler's per-entity queries. Critically,
// unlike applyBrowseTitles, it loads ExtraMetadataForVideos: studio/actors
// (both critical facets) resolve from file tags that live only in
// ExtraMetadata, not on model.Video, so skipping it would misreport them as
// missing.
//
// f carries the caller's existing browse filters (tags/person/studio/query/
// duration/year/mapped) so completeness sort and the missing-facet filter
// compose with them instead of scoring the whole library regardless of what
// the caller is looking at; f.Limit/Offset are ignored (ListAllVideos, D4).
func (h *Handlers) completenessForVideos(ctx context.Context, f repo.VideoFilter) ([]VideoCompleteness, error) {
	if h.mappings == nil {
		return nil, nil
	}
	videos, err := h.repo.ListAllVideos(ctx, f)
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
		// P0-10 (F50, ADR-075 RD9): same genre-writeback union getMedia applies
		// (handlers.go) — without it, a video whose only genre source is
		// manually-attached tags scores as missing genres despite its detail
		// page showing them (see applyGenreWriteback's doc comment).
		if field, ok := m.ByCanonical("genres"); ok {
			rawGenres, rawOK := resolvedByCanonical(resolved, "genres")
			if items, gerr := h.genreWritebackItemsFrom(ctx, v.ID, rawGenres, rawOK); gerr != nil {
				h.log.Warn("genre writeback items for completeness", "id", v.ID, "err", gerr)
			} else {
				resolved = applyGenreWriteback(resolved, field, items)
			}
		}

		out[i] = VideoCompleteness{
			Video:        v,
			Completeness: resolver.Complete(fields, resolved, notApplicableByVideo[v.ID]),
		}
	}
	return out, nil
}

// entityCompletenessBatch bundles the four per-entity-type batch loads
// completenessForPeople and completenessForStudios both need (D4) — factored
// out so the two functions don't repeat the same four-call fetch prologue.
type entityCompletenessBatch struct {
	enrichment    map[int64][]repo.EnrichmentRow
	curation      map[int64][]repo.CurationRow
	decisions     map[int64][]repo.DecisionRow
	notApplicable map[int64]map[string]bool
}

func (h *Handlers) loadEntityCompletenessBatch(ctx context.Context, entityType string, ids []int64) (entityCompletenessBatch, error) {
	var b entityCompletenessBatch
	var err error
	if b.enrichment, err = h.repo.EnrichmentForEntities(ctx, entityType, ids); err != nil {
		return b, fmt.Errorf("enrichment for %s: %w", entityType, err)
	}
	if b.curation, err = h.repo.CurationForEntities(ctx, entityType, ids); err != nil {
		return b, fmt.Errorf("curation for %s: %w", entityType, err)
	}
	if b.decisions, err = h.repo.DecisionsForEntities(ctx, entityType, ids); err != nil {
		return b, fmt.Errorf("decisions for %s: %w", entityType, err)
	}
	if b.notApplicable, err = h.repo.FacetsNotApplicableForEntities(ctx, entityType, ids); err != nil {
		return b, fmt.Errorf("facets not applicable for %s: %w", entityType, err)
	}
	return b, nil
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

	batch, err := h.loadEntityCompletenessBatch(ctx, model.EnrichEntityPerson, ids)
	if err != nil {
		return nil, err
	}

	photoLabel := registry.Lookup("photo").Label
	out := make([]PersonCompleteness, len(people))
	for i, p := range people {
		rows := batch.enrichment[p.ID]
		cur := curationFromRows(batch.curation[p.ID])
		dec := decisionsFromRows(batch.decisions[p.ID])

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
			Completeness: resolver.Complete(fields, resolved, batch.notApplicable[p.ID]),
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

	batch, err := h.loadEntityCompletenessBatch(ctx, model.EnrichEntityStudio, ids)
	if err != nil {
		return nil, err
	}

	brandingLabel := registry.Lookup("branding_image").Label
	out := make([]StudioCompleteness, len(studios))
	for i, s := range studios {
		rows := batch.enrichment[s.ID]
		cur := curationFromRows(batch.curation[s.ID])
		dec := decisionsFromRows(batch.decisions[s.ID])

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
		// Every other studio-serializing path (listStudios, listStudiosByCompleteness,
		// getStudio) populates the derived IconURL/LogoURL/PosterURL before returning
		// the Studio; do the same here so queue/browse consumers don't always see it empty.
		setStudioImageURLs(&s)

		out[i] = StudioCompleteness{
			Studio:       s,
			Completeness: resolver.Complete(fields, resolved, batch.notApplicable[s.ID]),
		}
	}
	return out, nil
}

// Sort keys the browse "Completeness" sort recognizes (F55.5); a listMedia/
// listPeople/listStudios request specifying either one, or any missing_facet
// param, routes to the Go-side completeness path below instead of the normal
// SQL-paginated one.
const (
	sortCompletenessAsc  = "completeness_asc"
	sortCompletenessDesc = "completeness_desc"
)

// wantsCompleteness reports whether a listMedia/listPeople/listStudios request
// needs the owner-gated completeness path — either sort asks for it, or any
// missing_facet is present.
func wantsCompleteness(sort string, missingFacets []string) bool {
	return sort == sortCompletenessAsc || sort == sortCompletenessDesc || len(missingFacets) > 0
}

// FacetSummary is one scored facet's metadata plus how many entities in the
// scored set are currently missing it — the "Missing facet" filter chip's
// option list and live counts (F55.6). Built by summarizeFacets from the same
// completeness pass a sort/filter request scores, so the chip's counts can
// never disagree with what the filter itself returns.
type FacetSummary struct {
	Canonical    string `json:"canonical"`
	Label        string `json:"label"`
	Criticality  string `json:"criticality"`
	MissingCount int    `json:"missing_count"`
}

// summarizeFacets aggregates missing-facet counts across a scored entity set,
// in first-seen facet order (stable, since every entity in a set is scored
// against the same field list). Not-applicable facets never count as missing.
func summarizeFacets[T any](items []T, score func(T) resolver.Completeness) []FacetSummary {
	var order []string
	byCanonical := make(map[string]*FacetSummary)
	for _, item := range items {
		for _, f := range score(item).Facets {
			if f.NotApplicable {
				continue
			}
			s, ok := byCanonical[f.Canonical]
			if !ok {
				s = &FacetSummary{Canonical: f.Canonical, Label: f.Label, Criticality: f.Criticality}
				byCanonical[f.Canonical] = s
				order = append(order, f.Canonical)
			}
			if f.Tier == resolver.TierMissing {
				s.MissingCount++
			}
		}
	}
	out := make([]FacetSummary, len(order))
	for i, c := range order {
		out[i] = *byCanonical[c]
	}
	return out
}

// isMissingAll reports whether every canonical in want is a missing, non-not-
// applicable facet on c — AND semantics across multiple selections, matching
// this app's existing multi-select filter convention (TagIDs, StudioIDs).
func isMissingAll(c resolver.Completeness, want []string) bool {
	if len(want) == 0 {
		return true
	}
	missing := make(map[string]bool, len(c.Facets))
	for _, f := range c.Facets {
		if f.Tier == resolver.TierMissing && !f.NotApplicable {
			missing[f.Canonical] = true
		}
	}
	for _, w := range want {
		if !missing[w] {
			return false
		}
	}
	return true
}

// sortByScore orders items by completeness score in place; desc for highest-
// first. Stable, so items tied on score keep the caller's prior relative
// order (e.g. the underlying SQL sort, or ListAllVideos' added-desc default).
func sortByScore[T any](items []T, score func(T) int, desc bool) {
	sort.SliceStable(items, func(i, j int) bool {
		si, sj := score(items[i]), score(items[j])
		if desc {
			return si > sj
		}
		return si < sj
	})
}

// writeCompletenessList is listPeopleByCompleteness/listStudiosByCompleteness's
// shared body: filter by missing_facet, sort by score, map each scored item down
// to its plain entity, and write the {"items": [...]} response. completenessOf
// and toEntity let each caller supply its own pairing type and entity mapping
// (e.g. studios also stamp image URLs) without duplicating the rest.
func writeCompletenessList[T any, E any](w http.ResponseWriter, scored []T, missingFacets []string, desc bool, completenessOf func(T) resolver.Completeness, toEntity func(T) E) {
	filtered := make([]T, 0, len(scored))
	for _, item := range scored {
		if isMissingAll(completenessOf(item), missingFacets) {
			filtered = append(filtered, item)
		}
	}
	sortByScore(filtered, func(item T) int { return completenessOf(item).Score }, desc)
	items := make([]E, len(filtered))
	for i, item := range filtered {
		items[i] = toEntity(item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
