package api

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"holodex/internal/fieldsource"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/query"
	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// The cache keys of the three images, which come from an asset rather than a value
// (field-resolution.md, "Images"). Images are never enriched.
const (
	posterStateKey   = "poster"
	photoStateKey    = "photo"
	brandingStateKey = "branding"
)

// fieldStates is one entity's entity_field_state rows (F90) from the drain's resolver pass:
// a field is shown when the page displays a value for it, from any source, and enriched when
// a declared provider stored data under a key the field maps from, whether or not that won.
// fields is the effective field list the pass resolved with; rows is the entity's own
// shadow-store rows, so a Film's data never counts for its videos (F90 RD3).
func fieldStates(fields []mapping.Field, resolved []resolver.ResolvedField, rows []repo.EnrichmentRow, declared []string) []repo.FieldState {
	var out []repo.FieldState
	at := map[string]int{}
	sources := map[string]string{} // "provider:key", lowercased → canonical
	for _, f := range fields {
		if !holdsValue(f) {
			continue
		}
		if _, dup := at[f.Canonical]; !dup {
			at[f.Canonical] = len(out)
			out = append(out, repo.FieldState{Key: f.Canonical})
		}
		for _, s := range f.ParsedSources {
			ns, key := strings.ToLower(strings.TrimSpace(s.Namespace)), strings.ToLower(strings.TrimSpace(s.Key))
			if ns != "" && ns != fieldsource.File && key != "" {
				sources[ns+":"+key] = f.Canonical
			}
		}
	}
	for _, rf := range resolved {
		if i, ok := at[rf.Canonical]; ok && shownValue(rf) {
			out[i].Shown = true
		}
	}
	for _, row := range rows {
		if strings.HasPrefix(row.FieldKey, "_") || !slices.Contains(declared, row.Provider) {
			continue
		}
		if c, ok := sources[strings.ToLower(row.Provider)+":"+strings.ToLower(row.FieldKey)]; ok {
			out[at[c]].Enriched = true
		}
	}
	return out
}

// shownValue reports whether the page displays a value for a resolved field: it has a winning
// source, the presence rule resolver.Complete uses. A field the owner cleared, or whose every
// value they removed, resolves to no winner.
func shownValue(rf resolver.ResolvedField) bool { return rf.WinningSource != "" }

// posterShown reports whether a video presents a real poster: whatever the page shows other
// than the automatic slot (F90 P0-2). resolved must come from a pass given the video's local
// slots and file cover, as the detail page's is.
func posterShown(resolved []resolver.ResolvedField) bool {
	i := slices.IndexFunc(resolved, func(rf resolver.ResolvedField) bool { return rf.Canonical == fieldsource.LocalField })
	if i < 0 || !shownValue(resolved[i]) {
		return false
	}
	automatic := fieldsource.LocalToken(fieldsource.LocalAutomatic, "") // the value is "local:automatic#<hash>"
	return slices.ContainsFunc(resolved[i].Values, func(v string) bool { return v != "" && !strings.HasPrefix(v, automatic) })
}

// filterField is one row of a list's Fields control (F90 P0-3): the key a has/missing
// condition names, and its label.
type filterField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// entityFilterFields is what the People or Studios filters may name (SEC-14): the name, the
// image and the entity's own fields, then every field the owner promoted for that entity type
// (F44) that holds a value, which the drain already caches like any other. Never taken from
// request text. A promotion's own label wins, else the registry's title-case floor.
func (h *Handlers) entityFilterFields(ctx context.Context, entityType string) []filterField {
	keys := append([]string{"name", photoStateKey}, personScalarFields...)
	labels := map[string]string{"name": "Name", photoStateKey: "Photo"}
	if entityType == model.EnrichEntityStudio {
		keys = append([]string{"name", brandingStateKey}, studioScalarFields...)
		labels = map[string]string{"name": "Name", brandingStateKey: "Branding"}
	}
	out := make([]filterField, 0, len(keys))
	for _, k := range keys {
		l := labels[k]
		if l == "" {
			l = registry.Lookup(k).Label
		}
		out = append(out, filterField{k, l})
	}
	merged, promoted := h.mergePromotions(ctx, entityType, nil, nil)
	for _, f := range merged {
		if !promoted[f.Canonical] || !holdsValue(f) || slices.ContainsFunc(out, func(o filterField) bool { return o.Key == f.Canonical }) {
			continue
		}
		l := f.Label
		if l == "" {
			l = registry.Lookup(f.Canonical).Label
		}
		out = append(out, filterField{f.Canonical, l})
	}
	return out
}

// filterFieldList handles GET /filter-fields?list=media|people|studios, owner-only like
// every condition it feeds: the fields that list's Fields control offers, in mapping order.
// Media's are the fields has: accepts, with released: standing in for the release date.
func (h *Handlers) filterFieldList(w http.ResponseWriter, r *http.Request) {
	if !h.requireOwnerInline(w, r) {
		return
	}
	var out []filterField
	switch r.URL.Query().Get("list") {
	case "media":
		if h.mappings != nil {
			for _, f := range h.mappings.Current().Fields() {
				if spec, ok := queryFieldSpec(f); ok && f.Canonical != query.ReleaseDateKey {
					out = append(out, filterField{f.Canonical, spec.Label})
				}
			}
		}
		out = append(out, filterField{query.FieldReleased, "Released"}, filterField{query.HasPoster, "Poster"})
	case "people":
		out = h.entityFilterFields(r.Context(), model.EnrichEntityPerson)
	case "studios":
		out = h.entityFilterFields(r.Context(), model.EnrichEntityStudio)
	default:
		writeError(w, http.StatusBadRequest, "list is media, people or studios")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"fields": out})
}

// gateNamedList is the one check the People and Studios lists run before any SQL (SEC-13):
// a visitor asking for missing_facet, link, has, missing or an owner-only sort gets the 401
// missing_facet has always had, refused rather than ignored. The owner's link value and field
// names must be ones the list knows (entityFilterFields), else 400. It reports whether the
// read may go on.
func (h *Handlers) gateNamedList(w http.ResponseWriter, r *http.Request, f repo.NamedListFilter, entityType string) bool {
	ownerOnly := wantsCompleteness(f.Sort, f.MissingFacets) || f.Link != "" || len(f.Has) > 0 || len(f.Missing) > 0
	if ownerOnly && !h.requireOwnerInline(w, r) {
		return false
	}
	if f.Link != "" && f.Link != repo.LinkLinked && f.Link != repo.LinkUnlinked {
		writeError(w, http.StatusBadRequest, "link is linked or unlinked")
		return false
	}
	if len(f.Has)+len(f.Missing) == 0 {
		return true
	}
	fields := h.entityFilterFields(r.Context(), entityType)
	for _, k := range slices.Concat(f.Has, f.Missing) {
		if !slices.ContainsFunc(fields, func(ff filterField) bool { return ff.Key == k }) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("%q isn't a field this list can filter by", k))
			return false
		}
	}
	return true
}
