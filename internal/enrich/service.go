package enrich

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode"

	"holodex/internal/model"
	"holodex/internal/registry"
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


// EnrichRepo is the shadow-store subset the service needs (satisfied by *repo.Repo).
type EnrichRepo interface {
	UpsertEnrichment(ctx context.Context, entityType string, entityID int64, provider, externalID string, fields map[string][]string) error
	EnrichmentForEntity(ctx context.Context, entityType string, entityID int64) ([]repo.EnrichmentRow, error)
	MatchExternalID(ctx context.Context, entityType string, entityID int64, provider string) (string, bool, error)
	DeleteEnrichmentByProvider(ctx context.Context, entityType string, entityID int64, provider string) (int64, error)
	// RecordJobRun appends an enrich pass to the activity history (F22.6b). Best
	// effort — a recording failure never fails the enrichment.
	RecordJobRun(ctx context.Context, run model.JobRun) error
}

// ImageSink stores a downloaded, normalized provider asset as a person image (F25,
// ADR-038). It is satisfied by an adapter over personimage + the repo, wired in
// main; nil disables asset download (the v1-without-images path). Kept an interface
// so the enrich package needn't import personimage/repo for the image write and so
// tests can assert what would be stored with no disk.
type ImageSink interface {
	// StoreAsset normalizes raw image bytes (metadata strip) and stores them as the
	// given core role for a person, recording provenance (provider + externalID).
	StoreAsset(ctx context.Context, personID int64, role, provider, externalID string, raw []byte) error
}

// Service orchestrates on-demand enrichment (ADR-033). It is the only thing that
// dials a provider, and only from an explicit owner action — there is no
// scheduler and no enrich-on-scan (F22.6). newClient is injectable so tests run
// against a fake with no network.
type Service struct {
	store       *Store
	repo        EnrichRepo
	log         *slog.Logger
	newClient   func(Source) ProviderClient
	images      ImageSink // F25 asset download; nil = disabled
	newAssetGet func(Source) assetFetcher
}

// assetFetcher is the SSRF-guarded asset transport (satisfied by *AssetClient);
// injectable so tests fetch from an httptest server without a real provider host.
type assetFetcher interface {
	Fetch(ctx context.Context, rawURL string) ([]byte, error)
}

// NewService wires the enrichment service over the provider registry and the
// shadow store. The default client is the HTTP transport.
func NewService(store *Store, r EnrichRepo, log *slog.Logger) *Service {
	return &Service{
		store:       store,
		repo:        r,
		log:         log,
		newClient:   func(s Source) ProviderClient { return newHTTPClient(s) },
		newAssetGet: func(s Source) assetFetcher { return newAssetClient(s) },
	}
}

// SetImageSink wires person-image asset download (F25, ADR-038). With a sink set, a
// person enrich run that returns image assets fetches and stores them; without one,
// assets are ignored (the field-only path). Called once at startup.
func (s *Service) SetImageSink(sink ImageSink) { s.images = sink }

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
// provenance (F22.5/F22.7). It records the pass in the activity history (F22.6b).
// For a person, any image assets are fetched and stored as person images (F25).
func (s *Service) Enrich(ctx context.Context, entityType string, entityID int64, provider, externalID string) ([]model.EnrichedField, error) {
	started := time.Now()
	fields, err := s.runEnrich(ctx, entityType, entityID, provider, externalID)
	s.recordEnrichJob(started, provider, entityType, entityID, len(fields), err)
	return fields, err
}

// runEnrich is the core fetch → sanitize → store → re-read; Enrich wraps it with
// activity-history recording.
func (s *Service) runEnrich(ctx context.Context, entityType string, entityID int64, provider, externalID string) ([]model.EnrichedField, error) {
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
	// Download any image assets the provider returned (F25, ADR-038). People-only in
	// v1; best-effort — a failed fetch/normalize is logged and skipped, never failing
	// the field enrichment that already succeeded.
	if s.images != nil && entityType == model.EnrichEntityPerson && len(res.Assets) > 0 {
		s.downloadAssets(ctx, entityID, provider, externalID, res.Assets)
	}
	return s.Fields(ctx, entityType, entityID)
}

// downloadAssets fetches provider image assets through the SSRF-guarded asset client
// and stores them via the image sink (F25, ADR-038/039). Assets are preference-ordered;
// only the first successful fetch per role is stored — later entries of the same kind
// are treated as fallbacks and skipped once a role is filled (ADR-039 §5).
func (s *Service) downloadAssets(ctx context.Context, entityID int64, provider, externalID string, assets []Asset) {
	src, ok := s.store.Current().ByName(provider)
	if !ok { // unreachable after verifiedClient, but keep the allowlist explicit
		return
	}
	fetcher := s.newAssetGet(src)
	done := make(map[string]bool) // role → already stored successfully
	for _, a := range assets {
		role, ok := assetRoleFor(a.Kind)
		if !ok || done[role] {
			continue // unknown kind or role already filled
		}
		raw, err := fetcher.Fetch(ctx, a.URL)
		if err != nil {
			s.log.Warn("asset fetch refused/failed", "provider", provider, "kind", a.Kind, "err", err)
			continue
		}
		if err := s.images.StoreAsset(ctx, entityID, role, provider, externalID, raw); err != nil {
			s.log.Warn("asset store failed", "provider", provider, "kind", a.Kind, "err", err)
			continue
		}
		done[role] = true
	}
}

// recordEnrichJob appends the enrich pass to the 30-day activity history (F22.6b).
// Best effort — a recording failure is logged, never returned. It uses a detached
// context (like the scanner's recordRun) so a failed/cancelled enrich — the case
// the history most needs to capture — still records instead of no-op'ing on a
// cancelled request context. The detail carries provider + entity + field count
// only: no filesystem path, env value, or token (the no-secrets invariant,
// ADR-028); on error the raw provider error is omitted (it can include base_url).
func (s *Service) recordEnrichJob(started time.Time, provider, entityType string, entityID int64, n int, enrichErr error) {
	now := time.Now()
	run := model.JobRun{
		Kind:       model.JobKindEnrich,
		Trigger:    model.TriggerManual,
		Status:     model.JobStatusOK,
		StartedAt:  started,
		FinishedAt: now,
		DurationMs: now.Sub(started).Milliseconds(),
	}
	if enrichErr != nil {
		run.Status = model.JobStatusErr
		run.Errors = 1
		run.Detail = fmt.Sprintf("%s → %s #%d (failed)", provider, entityType, entityID)
		run.ErrorMessage = "enrichment failed"
	} else {
		field := "fields"
		if n == 1 {
			field = "field"
		}
		run.Detail = fmt.Sprintf("%s → %s #%d (%d %s)", provider, entityType, entityID, n, field)
	}
	recCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.repo.RecordJobRun(recCtx, run); err != nil {
		s.log.Warn("record enrich job", "err", err)
	}
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
	return s.FieldsFromRows(rows), nil
}

// FieldsFromRows converts pre-fetched enrichment rows to display fields. It lets
// callers that already hold the rows (e.g. getMedia, which uses the same rows for
// the resolver) avoid a second repo round-trip.
func (s *Service) FieldsFromRows(rows []repo.EnrichmentRow) []model.EnrichedField {
	if len(rows) == 0 {
		return nil
	}
	out := make([]model.EnrichedField, 0, len(rows))
	for _, r := range rows {
		def := registry.Lookup(r.FieldKey)
		out = append(out, model.EnrichedField{
			Canonical:  r.FieldKey,
			Label:      def.Label,
			Display:    def.Display,
			Values:     r.Values,
			Provider:   r.Provider,
			ExternalID: r.ExternalID,
			FetchedAt:  r.FetchedAt,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
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
			if v = SanitizeValue(v); v != "" {
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
		in[i].Label = SanitizeValue(in[i].Label)
		in[i].Disambiguation = SanitizeValue(in[i].Disambiguation)
		in[i].ExternalID = strings.TrimSpace(in[i].ExternalID)
		in[i].Namespace = strings.TrimSpace(in[i].Namespace)
	}
	return in
}

// SanitizeValue removes control characters (keeping normal whitespace), collapses
// surrounding space, and caps length. Newline is stripped because the shadow
// store uses it as the multi-value separator. Also used by the writeback handler.
func SanitizeValue(v string) string {
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
