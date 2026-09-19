package api

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"holodex/internal/fieldsource"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// Entity completeness (F55; v2 F65, ADR-099). The three completenessFor*
// functions below resolve and score a set of entities exactly as their detail
// handlers do — the one backend predicate the design handoff (§9/§1) requires.
// Two consumers remain on the live pass: the remediation queue (it needs
// actionability, whose candidate inputs are not stored) and the dirty-set
// drain (drainCompleteness), which is how the materialized store
// (entity_completeness, ADR-099 D3) gets filled. Every list surface — browse
// sort, the "Missing facet" chip and its counts, the ring badge — reads the
// store in SQL instead (repo.VideoFilter / repo.NamedListFilter).

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

// injectSyntheticFacet appends a synthetic field/resolved-row pair for a scored facet
// that has no row from the normal resolve pipeline by construction — either because it
// is delivered as an image asset rather than a field value (person `photo`, studio
// `branding_image`) or because it lives in the identity spine rather than in the field
// model at all (`alternate_names`, F58/ADR-088 D7). Every registry.go doc comment for
// these says so explicitly. Named for the shape rather than for assets, since the
// asset-ness was always incidental to what this does. Mirrors
// Derive's computed-row shape (the pattern the D3 flightplan entry calls for
// branding_image specifically). present is stamped as a "manual:" winning source
// so classifyTier scores it at the curated tier: an asset is a binary present/
// absent state with no unapplied-candidate concept for the actionability signal
// to target, unlike a text field's provider/curated split — curated is the only
// tier that makes sense once the asset exists. Absent stays unrepresented in
// resolved (Complete's own missing-row convention), not a placeholder row.
func injectSyntheticFacet(fields []mapping.Field, resolved []resolver.ResolvedField, canonical, label string, present bool) ([]mapping.Field, []resolver.ResolvedField) {
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

// completenessForVideos resolves and scores every active video matching f —
// the drain's dirty ids (f.IDs) or the queue's whole library. Mirrors getMedia's
// resolve pipeline per video, batch-loading each input instead of the detail
// handler's per-entity queries. Critically,
// unlike applyBrowseTitles, it loads ExtraMetadataForVideos: studio/actors
// (both critical facets) resolve from file tags that live only in
// ExtraMetadata, not on model.Video, so skipping it would misreport them as
// missing.
//
// f carries the caller's existing browse filters (tags/person/studio/query/
// duration/year/mapped); f.Limit/Offset are ignored (ListAllVideos).
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

// entityCompletenessBatch bundles the per-entity-type batch loads
// completenessForPeople and completenessForStudios both need (D4) — factored
// out so the two functions do not repeat the same fetch prologue.
type entityCompletenessBatch struct {
	enrichment    map[int64][]repo.EnrichmentRow
	curation      map[int64][]repo.CurationRow
	decisions     map[int64][]repo.DecisionRow
	notApplicable map[int64]map[string]bool
	aliases       map[int64][]model.EntityAlias
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
	// alternate_names scores off the identity spine, which no resolve pass reads
	// (F58/ADR-088 D7). AliasesForEntities is the existing batch read, so this stays
	// one query for the whole page rather than a per-entity N+1.
	if b.aliases, err = h.repo.AliasesForEntities(ctx, entityType, ids); err != nil {
		return b, fmt.Errorf("aliases for %s: %w", entityType, err)
	}
	return b, nil
}

// completenessForPeople resolves and scores every person with at least one
// active video matching f (the drain's dirty ids, or the queue's whole set).
// Mirrors personResolved's pipeline per person, batch-loading each input
// instead of personResolved's per-entity queries.
func (h *Handlers) completenessForPeople(ctx context.Context, f repo.NamedListFilter) ([]PersonCompleteness, error) {
	people, err := h.repo.ListPeopleFiltered(ctx, f)
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
	aliasLabel := registry.Lookup("alternate_names").Label
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
		fields, resolved = injectSyntheticFacet(fields, resolved, "photo", photoLabel, p.HeadshotVersion != 0)
		// alternate_names replaces the retired `aliases` field facet (F58/ADR-088 D1/D7):
		// scored off entity_aliases directly, and blind to `source` — a name the owner
		// typed and one a provider supplied count exactly the same.
		fields, resolved = injectSyntheticFacet(fields, resolved, "alternate_names", aliasLabel, len(batch.aliases[p.ID]) > 0)

		out[i] = PersonCompleteness{
			Person:       p,
			Completeness: resolver.Complete(fields, resolved, batch.notApplicable[p.ID]),
		}
	}
	return out, nil
}

// completenessForStudios resolves and scores every studio with at least one
// active video matching f (the drain's dirty ids, or the queue's whole set).
// Mirrors resolveStudio's pipeline per studio, batch-loading each input
// instead of resolveStudio's per-entity queries.
func (h *Handlers) completenessForStudios(ctx context.Context, f repo.NamedListFilter) ([]StudioCompleteness, error) {
	studios, err := h.repo.ListStudiosFiltered(ctx, f)
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
	aliasLabel := registry.Lookup("alternate_names").Label
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
		fields, resolved = injectSyntheticFacet(fields, resolved, "branding_image", brandingLabel, len(s.ImageVersions) > 0)
		fields, resolved = injectSyntheticFacet(fields, resolved, "alternate_names", aliasLabel, len(batch.aliases[s.ID]) > 0)
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

// wantsCompleteness reports whether a listMedia/listPeople/listStudios request
// touches the owner-only completeness surface (F55.5/F55.6) — either sort asks
// for it, or any missing_facet is present — so a visitor is rejected rather
// than silently served the default order.
func wantsCompleteness(sort string, missingFacets []string) bool {
	return sort == repo.SortCompletenessAsc || sort == repo.SortCompletenessDesc || len(missingFacets) > 0
}

// drainCompleteness recomputes every entity in completeness_dirty and writes
// the store (ADR-099 D4): the triggers in migration 0049 fill the set on every
// input-table write, and the owner-gated store readers (listMedia, listPeople,
// listStudios, completenessFacets) call this first, so a badge or sort never
// reflects a stale row. Cost is O(entities mutated since the last owner read);
// the repo holds writeMu for the whole read, resolve, write, which also
// serializes two owner requests in flight together (the SPA fires /media and
// /completeness/facets side by side) — the second finds an empty set.
//
// Best-effort by design: the badge is not the page. A failure is logged and the
// caller serves the list from whatever the store holds (a stale ring, or none)
// rather than turning every owner browse page into a 500 over one entity whose
// resolve blew up; the ids stay dirty, so the next owner read retries.
func (h *Handlers) drainCompleteness(ctx context.Context) {
	if err := h.drainCompletenessStrict(ctx); err != nil {
		h.log.Warn("completeness drain", "err", err)
	}
}

func (h *Handlers) drainCompletenessStrict(ctx context.Context) error {
	return h.repo.DrainCompleteness(ctx, func(entityType string, ids []int64) ([]repo.CompletenessRow, error) {
		var rows []repo.CompletenessRow
		switch entityType {
		case model.EnrichEntityVideo:
			scored, err := h.completenessForVideos(ctx, repo.VideoFilter{IDs: ids})
			if err != nil {
				return nil, err
			}
			for _, vc := range scored {
				rows = append(rows, completenessRow(entityType, vc.Video.ID, vc.Completeness))
			}
		case model.EnrichEntityPerson:
			scored, err := h.completenessForPeople(ctx, repo.NamedListFilter{IDs: ids})
			if err != nil {
				return nil, err
			}
			for _, pc := range scored {
				rows = append(rows, completenessRow(entityType, pc.Person.ID, pc.Completeness))
			}
		case model.EnrichEntityStudio:
			scored, err := h.completenessForStudios(ctx, repo.NamedListFilter{IDs: ids})
			if err != nil {
				return nil, err
			}
			for _, sc := range scored {
				rows = append(rows, completenessRow(entityType, sc.Studio.ID, sc.Completeness))
			}
			// default: nothing scores this entity type (a film's shadow-store
			// write lands here) — no rows, and the drain clears the flags.
		}
		return rows, nil
	})
}

// completenessRow projects a live Completeness onto the store's shape: the
// two bands plus the missing scored facets with their band.
func completenessRow(entityType string, id int64, c resolver.Completeness) repo.CompletenessRow {
	row := repo.CompletenessRow{EntityType: entityType, EntityID: id, Required: c.Required, Extras: c.Extras}
	for _, f := range c.Missing() {
		row.Missing = append(row.Missing, repo.MissingFacet{Canonical: f.Canonical, Band: f.Criticality})
	}
	return row
}

// selfHealCompleteness is the detail page's belt-and-braces over the triggers
// (ADR-099 D3): the page always computes live, and when that result differs
// from the stored row — or there is none — it rewrites the row, so a stale
// badge can never outlive a look at the entity. The dirty flag is left to the
// drain (see repo.StoreCompleteness). Best-effort: a store failure logs and
// the page is served regardless.
func (h *Handlers) selfHealCompleteness(ctx context.Context, entityType string, id int64, c resolver.Completeness) {
	want := completenessRow(entityType, id, c)
	have, err := h.repo.StoredCompleteness(ctx, entityType, id)
	switch {
	case err == nil && sameCompletenessRow(*have, want):
		return
	case err != nil && !errors.Is(err, repo.ErrNotFound):
		h.log.Warn("completeness self-heal read", "type", entityType, "id", id, "err", err)
		return
	}
	if err := h.repo.StoreCompleteness(ctx, want); err != nil {
		h.log.Warn("completeness self-heal write", "type", entityType, "id", id, "err", err)
	}
}

func sameCompletenessRow(a, b repo.CompletenessRow) bool {
	return sameIntPtr(a.Required, b.Required) && sameIntPtr(a.Extras, b.Extras) &&
		missingKey(a.Missing) == missingKey(b.Missing)
}

func sameIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// missingKey renders a missing-facet set order-independently for comparison.
func missingKey(m []repo.MissingFacet) string {
	parts := make([]string, len(m))
	for i, f := range m {
		parts[i] = f.Canonical + "=" + f.Band
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// attachCompleteness stamps the owner-only ring-badge payload (F65.5, ADR-099
// D5) onto one page of list items from the store. Never called for a visitor
// — the caller gates on h.auth.authorized, the same seam that strips file
// metadata — so a visitor's items simply never carry the field.
func attachCompleteness[T any](ctx context.Context, h *Handlers, entityType string, items []T, id func(*T) int64, slot func(*T) **model.CompletenessSummary) error {
	ids := make([]int64, len(items))
	for i := range items {
		ids[i] = id(&items[i])
	}
	stored, err := h.repo.CompletenessForEntities(ctx, entityType, ids)
	if err != nil {
		return err
	}
	for i := range items {
		if c, ok := stored[id(&items[i])]; ok {
			*slot(&items[i]) = &c
		}
	}
	return nil
}

func (h *Handlers) attachVideoCompleteness(ctx context.Context, items []model.Video) error {
	return attachCompleteness(ctx, h, model.EnrichEntityVideo, items,
		func(v *model.Video) int64 { return v.ID },
		func(v *model.Video) **model.CompletenessSummary { return &v.Completeness })
}

func (h *Handlers) attachPersonCompleteness(ctx context.Context, items []model.Person) error {
	return attachCompleteness(ctx, h, model.EnrichEntityPerson, items,
		func(p *model.Person) int64 { return p.ID },
		func(p *model.Person) **model.CompletenessSummary { return &p.Completeness })
}

func (h *Handlers) attachStudioCompleteness(ctx context.Context, items []model.Studio) error {
	return attachCompleteness(ctx, h, model.EnrichEntityStudio, items,
		func(s *model.Studio) int64 { return s.ID },
		func(s *model.Studio) **model.CompletenessSummary { return &s.Completeness })
}

// FacetSummary is one scored facet's metadata plus how many entities of the
// type are currently missing it — the "Missing facet" filter chip's option
// list and counts (F55.6). Read from entity_completeness_missing (F65.7), the
// same rows the missing_facet filter selects on, so the chip's counts can
// never disagree with what the filter itself returns. A facet nobody is
// missing has no row and is not offered — there is nothing to filter to.
type FacetSummary struct {
	Canonical    string `json:"canonical"`
	Label        string `json:"label"`
	Criticality  string `json:"criticality"`
	MissingCount int    `json:"missing_count"`
}

func facetSummaries(counts []repo.MissingFacetCount) []FacetSummary {
	out := make([]FacetSummary, 0, len(counts)) // non-nil: `[]`, never `null`
	for _, c := range counts {
		out = append(out, FacetSummary{
			Canonical:    c.Canonical,
			Label:        registry.Lookup(c.Canonical).Label,
			Criticality:  c.Band,
			MissingCount: c.Count,
		})
	}
	return out
}
