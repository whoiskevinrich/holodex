package api

import (
	"context"
	"testing"

	"holodex/internal/model"
	"holodex/internal/registry"
)

func facetGroupByCanonical(groups []FacetGroup, canonical string) (FacetGroup, bool) {
	for _, g := range groups {
		if g.Canonical == canonical {
			return g, true
		}
	}
	return FacetGroup{}, false
}

// TestSortFacetGroups_CriticalFirstThenCountDesc covers design handoff §1 DD1:
// critical facets before nice-to-have, and within the same criticality tier,
// larger groups first. Stable, so a tie on both keys keeps input order.
func TestSortFacetGroups_CriticalFirstThenCountDesc(t *testing.T) {
	small := func(n int) []QueueRow { return make([]QueueRow, n) }
	groups := []FacetGroup{
		{Canonical: "genres", Criticality: registry.CriticalityNiceToHave, NeedsResearch: small(40)},
		{Canonical: "poster_url", Criticality: registry.CriticalityCritical, NeedsResearch: small(3)},
		{Canonical: "actors", Criticality: registry.CriticalityCritical, NeedsResearch: small(9)},
		{Canonical: "overview", Criticality: registry.CriticalityNiceToHave, NeedsResearch: small(1)},
	}
	sortFacetGroups(groups)

	want := []string{"actors", "poster_url", "genres", "overview"}
	got := make([]string, len(groups))
	for i, g := range groups {
		got[i] = g.Canonical
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// TestRemediationQueue_GroupsByFacet covers F55.7's basic shape: a video
// missing a facet with no cached candidate lands in that facet's
// needs-research bucket, keyed by canonical, with entity data attached.
func TestRemediationQueue_GroupsByFacet(t *testing.T) {
	h, r := newCompletenessHandlers(t)
	ctx := context.Background()

	seedVideo(t, r, model.ExtraMetadata{SourceKey: "Publisher", Value: "Acme"})

	groups, err := h.remediationQueue(ctx)
	if err != nil {
		t.Fatalf("remediationQueue: %v", err)
	}
	g, ok := facetGroupByCanonical(groups, "poster_url")
	if !ok {
		t.Fatalf("poster_url group missing: %+v", groups)
	}
	if len(g.CandidateReady) != 0 || len(g.NeedsResearch) != 1 {
		t.Fatalf("poster_url group = %+v, want 0 candidate-ready, 1 needs-research", g)
	}
	row := g.NeedsResearch[0]
	if row.EntityType != model.EnrichEntityVideo || row.Name != "A" {
		t.Errorf("row = %+v, want video A", row)
	}
}

// TestRemediationQueue_ActionableSplit covers F55.7/F55.8: a missing facet
// with a cached, unapplied provider candidate lands in candidate-ready with
// its Provider set, not needs-research. The "file" decision has no baseline
// value to back it (poster_url carries only a tmdb source in this fixture's
// mapping), so the field stays missing even though the tmdb match is cached
// — the same "pinned-but-empty leaves an unapplied candidate" shape the spec's
// worked example describes.
func TestRemediationQueue_ActionableSplit(t *testing.T) {
	h, r := newCompletenessHandlers(t)
	ctx := context.Background()

	id := seedVideo(t, r, model.ExtraMetadata{SourceKey: "Publisher", Value: "Acme"})
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, id, "tmdb", "ext-1", map[string][]string{
		"poster_url": {"https://cdn.example/poster.jpg"},
	}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, id, "poster_url", "file", ""); err != nil {
		t.Fatalf("set decision: %v", err)
	}

	groups, err := h.remediationQueue(ctx)
	if err != nil {
		t.Fatalf("remediationQueue: %v", err)
	}
	g, ok := facetGroupByCanonical(groups, "poster_url")
	if !ok {
		t.Fatalf("poster_url group missing: %+v", groups)
	}
	if len(g.CandidateReady) != 1 || len(g.NeedsResearch) != 0 {
		t.Fatalf("poster_url group = %+v, want 1 candidate-ready, 0 needs-research", g)
	}
	if got := g.CandidateReady[0].Provider; got != "tmdb" {
		t.Errorf("candidate provider = %q, want tmdb", got)
	}
}
