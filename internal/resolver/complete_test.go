package resolver

import (
	"slices"
	"strconv"
	"testing"

	"holodex/internal/mapping"
)

func fld(canonical string) mapping.Field { return mapping.Field{Canonical: canonical} }

// bands renders (Required, Extras) as "75/100" with "null" for a nil band, so a
// table test reads like the spec's § Worked examples column pair.
func bands(c Completeness) string {
	f := func(p *int) string {
		if p == nil {
			return "null"
		}
		return strconv.Itoa(*p)
	}
	return f(c.Required) + "/" + f(c.Extras)
}

// TestComplete_WorkedExamples reproduces the spec's § Worked examples table
// verbatim (docs/specs/entity-completeness-score.md, F65): required is the
// critical band alone, extras the nice_to_have band, both binary-presence, and
// a band with no applicable facet is null rather than 100.
func TestComplete_WorkedExamples(t *testing.T) {
	videoFields := []mapping.Field{
		fld("title"), fld("studio"), fld("actors"), fld("poster_url"),
		fld("overview"), fld("release_date"), fld("genres"), fld("external_provider_id"),
	}
	present := func(canonicals ...string) []ResolvedField {
		out := make([]ResolvedField, 0, len(canonicals))
		for _, c := range canonicals {
			out = append(out, ResolvedField{Canonical: c, WinningSource: "file:" + c})
		}
		return out
	}
	cases := []struct {
		name          string
		fields        []mapping.Field
		resolved      []ResolvedField
		notApplicable map[string]bool
		want          string
	}{
		{"Video A — required done, extras patchy", videoFields,
			present("title", "studio", "actors", "poster_url", "overview", "genres"),
			map[string]bool{"external_provider_id": true}, "100/67"},
		{"Video B — poster missing, every extra filled", videoFields,
			present("title", "studio", "actors", "overview", "release_date", "genres", "external_provider_id"),
			nil, "75/100"},
		{"Video C — provider-resolved is present", videoFields,
			[]ResolvedField{
				{Canonical: "title", WinningSource: "file:Title"},
				{Canonical: "studio", WinningSource: "tmdb:studio"},
				{Canonical: "actors", WinningSource: "tmdb:cast"},
				{Canonical: "poster_url", WinningSource: "tmdb:poster"},
			}, nil, "100/0"},
		{"Person — photo missing, bio only",
			[]mapping.Field{fld("photo"), fld("bio"), fld("birthdate")},
			present("bio"), nil, "0/50"},
		{"Studio — branding set", []mapping.Field{fld("branding_image")},
			present("branding_image"), nil, "null/100"},
		{"Studio — nothing set", []mapping.Field{fld("branding_image")}, nil, nil, "null/0"},
		{"Video — every critical facet not-applicable", videoFields,
			present("overview"),
			map[string]bool{"title": true, "studio": true, "actors": true, "poster_url": true}, "null/25"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bands(Complete(tc.fields, tc.resolved, tc.notApplicable)); got != tc.want {
				t.Errorf("required/extras = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestComplete_FacetsAndActionability covers the per-facet payload the panel and
// the remediation queue read: tier, actionability (a cached unapplied candidate
// on a missing facet), not-applicable listing, and Missing().
func TestComplete_FacetsAndActionability(t *testing.T) {
	fields := []mapping.Field{
		fld("title"), fld("studio"), fld("actors"), fld("poster_url"),
		fld("overview"), fld("release_date"), fld("genres"), fld("external_provider_id"),
	}
	resolved := []ResolvedField{
		{Canonical: "title", WinningSource: "file:Title"},
		{Canonical: "studio", WinningSource: "manual:studio"},
		{Canonical: "actors", WinningSource: "tmdb:cast"},
		// poster_url: no winning source — missing, with a cached tmdb candidate.
		{Canonical: "overview", WinningSource: "tmdb:overview"},
		{Canonical: "release_date", WinningSource: "file:ReleaseDate"},
		// genres: no row — missing, and no cached candidate (needs-research).
		{Canonical: "poster_url", Candidates: []FieldCandidate{
			{Source: "file", Value: ""},
			{Source: "provider:tmdb", Provider: "tmdb", Value: "https://cdn.example/poster.jpg"},
		}},
	}
	notApplicable := map[string]bool{"external_provider_id": true}

	got := Complete(fields, resolved, notApplicable)

	if b := bands(got); b != "75/67" {
		t.Errorf("required/extras = %s, want 75/67", b)
	}
	if got.Actionability == nil || *got.Actionability != 0.5 {
		t.Fatalf("Actionability = %v, want 0.5", got.Actionability)
	}

	byCanonical := map[string]FacetScore{}
	for _, f := range got.Facets {
		byCanonical[f.Canonical] = f
	}
	if len(byCanonical) != 8 {
		t.Fatalf("Facets = %d entries, want 8", len(byCanonical))
	}
	if fs := byCanonical["poster_url"]; fs.Tier != TierMissing || !fs.Actionable || fs.Provider != "tmdb" {
		t.Errorf("poster_url = %+v, want missing+actionable, provider tmdb", fs)
	}
	if fs := byCanonical["genres"]; fs.Tier != TierMissing || fs.Actionable {
		t.Errorf("genres = %+v, want missing, not actionable", fs)
	}
	if fs := byCanonical["external_provider_id"]; !fs.NotApplicable {
		t.Errorf("external_provider_id = %+v, want not_applicable", fs)
	}
	if fs := byCanonical["actors"]; fs.Tier != TierProvider || fs.Provider != "tmdb" {
		t.Errorf("actors = %+v, want provider tmdb", fs)
	}

	var missing []string
	for _, f := range got.Missing() {
		missing = append(missing, f.Canonical+":"+f.Criticality)
	}
	if want := []string{"poster_url:critical", "genres:nice_to_have"}; !slices.Equal(missing, want) {
		t.Errorf("Missing() = %v, want %v (not-applicable never missing)", missing, want)
	}
}

func TestComplete_NoMissingFacets(t *testing.T) {
	fields := []mapping.Field{fld("title")}
	resolved := []ResolvedField{{Canonical: "title", WinningSource: "file:Title"}}

	got := Complete(fields, resolved, nil)

	if b := bands(got); b != "100/null" {
		t.Errorf("required/extras = %s, want 100/null (no nice_to_have facet configured)", b)
	}
	if got.Actionability != nil {
		t.Errorf("Actionability = %v, want nil (no missing facets)", *got.Actionability)
	}
}

func TestComplete_AllExcludedYieldsNullBands(t *testing.T) {
	// deathdate carries no Criticality tag (excluded, F55) — nothing left to score.
	fields := []mapping.Field{fld("deathdate")}
	got := Complete(fields, nil, nil)

	if b := bands(got); b != "null/null" {
		t.Errorf("required/extras = %s, want null/null", b)
	}
	if len(got.Facets) != 0 {
		t.Errorf("Facets = %v, want empty (deathdate is unscored)", got.Facets)
	}
}

func TestComplete_ComputedFieldNeverScored(t *testing.T) {
	// age is Computed:true with no Criticality tag (D1's invariant) — even if it
	// somehow appears in resolved with a "computed:" winning source, it must not
	// be scored or listed.
	fields := []mapping.Field{fld("age")}
	resolved := []ResolvedField{{Canonical: "age", WinningSource: "computed:age", Computed: true}}

	got := Complete(fields, resolved, nil)

	if len(got.Facets) != 0 {
		t.Errorf("Facets = %v, want empty (age is Computed, never scored)", got.Facets)
	}
}

func TestComplete_MergeFieldMissingIsNeverActionable(t *testing.T) {
	// actors is a merge field in practice; even if a resolved row somehow carried
	// Candidates (it shouldn't per RD1), Complete only reads WinningSource for
	// tier — a genuinely missing merge field (no row at all) can't be actionable
	// since Candidates is always nil for it.
	fields := []mapping.Field{fld("actors")}
	got := Complete(fields, nil, nil)

	if len(got.Facets) != 1 || got.Facets[0].Tier != TierMissing || got.Facets[0].Actionable {
		t.Errorf("Facets = %+v, want one missing, non-actionable facet", got.Facets)
	}
}

// F60 RD11: only a plain-text replace field is Curatable — the shape the media
// page hands to an empty SourceBadge row when deep-linked. Image, long-text and
// merge fields have their own editors and must never be synthesised that way.
func TestComplete_CuratableIsPlainTextReplaceOnly(t *testing.T) {
	fields := []mapping.Field{
		fld("edition"),
		fld("tagline"),
		{Canonical: "genres", Merge: true},
		{Canonical: "actors", Multi: true},
		fld("poster_url"), // registry display image_url
		fld("overview"),   // registry display long_text
	}
	got := Complete(fields, nil, nil)
	want := map[string]bool{"edition": true, "tagline": true, "genres": false, "actors": false, "poster_url": false, "overview": false}
	for _, f := range got.Facets {
		if f.Curatable != want[f.Canonical] {
			t.Errorf("%s: Curatable = %v, want %v", f.Canonical, f.Curatable, want[f.Canonical])
		}
	}
	if len(got.Facets) != len(want) {
		t.Fatalf("got %d facets, want %d", len(got.Facets), len(want))
	}
}

// F60 RD6: an optional facet (edition — most files have none, and that is not a
// gap) is listed so the SPA can render its deep-linked empty row, but it never
// moves the score, the missing count or actionability.
func TestComplete_OptionalFacetListedNeverScored(t *testing.T) {
	fields := []mapping.Field{fld("title"), fld("edition")}
	resolved := []ResolvedField{{Canonical: "title", WinningSource: "file:Title"}}
	got := Complete(fields, resolved, nil)

	if got.Required == nil || *got.Required != 100 {
		t.Errorf("Required = %v, want 100 — a missing optional facet must not count", got.Required)
	}
	if len(got.Missing()) != 0 {
		t.Errorf("Missing() = %v, want none — optional is never missing", got.Missing())
	}
	if got.Actionability != nil {
		t.Errorf("Actionability = %v, want nil — no scored facet is missing", *got.Actionability)
	}
	var ed *FacetScore
	for i := range got.Facets {
		if got.Facets[i].Canonical == "edition" {
			ed = &got.Facets[i]
		}
	}
	if ed == nil {
		t.Fatalf("optional facet must still be listed, got %+v", got.Facets)
	}
	if ed.Tier != TierMissing || !ed.Curatable || ed.Actionable || ed.Criticality != "optional" {
		t.Errorf("optional facet = %+v, want missing · curatable · not actionable · criticality optional", *ed)
	}
}
