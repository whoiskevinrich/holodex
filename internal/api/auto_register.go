package api

import (
	"context"
	"strings"

	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// appendAutoRegistered appends the display-only, presence-driven auto-registered
// non-canonical fields (F39, ADR-056) after an entity's canonically-resolved fields.
// It is the shared glue for video/person/studio: rows are the entity's shadow-store
// rows, resolved the canonical resolve result. Label/render/order come from the
// persisted provider hints (tier 3) with the title-case floor (tier 4); an image_url
// value is gated by the provider's asset-host allowlist.
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
	if hm, err := h.repo.ProviderFieldHints(ctx); err != nil {
		h.log.Warn("provider field hints for auto-registration", "err", err)
	} else {
		hints = hm
	}
	hintFor := func(provider, key string) (resolver.AutoHint, bool) {
		if byKey, ok := hints[provider]; ok {
			if ph, ok := byKey[key]; ok {
				return resolver.AutoHint{Label: ph.Label, Display: ph.Render, Group: ph.Group, Order: ph.Order}, true
			}
		}
		return resolver.AutoHint{}, false
	}

	imageAllowed := func(provider, url string) bool {
		return h.enrich != nil && h.enrich.ImageURLAllowed(provider, url)
	}

	auto := resolver.AutoRegisterFields(fields, rendered, hintFor, imageAllowed)
	if len(auto) == 0 {
		return resolved
	}
	return append(resolved, auto...)
}
