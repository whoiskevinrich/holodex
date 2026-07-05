package api

import (
	"context"
	"strings"

	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// appendAutoRegistered appends the display-only, presence-driven auto-registered
// non-canonical fields (F39, ADR-056) after an entity's canonically-resolved fields.
// It is the shared glue for video/person/studio: rows are the entity's shadow-store
// rows, resolved the canonical resolve result. Label/render/order come from the
// cached provider hints (tier 3) with the title-case floor (tier 4).
func (h *Handlers) appendAutoRegistered(ctx context.Context, rows []repo.EnrichmentRow, resolved []resolver.ResolvedField) []resolver.ResolvedField {
	if len(rows) == 0 {
		return resolved
	}

	rendered := make(map[string]bool, len(resolved))
	for _, f := range resolved {
		rendered[strings.ToLower(strings.TrimSpace(f.Canonical))] = true
	}

	fields := make([]resolver.AutoField, 0, len(rows))
	for _, row := range rows {
		fields = append(fields, resolver.AutoField{Provider: row.Provider, Key: row.FieldKey, Values: row.Values})
	}

	var hints map[string]map[string]repo.ProviderFieldHint
	if h.enrich != nil {
		hints = h.enrich.FieldHints(ctx)
	}
	hintFor := func(provider, key string) (resolver.AutoHint, bool) {
		if byKey, ok := hints[provider]; ok {
			if ph, ok := byKey[key]; ok {
				return resolver.AutoHint{Label: ph.Label, Display: ph.Render, Group: ph.Group, Order: ph.Order}, true
			}
		}
		return resolver.AutoHint{}, false
	}

	auto := resolver.AutoRegisterFields(fields, rendered, hintFor)
	if len(auto) == 0 {
		return resolved
	}
	// F39 security gate (ADR-039/056): an image_url value whose host is not on the
	// provider's asset-host allowlist must not render as an <img> — degrade it to
	// text. Applied here, next to the allowlist, rather than inside the pure resolver.
	for i := range auto {
		f := &auto[i]
		if f.Display != registry.DisplayImageURL {
			continue
		}
		provider, _, _ := strings.Cut(f.WinningSource, ":")
		if len(f.Values) == 0 || h.enrich == nil || !h.enrich.ImageURLAllowed(provider, f.Values[0]) {
			f.Display = registry.DisplayText
		}
	}
	return append(resolved, auto...)
}
