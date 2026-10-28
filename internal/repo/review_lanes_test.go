package repo_test

import (
	"context"
	"slices"
	"testing"

	"holodex/internal/model"
)

// TestReviewPairsLane: one lane's pages, read in SQL, add up to exactly that type's slice
// of the whole list, in the same order, and its count agrees once a pair goes stale
// (F70 P0-8). Every named type gets a count, 0 included.
func TestReviewPairsLane(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	seedTwoTags(t, r, "sci-fi", "scifi")
	seedTwoTags(t, r, "rom-com", "romcom")
	staleID, _ := seedTwoTags(t, r, "k-pop", "kpop")

	page1, err := r.ListReviewPairsPage(ctx, model.EntityTag, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	page2, err := r.ListReviewPairsPage(ctx, model.EntityTag, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	all, err := r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := append(page1, page2...), tagPairsOnly(all); !slices.Equal(got, want) || len(want) != 3 {
		t.Fatalf("pages = %+v, want the whole tag list %+v", got, want)
	}
	for _, p := range page1 {
		if p.A.VideoCount != 1 || p.B.VideoCount != 1 {
			t.Errorf("paged pair counts = %+v, want 1 video a side", p)
		}
	}

	// A rename plus dropping the old spelling leaves the k-pop row stale: the count must
	// drop with the list.
	if _, err := r.RenameEntity(ctx, model.EntityTag, staleID, "ambient"); err != nil {
		t.Fatal(err)
	}
	aliases, err := r.AliasesForEntity(ctx, model.EntityTag, staleID)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range aliases {
		if err := r.DeleteEntityAlias(ctx, model.EntityTag, staleID, a.ID); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := r.ReviewPairCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{model.EntityTag: 2, model.EnrichEntityStudio: 0, model.EnrichEntityPerson: 0, model.EnrichEntityFilm: 0}
	for et, n := range want {
		if c, ok := counts[et]; !ok || c != n {
			t.Errorf("counts[%s] = %d (present %v), want %d", et, c, ok, n)
		}
	}
	lane, err := r.ListReviewPairsPage(ctx, model.EntityTag, -1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lane) != 2 {
		t.Errorf("lane after rename = %d pairs, want 2", len(lane))
	}
}
