package api

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// Film field resolution (F56, ADR-085). Film rides the same unified resolver +
// decision/curation model as person/studio; its baseline is the DB record (name
// only, filmBaseline). description/release_date are provider-backed replace
// fields, resolved exactly like studioScalarFields. Cast/tags/studios are NOT
// resolved fields -- they are a read-only set union over the film's attached
// videos, assembled separately (film_videos.go's filmCast/filmTags/filmStudios).

// filmScalarFields are the provider-backed replace film fields, in registry
// documentation order. name is synthesized separately (baseline-backed, read-only
// -- no rename in v1).
var filmScalarFields = []string{"description", "release_date"}

// filmFields synthesizes the []mapping.Field for film resolution, mirroring
// studioFields.
func filmFields(providers []string) []mapping.Field {
	fields := make([]mapping.Field, 0, len(filmScalarFields)+1)
	fields = append(fields, filmField("name", []mapping.Source{{Namespace: "file", Key: "name"}}))
	for _, canonical := range filmScalarFields {
		fields = append(fields, filmField(canonical, providerSources(providers, canonical)))
	}
	return fields
}

// filmField builds one synthesized replace field; mirrors studioField.
func filmField(canonical string, sources []mapping.Source) mapping.Field {
	return mapping.Field{
		Canonical:     canonical,
		Label:         registry.Lookup(canonical).Label,
		Sources:       rawSources(sources),
		ParsedSources: sources,
	}
}

// filmProviders lists the provider namespaces the synthesized film fields
// consult: registry film-capable providers plus any provider already matched to
// this film. Mirrors studioProviders.
func (h *Handlers) filmProviders(rows []repo.EnrichmentRow) []string {
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
			if slices.Contains(s.EntityTypes, model.EnrichEntityFilm) {
				add(s.Name)
			}
		}
	}
	for _, row := range rows {
		add(row.Provider)
	}
	return out
}

// resolveFilm resolves a film's fields through the unified resolver: the record
// baseline (name) + shadow enrichment + curation + standing decisions, in the
// record vocabulary. Mirrors resolveStudio, trimmed of the promotion/claims/
// completeness machinery studio accumulated across F44/F55 -- not required by the
// films spec.
func (h *Handlers) resolveFilm(ctx context.Context, id int64, f *model.Film) []resolver.ResolvedField {
	rows, err := h.repo.EnrichmentForEntity(ctx, model.EnrichEntityFilm, id)
	if err != nil {
		h.log.Warn("enrichment for film detail", "id", id, "err", err)
		rows = nil
	}
	var cur resolver.Curation
	if curRows, curErr := h.repo.CurationForEntity(ctx, model.EnrichEntityFilm, id); curErr != nil {
		h.log.Warn("curation for film detail", "id", id, "err", curErr)
	} else {
		cur = curationFromRows(curRows)
	}
	var dec resolver.Decisions
	if decRows, decErr := h.repo.DecisionsForEntity(ctx, model.EnrichEntityFilm, id); decErr != nil {
		h.log.Warn("decisions for film detail", "id", id, "err", decErr)
	} else {
		dec = decisionsFromRows(decRows)
	}
	fields := filmFields(h.filmProviders(rows))
	resolved := resolver.ResolveFields(resolver.NewFilmBaseline(f), enrichmentFromRows(rows), cur, fields, h.resolveOptions(dec))
	return recordizeResolved(resolved)
}

// filmLookupError translates a GetFilm error into a response.
func (h *Handlers) filmLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "film not found")
		return
	}
	h.fail(w, "get film", err)
}
