package resolver_test

import (
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/resolver"
)

// decide builds a one-field decision Options (file-first default) for brevity.
func decide(canonical, source, manual string) resolver.Options {
	return resolver.Options{Decisions: resolver.Decisions{
		canonical: {Source: source, ManualValue: manual},
	}}
}

// titleField is the canonical adversarial case: a scalar mapped provider-first.
func titleField() []mapping.Field {
	return []mapping.Field{stubField("title", true, "tmdb:title", "file:title")}
}

// --- File-first default (the F31 bug fix, RD4) -----------------------------------

func TestResolve_FileFirstDefault_ProviderNoLongerMasksFile(t *testing.T) {
	// Mapping lists tmdb:title before file:title, but the file-first default (the
	// zero-value Options) makes the file value win — the provider is only a candidate.
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, titleField(), resolver.Options{})
	if got[0].Values[0] != "filename_title" {
		t.Fatalf("file-first: want filename_title, got %q", got[0].Values[0])
	}
	if got[0].WinningSource != "file:title" {
		t.Errorf("want winning_source file:title, got %q", got[0].WinningSource)
	}
	// The provider survives as a selectable candidate.
	if v := providerCandidate(got[0], "tmdb"); v != "TMDB Title" {
		t.Errorf("want tmdb candidate 'TMDB Title', got %q", v)
	}
	// Undecided marker reports the implicit file selection; in sync by construction.
	if got[0].Decision == nil || got[0].Decision.Standing || got[0].Decision.Source != "file" {
		t.Errorf("undecided marker should be non-standing file, got %+v", got[0].Decision)
	}
	if got[0].InSync == nil || !*got[0].InSync {
		t.Errorf("undecided file-default must read in sync, got %+v", got[0].InSync)
	}
}

func TestResolve_MappingModeRestoresProviderFirst(t *testing.T) {
	// default_source: mapping restores the legacy first-non-empty order → provider wins.
	opts := resolver.Options{DefaultSource: resolver.DefaultSourceMapping}
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, titleField(), opts)
	if got[0].Values[0] != "TMDB Title" {
		t.Fatalf("mapping mode: want TMDB Title, got %q", got[0].Values[0])
	}
	if got[0].Decision.Standing || got[0].Decision.Source != "provider:tmdb" {
		t.Errorf("undecided mapping winner marker should be non-standing provider:tmdb, got %+v", got[0].Decision)
	}
}

// --- Decision short-circuit (replace) --------------------------------------------

func TestResolve_DecisionAdoptProvider(t *testing.T) {
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, titleField(), decide("title", "provider:tmdb", ""))
	if got[0].Values[0] != "TMDB Title" {
		t.Fatalf("adopt provider: want TMDB Title, got %q", got[0].Values[0])
	}
	if got[0].WinningSource != "tmdb:title" {
		t.Errorf("want winning_source tmdb:title, got %q", got[0].WinningSource)
	}
	if got[0].Decision == nil || !got[0].Decision.Standing || got[0].Decision.Source != "provider:tmdb" {
		t.Errorf("want standing provider:tmdb decision, got %+v", got[0].Decision)
	}
	// File ("filename_title") differs from the adopted value → out of sync.
	if got[0].InSync == nil || *got[0].InSync {
		t.Errorf("adopted provider differing from file must be out of sync, got %+v", got[0].InSync)
	}
}

func TestResolve_DecisionKeepFileOverridesMappingOrder(t *testing.T) {
	// A 'file' decision short-circuits even under mapping mode, where the provider
	// would otherwise win — proving the decision beats precedence, not just the default.
	opts := decide("title", "file", "")
	opts.DefaultSource = resolver.DefaultSourceMapping
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, titleField(), opts)
	if got[0].Values[0] != "filename_title" || got[0].WinningSource != "file:title" {
		t.Fatalf("keep-file decision should win: %+v", got[0])
	}
	if !got[0].Decision.Standing || got[0].Decision.Source != "file" {
		t.Errorf("want standing file decision, got %+v", got[0].Decision)
	}
	if got[0].InSync == nil || !*got[0].InSync {
		t.Errorf("keep-file decision is always in sync, got %+v", got[0].InSync)
	}
}

func TestResolve_DecisionManualLiteral(t *testing.T) {
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, titleField(), decide("title", "manual", "My Cut"))
	if got[0].Values[0] != "My Cut" {
		t.Fatalf("manual: want 'My Cut', got %q", got[0].Values[0])
	}
	if !got[0].Items[0].Manual || got[0].WinningSource != "manual:title" {
		t.Errorf("manual item flag/winner wrong: %+v", got[0])
	}
	if got[0].Decision.Source != "manual" || got[0].Decision.ManualValue != "My Cut" {
		t.Errorf("want manual decision carrying the literal, got %+v", got[0].Decision)
	}
}

// --- Source-pin, not value-pin (cardinal) ----------------------------------------

func TestResolve_SourcePinFollowsLiveLayer(t *testing.T) {
	// keep-file follows a later file re-extract.
	edited := &model.Video{ID: 1, Title: "edited_after_refresh"}
	got := resolver.Resolve(edited, nil, testEnrich, nil, titleField(), decide("title", "file", ""))
	if got[0].Values[0] != "edited_after_refresh" {
		t.Errorf("file decision must follow the live file value, got %q", got[0].Values[0])
	}

	// adopt-provider follows a re-enrich.
	reenriched := resolver.Enrichment{"tmdb": {"title": {"Re-enriched Title"}}}
	got = resolver.Resolve(testVideo, testExtra, reenriched, nil, titleField(), decide("title", "provider:tmdb", ""))
	if got[0].Values[0] != "Re-enriched Title" {
		t.Errorf("provider decision must follow the live enrichment value, got %q", got[0].Values[0])
	}

	// manual stays frozen regardless of file/provider churn.
	got = resolver.Resolve(edited, nil, reenriched, nil, titleField(), decide("title", "manual", "Frozen"))
	if got[0].Values[0] != "Frozen" {
		t.Errorf("manual decision must stay frozen, got %q", got[0].Values[0])
	}
}

// --- Candidates ------------------------------------------------------------------

func TestResolve_CandidatesListFileAndMatchedProviders(t *testing.T) {
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, titleField(), resolver.Options{})
	if len(got[0].Candidates) != 2 {
		t.Fatalf("want file + tmdb candidates, got %+v", got[0].Candidates)
	}
	if got[0].Candidates[0].Source != "file" || got[0].Candidates[0].Value != "filename_title" {
		t.Errorf("first candidate should be the file value, got %+v", got[0].Candidates[0])
	}
	if got[0].Candidates[1].Source != "provider:tmdb" || got[0].Candidates[1].Provider != "tmdb" {
		t.Errorf("second candidate should be the tmdb provider, got %+v", got[0].Candidates[1])
	}
}

func TestResolve_EmptyProviderYieldsNoCandidate(t *testing.T) {
	// tmdb has no title here → only the file candidate is offered (can't adopt empty).
	got := resolver.Resolve(testVideo, testExtra, resolver.Enrichment{}, nil, titleField(), resolver.Options{})
	if len(got[0].Candidates) != 1 || got[0].Candidates[0].Source != "file" {
		t.Errorf("empty provider must yield no adopt candidate, got %+v", got[0].Candidates)
	}
}

// --- Merge untouched (RD1 regression guard) --------------------------------------

func TestResolve_MergeFieldIgnoresDecision(t *testing.T) {
	enr := resolver.Enrichment{"tmdb": {"genres": {"Action", "Drama"}}}
	fields := []mapping.Field{mergeField("genres", "tmdb:genres", "file:genres")}
	// A decision keyed to a merge field must be inert: same union, no F36 markers.
	withDec := resolver.Resolve(testVideo, testExtra, enr, nil, fields, decide("genres", "provider:tmdb", ""))
	noDec := resolver.Resolve(testVideo, testExtra, enr, nil, fields, resolver.Options{})
	if len(withDec[0].Values) != len(noDec[0].Values) || len(withDec[0].Values) != 3 {
		t.Fatalf("merge union must be identical with/without a decision: %v vs %v", withDec[0].Values, noDec[0].Values)
	}
	if withDec[0].Decision != nil || withDec[0].Candidates != nil || withDec[0].InSync != nil {
		t.Errorf("merge fields must carry no F36 markers, got %+v", withDec[0])
	}
}

// providerCandidate returns the candidate value for a provider, or "".
func providerCandidate(f resolver.ResolvedField, provider string) string {
	for _, c := range f.Candidates {
		if c.Provider == provider {
			return c.Value
		}
	}
	return ""
}
