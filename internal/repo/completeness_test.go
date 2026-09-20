package repo_test

import (
	"context"
	"slices"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

func intp(n int) *int { return &n }

// ADR-099 D2: the completeness sort is the composite (required, extras) with
// the D1 null rule — an entity with no required band sorts on its extras.
func TestListVideos_CompletenessSort_CompositeWithNullFallback(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := map[string]int64{}
	for _, name := range []string{"A", "B", "C"} {
		id, err := r.UpsertVideo(ctx, sampleVideo("/m/"+name+".mkv", name, nil, nil), nil)
		if err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	rows := []repo.CompletenessRow{
		{EntityType: "video", EntityID: ids["A"], Required: intp(100), Extras: intp(0)},
		{EntityType: "video", EntityID: ids["B"], Required: intp(100), Extras: intp(50)},
		{EntityType: "video", EntityID: ids["C"], Required: nil, Extras: intp(75)}, // every critical facet not-applicable
	}
	for _, row := range rows {
		if err := r.StoreCompleteness(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	titles := func(sort string) []string {
		got, _, err := r.ListVideos(ctx, repo.VideoFilter{Sort: sort})
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(got))
		for i, v := range got {
			out[i] = v.Title
		}
		return out
	}
	if got, want := titles(repo.SortCompletenessAsc), []string{"C", "A", "B"}; !slices.Equal(got, want) {
		t.Errorf("asc = %v, want %v (C's null required falls back to extras 75; A/B tie on 100, broken by extras)", got, want)
	}
	if got, want := titles(repo.SortCompletenessDesc), []string{"B", "A", "C"}; !slices.Equal(got, want) {
		t.Errorf("desc = %v, want %v", got, want)
	}
}

// ADR-099 D4: DrainCompleteness hands each dirty (type, ids) chunk to compute
// and stores what comes back; a drained id compute does not return (soft-
// deleted, or a person with no active video) loses its store rows and its
// dirty flag — never a stale badge, never a tombstone the drain re-runs
// forever. StoreCompleteness (the detail self-heal) leaves the flag alone.
func TestDrainCompleteness_StoresComputedAndClearsUnscored(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	// The insert trigger flagged it; the drain scores it.
	var seen []string
	err = r.DrainCompleteness(ctx, func(entityType string, ids []int64) ([]repo.CompletenessRow, error) {
		seen = append(seen, entityType)
		if entityType != "video" || !slices.Equal(ids, []int64{id}) {
			t.Errorf("compute(%s, %v), want (video, [%d])", entityType, ids, id)
		}
		return []repo.CompletenessRow{{
			EntityType: "video", EntityID: id, Required: intp(50),
			Missing: []repo.MissingFacet{{Canonical: "poster_url", Band: "critical"}},
		}}, nil
	})
	if err != nil || !slices.Equal(seen, []string{"video"}) {
		t.Fatalf("drain: err=%v, compute called for %v", err, seen)
	}
	if got, err := r.StoredCompleteness(ctx, "video", id); err != nil || got.Required == nil || *got.Required != 50 || len(got.Missing) != 1 {
		t.Fatalf("stored = %+v, %v; want required 50 with one missing facet", got, err)
	}
	if counts, _ := r.MissingFacetCounts(ctx, "video"); len(counts) != 1 || counts[0].Canonical != "poster_url" || counts[0].Count != 1 {
		t.Errorf("missing counts = %+v, want poster_url ×1", counts)
	}
	if dirty, _ := r.DirtyCompleteness(ctx); len(dirty["video"]) != 0 {
		t.Errorf("dirty after drain = %v, want empty", dirty)
	}

	// Self-heal rewrites the row but must not touch the dirty flag.
	if err := r.MarkAllCompletenessDirty(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.StoreCompleteness(ctx, repo.CompletenessRow{EntityType: "video", EntityID: id, Required: intp(75)}); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.StoredCompleteness(ctx, "video", id); got == nil || *got.Required != 75 || len(got.Missing) != 0 {
		t.Errorf("stored after self-heal = %+v, want required 75, no missing", got)
	}
	dirty, _ := r.DirtyCompleteness(ctx)
	if !slices.Equal(dirty["video"], []int64{id}) {
		t.Fatalf("dirty after self-heal = %v, want video %d still flagged", dirty, id)
	}

	// Drain with an empty compute result: the id was drained but not scored.
	if err := r.DrainCompleteness(ctx, func(string, []int64) ([]repo.CompletenessRow, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := r.StoredCompleteness(ctx, "video", id); err != repo.ErrNotFound {
		t.Errorf("stored after unscored drain: err = %v, want ErrNotFound", err)
	}
	if dirty, _ := r.DirtyCompleteness(ctx); len(dirty["video"]) != 0 {
		t.Errorf("dirty after drain = %v, want empty", dirty)
	}
	if got, _ := r.CompletenessForEntities(ctx, "video", []int64{id}); len(got) != 0 {
		t.Errorf("CompletenessForEntities after drain = %v, want empty", got)
	}
}

// NamedListFilter: the missing-facet predicate and id restriction compose
// with the people list's own active-video join.
func TestListPeopleFiltered_MissingFacetAndIDs(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ReconcileVideoPeople(ctx, id, []repo.PersonRoleName{{Name: "Ada", Role: "actor"}, {Name: "Bob", Role: "actor"}}, nil); err != nil {
		t.Fatal(err)
	}
	people, err := r.ListPeople(ctx, false)
	if err != nil || len(people) != 2 {
		t.Fatalf("people = %v, %v", people, err)
	}
	ada, bob := people[0].ID, people[1].ID
	for _, row := range []repo.CompletenessRow{
		{EntityType: model.EnrichEntityPerson, EntityID: ada, Required: intp(0), Missing: []repo.MissingFacet{{Canonical: "photo", Band: "critical"}}},
		{EntityType: model.EnrichEntityPerson, EntityID: bob, Required: intp(100)},
	} {
		if err := r.StoreCompleteness(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	names := func(f repo.NamedListFilter) []string {
		got, err := r.ListPeopleFiltered(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(got))
		for i, p := range got {
			out[i] = p.Name
		}
		return out
	}
	if got := names(repo.NamedListFilter{MissingFacets: []string{"photo"}}); !slices.Equal(got, []string{"Ada"}) {
		t.Errorf("missing photo = %v, want [Ada]", got)
	}
	if got := names(repo.NamedListFilter{IDs: []int64{bob}}); !slices.Equal(got, []string{"Bob"}) {
		t.Errorf("ids = %v, want [Bob]", got)
	}
	if got := names(repo.NamedListFilter{Sort: repo.SortCompletenessDesc}); !slices.Equal(got, []string{"Bob", "Ada"}) {
		t.Errorf("completeness_desc = %v, want [Bob Ada]", got)
	}
}
