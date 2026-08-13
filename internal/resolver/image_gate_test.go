package resolver_test

import (
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/resolver"
)

// imageField builds a single-source image_url mapping.Field winning from
// namespace:key, mirroring stubField but with an explicit Display override
// (HOLODEX-212) — canonical poster_url/photo fields carry this in production
// via registry.Lookup, but a literal override keeps this test independent of
// the registry's current defaults.
func imageField(canonical, source string) mapping.Field {
	f := stubField(canonical, false, source)
	f.Display = registry.DisplayImageURL
	return f
}

var imageEnrich = resolver.Enrichment{
	"tmdb": {"poster_url": {"https://image.tmdb.org/poster.jpg"}},
}

var fileCoverExtra = []model.ExtraMetadata{{SourceKey: "Cover", Value: "https://file-embedded.example/cover.jpg"}}

var manualDecision = resolver.Decisions{"poster_url": {Source: "manual", ManualValue: "https://owner-typed.example/cover.jpg"}}

// TestResolveFields_ImageGate covers the ADR-039 asset-host allowlist gate on a
// resolved image_url field (HOLODEX-212): a disallowed provider host degrades to
// text, an allowed one keeps its image_url Display, a nil callback fails closed,
// and file/manual sources — never the untrusted provider vector this perimeter
// protects — always pass through ungated regardless of the callback.
func TestResolveFields_ImageGate(t *testing.T) {
	denyAll := func(provider, rawURL string) bool { return false }
	allowAll := func(provider, rawURL string) bool { return true }

	cases := []struct {
		name        string
		fields      []mapping.Field
		extra       []model.ExtraMetadata
		enrich      resolver.Enrichment
		opts        resolver.Options
		wantDisplay string
	}{
		{
			name:        "provider disallowed degrades to text",
			fields:      []mapping.Field{imageField("poster_url", "tmdb:poster_url")},
			enrich:      imageEnrich,
			opts:        resolver.Options{ImageURLAllowed: denyAll},
			wantDisplay: registry.DisplayText,
		},
		{
			name:        "provider allowed keeps image display",
			fields:      []mapping.Field{imageField("poster_url", "tmdb:poster_url")},
			enrich:      imageEnrich,
			opts:        resolver.Options{ImageURLAllowed: allowAll},
			wantDisplay: registry.DisplayImageURL,
		},
		{
			name:        "nil callback fails closed",
			fields:      []mapping.Field{imageField("poster_url", "tmdb:poster_url")},
			enrich:      imageEnrich,
			opts:        resolver.Options{},
			wantDisplay: registry.DisplayText,
		},
		{
			name:        "file source passes through ungated",
			fields:      []mapping.Field{imageField("poster_url", "file:Cover")},
			extra:       fileCoverExtra,
			enrich:      resolver.Enrichment{},
			opts:        resolver.Options{ImageURLAllowed: denyAll},
			wantDisplay: registry.DisplayImageURL,
		},
		{
			name:        "manual source passes through ungated",
			fields:      []mapping.Field{imageField("poster_url", "tmdb:poster_url")},
			enrich:      imageEnrich,
			opts:        resolver.Options{Decisions: manualDecision, ImageURLAllowed: denyAll},
			wantDisplay: registry.DisplayImageURL,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolver.Resolve(testVideo, tc.extra, tc.enrich, nil, tc.fields, tc.opts)
			if len(got) != 1 {
				t.Fatalf("want 1 field, got %d", len(got))
			}
			if got[0].Display != tc.wantDisplay {
				t.Errorf("Display = %q, want %q", got[0].Display, tc.wantDisplay)
			}
		})
	}
}
