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

// --- Undecided: the sole linked film decides by default (ADR-122) ----------------
//
// ADR-122 supersedes ADR-085 §4's "never auto-wins": with no standing decision, a
// sole attached film beats the file's Album tag, so the film entity's current name
// is what the video shows, and the field reports out of sync until written back.

var albumExtra = []model.ExtraMetadata{{SourceKey: "Album", Value: "Sunset Reel (2019)"}}

func TestResolveUndecided_SoleFilmBeatsFile(t *testing.T) {
	enrichment := resolver.Enrichment{"film:42": {"collection": {"Sunset Reel"}}}
	got := resolver.Resolve(&model.Video{}, albumExtra, enrichment, nil, collectionField(), resolver.Options{})
	f, ok := resolvedByCanonical(got, "collection")
	if !ok || len(f.Values) != 1 || f.Values[0] != "Sunset Reel" {
		t.Fatalf("sole linked film should win over the Album tag: %+v", f)
	}
	if f.WinningSource != "film:42:collection" {
		t.Errorf("want winning_source film:42:collection, got %q", f.WinningSource)
	}
	if f.Decision == nil || f.Decision.Source != "provider:film:42" || f.Decision.Standing {
		t.Errorf("want implicit (non-standing) provider:film:42 marker, got %+v", f.Decision)
	}
	if f.InSync == nil || *f.InSync {
		t.Errorf("film value differing from the Album tag must report out of sync, got %v", f.InSync)
	}
}

func TestResolveUndecided_SoleFilmMatchingFileIsInSync(t *testing.T) {
	enrichment := resolver.Enrichment{"film:42": {"collection": {"Sunset Reel (2019)"}}}
	got := resolver.Resolve(&model.Video{}, albumExtra, enrichment, nil, collectionField(), resolver.Options{})
	f, _ := resolvedByCanonical(got, "collection")
	if f.InSync == nil || !*f.InSync {
		t.Errorf("film value equal to the Album tag must report in sync, got %v", f.InSync)
	}
}

func TestResolveUndecided_SeveralFilmsStayRecordFirst(t *testing.T) {
	enrichment := resolver.Enrichment{
		"film:1": {"collection": {"First Film"}},
		"film:2": {"collection": {"Second Film"}},
	}
	got := resolver.Resolve(&model.Video{}, albumExtra, enrichment, nil, collectionField(), resolver.Options{})
	f, ok := resolvedByCanonical(got, "collection")
	if !ok || len(f.Values) != 1 || f.Values[0] != "Sunset Reel (2019)" {
		t.Fatalf("two linked films are ambiguous — the Album tag must stand: %+v", f)
	}
}

func TestResolveDecided_FileDecisionBeatsSoleFilm(t *testing.T) {
	enrichment := resolver.Enrichment{"film:42": {"collection": {"Sunset Reel"}}}
	got := resolver.Resolve(&model.Video{}, albumExtra, enrichment, nil, collectionField(), decide("collection", "file", ""))
	f, _ := resolvedByCanonical(got, "collection")
	if len(f.Values) != 1 || f.Values[0] != "Sunset Reel (2019)" {
		t.Fatalf("a standing file decision must still beat the linked film: %+v", f)
	}
}

func TestResolveUndecided_ManualAddBeatsSoleFilm(t *testing.T) {
	enrichment := resolver.Enrichment{"film:42": {"collection": {"Sunset Reel"}}}
	cur := resolver.Curation{"collection": {Add: []string{"Owner Typed"}}}
	got := resolver.Resolve(&model.Video{}, albumExtra, enrichment, cur, collectionField(), resolver.Options{})
	f, _ := resolvedByCanonical(got, "collection")
	if len(f.Values) != 1 || f.Values[0] != "Owner Typed" {
		t.Fatalf("a manual add must still beat the linked film: %+v", f)
	}
}

func TestResolveUndecided_DetachedFilmFallsBackToFile(t *testing.T) {
	got := resolver.Resolve(&model.Video{}, albumExtra, resolver.Enrichment{}, nil, collectionField(), resolver.Options{})
	f, _ := resolvedByCanonical(got, "collection")
	if len(f.Values) != 1 || f.Values[0] != "Sunset Reel (2019)" {
		t.Fatalf("with no film linked the Album tag must resolve: %+v", f)
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

// --- Candidates: a film source offers a selectable chip, named after the film ----
//
// replaceMarkers builds the SourceBadge candidate list from f.ParsedSources alone,
// which never declares a film namespace (by design). Without its own scan there, a
// video attached to a film would resolve the film's value correctly but SourceBadge
// would have no chip to offer it through — this only exercises Candidates, not Values.

func TestReplaceMarkers_FilmSourceOffersCandidateNamedAfterFilm(t *testing.T) {
	// Left fully undecided (no file value, no standing decision) — the realistic
	// shape for a freshly-attached video. This only stays in the output because of
	// the hasFilmCandidate carve-out — see
	// TestResolveUndecided_EmptyFieldWithFilmCandidateSurvives, which guards that
	// carve-out directly; without it this field would drop before Candidates is
	// ever computed, and the chip below would have no row to attach to.
	enrichment := resolver.Enrichment{"film:42": {"collection": {"Scene Test Film"}}}
	got := resolver.Resolve(&model.Video{}, nil, enrichment, nil, collectionField(), resolver.Options{})
	f, ok := resolvedByCanonical(got, "collection")
	if !ok {
		t.Fatalf("collection field missing from resolved output")
	}
	var found *resolver.FieldCandidate
	for i := range f.Candidates {
		if f.Candidates[i].Source == "provider:film:42" {
			found = &f.Candidates[i]
		}
	}
	if found == nil {
		t.Fatalf("want a provider:film:42 candidate for SourceBadge to offer, got %+v", f.Candidates)
	}
	if found.Provider != "Scene Test Film" || found.Value != "Scene Test Film" {
		t.Errorf("want candidate labeled with the film's own name, got %+v", found)
	}
}

// TestResolveUndecided_EmptyFieldWithFilmCandidateSurvives guards the ResolveFields
// carve-out the test above relies on: a replace field with no file value and no
// standing decision would normally be dropped entirely (the empty-drop rule), but a
// field offering only a film candidate must survive — otherwise SourceBadge has no
// row left to render the chip on, and the owner can never discover or decide it.
func TestResolveUndecided_EmptyFieldWithFilmCandidateSurvives(t *testing.T) {
	enrichment := resolver.Enrichment{"film:42": {"collection": {"Scene Test Film"}}}
	got := resolver.Resolve(&model.Video{}, nil, enrichment, nil, collectionField(), resolver.Options{})
	if _, ok := resolvedByCanonical(got, "collection"); !ok {
		t.Fatalf("undecided field with only a film candidate must not be dropped from resolved output")
	}
}

// TestResolveUndecided_TrulyEmptyFieldStillDrops confirms the empty-drop rule still
// applies when there's no film candidate either — the carve-out above is
// film-specific, not a general relaxation of the rule.
func TestResolveUndecided_TrulyEmptyFieldStillDrops(t *testing.T) {
	got := resolver.Resolve(&model.Video{}, nil, resolver.Enrichment{}, nil, collectionField(), resolver.Options{})
	if _, ok := resolvedByCanonical(got, "collection"); ok {
		t.Fatalf("undecided field with no value from any source should still drop")
	}
}

// --- Namespace collision: a real provider literally named "film" is not misread --
//
// filmSourceValue matches on the exact prefix "film:" (with the colon). A real
// metadata provider named "film" (ADR-086 Action Item 7/10) decides as
// "provider:film" — fieldsource.Provider strips the "provider:" prefix and leaves
// the bare namespace "film", which must NOT match strings.HasPrefix(ns, "film:")
// (4 chars vs. the 5-char literal prefix). If it ever did, a real "film" provider's
// value would be silently swallowed by the synthetic per-video film-attachment path
// instead of resolving through the normal provider scan below.

func TestResolveDecided_RealProviderNamedFilmIsNotMisreadAsFilmNamespace(t *testing.T) {
	field := stubField("collection", false, "film:Title")
	enrichment := resolver.Enrichment{
		"film":    {"Title": {"Real Provider Value"}},
		"film:42": {"collection": {"Scene Test Film"}}, // must not leak into this decision
	}
	got := resolver.Resolve(&model.Video{}, nil, enrichment, nil, []mapping.Field{field}, decide("collection", "provider:film", ""))
	f, ok := resolvedByCanonical(got, "collection")
	if !ok || len(f.Values) != 1 || f.Values[0] != "Real Provider Value" {
		t.Fatalf("a real provider named 'film' must resolve normally, not as a film-attachment namespace: %+v", f)
	}
	if f.WinningSource != "film:Title" {
		t.Errorf("want winning_source film:Title, got %q", f.WinningSource)
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
