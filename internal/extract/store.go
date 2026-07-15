package extract

import (
	"context"

	"holodex/internal/enrich"
)

// Provider is the entity_enrichment namespace filename-derived values are
// stored under (F48.2a) — parallel to "file" and "tmdb" (ADR-033). No schema
// change is needed: entity_enrichment's provider column already accepts any
// string.
const Provider = "filename"

// EnrichmentWriter is the narrow slice of *repo.Repo that Store needs. Matching
// it structurally (rather than importing internal/repo directly) keeps this
// pure-parsing-adjacent package dependency-light and easy to fake in tests.
type EnrichmentWriter interface {
	UpsertEnrichment(ctx context.Context, entityType string, entityID int64, provider, externalID string, fields map[string][]string) error
}

// Store sanitizes freshly extracted field values (F48.10b: filename-derived
// data is untrusted input) and writes them into the entity_enrichment shadow
// store under the filename namespace — the same UpsertEnrichment call a
// provider like tmdb uses, so no new write mechanism is introduced (F48.2a).
// externalID is always empty: a filename parse has no upstream record to match
// against. A nil/empty fields map is a no-op, matching UpsertEnrichment itself.
func Store(ctx context.Context, w EnrichmentWriter, entityType string, entityID int64, fields map[string][]string) error {
	sanitized := make(map[string][]string, len(fields))
	for key, vals := range fields {
		if v := enrich.SanitizeValues(vals); len(v) > 0 {
			sanitized[key] = v
		}
	}
	return w.UpsertEnrichment(ctx, entityType, entityID, Provider, "", sanitized)
}
