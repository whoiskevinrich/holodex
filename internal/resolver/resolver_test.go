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
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, fields)
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
	got := resolver.Resolve(testVideo, testExtra, resolver.Enrichment{}, nil, fields)
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
	got := resolver.Resolve(testVideo, testExtra, resolver.Enrichment{}, nil, fields)
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
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, fields)
	if len(got) != 0 {
		t.Errorf("want 0 resolved fields, got %d", len(got))
	}
}

func TestResolve_DisplayFromRegistry(t *testing.T) {
	fields := []mapping.Field{
		stubField("overview", false, "tmdb:overview"),
	}
	got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, fields)
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
	title, src := resolver.BrowseTitle(testVideo, nil, testEnrich, nil, fields)
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
	title, _ := resolver.BrowseTitle(testVideo, nil, testEnrich, nil, fields)
	if title != "" {
		t.Errorf("want empty title when no browse field, got %q", title)
	}
}

func TestBrowseTitle_FallbackToFileTitle(t *testing.T) {
	fields := []mapping.Field{
		stubField("title", true, "tmdb:title", "file:title"),
	}
	title, src := resolver.BrowseTitle(testVideo, nil, resolver.Enrichment{}, nil, fields)
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
	got := resolver.Resolve(testVideo, testExtra, resolver.Enrichment{}, nil, fields)
	if len(got) != 1 {
		t.Fatalf("want 1 field, got %d", len(got))
	}
	// "Action, Comedy" should split into two values.
	if len(got[0].Values) != 2 {
		t.Errorf("want 2 values after split, got %v", got[0].Values)
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
		[]mapping.Field{mergeField("genres", "tmdb:genres", "file:genres")})
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
		[]mapping.Field{mergeField("genres", "tmdb:genres", "file:genres")})
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
	got := resolver.Resolve(testVideo, extra, resolver.Enrichment{}, nil, []mapping.Field{lower})
	if got[0].Values[0] != "science fiction" {
		t.Errorf("lower casing: want 'science fiction', got %q", got[0].Values[0])
	}

	title := stubField("title", false, "file:title")
	title.Casing = "title"
	v := &model.Video{ID: 1, Title: "fight club"}
	got = resolver.Resolve(v, nil, resolver.Enrichment{}, nil, []mapping.Field{title})
	if got[0].Values[0] != "Fight Club" {
		t.Errorf("title casing: want 'Fight Club', got %q", got[0].Values[0])
	}
}

func TestResolve_ManualAddJoinsUnion(t *testing.T) {
	enr := resolver.Enrichment{"tmdb": {"genres": {"Drama"}}}
	cur := resolver.Curation{"genres": {Add: []string{"Sci-Fi"}}}
	got := resolver.Resolve(testVideo, nil, enr, cur,
		[]mapping.Field{mergeField("genres", "tmdb:genres")})
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
		[]mapping.Field{mergeField("genres", "tmdb:genres")})
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
		[]mapping.Field{mergeField("genres", "tmdb:genres")})
	if len(got[0].Items) != 1 || !got[0].Items[0].NoWrite {
		t.Fatalf("Drama should be shown but no_write; items=%v", got[0].Items)
	}
}

func TestResolve_ScalarManualOverride(t *testing.T) {
	enr := resolver.Enrichment{"tmdb": {"title": {"TMDB Title"}}}
	cur := resolver.Curation{"title": {Add: []string{"My Cut"}}}
	got := resolver.Resolve(testVideo, nil, enr, cur,
		[]mapping.Field{stubField("title", false, "tmdb:title", "file:title")})
	if got[0].Values[0] != "My Cut" {
		t.Errorf("want manual override 'My Cut', got %q", got[0].Values[0])
	}
	if got[0].WinningSource != "manual:title" {
		t.Errorf("want winning_source manual:title, got %q", got[0].WinningSource)
	}
}
