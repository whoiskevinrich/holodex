package enrich

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Untrusted-response caps (ADR-033 F22.9b). Provider data is bounded before it is
// stored or displayed so a hostile/buggy provider can't inject huge or malformed
// values into the library.
const (
	maxFieldLen       = 4096
	maxValuesPerField = 50
	maxFields         = 40
	maxCandidates     = 25
)

// personFieldLabels gives the canonical person fields human display labels
// (F22.5c). v1 is People-only; when series/video providers are added, labeling
// should move to a per-entity map (or be pushed to the provider Manifest / SPA),
// so a non-person field doesn't silently fall through to the title-cased default.
// Unknown keys fall back to a title-cased label.
var personFieldLabels = map[string]string{
	"bio":         "Bio",
	"birthdate":   "Born",
	"nationality": "Nationality",
	"website":     "Website",
	"aliases":     "Aliases",
	"photo":       "Photo",
}

// EnrichRepo is the shadow-store subset the service needs (satisfied by *repo.Repo).
type EnrichRepo interface {
	UpsertEnrichment(ctx context.Context, entityType string, entityID int64, provider, externalID string, fields map[string][]string) error
	EnrichmentForEntity(ctx context.Context, entityType string, entityID int64) ([]repo.EnrichmentRow, error)
	MatchExternalID(ctx context.Context, entityType string, entityID int64, provider string) (string, bool, error)
	DeleteEnrichmentByProvider(ctx context.Context, entityType string, entityID int64, provider string) (int64, error)
}

// Service orchestrates on-demand enrichment (ADR-033). It is the only thing that
// dials a provider, and only from an explicit owner action — there is no
// scheduler and no enrich-on-scan (F22.6). newClient is injectable so tests run
// against a fake with no network.
type Service struct {
	store     *Store
	repo      EnrichRepo
	log       *slog.Logger
	newClient func(Source) ProviderClient
}

// NewService wires the enrichment service over the provider registry and the
// shadow store. The default client is the HTTP transport.
func NewService(store *Store, r EnrichRepo, log *slog.Logger) *Service {
	return &Service{
		store:     store,
		repo:      r,
		log:       log,
		newClient: func(s Source) ProviderClient { return newHTTPClient(s) },
	}
}

// NewServiceWithClient is NewService with an injected client factory — used by
// tests and a future in-process provider to run with no network.
func NewServiceWithClient(store *Store, r EnrichRepo, log *slog.Logger, newClient func(Source) ProviderClient) *Service {
	s := NewService(store, r, log)
	s.newClient = newClient
	return s
}

// Store exposes the provider registry store so the reload endpoint can swap it
// atomically alongside the mapping config (F22.2d).
func (s *Service) Store() *Store { return s.store }

// SourceInfo is the registry view the SPA needs to offer enrich actions (no
// base_url or secrets — F22.9d).
type SourceInfo struct {
	Name        string   `json:"name"`
	EntityTypes []string `json:"entity_types"`
}

// Sources lists the enabled providers (names + entity types only).
func (s *Service) Sources() []SourceInfo {
	srcs := s.store.Current().Enabled()
	out := make([]SourceInfo, 0, len(srcs))
	for _, src := range srcs {
		out = append(out, SourceInfo{Name: src.Name, EntityTypes: src.EntityTypes})
	}
	return out
}

// client resolves an enabled provider by name (the SSRF allowlist) and returns a
// client for it. An unknown/disabled name is an error, never a dialed URL.
func (s *Service) client(provider string) (Source, ProviderClient, error) {
	src, ok := s.store.Current().ByName(provider)
	if !ok {
		return Source{}, nil, fmt.Errorf("unknown provider %q", provider)
	}
	return src, s.newClient(src), nil
}

// verifyProtocol refuses a provider whose contract major version this build does
// not speak (F22.1e) — fail loud, don't mis-parse.
func verifyProtocol(m Manifest) error {
	if m.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("provider protocol v%d unsupported (need v%d)", m.ProtocolVersion, ProtocolVersion)
	}
	return nil
}

// verifiedClient resolves an enabled provider (the SSRF allowlist), checks it
// supports the entity type, and verifies its protocol version — the shared
// preamble for every provider action.
func (s *Service) verifiedClient(ctx context.Context, provider, entityType string) (ProviderClient, error) {
	src, c, err := s.client(provider)
	if err != nil {
		return nil, err
	}
	if !src.Supports(entityType) {
		return nil, fmt.Errorf("provider %q does not support %q", provider, entityType)
	}
	m, err := c.Describe(ctx)
	if err != nil {
		return nil, err
	}
	if err := verifyProtocol(m); err != nil {
		return nil, err
	}
	return c, nil
}

// Resolve asks a provider for identity candidates (F22.5b). hint carries any
// embedded external ids (deterministic path) and/or a name query (fallback). The
// caller always confirms a candidate before Enrich — nothing is applied here.
func (s *Service) Resolve(ctx context.Context, provider, entityType string, hint Hint) ([]Candidate, error) {
	c, err := s.verifiedClient(ctx, provider, entityType)
	if err != nil {
		return nil, err
	}
	cands, err := c.Resolve(ctx, entityType, hint)
	if err != nil {
		return nil, err
	}
	return sanitizeCandidates(cands), nil
}

// ExistingMatch returns the external id a provider was last confirmed against for
// an entity (F22.4b), so the handler can re-enrich without re-showing the picker.
func (s *Service) ExistingMatch(ctx context.Context, entityType string, entityID int64, provider string) (string, bool, error) {
	return s.repo.MatchExternalID(ctx, entityType, entityID, provider)
}

// Enrich fetches an external record's fields for an entity, sanitizes and stores
// them in the shadow layer, and returns the entity's resolved fields with
// provenance (F22.5/F22.7). Asset download (photos) is deferred (v1 non-goal).
func (s *Service) Enrich(ctx context.Context, entityType string, entityID int64, provider, externalID string) ([]model.EnrichedField, error) {
	c, err := s.verifiedClient(ctx, provider, entityType)
	if err != nil {
		return nil, err
	}
	res, err := c.Enrich(ctx, entityType, externalID)
	if err != nil {
		return nil, err
	}
	fields := sanitizeFields(res.Fields)
	if err := s.repo.UpsertEnrichment(ctx, entityType, entityID, provider, externalID, fields); err != nil {
		return nil, err
	}
	return s.Fields(ctx, entityType, entityID)
}

// Clear removes a provider's contribution for an entity (F22.7b).
func (s *Service) Clear(ctx context.Context, entityType string, entityID int64, provider string) error {
	_, err := s.repo.DeleteEnrichmentByProvider(ctx, entityType, entityID, provider)
	return err
}

// Fields returns an entity's stored enrichment as display fields with provenance,
// ordered by label for stable rendering. Safe to call with no providers wired.
func (s *Service) Fields(ctx context.Context, entityType string, entityID int64) ([]model.EnrichedField, error) {
	rows, err := s.repo.EnrichmentForEntity(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}
	out := make([]model.EnrichedField, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.EnrichedField{
			Canonical:  r.FieldKey,
			Label:      labelFor(r.FieldKey),
			Values:     r.Values,
			Provider:   r.Provider,
			ExternalID: r.ExternalID,
			FetchedAt:  r.FetchedAt,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out, nil
}

func labelFor(key string) string {
	if l, ok := personFieldLabels[strings.ToLower(strings.TrimSpace(key))]; ok {
		return l
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

// sanitizeFields bounds an untrusted provider field map (F22.9b): strips control
// characters, caps value length and count per field, and caps the field count.
func sanitizeFields(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for key, vals := range in {
		if len(out) >= maxFields {
			break
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		cleaned := make([]string, 0, len(vals))
		for _, v := range vals {
			if len(cleaned) >= maxValuesPerField {
				break
			}
			if v = sanitizeValue(v); v != "" {
				cleaned = append(cleaned, v)
			}
		}
		if len(cleaned) > 0 {
			out[key] = cleaned
		}
	}
	return out
}

func sanitizeCandidates(in []Candidate) []Candidate {
	if len(in) > maxCandidates {
		in = in[:maxCandidates]
	}
	for i := range in {
		in[i].Label = sanitizeValue(in[i].Label)
		in[i].Disambiguation = sanitizeValue(in[i].Disambiguation)
		in[i].ExternalID = strings.TrimSpace(in[i].ExternalID)
		in[i].Namespace = strings.TrimSpace(in[i].Namespace)
	}
	return in
}

// sanitizeValue removes control characters (keeping normal whitespace), collapses
// surrounding space, and caps length. Newline is stripped because the shadow
// store uses it as the multi-value separator.
func sanitizeValue(v string) string {
	v = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, v)
	v = strings.TrimSpace(v)
	if len(v) > maxFieldLen {
		v = v[:maxFieldLen]
	}
	return v
}
