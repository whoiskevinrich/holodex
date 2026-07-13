package enrich

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"sort"
	"strings"
	"sync/atomic"
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
	// ReplaceProviderFieldHints persists a provider's advertised non-canonical render
	// hints (F39, ADR-056), refreshed whenever /describe is read. Best effort — a
	// persistence failure never fails the provider action.
	ReplaceProviderFieldHints(ctx context.Context, provider string, hints []repo.ProviderFieldHint) error
	// ProviderFieldHints reads every stored hint, keyed by provider then field key —
	// the source for the Service's in-memory cache the read path consults (F39).
	ProviderFieldHints(ctx context.Context) (map[string]map[string]repo.ProviderFieldHint, error)
}

// ImageSink stores a downloaded, normalized provider asset as a person image (F25,
// ADR-038). It is satisfied by an adapter over personimage + the repo, wired in
// main; nil disables asset download (the v1-without-images path). Kept an interface
// so the enrich package needn't import personimage/repo for the image write and so
// tests can assert what would be stored with no disk.
type ImageSink interface {
	// StoreAsset normalizes raw image bytes (metadata strip) and stores them under the
	// given role for a person, recording provenance (provider + externalID) and the
	// upstream asset URL (for delete-suppression, F25/ADR-043). overCap, set for an
	// owner/admin enrichment run (HOLODEX-174), lets a gallery 'extra' bypass
	// repo.GalleryCap the same way an owner's manual "Add anyway" upload does.
	StoreAsset(ctx context.Context, personID int64, role, provider, externalID, url string, raw []byte, overCap bool) error
	// StoreAssetIfAbsent stores under a core role only when that slot is currently empty
	// (no-op otherwise), so a poster can be seeded from the headshot portrait without
	// clobbering an existing owner/provider image (F25.29).
	StoreAssetIfAbsent(ctx context.Context, personID int64, role, provider, externalID, url string, raw []byte) error
	// SuppressedAssetURLs returns asset URLs the owner deleted for this person, so a
	// re-enrich skips re-adding them (F25, ADR-043).
	SuppressedAssetURLs(ctx context.Context, personID int64) (map[string]struct{}, error)
	// LockedCoreRoles returns the core roles the owner set by hand (upload/promoted),
	// which enrichment must never overwrite (F33, ADR-049). An empty or provider-set
	// slot is absent from the set and stays refreshable.
	LockedCoreRoles(ctx context.Context, personID int64) (map[string]struct{}, error)
	// ExistingAssetURLs returns asset URLs already stored for this person, so a gallery
	// asset whose URL we already hold is skipped before any download (F34/ADR-050 URL
	// fast-path). The content-hash check (in StoreAsset) remains the authoritative
	// guard for the same image under a different URL.
	ExistingAssetURLs(ctx context.Context, personID int64) (map[string]struct{}, error)
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
	// fieldHints caches the persisted provider render hints (F39, ADR-056) behind an
	// atomic pointer — lazily loaded and refreshed on /describe, so the visitor read
	// path never queries the table (mirrors the mapping/registry store idiom).
	fieldHints atomic.Pointer[map[string]map[string]repo.ProviderFieldHint]
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

// ImageURLAllowed reports whether a provider-hinted image_url value may render as an
// <img> — i.e. its host is on that provider's asset-host allowlist (base_url host or
// an operator asset_hosts entry, ADR-039). Used by F39 auto-registration to gate an
// image_url render mode; a disallowed value falls back to text. An unknown/disabled
// provider is not allowed.
func (s *Service) ImageURLAllowed(provider, rawURL string) bool {
	src, ok := s.store.Current().ByName(provider)
	if !ok {
		return false
	}
	return assetHostAllowed(src, rawURL)
}

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
	s.persistFieldHints(ctx, provider, m)
	return c, nil
}

// DescribeProvider fetches and protocol-verifies a provider's /describe manifest
// (ADR-059) — the accessor the provider-icon relink uses to read `brand_icon` without
// running a resolve/enrich. An unknown/disabled provider is an error, never a dialed
// URL (the SSRF allowlist). Does not persist field hints; icon relink is a boot /
// config-reload concern, not part of the owner enrich hot path.
func (s *Service) DescribeProvider(ctx context.Context, provider string) (Manifest, error) {
	_, c, err := s.client(provider)
	if err != nil {
		return Manifest{}, err
	}
	m, err := c.Describe(ctx)
	if err != nil {
		return Manifest{}, err
	}
	if err := verifyProtocol(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// FieldHints returns the cached provider render-hint map (F39, ADR-056), keyed by
// provider then field key. It is lazily loaded from the store on first use and
// refreshed whenever /describe is persisted, so the visitor read path never queries
// the table. Nil-safe for callers; the map is treated as immutable.
func (s *Service) FieldHints(ctx context.Context) map[string]map[string]repo.ProviderFieldHint {
	if p := s.fieldHints.Load(); p != nil {
		return *p
	}
	return s.reloadFieldHints(ctx)
}

// reloadFieldHints reads the hint table and swaps it into the cache atomically. A
// read error logs and caches an empty map (auto-registration then falls back to the
// title-case floor) rather than failing the page.
func (s *Service) reloadFieldHints(ctx context.Context) map[string]map[string]repo.ProviderFieldHint {
	m, err := s.repo.ProviderFieldHints(ctx)
	if err != nil {
		s.log.Warn("load provider field hints", "err", err)
		m = map[string]map[string]repo.ProviderFieldHint{}
	}
	s.fieldHints.Store(&m)
	return m
}

// persistFieldHints refreshes the stored non-canonical render hints for a provider
// from its /describe manifest (F39, ADR-056). /describe is read on every owner
// resolve/enrich, so the write is skipped when the provider's hints are unchanged
// from the cache — avoiding writeMu contention on a no-op. Best effort: a failure
// logs and is swallowed so it never blocks the owner's action.
func (s *Service) persistFieldHints(ctx context.Context, provider string, m Manifest) {
	sanitized := SanitizeFieldHints(m.FieldHints)
	desired := make(map[string]repo.ProviderFieldHint, len(sanitized))
	hints := make([]repo.ProviderFieldHint, 0, len(sanitized))
	for key, h := range sanitized {
		ph := repo.ProviderFieldHint{FieldKey: key, Label: h.Label, Render: h.Render, Group: h.Group, Order: h.Order}
		desired[key] = ph
		hints = append(hints, ph)
	}
	if cur := s.fieldHints.Load(); cur != nil && maps.Equal((*cur)[provider], desired) {
		return // unchanged since the last /describe — no write needed
	}
	if err := s.repo.ReplaceProviderFieldHints(ctx, provider, hints); err != nil {
		s.log.Warn("persist provider field hints", "provider", provider, "err", err)
		return
	}
	s.reloadFieldHints(ctx) // reflect the new state in the cache
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
// bypassGalleryCap, set when the caller is the owner/admin (HOLODEX-174), lets the
// gallery auto-fill keep adding provider assets past repo.GalleryCap; every current
// caller is behind requireOwner, so it is always true today, but the enrich service
// itself must not assume that — it takes the caller's privilege as an explicit input
// rather than inferring it.
func (s *Service) Enrich(ctx context.Context, entityType string, entityID int64, provider, externalID string, bypassGalleryCap bool) ([]model.EnrichedField, error) {
	started := time.Now()
	fields, err := s.runEnrich(ctx, entityType, entityID, provider, externalID, bypassGalleryCap)
	s.recordEnrichJob(started, provider, entityType, entityID, len(fields), err)
	return fields, err
}

// runEnrich is the core fetch → sanitize → store → re-read; Enrich wraps it with
// activity-history recording.
func (s *Service) runEnrich(ctx context.Context, entityType string, entityID int64, provider, externalID string, bypassGalleryCap bool) ([]model.EnrichedField, error) {
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
		s.downloadAssets(ctx, entityID, provider, externalID, res.Assets, bypassGalleryCap)
	}
	return s.Fields(ctx, entityType, entityID)
}

// downloadAssets fetches provider image assets through the SSRF-guarded asset client
// and stores them via the image sink (F25, ADR-038/039). Assets are preference-ordered.
// Core roles (headshot/banner/poster) fill on first success and then skip further
// entries of the same role (ADR-039 §5). The gallery role (extra) is unbounded on the
// provider side but capped at repo.GalleryCap by the store; once that cap is hit,
// remaining gallery assets are skipped — unless bypassGalleryCap is set (owner/admin
// enrichment run, HOLODEX-174), in which case a cap hit is treated as a per-asset
// skip rather than a role-wide stop, so later assets still get a shot at the store.
func (s *Service) downloadAssets(ctx context.Context, entityID int64, provider, externalID string, assets []Asset, bypassGalleryCap bool) {
	src, ok := s.store.Current().ByName(provider)
	if !ok { // unreachable after verifiedClient, but keep the allowlist explicit
		return
	}
	// Asset URLs the owner has deleted before — skip them so a re-enrich doesn't
	// silently re-add an image the owner removed (F25, ADR-043). A lookup failure
	// fails open (logs, treats nothing as suppressed) rather than blocking enrichment.
	suppressed, err := s.images.SuppressedAssetURLs(ctx, entityID)
	if err != nil {
		s.log.Warn("suppressed asset urls lookup failed", "provider", provider, "person", entityID, "err", err)
		suppressed = nil
	}
	// Core roles the owner set by hand (upload/promoted): enrichment never overwrites
	// them (F33, ADR-049). Like the suppression lookup this fails open — a lookup error
	// locks nothing rather than blocking enrichment.
	locked, err := s.images.LockedCoreRoles(ctx, entityID)
	if err != nil {
		s.log.Warn("locked core roles lookup failed", "provider", provider, "person", entityID, "err", err)
		locked = nil
	}
	// Asset URLs already stored for this person — skip re-fetching a gallery URL we
	// already hold (F34/ADR-050 URL fast-path), so a re-enrich doesn't re-download and
	// re-dedup the same image. Fails open like the lookups above. The content-hash
	// check in the store still catches the same image under a *different* URL.
	existingURLs, err := s.images.ExistingAssetURLs(ctx, entityID)
	if err != nil {
		s.log.Warn("existing asset urls lookup failed", "provider", provider, "person", entityID, "err", err)
		existingURLs = nil
	}
	fetcher := s.newAssetGet(src)
	done := make(map[string]bool) // role → filled (core roles) or capped (extra)
	// The portrait we stored as the headshot, kept so an empty poster can be seeded from
	// it after the loop (F25.29) — provider profiles are 2:3, a natural poster.
	var headshotRaw []byte
	var headshotURL string
	for _, a := range assets {
		role, ok := assetRoleFor(a.Kind)
		if !ok || done[role] {
			continue
		}
		if _, owned := locked[role]; owned {
			continue // owner set this core slot by hand; never overwrite (F33, ADR-049)
		}
		if _, skip := suppressed[a.URL]; skip {
			continue // owner deleted this exact image before; don't bring it back
		}
		// URL fast-path (F34/ADR-050): a gallery asset whose URL we already store is a
		// guaranteed duplicate — skip without downloading. Scoped to extra; core roles
		// replace in place, so re-fetching their URL is harmless and the seed logic below
		// relies on a fresh headshot fetch.
		if role == model.PersonImageExtra {
			if _, have := existingURLs[a.URL]; have {
				continue
			}
		}
		raw, err := fetcher.Fetch(ctx, a.URL)
		if err != nil {
			s.log.Warn("asset fetch refused/failed", "provider", provider, "kind", a.Kind, "err", err)
			continue
		}
		if err := s.images.StoreAsset(ctx, entityID, role, provider, externalID, a.URL, raw, bypassGalleryCap); err != nil {
			if errors.Is(err, repo.ErrGalleryFull) {
				done[role] = true // cap reached; skip remaining gallery assets
			} else {
				s.log.Warn("asset store failed", "provider", provider, "kind", a.Kind, "err", err)
			}
			continue
		}
		if role == model.PersonImageHeadshot {
			headshotRaw, headshotURL = raw, a.URL
		}
		if model.CorePersonImageRole(role) {
			done[role] = true // core slots are single-occupancy; first success wins
		}
		// extra/gallery: don't mark done — allow additional items up to the cap
	}
	// Seed a poster from the headshot portrait when this run filled a headshot but no
	// poster (F25.29) — the same image reused with no extra download, so people read
	// richly on video-credit surfaces. Only fills an EMPTY slot; never overwrites an
	// existing owner/provider poster. Like other core roles it refills on re-enrich
	// (core deletes don't suppress, ADR-043 F25.25). Best-effort.
	if _, posterLocked := locked[model.PersonImagePoster]; headshotRaw != nil && !done[model.PersonImagePoster] && !posterLocked {
		if err := s.images.StoreAssetIfAbsent(ctx, entityID, model.PersonImagePoster, provider, externalID, headshotURL, headshotRaw); err != nil {
			s.log.Warn("poster seed from headshot failed", "provider", provider, "person", entityID, "err", err)
		}
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

// FetchAsset downloads an asset URL through the named provider's SSRF-guarded asset
// client (ADR-039) and returns the raw bytes for the caller to normalize+store. It is
// the seam the studio-logo cache uses (HOLODEX-130, ADR-057): the resolved `logo`
// field's URL is fetched under the winning provider's own host allowlist — the same
// perimeter (host allowlist, https-for-cross-host, redirect refusal, 16 MiB / 15 s
// caps) that person portraits pass. An unknown/disabled provider is an error, never a
// dialed URL; the URL host must still be on that provider's allowlist or Fetch refuses.
func (s *Service) FetchAsset(ctx context.Context, provider, rawURL string) ([]byte, error) {
	src, ok := s.store.Current().ByName(provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", provider)
	}
	return s.newAssetGet(src).Fetch(ctx, rawURL)
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
		// Internal sidecar fields (ADR-054, e.g. _studio_external_ids) are provider→core
		// plumbing the core reads directly — never display them.
		if strings.HasPrefix(r.FieldKey, model.InternalFieldPrefix) {
			continue
		}
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
		in[i].ProfileURL = sanitizeProfileURL(in[i].ProfileURL)
	}
	return in
}

// sanitizeProfileURL bounds a candidate's provider-supplied profile_url (F47,
// RD6/P1-1): only an http(s) URL survives. It becomes a picker `href` client-side,
// so a hostile scheme (javascript:, data:, …) is dropped rather than erroring —
// the candidate itself is still usable, just without the "view source" link.
func sanitizeProfileURL(raw string) string {
	raw = SanitizeValue(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return raw
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

// SanitizeValues applies SanitizeValue to each input and drops any that are empty
// after cleaning. Shared by the writeback handler and the durable write queue so
// both apply identical input rules (F30).
func SanitizeValues(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if s := SanitizeValue(v); s != "" {
			out = append(out, s)
		}
	}
	return out
}
