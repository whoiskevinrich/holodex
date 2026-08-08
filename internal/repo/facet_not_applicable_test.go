package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
)

func TestFacetNotApplicable_SetGetClear(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, sampleVideo("/m/d.mkv", "Film", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// No exclusions initially.
	if facets, err := r.FacetsNotApplicableForEntity(ctx, model.EnrichEntityVideo, id); err != nil || len(facets) != 0 {
		t.Fatalf("want no exclusions, got %v err=%v", facets, err)
	}

	// Marking is idempotent — setting twice leaves one row.
	if err := r.SetFacetNotApplicable(ctx, model.EnrichEntityVideo, id, "imdb_id"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := r.SetFacetNotApplicable(ctx, model.EnrichEntityVideo, id, "imdb_id"); err != nil {
		t.Fatalf("re-set: %v", err)
	}

	facets, err := r.FacetsNotApplicableForEntity(ctx, model.EnrichEntityVideo, id)
	if err != nil || len(facets) != 1 || !facets["imdb_id"] {
		t.Fatalf("want {imdb_id: true}, got %v err=%v", facets, err)
	}

	// A different entity of the same type is unaffected.
	id2, _ := r.UpsertVideo(ctx, sampleVideo("/m/e.mkv", "Other", nil, nil), nil)
	if facets, err := r.FacetsNotApplicableForEntity(ctx, model.EnrichEntityVideo, id2); err != nil || len(facets) != 0 {
		t.Fatalf("other entity should be unaffected, got %v err=%v", facets, err)
	}

	// Clear removes the row; clearing again is an idempotent no-op.
	n, err := r.ClearFacetNotApplicable(ctx, model.EnrichEntityVideo, id, "imdb_id")
	if err != nil || n != 1 {
		t.Fatalf("clear: n=%d err=%v", n, err)
	}
	n, err = r.ClearFacetNotApplicable(ctx, model.EnrichEntityVideo, id, "imdb_id")
	if err != nil || n != 0 {
		t.Fatalf("second clear should affect 0 rows, got n=%d err=%v", n, err)
	}
	if facets, err := r.FacetsNotApplicableForEntity(ctx, model.EnrichEntityVideo, id); err != nil || len(facets) != 0 {
		t.Fatalf("want no exclusions after clear, got %v err=%v", facets, err)
	}
}

// TestFacetsNotApplicableForEntities_Batch covers the F55 list-wide generic batch
// loader (ADR-081 D4): entity-type-parameterized like FacetsNotApplicableForEntity,
// but batched across ids and scoped by entity_type — a person id and a video id
// sharing the same numeric value must not cross-contaminate.
func TestFacetsNotApplicableForEntities_Batch(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	vid, pid := seedVideoAndPerson(t, r)

	if err := r.SetFacetNotApplicable(ctx, model.EnrichEntityPerson, pid, "nationality"); err != nil {
		t.Fatalf("set person exclusion: %v", err)
	}
	if err := r.SetFacetNotApplicable(ctx, model.EnrichEntityVideo, vid, "imdb_id"); err != nil {
		t.Fatalf("set video exclusion: %v", err)
	}

	got, err := r.FacetsNotApplicableForEntities(ctx, model.EnrichEntityPerson, []int64{pid})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if len(got[pid]) != 1 || !got[pid]["nationality"] {
		t.Fatalf("person exclusions = %v, want {nationality: true}", got[pid])
	}
}
