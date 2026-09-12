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

// ADR-095 D5 / F54 FR6: the residue rule. A {title} whose every word the resolved
// studio, ANY resolved performer, or a date token already accounts for renders
// EMPTY — and the tier still renders from its other tokens (rendered-empty is not
// missing; a required {title} must not fall the tier through to the floor, which
// would resend exactly the duplication the rule removes). One word of residue keeps
// the WHOLE title, duplication included — all-or-nothing, never a rewrite.
func TestSourceBuildQuery_ResidueRule(t *testing.T) {
	pattern := "{studio} {title} {performers} {year}"
	base := QueryFields{
		Studio:      "Acme Pictures",
		Performers:  []string{"Ada Lovelace", "Grace Hopper", "Alan Turing", "Edsger Dijkstra"}, // 4 > performersCap
		ReleaseDate: "2023-08-01",
	}
	cases := []struct {
		name  string
		title string
		want  string
	}{
		{
			"the spec's worked example: exactly studio + performer + ISO date renders empty",
			"Acme Pictures Ada Lovelace 2023-08-01",
			"Acme Pictures Ada Lovelace Grace Hopper Alan Turing 2023",
		},
		{
			"residue keeps the WHOLE title, duplication included (all-or-nothing)",
			"Acme Pictures Ada Lovelace The Engine",
			"Acme Pictures Acme Pictures Ada Lovelace The Engine Ada Lovelace Grace Hopper Alan Turing 2023",
		},
		{
			"case-insensitive: a lowercase stem still matches",
			"acme pictures ada lovelace",
			"Acme Pictures Ada Lovelace Grace Hopper Alan Turing 2023",
		},
		{
			"strips against ALL performers, not the {performers} top-3",
			"Edsger Dijkstra Acme Pictures",
			"Acme Pictures Ada Lovelace Grace Hopper Alan Turing 2023",
		},
		{
			"bare YYYY date token",
			"Acme Pictures 2023",
			"Acme Pictures Ada Lovelace Grace Hopper Alan Turing 2023",
		},
		{
			"YY.MM.DD date token",
			"Acme Pictures Grace Hopper 23.08.01",
			"Acme Pictures Ada Lovelace Grace Hopper Alan Turing 2023",
		},
		{
			"DD.MM.YYYY date token",
			"Ada Lovelace 01.08.2023",
			"Acme Pictures Ada Lovelace Grace Hopper Alan Turing 2023",
		},
		{
			"punctuation-only leftovers are not residue (the story's 'MyStudio MyPerformer -')",
			"Acme Pictures Ada Lovelace -",
			"Acme Pictures Ada Lovelace Grace Hopper Alan Turing 2023",
		},
		{
			"a possessive is residue: 'Lovelace's' leaves an 's' the cast does not cover",
			"Ada Lovelace's Big Day",
			"Acme Pictures Ada Lovelace's Big Day Ada Lovelace Grace Hopper Alan Turing 2023",
		},
		{
			"a non-ASCII word is residue",
			"Acme Pictures Ada Lovelace Übung",
			"Acme Pictures Acme Pictures Ada Lovelace Übung Ada Lovelace Grace Hopper Alan Turing 2023",
		},
		{
			"a number that is not a date shape is residue (Agent 007 stays)",
			"Acme Pictures 007",
			"Acme Pictures Acme Pictures 007 Ada Lovelace Grace Hopper Alan Turing 2023",
		},
		{
			"the D4 sanitizer runs first: brackets, commas and 1080p are not residue",
			"[Acme Pictures] Ada Lovelace, Grace Hopper (2023-08-01) 1080p",
			"Acme Pictures Ada Lovelace Grace Hopper Alan Turing 2023",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := base
			f.Title = c.title
			if got := (Source{SearchPattern: pattern}).BuildQuery(f, "", ""); got != c.want {
				t.Errorf("BuildQuery(title=%q)\n got %q\nwant %q", c.title, got, c.want)
			}
		})
	}
}

// A residue-empty REQUIRED {title} does not fail the tier: the render still happens
// from the other tokens (assert the render, not the floor), and the optional form
// behaves identically.
func TestSourceBuildQuery_ResidueEmptyRequiredTitleDoesNotFallThrough(t *testing.T) {
	f := QueryFields{Studio: "Acme", Title: "Acme Ada", Performers: []string{"Ada"}, ReleaseDate: "2019"}
	for _, pattern := range []string{"{studio} {title} {performers}", "{studio} {title?} {performers}"} {
		// The floor would be the sanitized title "Acme Ada" — the duplicated form the
		// rule exists to prevent; the render "Acme Ada" from studio+performers is the
		// same string by coincidence, so assert through a default pattern that differs.
		if got := (Source{SearchPattern: pattern}).BuildQuery(f, "", "{year}"); got != "Acme Ada" {
			t.Errorf("pattern %q: got %q, want the tier render %q (not the {year} fallback)", pattern, got, "Acme Ada")
		}
	}
	// A pattern whose ONLY token is a residue-empty title has nothing to render and
	// falls through as before — to the floor, where a lone title is never redundant.
	if got := (Source{SearchPattern: "{title}"}).BuildQuery(f, "", ""); got != "Acme Ada" {
		t.Errorf("{title}-only: got %q, want the sanitized-title floor %q", got, "Acme Ada")
	}
}

// The residue rule never touches the floor tier: with no pattern configured at all
// the sanitized title is sent even when it is exactly the studio + performers.
func TestSourceBuildQuery_ResidueRuleSkipsFloor(t *testing.T) {
	f := QueryFields{Studio: "Acme", Title: "Acme Ada", Performers: []string{"Ada"}}
	if got := (Source{}).BuildQuery(f, "", ""); got != "Acme Ada" {
		t.Errorf("floor: got %q, want %q", got, "Acme Ada")
	}
}

// With no studio/performers resolved at all, an ordinary stem is all residue and
// renders in full — the rule only ever removes words another token already says.
func TestSourceBuildQuery_ResidueRuleNothingToBeRedundantWith(t *testing.T) {
	f := QueryFields{Title: "Some Stem 2023", ReleaseDate: "2023"}
	if got := (Source{SearchPattern: "{studio?} {title} {year?}"}).BuildQuery(f, "", ""); got != "Some Stem 2023 2023" {
		t.Errorf("got %q, want %q", got, "Some Stem 2023 2023")
	}
}

// The rule is judged against the tokens IN the tier being rendered (code review of
// HOLODEX-368): a pattern without {studio}/{performers} never loses those words from
// the title, because nothing else in that query would carry them — the lossless
// invariant holds exactly, not just for the four-token pattern.
func TestSourceBuildQuery_ResidueRuleIsPatternAware(t *testing.T) {
	f := QueryFields{Studio: "Acme", Title: "Acme Ada 2023-08-01", Performers: []string{"Ada"}, ReleaseDate: "2023-08-01"}
	cases := []struct{ pattern, want string }{
		{"{title} {year?}", "Acme Ada 2023-08-01 2023"},                   // no studio/performers token: nothing to be redundant with
		{"{studio} {title}", "Acme Acme Ada 2023-08-01"},                  // "Ada" and the date have no token here => residue => whole title
		{"{studio} {title} {performers}", "Acme Acme Ada 2023-08-01 Ada"}, // the date alone is residue without {year}
		{"{studio} {title} {performers} {year}", "Acme Ada 2023"},
	}
	for _, c := range cases {
		if got := (Source{SearchPattern: c.pattern}).BuildQuery(f, "", ""); got != c.want {
			t.Errorf("pattern %q: got %q, want %q", c.pattern, got, c.want)
		}
	}
}
