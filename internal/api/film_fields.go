package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/fieldsource"
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
// filmEnrichmentRows reads a film's shadow enrichment, logging and degrading to nil on
// error. Split out so getFilm can fetch once and share the result with both consumers
// (resolveFilm and filmBilledCast) — they previously issued the same query twice per
// detail read, which was wasted work and let the two be computed from different
// snapshots if a write landed between them.
func (h *Handlers) filmEnrichmentRows(ctx context.Context, id int64) []repo.EnrichmentRow {
	rows, err := h.repo.EnrichmentForEntity(ctx, model.EnrichEntityFilm, id)
	if err != nil {
		h.log.Warn("enrichment for film detail", "id", id, "err", err)
		return nil
	}
	return rows
}

// resolveFilm resolves a film's fields from already-fetched enrichment rows. Callers
// that need the rows for anything else pass the same slice, so one read serves both.
func (h *Handlers) resolveFilm(ctx context.Context, id int64, f *model.Film, rows []repo.EnrichmentRow) []resolver.ResolvedField {
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

// filmSchema is the provider-independent synthesized schema used for field
// validation (the provider list only widens sources), built once. Mirrors
// studioSchema.
var filmSchema = filmFields(nil)

// filmFieldByCanonical resolves a canonical name against the synthesized film
// schema. Mirrors studioFieldByCanonical.
func filmFieldByCanonical(canonical string) (mapping.Field, bool) {
	canonical = strings.ToLower(strings.TrimSpace(canonical))
	for _, f := range filmSchema {
		if f.Canonical == canonical {
			return f, true
		}
	}
	return mapping.Field{}, false
}

// mountFilmDecisions registers the owner-gated film per-field decision
// surface (F56, ADR-085 §7), mirroring mountStudioDecisions. DB-only — a film
// has no file, so there is no writeback and no rename here (name is
// baseline-backed and read-only in v1, same as studio).
func (h *Handlers) mountFilmDecisions(r chi.Router) {
	r.Put("/films/{id}/fields/{canonical}/decision", h.setFilmFieldDecision)
	r.Delete("/films/{id}/fields/{canonical}/decision", h.clearFilmFieldDecision)
}

// setFilmFieldDecision records a standing decision pinning a film replace
// field to a source. Mirrors setStudioFieldDecision.
func (h *Handlers) setFilmFieldDecision(w http.ResponseWriter, r *http.Request) {
	id, field, ok := h.filmDecisionTarget(w, r)
	if !ok {
		return
	}
	var body decisionBody
	if !decodeJSON(w, r, &body) {
		return
	}
	source, ok := recordDecisionSource(body.Source)
	if !ok {
		writeError(w, http.StatusBadRequest, "source must be 'record', 'manual', or 'provider:<name>'")
		return
	}
	manualValue := ""
	if source == fieldsource.Manual {
		if manualValue = enrich.SanitizeValue(body.ManualValue); manualValue == "" {
			writeError(w, http.StatusBadRequest, "manual_value required for a manual decision")
			return
		}
	}
	if p := fieldsource.Provider(source); p != "" && !h.providerMatched(r.Context(), model.EnrichEntityFilm, id, p) {
		writeError(w, http.StatusBadRequest, "provider is not matched to this film")
		return
	}

	if err := h.repo.SetDecision(r.Context(), model.EnrichEntityFilm, id, field.Canonical, source, manualValue); err != nil {
		h.fail(w, "set film decision", err)
		return
	}
	// Pinning release_date to a different source can change the year the film should
	// carry (F59/ADR-089 D3) — the fill follows the resolved value, so it belongs on
	// every path that changes resolution, not just enrich apply.
	if field.Canonical == "release_date" {
		h.syncFilmYear(r.Context(), id)
	}
	w.WriteHeader(http.StatusNoContent)
}

// clearFilmFieldDecision removes a film field's standing decision, reverting
// it to the record-first default. Mirrors clearStudioFieldDecision.
func (h *Handlers) clearFilmFieldDecision(w http.ResponseWriter, r *http.Request) {
	id, field, ok := h.filmDecisionTarget(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.ClearDecision(r.Context(), model.EnrichEntityFilm, id, field.Canonical); err != nil {
		h.fail(w, "clear film decision", err)
		return
	}
	if field.Canonical == "release_date" {
		h.syncFilmYear(r.Context(), id)
	}
	w.WriteHeader(http.StatusNoContent)
}

// filmDecisionTarget validates the {id}/{canonical} pair: the film must exist
// (404) and the canonical must name a decidable replace field (404 unknown,
// 400 name). Mirrors studioDecisionTarget.
func (h *Handlers) filmDecisionTarget(w http.ResponseWriter, r *http.Request) (id int64, field mapping.Field, ok bool) {
	id, ok = pathID(w, r)
	if !ok {
		return 0, mapping.Field{}, false
	}
	if _, err := h.repo.GetFilm(r.Context(), id); err != nil {
		h.filmLookupError(w, err)
		return 0, mapping.Field{}, false
	}
	field, ok = h.filmReplaceField(w, chi.URLParam(r, "canonical"))
	return id, field, ok
}

// filmReplaceField resolves a canonical name against the synthesized film
// schema and confirms a decision may target it: unknown -> 404, name -> 400
// (baseline-backed identity, read-only in v1). Mirrors studioReplaceField.
func (h *Handlers) filmReplaceField(w http.ResponseWriter, canonical string) (mapping.Field, bool) {
	f, ok := filmFieldByCanonical(canonical)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown field")
		return mapping.Field{}, false
	}
	if f.Canonical == "name" {
		writeError(w, http.StatusBadRequest, "film name is read-only in this release")
		return mapping.Field{}, false
	}
	return f, true
}
