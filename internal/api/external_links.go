package api

import (
	"context"
	"strings"
)

// ExternalLink is one badge-ready outbound link for a person/studio detail response
// (HOLODEX-266, ADR-083): a read-only projection of person_external_ids/
// studio_external_ids (ADR-054/055), one entry per stored external id (D3 — no
// "primary" selection, unlike video's single resolved badge). URL is empty when no
// provider currently advertises a link_templates entry for this (namespace, entity
// kind) — the degraded state the design handoff (docs/design/provider-link-badge-
// handoff.md §3) specs: the badge still renders, just non-interactive.
type ExternalLink struct {
	Provider string `json:"provider"` // the id's namespace, e.g. "imdb"
	Label    string `json:"label"`    // display label, e.g. "IMDb"
	URL      string `json:"url,omitempty"`
}

// namespaceLabels are the well-known namespace -> display label overrides this
// deployment ships with (e.g. "IMDb" rather than a naive title-case of "imdb").
// Deliberately NOT provider-declared (unlike link_templates, enrich.Manifest): a
// namespace is a shared identity space across providers (ADR-055 D2), so its display
// label must be provider-independent too — a provider that emits a foreign-
// namespaced id (TMDB emitting "imdb:"-prefixed values) must not relabel that
// namespace as its own provider name. An unrecognized namespace falls back to
// titleCaseNamespace.
var namespaceLabels = map[string]string{
	"imdb": "IMDb",
	"tmdb": "TMDB",
}

// namespaceLabel returns the display label for a namespace (HOLODEX-266, ADR-083):
// the well-known override if one exists, else a title-cased fallback of the raw
// namespace string so an unrecognized namespace still renders something readable.
func namespaceLabel(namespace string) string {
	if label, ok := namespaceLabels[namespace]; ok {
		return label
	}
	if namespace == "" {
		return ""
	}
	return strings.ToUpper(namespace[:1]) + namespace[1:]
}

// externalLinksForEntity projects a person/studio's stored external ids
// (person_external_ids/studio_external_ids, ADR-054/055) into badge-ready
// ExternalLinks (HOLODEX-266, ADR-083): one entry per stored id (D3), namespace
// split from the "<namespace>:<id>" value (ADR-082's value shape), with the outbound
// URL built server-side from whichever provider currently advertises a
// link_templates entry for it (D2) — empty when none does. Read-only: never touches
// the resolver or F55 completeness scoring (D1). h.enrich may be nil (enrichment
// disabled) — every id then renders label-only, no links.
func (h *Handlers) externalLinksForEntity(ctx context.Context, entityType string, entityID int64) ([]ExternalLink, error) {
	ids, err := h.repo.ExternalIDsForEntity(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	out := make([]ExternalLink, 0, len(ids))
	for _, raw := range ids {
		namespace, id, ok := strings.Cut(raw, ":")
		if !ok || namespace == "" || id == "" {
			continue
		}
		link := ExternalLink{Provider: namespace, Label: namespaceLabel(namespace)}
		if h.enrich != nil {
			if u, ok := h.enrich.BuildProviderLink(ctx, namespace, entityType, id); ok {
				link.URL = u
			}
		}
		out = append(out, link)
	}
	return out, nil
}
