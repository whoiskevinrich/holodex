package enrich

import (
	"context"

	"holodex/internal/model"
)

// Match is a persisted provider link for an entity: the provider name and the
// external record id it was confirmed against (F31, ADR-047). A refresh re-fetches
// each match without re-prompting for identity.
type Match struct {
	Provider   string `json:"provider"`
	ExternalID string `json:"external_id"`
}

// ProviderMatches returns the providers an entity is currently linked to, each with
// the external id it was confirmed against, so a refresh can re-fetch them with no
// picker (F31.3). Derived from the shadow store; a provider whose rows carry no
// external id is skipped (re-enrich needs the id). Order is stable — the store
// returns rows ordered by provider, and the first row per provider wins.
func (s *Service) ProviderMatches(ctx context.Context, entityType string, entityID int64) ([]Match, error) {
	rows, err := s.repo.EnrichmentForEntity(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []Match
	for _, row := range rows {
		if row.ExternalID == "" || seen[row.Provider] {
			continue
		}
		seen[row.Provider] = true
		out = append(out, Match{Provider: row.Provider, ExternalID: row.ExternalID})
	}
	return out, nil
}

// ReEnrich re-fetches and stores one provider's data for an entity using a known
// external id, WITHOUT recording its own activity row — a refresh records a single
// combined row for the whole operation (F31, ADR-047). It is the non-recording
// twin of Enrich; the identity is reused, never re-prompted. It writes only the
// provider's shadow-store layer, never the file-extracted fields (the
// non-destructive layering invariant).
func (s *Service) ReEnrich(ctx context.Context, entityType string, entityID int64, provider, externalID string) ([]model.EnrichedField, error) {
	return s.runEnrich(ctx, entityType, entityID, provider, externalID)
}
