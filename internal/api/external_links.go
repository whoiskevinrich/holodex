package api

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// ExternalLink is one badge-ready outbound link for a person/studio detail response
// (HOLODEX-266, ADR-083): a read-only projection of entity_external_ids
// (ADR-054/055, one table for every kind since ADR-096 D2), one entry per stored external id (D3 — no
// "primary" selection, unlike video's single resolved badge). URL is empty when no
// provider currently advertises a link_templates entry for this (namespace, entity
// kind) — the degraded state the design handoff (docs/design/provider-link-badge-
// handoff.md §3) specs: the badge still renders, just non-interactive.
type ExternalLink struct {
	// Namespace is the id's namespace (e.g. "imdb"), not necessarily the provider that
	// enriched this entity — a provider can emit a foreign-namespaced id (TMDB emitting
	// "imdb:"-prefixed values). Wire field stays "provider" for API compatibility with
	// existing frontend consumers.
	Namespace string `json:"provider"`
	Label     string `json:"label"` // display label, e.g. "IMDb"
	URL       string `json:"url,omitempty"`
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
	// Rune-safe, not a byte slice: namespace[:1] would split a multi-byte UTF-8 first
	// character and corrupt it.
	r, size := utf8.DecodeRuneInString(namespace)
	return string(unicode.ToUpper(r)) + namespace[size:]
}

// externalLinksForEntity projects a person/studio's stored external ids
// (entity_external_ids, ADR-054/055/096) into badge-ready
// ExternalLinks (HOLODEX-266, ADR-083): one entry per stored id (D3), namespace
// split from the "<namespace>:<id>" value (ADR-082's value shape), with the outbound
// URL built server-side from whichever provider currently advertises a
// link_templates entry for it (D2), else the provider's own stored _source_url when
// the namespace is that provider (ADR-098 D3) — empty when neither applies.
// Read-only: never touches the resolver or F55 completeness scoring (D1). h.enrich
// may be nil (enrichment disabled) — every id then renders label-only, no links.
// enrichRows are the entity's enrichment rows when the caller already fetched them
// (getFilm); nil reads them here, once, for the stored _source_url map.
func (h *Handlers) externalLinksForEntity(ctx context.Context, entityType string, entityID int64, enrichRows []repo.EnrichmentRow) ([]ExternalLink, error) {
	ids, err := h.repo.ExternalIDsForEntity(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	// The stored _source_url map is the fallback only (ADR-098 D3): a failed read
	// degrades every pill to template-or-nothing rather than dropping the list —
	// ADR-083 D2's "degrades, never breaks" — so it logs instead of failing the call.
	var stored map[string]string
	if h.enrich != nil {
		if enrichRows != nil {
			stored = enrich.SourceURLsFromRows(enrichRows)
		} else if stored, err = h.enrich.SourceURLs(ctx, entityType, entityID); err != nil {
			h.log.Warn("stored provider source urls", "entity_type", entityType, "id", entityID, "err", err)
		}
	}
	return h.linksFromIDs(ctx, entityType, ids, stored), nil
}

// linksFromIDs is the projection shared by every entity kind: each "<namespace>:<id>"
// becomes one ExternalLink, malformed values are skipped, and the list is deduped by
// namespace with the first occurrence winning — callers order ids by precedence.
func (h *Handlers) linksFromIDs(ctx context.Context, entityType string, ids []string, stored map[string]string) []ExternalLink {
	out := make([]ExternalLink, 0, len(ids))
	seenNamespaces := make(map[string]bool, len(ids))
	for _, raw := range ids {
		namespace, id, ok := strings.Cut(raw, ":")
		if !ok || namespace == "" || id == "" {
			continue
		}
		// Normalize case here (ProviderLink and namespaceLabel both key off a
		// lowercase namespace) and dedup by namespace: the frontend keys its badge list
		// on this value, so two ids under the same namespace would otherwise collide.
		namespace = strings.ToLower(namespace)
		if seenNamespaces[namespace] {
			continue
		}
		seenNamespaces[namespace] = true
		link := ExternalLink{Namespace: namespace, Label: namespaceLabel(namespace)}
		if h.enrich != nil {
			if u, ok := h.enrich.ProviderLink(ctx, namespace, entityType, id, stored); ok {
				link.URL = u
			}
		}
		out = append(out, link)
	}
	return out
}

// externalLinksForVideo is the video half of the projection (HOLODEX-394, ADR-098
// D4): video has no identity rows, so its ids come from two places — the resolver's
// winning external_provider_id, a "<namespace>:<id>" scalar (ADR-082), and the
// provider match stamped on each of its enrichment rows (HOLODEX-424, spec P0-7b:
// the same id the entity kinds hold in entity_external_ids, read from the rows
// getMedia already fetched; extraction rows carry "" and drop out). The resolved
// value is listed first so a file tag in the match's namespace keeps the value the
// resolver already chose. URLs come from the same ProviderLink precedence as
// person/studio/film, keyed on each id's namespace; a namespace no provider
// templates renders degraded (label, no URL) — the identity signal always renders
// (F63 P0-7). Nil when nothing yields an id, so the meta line stays byte-identical
// to today.
func (h *Handlers) externalLinksForVideo(ctx context.Context, resolved []resolver.ResolvedField, enrichRows []repo.EnrichmentRow) []ExternalLink {
	var ids []string
	if field, ok := resolvedByCanonical(resolved, "external_provider_id"); ok && len(field.Values) > 0 {
		ids = append(ids, field.Values[0])
	}
	// One id per provider, taken from its newest row: a re-match to a different id
	// upserts without clearing (Service.Enrich), so rows for keys the new payload
	// omitted still carry the old id — the freshest fetched_at is the current match.
	// Rows arrive ordered by provider, so the append order is deterministic.
	newest := make(map[string]repo.EnrichmentRow)
	var providers []string
	for _, row := range enrichRows {
		if row.ExternalID == "" {
			continue
		}
		cur, seen := newest[row.Provider]
		if !seen {
			providers = append(providers, row.Provider)
		}
		if !seen || row.FetchedAt.After(cur.FetchedAt) {
			newest[row.Provider] = row
		}
	}
	for _, provider := range providers {
		ids = append(ids, newest[provider].ExternalID)
	}
	if len(ids) == 0 {
		return nil
	}
	var stored map[string]string
	if h.enrich != nil {
		stored = enrich.SourceURLsFromRows(enrichRows)
	}
	links := h.linksFromIDs(ctx, model.EnrichEntityVideo, ids, stored)
	if len(links) == 0 {
		return nil
	}
	return links
}
