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

// mergeImageField builds a Merge-mode (F30) image_url field over several sources, so
// a test can exercise a field whose resolved value merges more than one provider's
// contribution — the case gateImageDisplay's per-winner check used to miss (HOLODEX-212).
func mergeImageField(canonical string, sources ...string) mapping.Field {
	f := stubField(canonical, false, sources...)
	f.Display = registry.DisplayImageURL
	f.Merge = true
	return f
}

var imageEnrich = resolver.Enrichment{
	"tmdb": {"poster_url": {"https://image.tmdb.org/poster.jpg"}},
}

var mergeImageEnrich = resolver.Enrichment{
	"tmdb":  {"poster_url": {"https://image.tmdb.org/a.jpg"}},
	"other": {"poster_url": {"https://evil.example/b.jpg"}},
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
		{
			// A merge field can carry values from more than one provider (F30); the gate
			// must check every merged value, not just the winner's — else a second,
			// disallowed provider could smuggle an unvetted URL into the same field.
			name:        "merge field: one disallowed value degrades the whole field",
			fields:      []mapping.Field{mergeImageField("poster_url", "tmdb:poster_url", "other:poster_url")},
			enrich:      mergeImageEnrich,
			opts:        resolver.Options{ImageURLAllowed: func(provider, rawURL string) bool { return provider == "tmdb" }},
			wantDisplay: registry.DisplayText,
		},
		{
			name:        "merge field: every value allowed keeps image display",
			fields:      []mapping.Field{mergeImageField("poster_url", "tmdb:poster_url", "other:poster_url")},
			enrich:      mergeImageEnrich,
			opts:        resolver.Options{ImageURLAllowed: allowAll},
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
