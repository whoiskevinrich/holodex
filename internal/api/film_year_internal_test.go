package api

import "testing"

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
// it and is synthesized separately with a baseline-only source. TestFilmFields_
// NameHasNoProviderSource (below) is the guard that keeps it that way.
func TestFilmFields_NameHasNoProviderSource(t *testing.T) {
	// A provider list is supplied precisely so the test fails if `name` ever starts
	// consuming it — with no providers the assertion would pass vacuously.
	fields := filmFields([]string{"tmdb", "fake"})

	var name *struct{ sources int }
	for _, f := range fields {
		if f.Canonical != "name" {
			continue
		}
		name = &struct{ sources int }{sources: len(f.ParsedSources)}
		for _, s := range f.ParsedSources {
			if s.Namespace != "file" {
				t.Errorf("film `name` gained a %q source — that is an ungated rename of half the "+
					"(name, year) identity key. Title enrichment belongs in ADR-061's name-edit "+
					"machinery, not in filmFields (ADR-089 D3, spec F59 Non-Goal 1).", s.Namespace)
			}
		}
	}
	if name == nil {
		t.Fatal("filmFields no longer synthesizes a `name` field")
	}
	if name.sources != 1 {
		t.Errorf("film `name` has %d sources, want exactly 1 (the record baseline)", name.sources)
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
