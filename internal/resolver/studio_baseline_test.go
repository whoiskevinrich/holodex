package resolver_test

import (
	"slices"
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/resolver"
)

// studioTestFields mirrors the API's synthesized studio schema (F38): name is the
// only baseline-backed field (record-only, no provider name candidate), the scalars
// are provider-only. No merge field.
func studioTestFields() []mapping.Field {
	return []mapping.Field{
		stubField("name", false, "file:name"),
		stubField("description", false, "tmdb:description"),
		stubField("country", false, "tmdb:country"),
		stubField("website", false, "tmdb:website"),
	}
}

var (
	testStudio   = &model.Studio{ID: 3, Name: "Studio Ghibli"}
	studioEnrich = resolver.Enrichment{"tmdb": {
		"description": {"Japanese animation studio."},
		"country":     {"JP"},
		"website":     {"https://example.org"},
	}}
)

func TestStudioBaseline_NameResolvesFromRecord(t *testing.T) {
	got := resolver.ResolveFields(resolver.NewStudioBaseline(testStudio), studioEnrich, nil, studioTestFields(), resolver.Options{})
	name, ok := resolvedByCanonical(got, "name")
	if !ok || name.Values[0] != "Studio Ghibli" || name.WinningSource != "file:name" {
		t.Fatalf("record name should win: %+v", name)
	}
}

func TestStudioBaseline_NilStudioIsEmptyBaseline(t *testing.T) {
	got := resolver.ResolveFields(resolver.NewStudioBaseline(nil), studioEnrich, nil, studioTestFields(), resolver.Options{})
	// A nil studio has no name and no provider name source, so the name field
	// carries no value (dropped, or present-but-empty) — never a bogus value.
	if name, ok := resolvedByCanonical(got, "name"); ok && len(name.Values) != 0 {
		t.Fatalf("empty record name must not resolve to a value: %+v", name)
	}
}

// RD6 additivity: an enriched studio with zero decisions resolves every scalar to
// exactly the raw enrichment values — inheriting the model changes nothing until the
// owner decides.
func TestStudioBaseline_RD6Additivity(t *testing.T) {
	got := resolver.ResolveFields(resolver.NewStudioBaseline(testStudio), studioEnrich, nil, studioTestFields(), resolver.Options{})
	for canonical, want := range map[string][]string{
		"description": studioEnrich["tmdb"]["description"],
		"country":     studioEnrich["tmdb"]["country"],
		"website":     studioEnrich["tmdb"]["website"],
	} {
		f, ok := resolvedByCanonical(got, canonical)
		if !ok || !slices.Equal(f.Values, want) {
			t.Errorf("%s: undecided resolution must equal raw enrichment: got %+v want %v", canonical, f, want)
		}
	}
}

func TestStudioBaseline_RecordBlankPinSuppressesProvider(t *testing.T) {
	// A standing record decision on an enrichment-only field pins it blank: the
	// provider value is suppressed but the field stays visible so the pin is
	// re-decidable, and a re-enrich can't resurrect it.
	got := resolver.ResolveFields(resolver.NewStudioBaseline(testStudio), studioEnrich, nil, studioTestFields(), decide("description", "file", ""))
	f, ok := resolvedByCanonical(got, "description")
	if !ok || len(f.Values) != 0 {
		t.Fatalf("blank-pinned field must stay visible with no values: %+v", got)
	}
	if f.Decision == nil || !f.Decision.Standing || f.Decision.Source != "file" {
		t.Errorf("want standing file (record) decision, got %+v", f.Decision)
	}
}
