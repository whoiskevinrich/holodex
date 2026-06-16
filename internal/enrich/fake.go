package enrich

import (
	"context"
	"fmt"
	"strings"

	"holodex/internal/model"
)

// Fake is an in-process ProviderClient implementing the contract with canned data
// (ADR-033 F22.10). It lets the enrichment flow run end-to-end with no network or
// API keys — the basis of the CI test path and a local dev stand-in. It models a
// tiny "person" source: name-search returns candidates, enrich returns fields.
type Fake struct {
	Name     string
	Protocol int                   // 0 => use the supported ProtocolVersion
	People   map[string]FakePerson // keyed by external id (e.g. "tmdb:608")
	Calls    int                   // number of network-equivalent calls made
}

// FakePerson is one canned upstream record.
type FakePerson struct {
	Label  string
	Fields map[string][]string
	Assets []Asset // optional image assets (F24) the enrich response carries
}

// NewFake builds a fake person provider with one well-known record.
func NewFake(name string) *Fake {
	return &Fake{
		Name:     name,
		Protocol: ProtocolVersion,
		People: map[string]FakePerson{
			"tmdb:608": {
				Label: "Hayao Miyazaki",
				Fields: map[string][]string{
					"bio":         {"Japanese filmmaker and co-founder of Studio Ghibli."},
					"birthdate":   {"1941-01-05"},
					"nationality": {"Japanese"},
					"aliases":     {"宮崎駿", "Miyazaki Hayao"},
				},
			},
		},
	}
}

func (f *Fake) Describe(_ context.Context) (Manifest, error) {
	f.Calls++
	p := f.Protocol
	if p == 0 {
		p = ProtocolVersion
	}
	return Manifest{
		Provider:        f.Name,
		Version:         "fake-0.1.0",
		ProtocolVersion: p,
		EntityTypes:     []string{model.EnrichEntityPerson},
		IDNamespaces:    []string{"tmdb", "imdb"},
		Fields:          []string{"bio", "birthdate", "nationality", "website", "aliases", "photo"},
	}, nil
}

func (f *Fake) Resolve(_ context.Context, _ string, hint Hint) ([]Candidate, error) {
	f.Calls++
	// Embedded-id path: echo back any provided id as a strong match.
	for _, id := range hint.ExternalIDs {
		if p, ok := f.People[id]; ok {
			return []Candidate{{ExternalID: id, Namespace: idNamespace(id), Label: p.Label, Confidence: 1}}, nil
		}
	}
	// Name-search fallback: substring match on the canned labels.
	var out []Candidate
	q := strings.ToLower(strings.TrimSpace(hint.Query))
	for id, p := range f.People {
		if q != "" && strings.Contains(strings.ToLower(p.Label), q) {
			out = append(out, Candidate{
				ExternalID: id, Namespace: idNamespace(id), Label: p.Label,
				Confidence: 0.9, Disambiguation: "Director · Studio Ghibli",
			})
		}
	}
	return out, nil
}

func (f *Fake) Enrich(_ context.Context, _, externalID string) (EnrichResult, error) {
	f.Calls++
	p, ok := f.People[externalID]
	if !ok {
		return EnrichResult{}, fmt.Errorf("unknown record %q", externalID)
	}
	return EnrichResult{Fields: p.Fields, Assets: p.Assets}, nil
}

func idNamespace(id string) string {
	if i := strings.IndexByte(id, ':'); i > 0 {
		return id[:i]
	}
	return ""
}
