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
// tiny "person" source, a tiny "studio" source (F38 S3), and a tiny "film" source
// (F56/ADR-086): name-search returns candidates, enrich returns fields.
type Fake struct {
	Name     string
	Protocol int                   // 0 => use the supported ProtocolVersion
	People   map[string]FakePerson // keyed by external id (e.g. "tmdb:608")
	Studios  map[string]FakePerson // keyed by external id (e.g. "tmdb:10342")
	Films    map[string]FakePerson // keyed by external id (e.g. "tmdb:129")
	Calls    int                   // number of network-equivalent calls made
}

// FakePerson is one canned upstream record (used for people, studios, and video
// alike — a label plus fields/assets/people is all the contract needs).
type FakePerson struct {
	Label          string
	Disambiguation string // the picker hint (entity-appropriate: known-for, origin country, …)
	ProfileURL     string // optional view-source link (F47, RD6/P1-1); tests may set a hostile scheme
	Fields         map[string][]string
	Assets         []Asset          // optional image assets (F25) the enrich response carries
	People         []ProviderPerson // structured video credits (F32, contract §4.5)
}

// NewFake builds a fake provider with one well-known person and one studio record.
func NewFake(name string) *Fake {
	return &Fake{
		Name:     name,
		Protocol: ProtocolVersion,
		People: map[string]FakePerson{
			"tmdb:608": {
				Label:          "Hayao Miyazaki",
				Disambiguation: "Director · Studio Ghibli",
				Fields: map[string][]string{
					"bio":         {"Japanese filmmaker and co-founder of Studio Ghibli."},
					"birthdate":   {"1941-01-05"},
					"nationality": {"Japanese"},
					"aliases":     {"宮崎駿", "Miyazaki Hayao"},
				},
			},
		},
		Studios: map[string]FakePerson{
			"tmdb:10342": {
				Label:          "Studio Ghibli",
				Disambiguation: "JP",
				Fields: map[string][]string{
					"description": {"Japanese animation film studio founded in 1985."},
					"country":     {"JP"},
					"website":     {"https://www.ghibli.jp"},
				},
				// logo arrives as an image asset (F51, ADR-079), not a plain field — the
				// studio-image asset-slot model, mirroring how a person's photo arrives.
				Assets: []Asset{{Kind: "logo", URL: "https://image.tmdb.org/t/p/original/ghibli.png"}},
			},
		},
		Films: map[string]FakePerson{
			"tmdb:129": {
				Label:          "Spirited Away",
				Disambiguation: "2001",
				Fields: map[string][]string{
					// description is film's canonical synopsis key (ADR-086 §3), not
					// overview — the real tmdb provider remaps overview -> description
					// for entity_type "film"; the fake just cans the post-remap shape.
					"description": {"A girl wanders into a world ruled by gods and witches."},
				},
				// poster arrives as an image asset (F56/ADR-086 §3), never a resolved
				// field, mirroring how a studio's logo arrives.
				Assets: []Asset{{Kind: "poster", URL: "https://image.tmdb.org/t/p/original/spirited-away.png"}},
			},
		},
	}
}

// records returns the canned map for an entity type (studio → Studios, film → Films,
// else People).
func (f *Fake) records(entityType string) map[string]FakePerson {
	switch entityType {
	case model.EnrichEntityStudio:
		return f.Studios
	case model.EnrichEntityFilm:
		return f.Films
	default:
		return f.People
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
		EntityTypes:     []string{model.EnrichEntityPerson, model.EnrichEntityStudio, model.EnrichEntityFilm},
		IDNamespaces:    []string{"tmdb", "imdb"},
		Fields:          []string{"bio", "birthdate", "nationality", "website", "aliases", "description", "country"},
		AssetKinds:      []string{"photo", "logo", "poster"},
	}, nil
}

func (f *Fake) Resolve(_ context.Context, entityType string, hint Hint) ([]Candidate, error) {
	f.Calls++
	records := f.records(entityType)
	// Embedded-id path: echo back any provided id as a strong match.
	for _, id := range hint.ExternalIDs {
		if p, ok := records[id]; ok {
			return []Candidate{{
				ExternalID: id, Namespace: idNamespace(id), Label: p.Label,
				Confidence: 1, ProfileURL: p.ProfileURL,
			}}, nil
		}
	}
	// Name-search fallback: substring match on the canned labels.
	var out []Candidate
	q := strings.ToLower(strings.TrimSpace(hint.Query))
	for id, p := range records {
		if q != "" && strings.Contains(strings.ToLower(p.Label), q) {
			out = append(out, Candidate{
				ExternalID: id, Namespace: idNamespace(id), Label: p.Label,
				Confidence: 0.9, Disambiguation: p.Disambiguation, ProfileURL: p.ProfileURL,
			})
		}
	}
	return out, nil
}

func (f *Fake) Enrich(_ context.Context, entityType, externalID string) (EnrichResult, error) {
	f.Calls++
	p, ok := f.records(entityType)[externalID]
	if !ok {
		return EnrichResult{}, fmt.Errorf("unknown record %q", externalID)
	}
	return EnrichResult{Fields: p.Fields, Assets: p.Assets, People: p.People}, nil
}

func idNamespace(id string) string {
	if i := strings.IndexByte(id, ':'); i > 0 {
		return id[:i]
	}
	return ""
}
