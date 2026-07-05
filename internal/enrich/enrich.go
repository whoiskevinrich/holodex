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
}

// Supports reports whether the source advertises an entity type (case-insensitive).
func (s Source) Supports(entityType string) bool {
	for _, t := range s.EntityTypes {
		if strings.EqualFold(strings.TrimSpace(t), entityType) {
			return true
		}
	}
	return false
}

type fileConfig struct {
	Sources []Source `yaml:"sources"`
}

// Registry is an immutable, parsed provider list (swapped atomically on reload,
// never mutated in place — like the mapping config).
type Registry struct {
	sources []Source
}

// Empty is the no-providers configuration (the default when no file is present).
func Empty() *Registry { return &Registry{} }

func parse(data []byte) (*Registry, error) {
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse metadata sources: %w", err)
	}
	reg := &Registry{}
	for _, s := range fc.Sources {
		s.Name = strings.TrimSpace(s.Name)
		s.BaseURL = strings.TrimSpace(s.BaseURL)
		if s.Name == "" || s.BaseURL == "" {
			continue // skip malformed entries rather than failing the whole load
		}
		reg.sources = append(reg.sources, s)
	}
	return reg, nil
}

// Load reads the provider registry from path. A missing file yields Empty
// (providers are optional), not an error.
func Load(path string) (*Registry, error) {
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
	return parse(data)
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
// shape as the mapping.Store.
type Store struct {
	path string
	cur  atomic.Pointer[Registry]
}

// NewStore loads the initial registry from path (missing file is fine).
func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	reg, err := Load(path)
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
	reg, err := Load(s.path)
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

// Candidate is one ranked match from `POST /resolve`. Confidence is advisory —
// v1 always has the owner confirm (no silent auto-apply).
type Candidate struct {
	ExternalID     string  `json:"external_id"`
	Namespace      string  `json:"namespace"`
	Label          string  `json:"label"`
	Confidence     float64 `json:"confidence"`
	Disambiguation string  `json:"disambiguation,omitempty"`
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
