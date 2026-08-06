// Package enrich implements the metadata source plugin seam (F22, ADR-033):
// external providers (sidecar containers) speaking a small HTTP/JSON contract
// enrich local entities with data the files don't carry. A provider is "just
// another source" feeding the field layer; its data is stored in a shadow layer
// (repo.entity_enrichment) kept distinct from file-extracted metadata, merged for
// display with provenance.
//
// The package is split so the orchestration (Service) is unit-testable against an
// injected ProviderClient with no network: client.go is the HTTP transport,
// fake.go an in-process implementation, service.go the orchestration over the
// repo, and this file the config registry + wire types.
package enrich

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"unicode"

	"gopkg.in/yaml.v3"

	"holodex/internal/model"
	"holodex/internal/registry"
)

// ProtocolVersion is the provider contract major version this build speaks
// (ADR-033 F22.1e). A provider whose /describe reports a different major is
// refused loudly rather than mis-parsed.
const ProtocolVersion = 1

// Source is one configured provider (a sidecar). Declared in metadata-sources.yaml
// — mirroring metadata-mappings.yaml — never compiled in (ADR-033).
type Source struct {
	Name        string   `yaml:"name"`
	BaseURL     string   `yaml:"base_url"`
	EntityTypes []string `yaml:"entity_types"`
	// AssetHosts is the operator-configured CDN host allowlist for asset URLs (ADR-039).
	// The provider's own base_url host is always implicitly allowed. Asset URLs whose
	// host is neither the base host nor in this list are refused.
	AssetHosts []string `yaml:"asset_hosts"`
	Enabled    bool     `yaml:"enabled"`
	// SearchPattern is an optional operator override of this provider's /resolve
	// search-query shape (ADR-080 D2 tier 1, highest precedence — outranks both the
	// provider's own advertised preference and the global default). See query.go for
	// the {name}/{name?} grammar. Validated at config-load time by parse(); an
	// unknown token name drops just this value (logged, provider unaffected).
	SearchPattern string `yaml:"search_pattern"`
}

// Supports reports whether the source advertises an entity type (case-insensitive).
func (s Source) Supports(entityType string) bool {
	return entityTypesSupport(s.EntityTypes, entityType)
}

// entityTypesSupport reports whether entityType (case-insensitive) is among types —
// the shared check behind both Source.Supports and SourceInfo.Supports.
func entityTypesSupport(types []string, entityType string) bool {
	for _, t := range types {
		if strings.EqualFold(strings.TrimSpace(t), entityType) {
			return true
		}
	}
	return false
}

type fileConfig struct {
	Sources []Source `yaml:"sources"`
	// DefaultSearchPattern is the fleet-wide fallback search-query pattern (ADR-080 D2
	// tier 3), applied to any enabled provider that sets neither its own
	// SearchPattern nor advertises a preferred_search_pattern. Same grammar and
	// config-load validation as Source.SearchPattern.
	DefaultSearchPattern string `yaml:"default_search_pattern"`
}

// Registry is an immutable, parsed provider list (swapped atomically on reload,
// never mutated in place — like the mapping config).
type Registry struct {
	sources        []Source
	defaultPattern string
}

// Empty is the no-providers configuration (the default when no file is present).
func Empty() *Registry { return &Registry{} }

// DefaultSearchPattern returns the global fallback search-query pattern (ADR-080 D2
// tier 3), or "" when unconfigured or invalid.
func (reg *Registry) DefaultSearchPattern() string { return reg.defaultPattern }

// validatedPattern trims pattern and returns it unchanged when empty (no tier
// configured, not an error) or well-formed. A malformed pattern — an unknown token
// name, or anything not a space-joined {name}/{name?} list (ADR-080 D3) — is logged
// and dropped (returns ""), so BuildQuery never has to re-validate or re-warn on
// every render; what named this pattern (e.g. "tmdb.search_pattern") is logged so a
// warning is actionable. log may be nil (tests that don't care about warnings).
func validatedPattern(pattern, what string, log *slog.Logger) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || ValidatePattern(pattern) {
		return pattern
	}
	if log != nil {
		log.Warn("invalid search query pattern, ignoring", "field", what, "pattern", pattern)
	}
	return ""
}

func parse(data []byte, log *slog.Logger) (*Registry, error) {
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse metadata sources: %w", err)
	}
	reg := &Registry{defaultPattern: validatedPattern(fc.DefaultSearchPattern, "default_search_pattern", log)}
	for _, s := range fc.Sources {
		s.Name = strings.TrimSpace(s.Name)
		s.BaseURL = strings.TrimSpace(s.BaseURL)
		if s.Name == "" || s.BaseURL == "" {
			continue // skip malformed entries rather than failing the whole load
		}
		s.SearchPattern = validatedPattern(s.SearchPattern, s.Name+".search_pattern", log)
		reg.sources = append(reg.sources, s)
	}
	return reg, nil
}

// Load reads the provider registry from path. A missing file yields Empty
// (providers are optional), not an error. log receives config-validation warnings
// (e.g. a malformed search_pattern) and may be nil.
func Load(path string, log *slog.Logger) (*Registry, error) {
	if path == "" {
		return Empty(), nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Empty(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read metadata sources %s: %w", path, err)
	}
	return parse(data, log)
}

// Enabled returns the configured providers that are switched on.
func (reg *Registry) Enabled() []Source {
	out := make([]Source, 0, len(reg.sources))
	for _, s := range reg.sources {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out
}

// ByName resolves an enabled source by name (case-insensitive). A disabled or
// unknown name returns ok=false — the allowlist that prevents an arbitrary
// base_url from being dialed (ADR-033 F22.2b SSRF guard).
func (reg *Registry) ByName(name string) (Source, bool) {
	name = strings.TrimSpace(name)
	for _, s := range reg.sources {
		if s.Enabled && strings.EqualFold(s.Name, name) {
			return s, true
		}
	}
	return Source{}, false
}

// Store holds the current Registry behind an atomic pointer so reads are
// lock-free and reload (POST /admin/reload-config) swaps atomically — the same
// atomic-swap shape as mapping.Store, plus a held logger (mapping.Store has none)
// so config-validation warnings (e.g. an invalid search_pattern) have somewhere to
// go on both the initial load and every subsequent Reload.
type Store struct {
	path string
	log  *slog.Logger
	cur  atomic.Pointer[Registry]
}

// NewStore loads the initial registry from path (missing file is fine). log receives
// config-validation warnings on load and every subsequent Reload; may be nil.
func NewStore(path string, log *slog.Logger) (*Store, error) {
	s := &Store{path: path, log: log}
	reg, err := Load(path, log)
	if err != nil {
		return nil, err
	}
	s.cur.Store(reg)
	return s, nil
}

// Current returns the live registry; callers treat the result as immutable.
func (s *Store) Current() *Registry { return s.cur.Load() }

// Reload re-reads the config file and swaps it in atomically.
func (s *Store) Reload() error {
	reg, err := Load(s.path, s.log)
	if err != nil {
		return err
	}
	s.cur.Store(reg)
	return nil
}

// ---- provider protocol wire types (ADR-033 F22.1) ----

// Manifest is the provider capability description (`GET /describe`).
type Manifest struct {
	Provider        string   `json:"provider"`
	Version         string   `json:"version"`
	ProtocolVersion int      `json:"protocol_version"`
	EntityTypes     []string `json:"entity_types"`
	IDNamespaces    []string `json:"id_namespaces"`
	Fields          []string `json:"fields"`
	// AssetKinds advertises the asset kinds this provider may return in /enrich assets[].
	// Advisory/display only; Holodex does not gate fetching on it (ADR-039 §2).
	AssetKinds []string `json:"asset_kinds,omitempty"`
	// FieldHints carries optional per-field presentation hints (label / render mode /
	// ordering group) for the provider's advertised *non-canonical* keys (F39,
	// ADR-056), keyed by field key. Additive and forward-compatible: an older Holodex
	// ignores it, and a provider that omits it is unaffected. Hints are untrusted and
	// sanitized on ingest (SanitizeFieldHints); a hint for a canonical or `_`-prefixed
	// key is dropped, and only the render/group vocabulary in internal/registry is
	// honored.
	FieldHints map[string]FieldHint `json:"field_hints,omitempty"`
	// BrandIcon is the provider's optional self-brand icon (ADR-059): one
	// provider-level image URL Holodex downloads, normalizes, self-hosts, and shows in
	// place of the repeated "from <provider>" provenance text. Additive/forward-
	// compatible (an older Holodex ignores it; a provider that omits it is unaffected).
	// A pointer so an absent key (drop any cached icon) is distinguishable from an empty
	// object. The URL is fetched through the same ADR-039 asset perimeter as portraits.
	BrandIcon *IconRef `json:"brand_icon,omitempty"`
	// PreferredSearchPattern is the provider's advertised /resolve search-query shape
	// (ADR-080 D2 tier 2, FR2): a {name}/{name?} pattern over studio/title/performers/
	// year (query.go), consulted only when the operator has set no search_pattern
	// override for this provider. Additive/forward-compatible: an older provider that
	// omits it, or an older Holodex that doesn't parse it, both work unchanged — no
	// protocol version bump. Untrusted provider input; validated (ValidatePattern) and
	// cached by the Service on every /describe, same posture as FieldHints/BrandIcon.
	PreferredSearchPattern string `json:"preferred_search_pattern,omitempty"`
}

// IconRef is a single provider-level image reference — currently only the brand icon
// (ADR-059). Separate from Asset (which carries a per-entity Kind) because a brand icon
// belongs to the provider, not to any entity or external_id.
type IconRef struct {
	URL string `json:"url"`
}

// FieldHint is one provider-advertised presentation hint for a non-canonical field
// (F39, ADR-056). Every field is optional; absent/invalid values fall back to safe
// defaults (title-cased label, text render, extended group, order 0).
type FieldHint struct {
	Label  string `json:"label,omitempty"`
	Render string `json:"render,omitempty"`
	Group  string `json:"group,omitempty"`
	Order  int    `json:"order,omitempty"`
}

// maxHintLabelLen bounds an untrusted provider-supplied label (F39). Field values
// are capped at maxFieldLen; a label is a short display string, capped tighter.
const maxHintLabelLen = 64

// SanitizeFieldHints coerces an untrusted /describe.field_hints map to storable,
// display-safe hints (F39, ADR-056): it drops hints for canonical or `_`-prefixed
// keys (a provider may not relabel a canonical field, and reserved sidecars never
// display), strips control characters and caps the label, and normalizes the render
// mode and ordering group to the known vocabulary. Returns nil when nothing survives.
func SanitizeFieldHints(in map[string]FieldHint) map[string]FieldHint {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]FieldHint, len(in))
	for key, h := range in {
		k := strings.ToLower(strings.TrimSpace(key))
		if k == "" || strings.HasPrefix(k, model.InternalFieldPrefix) || registry.IsKnown(k) {
			continue // reserved sidecar, or canonical (registry owns it) — hint is inert
		}
		out[k] = FieldHint{
			Label:  sanitizeHintLabel(h.Label),
			Render: registry.NormalizeDisplay(h.Render),
			Group:  registry.NormalizeGroup(h.Group),
			Order:  h.Order,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SanitizeFieldLabel exposes the F39 label sanitizer for the in-app field-promotion
// ingest (F44, ADR-062): an owner-supplied promotion label is owner-authored but still
// bounded and control-char-stripped on the way in (defense in depth), sharing one
// sanitizer with the provider-hint path so the two stay in lockstep.
func SanitizeFieldLabel(s string) string { return sanitizeHintLabel(s) }

// sanitizeHintLabel strips control characters (collapsing to spaces), trims, and
// caps a provider-supplied label. Rendering escapes it, so this only bounds size and
// removes control noise.
func sanitizeHintLabel(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	if len(s) > maxHintLabelLen {
		s = strings.TrimSpace(s[:maxHintLabelLen])
	}
	return s
}

// Hint is the identity input to a resolve call: embedded external ids (the
// deterministic path) and/or a free-text name query (the fallback).
type Hint struct {
	Query       string   `json:"query,omitempty"`
	ExternalIDs []string `json:"external_ids,omitempty"`
}

// Candidate is one ranked match from `POST /resolve`. Confidence stays
// provider-native and non-normalized (ADR-033 §2.3). AutoApply is set once in
// sanitizeCandidates (see StrongMatchThreshold) — every other consumer, in this
// package and the frontend, reads AutoApply rather than re-deriving it.
type Candidate struct {
	ExternalID     string  `json:"external_id"`
	Namespace      string  `json:"namespace"`
	Label          string  `json:"label"`
	Confidence     float64 `json:"confidence"`
	Disambiguation string  `json:"disambiguation,omitempty"`
	AutoApply      bool    `json:"auto_apply"`
	// ProfileURL is an optional provider-supplied link to its own page for this
	// candidate (e.g. a TMDB person/company page), rendered as "view source ↗" in
	// the picker (F47, RD6/P1-1). sanitizeCandidates drops anything that isn't
	// http(s) before this ever reaches an API response — it becomes an `href`.
	ProfileURL string `json:"profile_url,omitempty"`
}

// StrongMatchThreshold is the auto-apply confidence cutoff (ADR-066 D1) — the sole
// source of truth for Candidate.AutoApply, computed once in sanitizeCandidates.
const StrongMatchThreshold = 0.85

// SingleStrongMatch reports the sole candidate an auto-apply should apply (ADR-066 D1):
// exactly one candidate with AutoApply=true. Zero, or two-or-more, strong candidates
// return ok=false — ambiguity always stops at the owner. Callers must pass candidates
// that already went through sanitizeCandidates (every h.enrich.Resolve result does).
func SingleStrongMatch(cands []Candidate) (Candidate, bool) {
	var strong Candidate
	n := 0
	for _, c := range cands {
		if c.AutoApply {
			strong = c
			n++
		}
	}
	if n == 1 {
		return strong, true
	}
	return Candidate{}, false
}

// EnrichResult is the payload of `POST /enrich`: canonical field -> value(s), and
// optional asset URLs. For a person, image assets are fetched through the SSRF-guarded
// asset client and stored as person images (F25, ADR-038/039).
type EnrichResult struct {
	Fields map[string][]string `json:"fields"`
	Assets []Asset             `json:"assets,omitempty"`
}

// Asset is a binary the provider can supply (e.g. a portrait). Kind maps to a
// person-image role via assetRoleFor (F25); an unknown kind is skipped.
type Asset struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

// ProviderClient is the transport-agnostic provider contract. The HTTP client
// (client.go) is the production impl; tests inject a fake.
type ProviderClient interface {
	Describe(ctx context.Context) (Manifest, error)
	Resolve(ctx context.Context, entityType string, hint Hint) ([]Candidate, error)
	Enrich(ctx context.Context, entityType, externalID string) (EnrichResult, error)
}
