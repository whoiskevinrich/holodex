package resolver_test

import (
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/resolver"
)

// Owner-offered empty fields (ADR-113 D1): a replace field with no value, no
// decision and no film candidate is kept when Options.Offer names it, in the
// undecided shape SourceRadioList renders as File "No value" + Custom.

func overviewOnly() []mapping.Field {
	return []mapping.Field{stubField("overview", false, "file:Comment", "tmdb:overview")}
}

func TestResolveOffer_EmptyOfferedFieldIsKeptUndecided(t *testing.T) {
	opts := resolver.Options{Offer: registry.OffersWhenEmpty}
	got := resolver.Resolve(&model.Video{}, nil, resolver.Enrichment{}, nil, overviewOnly(), opts)
	f, ok := resolvedByCanonical(got, "overview")
	if !ok {
		t.Fatalf("offered empty overview must stay in resolved[]")
	}
	if len(f.Values) != 0 || f.Values == nil {
		t.Errorf("want empty, non-nil values, got %#v", f.Values)
	}
	if f.Display != registry.DisplayLongText {
		t.Errorf("want long_text display, got %q", f.Display)
	}
	if f.Decision == nil || f.Decision.Standing || f.Decision.Source != "file" {
		t.Errorf("want a non-standing file decision, got %+v", f.Decision)
	}
	var fileCand bool
	for _, c := range f.Candidates {
		if c.Source == "file" && c.Value == "" {
			fileCand = true
		}
	}
	if !fileCand {
		t.Errorf("want an empty file candidate for the \"No value\" row, got %+v", f.Candidates)
	}
}

func TestResolveOffer_NilOfferStillDrops(t *testing.T) {
	got := resolver.Resolve(&model.Video{}, nil, resolver.Enrichment{}, nil, overviewOnly(), resolver.Options{})
	if _, ok := resolvedByCanonical(got, "overview"); ok {
		t.Fatalf("without Offer (the visitor path) an empty overview must still drop")
	}
}

func TestResolveOffer_UnflaggedFieldStillDrops(t *testing.T) {
	fields := []mapping.Field{stubField("tagline", false, "file:Description")}
	opts := resolver.Options{Offer: registry.OffersWhenEmpty}
	got := resolver.Resolve(&model.Video{}, nil, resolver.Enrichment{}, nil, fields, opts)
	if _, ok := resolvedByCanonical(got, "tagline"); ok {
		t.Fatalf("a field that has not adopted OfferWhenEmpty must still drop")
	}
}

func TestResolveOffer_MergeFieldIsNeverOffered(t *testing.T) {
	field := stubField("genres", false, "file:Genre")
	field.Merge = true
	fields := []mapping.Field{field}
	opts := resolver.Options{Offer: func(string) bool { return true }}
	got := resolver.Resolve(&model.Video{}, nil, resolver.Enrichment{}, nil, fields, opts)
	if _, ok := resolvedByCanonical(got, "genres"); ok {
		t.Fatalf("offering is replace-only (ADR-051 RD1); an empty merge field must still drop")
	}
}

func TestResolveOffer_PopulatedFieldIsUnchanged(t *testing.T) {
	enr := resolver.Enrichment{"tmdb": {"overview": {"A keeper waits."}}}
	with := resolver.Resolve(&model.Video{}, nil, enr, nil, overviewOnly(), resolver.Options{Offer: registry.OffersWhenEmpty})
	without := resolver.Resolve(&model.Video{}, nil, enr, nil, overviewOnly(), resolver.Options{})
	a, _ := resolvedByCanonical(with, "overview")
	b, _ := resolvedByCanonical(without, "overview")
	if len(a.Values) != 1 || a.Values[0] != b.Values[0] || a.WinningSource != b.WinningSource {
		t.Fatalf("Offer must not change a field that has a value: with=%+v without=%+v", a, b)
	}
}
