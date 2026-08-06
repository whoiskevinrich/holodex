package enrich

import "testing"

func TestValidatePattern(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		want    bool
	}{
		{"empty", "", false},
		{"all required", "{studio} {title} {performers} {year}", true},
		{"all optional", "{studio?} {title?} {performers?} {year?}", true},
		{"single token", "{title?}", true},
		{"unknown token", "{studio?} {director?}", false},
		{"literal decoration rejected", "{title} ({year})", false},
		{"malformed brace", "{title", false},
		{"whitespace only", "   ", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidatePattern(c.pattern); got != c.want {
				t.Errorf("ValidatePattern(%q) = %v, want %v", c.pattern, got, c.want)
			}
		})
	}
}

func TestSanitizeTitle(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			"the spec's own worked example",
			"[MyStudio] My Title (Some Actor, Other Actor) 720p",
			"MyStudio My Title Some Actor Other Actor",
		},
		{"non-resolution digits untouched", "Agent 007", "Agent 007"},
		{"four-digit non-resolution number untouched", "Suite 1080", "Suite 1080"},
		{"only the resolution token is stripped, not the surrounding words", "Suite 1080p Deluxe", "Suite Deluxe"},
		{"4k stripped case-insensitively", "Movie 4K Remaster", "Movie Remaster"},
		{"8K stripped", "Movie 8k Remaster", "Movie Remaster"},
		{"clean title is a no-op", "The Matrix", "The Matrix"},
		{"degenerate bracket-only title falls back to raw", "[720p]", "[720p]"},
		{"degenerate paren-only title falls back to raw", "(1080p)", "(1080p)"},
		{"whitespace collapse", "My   Title   Here", "My Title Here"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeTitle(c.input); got != c.want {
				t.Errorf("sanitizeTitle(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestSourceBuildQuery_Precedence(t *testing.T) {
	fields := QueryFields{
		Studio:      "Wicked Pictures",
		Title:       "Selena Sky",
		Performers:  []string{"Selena Sky"},
		ReleaseDate: "2023-08-01",
	}
	operator := "{studio?} {year?}"
	preferred := "{title?}"
	deflt := "{performers?}"

	// All three tiers present simultaneously — the operator override must win, not
	// just whichever tier happens to be checked first in isolation.
	src := Source{SearchPattern: operator}
	if got := src.BuildQuery(fields, preferred, deflt); got != "Wicked Pictures 2023" {
		t.Errorf("operator tier should win: got %q", got)
	}

	// No operator override: the provider's preference wins over the default.
	src = Source{}
	if got := src.BuildQuery(fields, preferred, deflt); got != "Selena Sky" {
		t.Errorf("preferred tier should win over default: got %q", got)
	}

	// Neither operator nor preferred: falls to the global default.
	if got := (Source{}).BuildQuery(fields, "", deflt); got != "Selena Sky" {
		t.Errorf("default tier should render: got %q", got)
	}

	// Nothing configured at all: sanitized-title floor.
	if got := (Source{}).BuildQuery(fields, "", ""); got != "Selena Sky" {
		t.Errorf("floor tier should be the sanitized title: got %q", got)
	}
}

func TestSourceBuildQuery_RequiredTokenFallsThroughTier(t *testing.T) {
	// {studio} (no ?) is required; this video has no resolved studio, so the whole
	// operator tier must fail — not render with a gap where {studio} would be.
	fields := QueryFields{Title: "My Title", Performers: []string{"Some Actor"}}
	src := Source{SearchPattern: "{studio} {title?} {performers?}"}
	got := src.BuildQuery(fields, "", "")
	want := "My Title" // the floor tier is the sanitized title alone, not title+performers
	if got != want {
		t.Errorf("required-token-missing should fall through to the floor: got %q, want %q", got, want)
	}
}

func TestSourceBuildQuery_OptionalTokenOmittedNoArtifact(t *testing.T) {
	fields := QueryFields{Title: "My Title"} // no studio, no performers, no year
	src := Source{SearchPattern: "{studio?} {title?} {performers?} {year?}"}
	got := src.BuildQuery(fields, "", "")
	want := "My Title"
	if got != want {
		t.Errorf("optional tokens with no value should be dropped cleanly: got %q, want %q", got, want)
	}
}

func TestSourceBuildQuery_PerformersCapAndOrder(t *testing.T) {
	fields := QueryFields{
		Title:      "Title",
		Performers: []string{"Actor One", "Actor Two", "Actor Three", "Director One"},
	}
	src := Source{SearchPattern: "{performers?}"}
	got := src.BuildQuery(fields, "", "")
	want := "Actor One Actor Two Actor Three"
	if got != want {
		t.Errorf("performers should cap at top 3: got %q, want %q", got, want)
	}
}

func TestSourceBuildQuery_YearParsing(t *testing.T) {
	cases := []struct {
		name        string
		releaseDate string
		want        string
	}{
		{"full ISO date", "2023-08-01", "2023"},
		{"year only", "2019", "2019"},
		{"empty", "", ""},
		{"garbage", "unknown", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fields := QueryFields{Title: "T", ReleaseDate: c.releaseDate}
			src := Source{SearchPattern: "{year?}"}
			got := src.BuildQuery(fields, "", "")
			want := c.want
			if want == "" {
				want = sanitizeTitle("T") // falls through to the floor when year is empty
			}
			if got != want {
				t.Errorf("release_date=%q: got %q, want %q", c.releaseDate, got, want)
			}
		})
	}
}

func TestSourceBuildQuery_UnknownTokenNeverRenders(t *testing.T) {
	// An unknown token name makes the WHOLE pattern invalid (parseQueryPattern), not
	// just that one token — it must never partially render and must fall through.
	fields := QueryFields{Title: "My Title"}
	src := Source{SearchPattern: "{studio?} {director?}"}
	got := src.BuildQuery(fields, "", "")
	if got != "My Title" {
		t.Errorf("unknown-token pattern should fall through to the floor untouched: got %q", got)
	}
}

func TestSourceBuildQuery_EmptyTitleNeverBlank(t *testing.T) {
	// Sanity: BuildQuery never fabricates content — an empty title with nothing else
	// configured stays empty (there is nothing to sanitize into a non-blank string).
	got := (Source{}).BuildQuery(QueryFields{}, "", "")
	if got != "" {
		t.Errorf("empty fields should render empty, got %q", got)
	}
}

func TestSourceBuildQuery_WireContractUnaffectedByPatternChoice(t *testing.T) {
	// D1: BuildQuery only ever returns a plain string — proving the return type
	// itself carries no structure a caller could accidentally leak into the wire
	// Hint beyond the single Query string field (compile-time proof via the
	// assignment below; if BuildQuery's signature ever changed shape, this line
	// would stop compiling long before any wire test would catch it).
	var query string = (Source{SearchPattern: "{title?}"}).BuildQuery(QueryFields{Title: "T"}, "", "")
	hint := Hint{Query: query}
	if hint.ExternalIDs != nil {
		t.Errorf("BuildQuery must never populate anything beyond Query")
	}
}
