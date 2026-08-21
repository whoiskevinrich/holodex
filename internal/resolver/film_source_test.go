package resolver_test

import (
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/resolver"
)

// collectionField mirrors the registry's "collection" (label "Film") field: a
// replace field mapped file-first from the Album tag, with no static provider
// source — a film source (F56/ADR-085 §4) is never YAML-declared, only decided at
// runtime, so ParsedSources deliberately omits it.
func collectionField() []mapping.Field {
	return []mapping.Field{stubField("collection", false, "file:Album")}
}

// --- Decided: a film source wins, keyed by the field's own canonical name --------

func TestResolveDecided_FilmSourceWins(t *testing.T) {
	enrichment := resolver.Enrichment{"film:42": {"collection": {"Scene Test Film"}}}
	got := resolver.Resolve(&model.Video{}, nil, enrichment, nil, collectionField(), decide("collection", "provider:film:42", ""))
	f, ok := resolvedByCanonical(got, "collection")
	if !ok || len(f.Values) != 1 || f.Values[0] != "Scene Test Film" {
		t.Fatalf("decided film source should win: %+v", f)
	}
	if f.WinningSource != "film:42:collection" {
		t.Errorf("want winning_source film:42:collection, got %q", f.WinningSource)
	}
}

// --- Undecided: a film candidate never silently wins ------------------------------
//
// Films have no static ParsedSources entry (by design, ADR-085 §4 Context/Q1), so
// an undecided replace field must fall through to the file-first default and never
// pick up a film candidate on its own — only an explicit decision reaches it.

func TestResolveUndecided_FilmSourceNeverAutoWins(t *testing.T) {
	enrichment := resolver.Enrichment{"film:42": {"collection": {"Scene Test Film"}}}
	got := resolver.Resolve(&model.Video{}, nil, enrichment, nil, collectionField(), resolver.Options{})
	f, ok := resolvedByCanonical(got, "collection")
	if ok && len(f.Values) != 0 {
		t.Fatalf("undecided field must not resolve to an unrequested film candidate: %+v", f)
	}
}

// --- Suspended: films_enabled=false (or a detached film) omits the namespace -----
//
// The call site simply doesn't inject the "film:<id>" namespace into enrichment —
// no new resolver state (ADR-085 §5). The decided source then finds nothing, exactly
// like a decided-but-currently-unmatched real provider: the field drops, it does not
// fall back to file.

func TestResolveDecided_FilmSourceSuspendedDropsField(t *testing.T) {
	got := resolver.Resolve(&model.Video{}, nil, resolver.Enrichment{}, nil, collectionField(), decide("collection", "provider:film:42", ""))
	f, ok := resolvedByCanonical(got, "collection")
	if ok && len(f.Values) != 0 {
		t.Fatalf("suspended film source must resolve to no value, not fall back to file: %+v", f)
	}
}

// --- Multi-film: the decided namespace disambiguates which film wins -------------

func TestResolveDecided_MultiFilmDisambiguatesByNamespace(t *testing.T) {
	enrichment := resolver.Enrichment{
		"film:1": {"collection": {"First Film"}},
		"film:2": {"collection": {"Second Film"}},
	}
	got := resolver.Resolve(&model.Video{}, nil, enrichment, nil, collectionField(), decide("collection", "provider:film:2", ""))
	f, ok := resolvedByCanonical(got, "collection")
	if !ok || len(f.Values) != 1 || f.Values[0] != "Second Film" {
		t.Fatalf("decision on film:2 must resolve film:2's value, not film:1's: %+v", f)
	}
}
