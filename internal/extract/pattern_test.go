package extract_test

import (
	"reflect"
	"testing"

	"holodex/internal/extract"
)

func TestCompile_InvalidPatterns(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"no tokens", "just a literal filename"},
		{"repeated token", "{title} - {title}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := extract.Compile(tc.pattern); err == nil {
				t.Fatalf("Compile(%q) = nil error, want error", tc.pattern)
			}
		})
	}
}

func TestPattern_Match(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		delimiter string
		filename  string
		want      map[string][]string
		wantOK    bool
	}{
		{
			name:     "full pattern, four mapped tokens, one unmapped consumed (F48.1c)",
			pattern:  "[{studio}] {title} ({people}, {year}) {resolution}",
			filename: "[Acme Studios] Big Movie (Alice Smith, Bob Jones, 2020) 1080p.mp4",
			want: map[string][]string{
				"studio":       {"Acme Studios"},
				"title":        {"Big Movie"},
				"people":       {"Alice Smith", "Bob Jones"},
				"release_date": {"2020"},
			},
			wantOK: true,
		},
		{
			name:     "single person, no comma inside the people token",
			pattern:  "[{studio}] {title} ({people}, {year}) {resolution}",
			filename: "[Acme Studios] Solo Feature (Alice Smith, 2019) 720p.mp4",
			want: map[string][]string{
				"studio":       {"Acme Studios"},
				"title":        {"Solo Feature"},
				"people":       {"Alice Smith"},
				"release_date": {"2019"},
			},
			wantOK: true,
		},
		{
			name:      "custom delimiter (F48.1d)",
			pattern:   "{title} ({people})",
			delimiter: " | ",
			filename:  "Reunion (Alice | Bob | Carol).mkv",
			want: map[string][]string{
				"title":  {"Reunion"},
				"people": {"Alice", "Bob", "Carol"},
			},
			wantOK: true,
		},
		{
			name:     "simple title/year pattern",
			pattern:  "{title} ({year})",
			filename: "Fight Club (1999).mp4",
			want: map[string][]string{
				"title":        {"Fight Club"},
				"release_date": {"1999"},
			},
			wantOK: true,
		},
		{
			name:     "no match falls through (F48.1b)",
			pattern:  "{title} ({year})",
			filename: "does not follow the convention at all.mp4",
			want:     nil,
			wantOK:   false,
		},
		{
			name:     "year token requires exactly four digits",
			pattern:  "{title} ({year})",
			filename: "Fight Club (99).mp4",
			want:     nil,
			wantOK:   false,
		},
		{
			name:     "extension is stripped before matching",
			pattern:  "{title}",
			filename: "Just A Title.mkv",
			want:     map[string][]string{"title": {"Just A Title"}},
			wantOK:   true,
		},
		{
			// A year in the leading parenthetical matches {people} under this
			// two-parenthetical pattern; it must be dropped, not surfaced as a
			// person (HOLODEX-196 #3). The pattern still matches (ok=true) and
			// consumes the file — it just yields no people.
			name:     "bare year in people position is dropped, not a person",
			pattern:  "[{studio}] {title} ({people}) ({resolution})",
			filename: "[MyStudio] Some Title (2011) (1080p).mkv",
			want: map[string][]string{
				"studio": {"MyStudio"},
				"title":  {"Some Title"},
			},
			wantOK: true,
		},
		{
			// Only a value that is entirely digits/date/resolution-shaped is
			// dropped — a real multi-word name that merely contains digits
			// survives.
			name:     "name containing digits is kept",
			pattern:  "{title} ({people})",
			filename: "Doc (Studio 54).mkv",
			want: map[string][]string{
				"title":  {"Doc"},
				"people": {"Studio 54"},
			},
			wantOK: true,
		},
		{
			// A short numeric value (e.g. a sequel/take number) in the
			// {people} position is dropped, not a person (HOLODEX-197).
			name:     "bare short number in people position is dropped, not a person",
			pattern:  "[{studio}] {title} ({people}) ({resolution})",
			filename: "[MyStudio] MyTitle (2) (1080p).mkv",
			want: map[string][]string{
				"studio": {"MyStudio"},
				"title":  {"MyTitle"},
			},
			wantOK: true,
		},
		{
			// An ISO date filling the {people} slot is dropped, not a
			// person (HOLODEX-197).
			name:     "ISO date in people position is dropped, not a person",
			pattern:  "[{studio}] {title} ({people}) ({resolution})",
			filename: "[MyStudio] MyTitle (2024-02-13) (720p).mkv",
			want: map[string][]string{
				"studio": {"MyStudio"},
				"title":  {"MyTitle"},
			},
			wantOK: true,
		},
		{
			// A resolution-shaped value landing in the {people} slot is
			// dropped, not a person (HOLODEX-197).
			name:     "resolution string in people position is dropped, not a person",
			pattern:  "{title} ({people})",
			filename: "MyTitle (1080p).mkv",
			want: map[string][]string{
				"title": {"MyTitle"},
			},
			wantOK: true,
		},
		{
			// A real person name survives alongside a dropped numeric value in
			// the same multi-value split (HOLODEX-197).
			name:      "person name survives alongside dropped numeric value",
			pattern:   "[{studio}] {title} ({people}) ({resolution})",
			delimiter: ", ",
			filename:  "[MyStudio] MyTitle (2, PersonName) (720p).mkv",
			want: map[string][]string{
				"studio": {"MyStudio"},
				"title":  {"MyTitle"},
				"people": {"PersonName"},
			},
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := extract.Compile(tc.pattern)
			if err != nil {
				t.Fatalf("Compile(%q) error: %v", tc.pattern, err)
			}
			fields, ok := extract.MatchFirst([]*extract.Pattern{p}, tc.filename, tc.delimiter)
			if ok != tc.wantOK {
				t.Fatalf("Match ok = %v, want %v", ok, tc.wantOK)
			}
			if !reflect.DeepEqual(fields, tc.want) {
				t.Fatalf("Match fields = %#v, want %#v", fields, tc.want)
			}
		})
	}
}

func TestMatchFirst_TriesPatternsInOrder(t *testing.T) {
	patterns, err := extract.CompileAll([]string{
		"[{studio}] {title} ({people}, {year}) {resolution}",
		"{title} ({year})",
	})
	if err != nil {
		t.Fatalf("CompileAll error: %v", err)
	}

	// Doesn't match the first (more specific) pattern, falls through to the second.
	fields, ok := extract.MatchFirst(patterns, "Fight Club (1999).mp4", "")
	if !ok {
		t.Fatalf("MatchFirst ok = false, want true")
	}
	want := map[string][]string{"title": {"Fight Club"}, "release_date": {"1999"}}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("MatchFirst fields = %#v, want %#v", fields, want)
	}
}

func TestMatchFirst_NoPatternMatches(t *testing.T) {
	patterns, err := extract.CompileAll([]string{"{title} ({year})"})
	if err != nil {
		t.Fatalf("CompileAll error: %v", err)
	}
	fields, ok := extract.MatchFirst(patterns, "totally unstructured filename.mp4", "")
	if ok || fields != nil {
		t.Fatalf("MatchFirst = (%#v, %v), want (nil, false)", fields, ok)
	}
}
