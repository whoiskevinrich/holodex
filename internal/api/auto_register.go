package api

import (
	"context"
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// appendAutoRegistered appends the display-only, presence-driven auto-registered
// non-canonical fields (F39, ADR-056) after an entity's canonically-resolved fields.
// It is the shared glue for video/person/studio: rows are the entity's shadow-store
// rows, effective the merged field set that produced resolved (post-mergePromotions),
// and resolved the canonical resolve result. Label/render/order come from the cached
// provider hints (tier 3) with the title-case floor (tier 4).
//
// effective is what supplies the F49 claimed set (ADR-074 §D2) — a key already listed
// as a candidate source of a canonical field does not also auto-register as its own
// row, which is the GH #178 fix.
func (h *Handlers) appendAutoRegistered(ctx context.Context, rows []repo.EnrichmentRow, effective []mapping.Field, resolved []resolver.ResolvedField) []resolver.ResolvedField {
	if len(rows) == 0 {
		return resolved
	}

	rendered := make(map[string]bool, len(resolved))
	for _, f := range resolved {
		rendered[strings.ToLower(strings.TrimSpace(f.Canonical))] = true
	}
	claimed := resolver.ClaimedKeys(effective)

	fields := make([]resolver.AutoField, 0, len(rows))
	for _, row := range rows {
		fields = append(fields, resolver.AutoField{Provider: row.Provider, Key: row.FieldKey, Values: row.Values})
	}

	var hints map[string]map[string]repo.ProviderFieldHint
	if h.enrich != nil {
		hints = h.enrich.FieldHints(ctx)
	}
	hintFor := func(provider, key string) (resolver.AutoHint, bool) {
		if ph, ok := lookupHint(hints, provider, key); ok {
			return resolver.AutoHint{Label: ph.Label, Display: ph.Render, Group: ph.Group, Order: ph.Order}, true
		}
		return resolver.AutoHint{}, false
	}

	auto := resolver.AutoRegisterFields(fields, rendered, claimed, hintFor)
	if len(auto) == 0 {
		return resolved
	}
	// F39 security gate (ADR-039/056): an image_url value whose host is not on the
	// provider's asset-host allowlist must not render as an <img> — degrade it to text.
	// Shared with the F44 promotion path (markPromoted) via gateImageURL.
	for i := range auto {
		auto[i].Display = h.gateImageURL(auto[i].Display, auto[i].WinningSource, auto[i].Values)
	}
	return append(resolved, auto...)
}
