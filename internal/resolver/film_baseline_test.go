package resolver_test

import (
	"slices"
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/resolver"
)

// filmTestFields mirrors the API's synthesized film schema (F56): name is the
// only baseline-backed field (record-only, no provider name candidate), the
// scalars are provider-only. No merge field.
func filmTestFields() []mapping.Field {
	return []mapping.Field{
		stubField("name", false, "file:name"),
		stubField("description", false, "tmdb:description"),
		stubField("release_date", false, "tmdb:release_date"),
	}
}

var (
	testFilm   = &model.Film{ID: 42, Name: "Scene Test Film", Year: 2020}
	filmEnrich = resolver.Enrichment{"tmdb": {
		"description":  {"A test film."},
		"release_date": {"2020-01-01"},
	}}
)

func TestFilmBaseline_NameResolvesFromRecord(t *testing.T) {
	got := resolver.ResolveFields(resolver.NewFilmBaseline(testFilm), filmEnrich, nil, filmTestFields(), resolver.Options{})
	name, ok := resolvedByCanonical(got, "name")
	if !ok || name.Values[0] != "Scene Test Film" || name.WinningSource != "file:name" {
		t.Fatalf("record name should win: %+v", name)
	}
}

func TestFilmBaseline_NilFilmIsEmptyBaseline(t *testing.T) {
	got := resolver.ResolveFields(resolver.NewFilmBaseline(nil), filmEnrich, nil, filmTestFields(), resolver.Options{})
	// A nil film has no name and no provider name source, so the name field
	// carries no value (dropped, or present-but-empty) — never a bogus value.
	if name, ok := resolvedByCanonical(got, "name"); ok && len(name.Values) != 0 {
		t.Fatalf("empty record name must not resolve to a value: %+v", name)
	}
}

// RD6 additivity: an enriched film with zero decisions resolves every scalar to
// exactly the raw enrichment values — inheriting the model changes nothing until
// the owner decides.
func TestFilmBaseline_RD6Additivity(t *testing.T) {
	got := resolver.ResolveFields(resolver.NewFilmBaseline(testFilm), filmEnrich, nil, filmTestFields(), resolver.Options{})
	for canonical, want := range map[string][]string{
		"description":  filmEnrich["tmdb"]["description"],
		"release_date": filmEnrich["tmdb"]["release_date"],
	} {
		f, ok := resolvedByCanonical(got, canonical)
		if !ok || !slices.Equal(f.Values, want) {
			t.Errorf("%s: undecided resolution must equal raw enrichment: got %+v want %v", canonical, f, want)
		}
	}
}

func TestFilmBaseline_RecordBlankPinSuppressesProvider(t *testing.T) {
	// A standing record decision on an enrichment-only field pins it blank: the
	// provider value is suppressed but the field stays visible so the pin is
	// re-decidable, and a re-enrich can't resurrect it.
	got := resolver.ResolveFields(resolver.NewFilmBaseline(testFilm), filmEnrich, nil, filmTestFields(), decide("description", "file", ""))
	f, ok := resolvedByCanonical(got, "description")
	if !ok || len(f.Values) != 0 {
		t.Fatalf("blank-pinned field must stay visible with no values: %+v", got)
	}
	if f.Decision == nil || !f.Decision.Standing || f.Decision.Source != "file" {
		t.Errorf("want standing file (record) decision, got %+v", f.Decision)
	}
}
