package resolver_test

import (
	"strings"
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/resolver"
)

// stubField builds a minimal mapping.Field with already-parsed sources for tests.
func stubField(canonical string, browse bool, sources ...string) mapping.Field {
	parsed := make([]mapping.Source, 0, len(sources))
	for _, s := range sources {
		ns, key, _ := strings.Cut(s, ":")
		if key == "" {
			ns, key = "file", ns // legacy bare key
		}
		parsed = append(parsed, mapping.Source{Namespace: ns, Key: key})
	}
	return mapping.Field{
		Canonical:     canonical,
		Label:         canonical,
		Sources:       sources,
		ParsedSources: parsed,
		Browse:        browse,
	}
}

var (
	testVideo = &model.Video{ID: 1, Title: "filename_title"}
	testExtra = []model.ExtraMetadata{
		{SourceKey: "Publisher", Value: "Acme"},
		{SourceKey: "genres", Value: "Action, Comedy"},
	}
	testEnrich = resolver.Enrichment{
		"tmdb": {
			"title":    {"TMDB Title"},
			"overview": {"A long overview."},
			"genres":   {"Action", "Comedy"},
		},
	}
)

func TestResolve_ProviderWinsOverFile(t *testing.T) {
	fields := []mapping.Field{
		stubField("title", true, "tmdb:title", "file:title"),
	}
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, fields, resolver.Options{DefaultSource: resolver.DefaultSourceMapping})
	if len(got) != 1 {
		t.Fatalf("want 1 field, got %d", len(got))
	}
	if got[0].Values[0] != "TMDB Title" {
		t.Errorf("want TMDB Title, got %q", got[0].Values[0])
	}
	if got[0].WinningSource != "tmdb:title" {
		t.Errorf("want winning_source=tmdb:title, got %q", got[0].WinningSource)
	}
}

func TestResolve_FileFilTitleFallback(t *testing.T) {
	// TMDB has no title — falls through to file:title (video.Title).
	fields := []mapping.Field{
		stubField("title", true, "tmdb:title", "file:title"),
	}
	got := resolver.Resolve(testVideo, testExtra, resolver.Enrichment{}, nil, fields, resolver.Options{})
	if len(got) != 1 {
		t.Fatalf("want 1 field, got %d", len(got))
	}
	if got[0].Values[0] != "filename_title" {
		t.Errorf("want filename_title, got %q", got[0].Values[0])
	}
	if got[0].WinningSource != "file:title" {
		t.Errorf("want winning_source=file:title, got %q", got[0].WinningSource)
	}
}

func TestResolve_FileTagKey(t *testing.T) {
	fields := []mapping.Field{
		stubField("studio", false, "file:Publisher"),
	}
	got := resolver.Resolve(testVideo, testExtra, resolver.Enrichment{}, nil, fields, resolver.Options{})
	if len(got) != 1 {
		t.Fatalf("want 1 field, got %d", len(got))
	}
	if got[0].Values[0] != "Acme" {
		t.Errorf("want Acme, got %q", got[0].Values[0])
	}
}

func TestResolve_EmptyWhenAllSourcesMiss(t *testing.T) {
	fields := []mapping.Field{
		stubField("director", false, "tmdb:director", "file:Director"),
	}
	// Neither enrichment nor extra has Director.
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, fields, resolver.Options{})
	if len(got) != 0 {
		t.Errorf("want 0 resolved fields, got %d", len(got))
	}
}

func TestResolve_DisplayFromRegistry(t *testing.T) {
	fields := []mapping.Field{
		stubField("overview", false, "tmdb:overview"),
	}
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, fields, resolver.Options{})
	if len(got) != 1 {
		t.Fatalf("want 1 field, got %d", len(got))
	}
	if got[0].Display != "long_text" {
		t.Errorf("want display=long_text, got %q", got[0].Display)
	}
}

func TestBrowseTitle_ProviderWins(t *testing.T) {
	fields := []mapping.Field{
		stubField("title", true, "tmdb:title", "file:title"),
	}
	title, src := resolver.BrowseTitle(testVideo, nil, testEnrich, nil, fields, resolver.Options{DefaultSource: resolver.DefaultSourceMapping})
	if title != "TMDB Title" {
		t.Errorf("want TMDB Title, got %q", title)
	}
	if src != "tmdb:title" {
		t.Errorf("want src=tmdb:title, got %q", src)
	}
}

func TestBrowseTitle_NoBrowseField(t *testing.T) {
	fields := []mapping.Field{
		stubField("overview", false, "tmdb:overview"), // browse=false
	}
	title, _ := resolver.BrowseTitle(testVideo, nil, testEnrich, nil, fields, resolver.Options{})
	if title != "" {
		t.Errorf("want empty title when no browse field, got %q", title)
	}
}

func TestBrowseTitle_FallbackToFileTitle(t *testing.T) {
	fields := []mapping.Field{
		stubField("title", true, "tmdb:title", "file:title"),
	}
	title, src := resolver.BrowseTitle(testVideo, nil, resolver.Enrichment{}, nil, fields, resolver.Options{})
	if title != "filename_title" {
		t.Errorf("want filename_title, got %q", title)
	}
	if src != "file:title" {
		t.Errorf("want src=file:title, got %q", src)
	}
}

func TestResolve_MultiValueSplit(t *testing.T) {
	fields := []mapping.Field{
		{
			Canonical:     "genres",
			Label:         "Genres",
			Sources:       []string{"file:genres"},
			ParsedSources: []mapping.Source{{Namespace: "file", Key: "genres"}},
			Multi:         true,
		},
	}
	got := resolver.Resolve(testVideo, testExtra, resolver.Enrichment{}, nil, fields, resolver.Options{})
	if len(got) != 1 {
		t.Fatalf("want 1 field, got %d", len(got))
	}
	// "Action, Comedy" should split into two values.
	if len(got[0].Values) != 2 {
		t.Errorf("want 2 values after split, got %v", got[0].Values)
	}
}

// ---- F36 P1-2: inter-provider trust order (HOLODEX-118) ----

// twoProviderEnrich has two matched providers disagreeing on the studio field.
var twoProviderEnrich = resolver.Enrichment{
	"tmdb":  {"studio": {"TMDB Studio"}},
	"other": {"studio": {"Other Studio"}},
}

func TestResolve_ProviderTrustOrder_PicksFirstListed(t *testing.T) {
	// No file value for studio, so the two providers compete; the trust order
	// decides the winner among them (all-provider field path).
	fields := []mapping.Field{stubField("studio", false, "tmdb:studio", "other:studio")}

	got := resolver.Resolve(testVideo, nil, twoProviderEnrich, nil, fields,
		resolver.Options{ProviderTrustOrder: []string{"other", "tmdb"}})
	if got[0].Values[0] != "Other Studio" {
		t.Errorf("trust order [other,tmdb]: want Other Studio, got %q", got[0].Values[0])
	}
	if got[0].WinningSource != "other:studio" {
		t.Errorf("want winning_source=other:studio, got %q", got[0].WinningSource)
	}

	// Reversing the trust order flips the winner — even though mapping order is
	// unchanged (tmdb still listed first in the field's sources).
	got = resolver.Resolve(testVideo, nil, twoProviderEnrich, nil, fields,
		resolver.Options{ProviderTrustOrder: []string{"tmdb", "other"}})
	if got[0].Values[0] != "TMDB Studio" {
		t.Errorf("trust order [tmdb,other]: want TMDB Studio, got %q", got[0].Values[0])
	}
}

func TestResolve_ProviderTrustOrder_FileStillWins(t *testing.T) {
	// AC: the file/baseline layer stays ahead of all providers under the file-first
	// default, no matter what the trust order says.
	extra := []model.ExtraMetadata{{SourceKey: "Studio", Value: "File Studio"}}
	fields := []mapping.Field{stubField("studio", false, "tmdb:studio", "other:studio", "file:Studio")}

	got := resolver.Resolve(testVideo, extra, twoProviderEnrich, nil, fields,
		resolver.Options{ProviderTrustOrder: []string{"other", "tmdb"}})
	if got[0].Values[0] != "File Studio" {
		t.Errorf("file must beat all providers under file-first, got %q", got[0].Values[0])
	}
}

func TestResolve_ProviderTrustOrder_UnlistedKeepsMappingOrder(t *testing.T) {
	// A provider absent from the trust order ranks behind every listed one but keeps
	// its mapping order relative to other unlisted providers. Here only "other" is
	// ranked, so it wins over the unlisted "tmdb" despite tmdb being listed first.
	fields := []mapping.Field{stubField("studio", false, "tmdb:studio", "other:studio")}

	got := resolver.Resolve(testVideo, nil, twoProviderEnrich, nil, fields,
		resolver.Options{ProviderTrustOrder: []string{"other"}})
	if got[0].Values[0] != "Other Studio" {
		t.Errorf("ranked provider beats unlisted: want Other Studio, got %q", got[0].Values[0])
	}
}

func TestResolve_MultiProvider_NoTrustOrderKeepsMappingOrder(t *testing.T) {
	// AC: with no trust order configured, behavior is unchanged — the first source in
	// the field's mapping order wins among providers.
	fields := []mapping.Field{stubField("studio", false, "other:studio", "tmdb:studio")}

	got := resolver.Resolve(testVideo, nil, twoProviderEnrich, nil, fields, resolver.Options{})
	if got[0].Values[0] != "Other Studio" {
		t.Errorf("no trust order: mapping order wins, want Other Studio, got %q", got[0].Values[0])
	}
}

func TestResolve_ProviderTrustOrder_DecisionOverrides(t *testing.T) {
	// AC: a standing per-field decision short-circuits the trust order entirely.
	fields := []mapping.Field{stubField("studio", false, "tmdb:studio", "other:studio")}
	opts := resolver.Options{
		ProviderTrustOrder: []string{"tmdb", "other"}, // trust prefers tmdb
		Decisions:          resolver.Decisions{"studio": {Source: "provider:other"}},
	}
	got := resolver.Resolve(testVideo, nil, twoProviderEnrich, nil, fields, opts)
	if got[0].Values[0] != "Other Studio" {
		t.Errorf("decision must override trust order: want Other Studio, got %q", got[0].Values[0])
	}
}

// ---- F30: cross-source merge, dedup, casing, curation ----

func mergeField(canonical string, sources ...string) mapping.Field {
	f := stubField(canonical, false, sources...)
	f.Multi = true
	return f
}

func valueSet(items []resolver.ResolvedValue) map[string]resolver.ResolvedValue {
	m := make(map[string]resolver.ResolvedValue, len(items))
	for _, it := range items {
		m[it.Value] = it
	}
	return m
}

func TestResolve_MergeUnionAcrossSources(t *testing.T) {
	// File has [Action, Comedy]; TMDB has [Action, Drama]. Union = 3 distinct.
	enr := resolver.Enrichment{"tmdb": {"genres": {"Action", "Drama"}}}
	got := resolver.Resolve(testVideo, testExtra, enr, nil,
		[]mapping.Field{mergeField("genres", "tmdb:genres", "file:genres")}, resolver.Options{})
	if len(got) != 1 {
		t.Fatalf("want 1 field, got %d", len(got))
	}
	if len(got[0].Values) != 3 {
		t.Fatalf("want 3 merged values, got %v", got[0].Values)
	}
	vs := valueSet(got[0].Items)
	if len(vs["Action"].Sources) != 2 { // present in both sources
		t.Errorf("Action should carry 2 sources, got %v", vs["Action"].Sources)
	}
}

func TestResolve_MergeDedupCaseInsensitive(t *testing.T) {
	extra := []model.ExtraMetadata{{SourceKey: "genres", Value: "Science Fiction"}}
	enr := resolver.Enrichment{"tmdb": {"genres": {"science fiction"}}}
	got := resolver.Resolve(testVideo, extra, enr, nil,
		[]mapping.Field{mergeField("genres", "tmdb:genres", "file:genres")}, resolver.Options{})
	if len(got[0].Values) != 1 {
		t.Fatalf("want 1 deduped value, got %v", got[0].Values)
	}
	if got[0].Values[0] != "science fiction" { // tmdb first → its casing wins
		t.Errorf("want first-source casing 'science fiction', got %q", got[0].Values[0])
	}
}

func TestResolve_CasingLowerAndTitle(t *testing.T) {
	extra := []model.ExtraMetadata{{SourceKey: "genres", Value: "Science Fiction"}}
	lower := mergeField("genres", "file:genres")
	lower.Casing = "lower"
	got := resolver.Resolve(testVideo, extra, resolver.Enrichment{}, nil, []mapping.Field{lower}, resolver.Options{})
	if got[0].Values[0] != "science fiction" {
		t.Errorf("lower casing: want 'science fiction', got %q", got[0].Values[0])
	}

	title := stubField("title", false, "file:title")
	title.Casing = "title"
	v := &model.Video{ID: 1, Title: "fight club"}
	got = resolver.Resolve(v, nil, resolver.Enrichment{}, nil, []mapping.Field{title}, resolver.Options{})
	if got[0].Values[0] != "Fight Club" {
		t.Errorf("title casing: want 'Fight Club', got %q", got[0].Values[0])
	}
}

func TestResolve_ManualAddJoinsUnion(t *testing.T) {
	enr := resolver.Enrichment{"tmdb": {"genres": {"Drama"}}}
	cur := resolver.Curation{"genres": {Add: []string{"Sci-Fi"}}}
	got := resolver.Resolve(testVideo, nil, enr, cur,
		[]mapping.Field{mergeField("genres", "tmdb:genres")}, resolver.Options{})
	vs := valueSet(got[0].Items)
	if !vs["Sci-Fi"].Manual {
		t.Errorf("Sci-Fi should be a manual value; items=%v", got[0].Items)
	}
	if len(got[0].Values) != 2 {
		t.Errorf("want Drama + Sci-Fi, got %v", got[0].Values)
	}
}

func TestResolve_SuppressSurvivesReenrich(t *testing.T) {
	enr := resolver.Enrichment{"tmdb": {"genres": {"Drama", "Action"}}}
	cur := resolver.Curation{"genres": {Suppress: map[string]bool{"drama": true}}}
	got := resolver.Resolve(testVideo, nil, enr, cur,
		[]mapping.Field{mergeField("genres", "tmdb:genres")}, resolver.Options{})
	for _, v := range got[0].Values {
		if v == "Drama" {
			t.Fatalf("suppressed value leaked: %v", got[0].Values)
		}
	}
	if len(got[0].Values) != 1 || got[0].Values[0] != "Action" {
		t.Errorf("want [Action], got %v", got[0].Values)
	}
}

func TestResolve_NoWriteFlaggedButShown(t *testing.T) {
	enr := resolver.Enrichment{"tmdb": {"genres": {"Drama"}}}
	cur := resolver.Curation{"genres": {NoWrite: map[string]bool{"drama": true}}}
	got := resolver.Resolve(testVideo, nil, enr, cur,
		[]mapping.Field{mergeField("genres", "tmdb:genres")}, resolver.Options{})
	if len(got[0].Items) != 1 || !got[0].Items[0].NoWrite {
		t.Fatalf("Drama should be shown but no_write; items=%v", got[0].Items)
	}
}

func TestResolve_ScalarManualOverride(t *testing.T) {
	enr := resolver.Enrichment{"tmdb": {"title": {"TMDB Title"}}}
	cur := resolver.Curation{"title": {Add: []string{"My Cut"}}}
	got := resolver.Resolve(testVideo, nil, enr, cur,
		[]mapping.Field{stubField("title", false, "tmdb:title", "file:title")}, resolver.Options{})
	if got[0].Values[0] != "My Cut" {
		t.Errorf("want manual override 'My Cut', got %q", got[0].Values[0])
	}
	if got[0].WinningSource != "manual:title" {
		t.Errorf("want winning_source manual:title, got %q", got[0].WinningSource)
	}
}

// fakeBaseline is a non-video BaselineSource: it owns one namespace and serves
// canned values from a map. It exercises ResolveFields with a baseline that is
// not the video file layer (ADR-052 entity-agnostic seam).
type fakeBaseline struct {
	ns   string
	vals map[string][]string
}

func (b fakeBaseline) Baseline(src mapping.Source) ([]string, bool) {
	if src.Namespace != b.ns {
		return nil, false // not a baseline source → resolver consults enrichment
	}
	return b.vals[strings.ToLower(src.Key)], true // owned, even when empty
}

func TestResolveFields_DelegatesToVideoBaseline(t *testing.T) {
	// Resolve is exactly ResolveFields wrapped with NewVideoBaseline.
	fields := []mapping.Field{stubField("title", true, "tmdb:title", "file:title")}
	want := resolver.Resolve(testVideo, testExtra, testEnrich, nil, fields, resolver.Options{})
	got := resolver.ResolveFields(resolver.NewVideoBaseline(testVideo, testExtra), testEnrich, nil, fields, resolver.Options{})
	if len(got) != len(want) {
		t.Fatalf("want %d fields, got %d", len(want), len(got))
	}
	if got[0].Values[0] != want[0].Values[0] || got[0].WinningSource != want[0].WinningSource {
		t.Errorf("ResolveFields(NewVideoBaseline) must equal Resolve: got %+v want %+v", got[0], want[0])
	}
}

func TestResolveFields_EntityAgnosticBaseline(t *testing.T) {
	// A person-like entity with no *model.Video: baseline namespace "person"
	// supplies a name; a provider supplies the bio the baseline lacks.
	baseline := fakeBaseline{ns: "person", vals: map[string][]string{"name": {"Jane Roe"}}}
	enr := resolver.Enrichment{"tmdb": {"bio": {"An actor."}}}
	fields := []mapping.Field{
		stubField("name", false, "person:name", "tmdb:name"),
		stubField("bio", false, "person:bio", "tmdb:bio"),
	}
	got := resolver.ResolveFields(baseline, enr, nil, fields, resolver.Options{})
	if len(got) != 2 {
		t.Fatalf("want 2 fields, got %d", len(got))
	}
	// Baseline owns the name and wins precedence over the provider.
	if got[0].Values[0] != "Jane Roe" || got[0].WinningSource != "person:name" {
		t.Errorf("baseline name should win: %+v", got[0])
	}
	// Baseline returns (nil, true) for the bio it lacks; precedence falls through
	// to the next configured source (the provider), not to a provider for the same
	// baseline source.
	if got[1].Values[0] != "An actor." || got[1].WinningSource != "tmdb:bio" {
		t.Errorf("provider bio should resolve via enrichment: %+v", got[1])
	}
}
