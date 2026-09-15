package api

import (
	"testing"

	"holodex/internal/mapping"
)

// filmReleaseYear feeds the films.year identity fill (F59/ADR-089 D3), so its
// failure mode is not "wrong label" but "wrong identity" — a garbage parse would
// try to claim (name, <nonsense>) as a film's identity key. It therefore fails
// closed: anything it cannot read confidently yields 0, which FillFilmYear treats
// as "nothing to do" rather than as a value to write.
func TestFilmReleaseYear(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  int
	}{
		{"contract-preferred full date", "2001-07-20", 2001},
		{"bare year", "2001", 2001},
		{"leading and trailing space", "  1999-01-01  ", 1999},
		{"slash-separated date still leads with the year", "2001/07/20", 2001},

		// Everything below must yield 0 rather than a plausible-looking number.
		{"empty", "", 0},
		{"too short to hold a year", "201", 0},
		{"day-first date would otherwise parse the day as a year", "20-07-2001", 0},
		{"non-numeric", "soon", 0},
		{"partially numeric", "20x1-07-20", 0},
		{"explicit zero is not a release year", "0000-01-01", 0},
		{"negative", "-999", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := filmReleaseYear(tc.value); got != tc.want {
				t.Errorf("filmReleaseYear(%q) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

// filmScalarFields is the film replace vocabulary; name is deliberately absent from
// it and is synthesized separately: the record baseline first, then each provider's
// `title` spelling (ADR-096 D5, F60 RD9). ADR-089 D3 kept name baseline-only because a
// provider source once meant an ungated rename of half the (name, year) identity key;
// under D5 a name decision is display-only and the column is never written
// (TestFilmFieldDecision pins that), so the provider spelling may be a candidate.
// The guard here is the shape: baseline anchors first, and the provider key is the
// sidecar's `title`, not `name` — the resolver reads enrichment[provider][key], so a
// `name` key would silently never match.
func TestFilmFields_NameSourcesAreBaselineThenProviderTitle(t *testing.T) {
	providers := []string{"tmdb", "fake"}
	fields := filmFields(providers)

	var name *mapping.Field
	for i := range fields {
		if fields[i].Canonical == "name" {
			name = &fields[i]
		}
	}
	if name == nil {
		t.Fatal("filmFields no longer synthesizes a `name` field")
	}
	if len(name.ParsedSources) != 1+len(providers) {
		t.Fatalf("film `name` has %d sources, want the record baseline + one per provider (%d)", len(name.ParsedSources), 1+len(providers))
	}
	if first := name.ParsedSources[0]; first.Namespace != "file" || first.Key != "name" {
		t.Errorf("film `name` baseline must anchor first, got %+v", first)
	}
	for i, p := range providers {
		if s := name.ParsedSources[1+i]; s.Namespace != p || s.Key != "title" {
			t.Errorf("film `name` provider source %d = %+v, want %s:title", i, s, p)
		}
	}

	// release_date, by contrast, *must* stay provider-backed — it is what the year
	// fill reads. A regression that dropped it would make the fill silently dead.
	var haveReleaseDate bool
	for _, f := range fields {
		if f.Canonical == "release_date" && len(f.ParsedSources) > 0 {
			haveReleaseDate = true
		}
	}
	if !haveReleaseDate {
		t.Error("film `release_date` has no provider sources — the films.year fill reads it")
	}
}
