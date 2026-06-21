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
	got := resolver.Resolve(testVideo, testExtra, testEnrich, fields)
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
	got := resolver.Resolve(testVideo, testExtra, resolver.Enrichment{}, fields)
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
	got := resolver.Resolve(testVideo, testExtra, resolver.Enrichment{}, fields)
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
	got := resolver.Resolve(testVideo, testExtra, testEnrich, fields)
	if len(got) != 0 {
		t.Errorf("want 0 resolved fields, got %d", len(got))
	}
}

func TestResolve_DisplayFromRegistry(t *testing.T) {
	fields := []mapping.Field{
		stubField("overview", false, "tmdb:overview"),
	}
	got := resolver.Resolve(testVideo, testExtra, testEnrich, fields)
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
	title, src := resolver.BrowseTitle(testVideo, nil, testEnrich, fields)
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
	title, _ := resolver.BrowseTitle(testVideo, nil, testEnrich, fields)
	if title != "" {
		t.Errorf("want empty title when no browse field, got %q", title)
	}
}

func TestBrowseTitle_FallbackToFileTitle(t *testing.T) {
	fields := []mapping.Field{
		stubField("title", true, "tmdb:title", "file:title"),
	}
	title, src := resolver.BrowseTitle(testVideo, nil, resolver.Enrichment{}, fields)
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
	got := resolver.Resolve(testVideo, testExtra, resolver.Enrichment{}, fields)
	if len(got) != 1 {
		t.Fatalf("want 1 field, got %d", len(got))
	}
	// "Action, Comedy" should split into two values.
	if len(got[0].Values) != 2 {
		t.Errorf("want 2 values after split, got %v", got[0].Values)
	}
}
