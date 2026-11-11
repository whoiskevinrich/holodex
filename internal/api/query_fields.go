package api

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/query"
	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// Mapped fields in queries (F89 P0-6, SEC-8; field-resolution.md, "Which fields").

// queryFields is the catalogue of queryable fields: the owner's mapping, less anything
// that isn't a value to compare. Nothing outside it is indexed or compiled.
func (h *Handlers) queryFields() query.Fields {
	out := query.Fields{query.FieldReleased: {
		Kind:      query.KindDate,
		OwnerOnly: registry.HiddenFromVisitors(query.ReleaseDateKey),
	}}
	if h.mappings == nil {
		return out
	}
	for _, f := range h.mappings.Current().Fields() {
		if spec, ok := queryFieldSpec(f); ok {
			out[f.Canonical] = spec
		}
	}
	return out
}

func queryFieldSpec(f mapping.Field) (query.FieldSpec, bool) {
	def := registry.Lookup(f.Canonical)
	if !holdsValue(f) || query.IsBuiltin(f.Canonical) {
		return query.FieldSpec{}, false
	}
	spec := query.FieldSpec{Kind: query.KindText, Label: f.Label, OwnerOnly: registry.HiddenFromVisitors(f.Canonical)}
	if spec.Label == "" {
		spec.Label = def.Label
	}
	switch {
	case def.QueryKind != "":
		spec.Kind = def.QueryKind
	case f.Multi || f.Merge:
		spec.Kind = query.KindList
	}
	return spec, true
}

// holdsValue reports whether a mapped field carries a value the owner reads, rather than an
// internal `_` key, a computed field or an image (field-resolution.md, "Which fields").
func holdsValue(f mapping.Field) bool {
	def := registry.Lookup(f.Canonical)
	display := f.Display
	if display == "" {
		display = def.Display
	}
	return !strings.HasPrefix(f.Canonical, "_") && !def.Computed && display != "image_url"
}

// resolvedValueRows is what one video's resolve puts in the index.
func resolvedValueRows(fields query.Fields, resolved []resolver.ResolvedField) []repo.ResolvedValue {
	var out []repo.ResolvedValue
	for _, r := range resolved {
		spec, ok := fields[r.Canonical]
		if !ok {
			continue
		}
		for _, v := range r.Values {
			if norm, ok := query.NormValue(spec.Kind, v); ok {
				out = append(out, repo.ResolvedValue{FieldKey: r.Canonical, Norm: norm, Value: v})
			}
		}
	}
	return out
}

// drainForFilter brings the index and the change stamp up to date before a read that
// depends on them, a visitor's included: the one exception to "visitors never drain".
func (h *Handlers) drainForFilter(ctx context.Context, expr query.Node, sort string) {
	if sort == "updated_desc" || sort == "updated_asc" || (expr != nil && query.NeedsIndex(expr)) {
		h.drainCompleteness(ctx)
	}
}

// queryFieldList handles GET /query/fields: the mapped fields this reader may name, for
// suggestions. A visitor is never offered a hidden one (SEC-8).
func (h *Handlers) queryFieldList(w http.ResponseWriter, r *http.Request) {
	type row struct {
		Field     string `json:"field"`
		Label     string `json:"label"`
		Kind      string `json:"kind"`
		OwnerOnly bool   `json:"owner_only,omitempty"`
	}
	isOwner := h.auth.authorized(r)
	out := []row{}
	for field, spec := range h.queryFields() {
		if query.IsBuiltin(field) || (spec.OwnerOnly && !isOwner) {
			continue
		}
		out = append(out, row{field, spec.Label, spec.Kind, spec.OwnerOnly})
	}
	slices.SortFunc(out, func(a, b row) int { return strings.Compare(a.Field, b.Field) })
	writeJSON(w, http.StatusOK, map[string]any{"fields": out})
}

// queryFieldValues handles GET /query/values?field=&q=: a text field's existing values. A
// field this reader can't name, or whose values aren't worth listing, offers none.
func (h *Handlers) queryFieldValues(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")
	spec, ok := h.queryFields()[field]
	listable := ok && !query.IsBuiltin(field) && (spec.Kind == query.KindText || spec.Kind == query.KindList) &&
		registry.Lookup(field).Display != "long_text"
	if !listable || (spec.OwnerOnly && !h.auth.authorized(r)) {
		writeJSON(w, http.StatusOK, map[string]any{"values": []string{}})
		return
	}
	values, err := h.repo.ResolvedFieldValues(r.Context(), field, r.URL.Query().Get("q"), h.filmsEnabled, 20)
	if err != nil {
		h.fail(w, "query field values", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"values": values})
}
