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

	"gopkg.in/yaml.v3"
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
	Enabled     bool     `yaml:"enabled"`
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
// optional asset URLs. Asset download (e.g. person photos) is deferred (ADR-033
// non-goal for v1 — see Phase 3 F14.3), so Assets is parsed but not yet fetched.
type EnrichResult struct {
	Fields map[string][]string `json:"fields"`
	Assets []Asset             `json:"assets,omitempty"`
}

// Asset is a binary the provider can supply (e.g. a portrait). Reserved; not
// fetched in v1.
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
