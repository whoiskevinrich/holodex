package resolver

import (
	"slices"
	"testing"

	"holodex/internal/mapping"
)

func fld(canonical string) mapping.Field { return mapping.Field{Canonical: canonical} }

// TestComplete_Facets covers the per-facet payload the panel and the remediation
// queue read: tier, actionable (a cached unapplied candidate on a missing facet),
// not-applicable listing, and Missing(). The score is retired (F90 P0-6).
func TestComplete_Facets(t *testing.T) {
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

	if len(got.Missing()) != 0 || len(got.Facets) != 1 {
		t.Errorf("got %+v, want one present facet and nothing missing", got)
	}
}

func TestComplete_UnscoredFieldNotListed(t *testing.T) {
	// deathdate carries no Criticality tag (excluded, F55) — nothing left to score.
	fields := []mapping.Field{fld("deathdate")}
	got := Complete(fields, nil, nil)

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
// gap) is listed so the SPA can render its deep-linked empty row, but it is never
// missing or actionable.
func TestComplete_OptionalFacetListedNeverScored(t *testing.T) {
	fields := []mapping.Field{fld("title"), fld("edition")}
	resolved := []ResolvedField{{Canonical: "title", WinningSource: "file:Title"}}
	got := Complete(fields, resolved, nil)

	if len(got.Missing()) != 0 {
		t.Errorf("Missing() = %v, want none — optional is never missing", got.Missing())
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
