package api

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"holodex/internal/fieldsource"
	"holodex/internal/mapping"
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

// The fields the People and Studios filters may name (SEC-14): the entity's fields, plus its
// image. Never taken from request text.
var (
	personFilterFields = append([]string{"name", photoStateKey}, personScalarFields...)
	studioFilterFields = append([]string{"name", brandingStateKey}, studioScalarFields...)
)

// gateNamedList is the one check the People and Studios lists run before any SQL (SEC-13):
// a visitor asking for missing_facet, link, has, missing or an owner-only sort gets the 401
// missing_facet has always had, refused rather than ignored. The owner's link value and field
// names must be ones the list knows, else 400. It reports whether the read may go on.
func (h *Handlers) gateNamedList(w http.ResponseWriter, r *http.Request, f repo.NamedListFilter, fields []string) bool {
	ownerOnly := wantsCompleteness(f.Sort, f.MissingFacets) || f.Link != "" || len(f.Has) > 0 || len(f.Missing) > 0
	if ownerOnly && !h.requireOwnerInline(w, r) {
		return false
	}
	if f.Link != "" && f.Link != repo.LinkLinked && f.Link != repo.LinkUnlinked {
		writeError(w, http.StatusBadRequest, "link is linked or unlinked")
		return false
	}
	for _, k := range slices.Concat(f.Has, f.Missing) {
		if !slices.Contains(fields, k) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("%q isn't a field this list can filter by", k))
			return false
		}
	}
	return true
}
