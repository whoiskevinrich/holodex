package api

import (
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
