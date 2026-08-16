package resolver

import (
	"testing"

	"holodex/internal/mapping"
)

func fld(canonical string) mapping.Field { return mapping.Field{Canonical: canonical} }

// TestComplete_WorkedExample reproduces the spec's § Scoring model worked example
// verbatim (docs/specs/entity-completeness-score.md): score 65, actionability 50%.
func TestComplete_WorkedExample(t *testing.T) {
	fields := []mapping.Field{
		fld("title"), fld("studio"), fld("actors"), fld("poster_url"),
		fld("overview"), fld("release_date"), fld("genres"), fld("external_provider_id"),
	}
	resolved := []ResolvedField{
		{Canonical: "title", WinningSource: "file:Title"},
		{Canonical: "studio", WinningSource: "manual:studio"},
		{Canonical: "actors", WinningSource: "tmdb:cast"},
		// poster_url: no row — never resolved (missing).
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

	if got.Score != 65 {
		t.Errorf("Score = %d, want 65", got.Score)
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
	if fs := byCanonical["actors"]; fs.Tier != TierProvider {
		t.Errorf("actors = %+v, want provider", fs)
	}
}

func TestComplete_NoMissingFacets(t *testing.T) {
	fields := []mapping.Field{fld("title")}
	resolved := []ResolvedField{{Canonical: "title", WinningSource: "file:Title"}}

	got := Complete(fields, resolved, nil)

	if got.Score != 100 {
		t.Errorf("Score = %d, want 100", got.Score)
	}
	if got.Actionability != nil {
		t.Errorf("Actionability = %v, want nil (no missing facets)", *got.Actionability)
	}
}

func TestComplete_AllExcludedYieldsZeroScore(t *testing.T) {
	// deathdate carries no Criticality tag (excluded, F55) — nothing left to score.
	fields := []mapping.Field{fld("deathdate")}
	got := Complete(fields, nil, nil)

	if got.Score != 0 {
		t.Errorf("Score = %d, want 0", got.Score)
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
