package enrich

import (
	"context"
	"testing"

	"holodex/internal/model"
)

// ProviderMatches enumerates the providers an entity is linked to — one Match per
// provider (collapsing its many field rows), carrying the persisted external id,
// and scoped to the entity (F31.3).
func TestProviderMatches(t *testing.T) {
	svc, r := newSvc(t, NewFake("fake"))
	ctx := context.Background()

	// Video #1 matched to two providers, each with multiple fields — the dedup
	// must collapse to one Match per provider.
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, 1, "tmdb", "tmdb:42",
		map[string][]string{"title": {"X"}, "genres": {"A", "B"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, 1, "imdb", "tt7",
		map[string][]string{"title": {"Y"}}); err != nil {
		t.Fatal(err)
	}
	// A different entity must not leak into #1's matches.
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, 2, "tmdb", "tmdb:99",
		map[string][]string{"title": {"Z"}}); err != nil {
		t.Fatal(err)
	}

	matches, err := svc.ProviderMatches(ctx, model.EnrichEntityVideo, 1)
	if err != nil {
		t.Fatalf("ProviderMatches: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("want one match per provider, got %+v", matches)
	}
	got := map[string]string{}
	for _, m := range matches {
		got[m.Provider] = m.ExternalID
	}
	if got["tmdb"] != "tmdb:42" || got["imdb"] != "tt7" {
		t.Fatalf("matches = %+v", got)
	}

	// An unmatched entity yields no matches (clean file-only refresh).
	if ms, err := svc.ProviderMatches(ctx, model.EnrichEntityVideo, 999); err != nil || len(ms) != 0 {
		t.Fatalf("unmatched: ms=%+v err=%v", ms, err)
	}
}
