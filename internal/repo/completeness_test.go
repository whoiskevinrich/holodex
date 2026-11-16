package repo_test

import (
	"context"
	"slices"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// ADR-099 D4: DrainCompleteness hands each dirty (type, ids) chunk to compute
// and stores what comes back; a drained id compute does not return (soft-
// deleted, or a person with no active video) loses its store rows and its
// dirty flag — never a stale queue row, never a tombstone the drain re-runs
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
			EntityType: "video", EntityID: id,
			Missing: []repo.MissingFacet{{Canonical: "poster_url", Band: "critical"}},
		}}, nil
	})
	if err != nil || !slices.Equal(seen, []string{"video"}) {
		t.Fatalf("drain: err=%v, compute called for %v", err, seen)
	}
	if got, err := r.StoredMissing(ctx, "video", id); err != nil || len(got) != 1 {
		t.Fatalf("stored = %+v, %v; want one missing facet", got, err)
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
	if err := r.StoreCompleteness(ctx, repo.CompletenessRow{EntityType: "video", EntityID: id}); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.StoredMissing(ctx, "video", id); len(got) != 0 {
		t.Errorf("stored after self-heal = %+v, want no missing", got)
	}
	dirty, _ := r.DirtyCompleteness(ctx)
	if !slices.Equal(dirty["video"], []int64{id}) {
		t.Fatalf("dirty after self-heal = %v, want video %d still flagged", dirty, id)
	}

	// Drain with an empty compute result: the id was drained but not scored.
	if err := r.DrainCompleteness(ctx, func(string, []int64) ([]repo.CompletenessRow, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if dirty, _ := r.DirtyCompleteness(ctx); len(dirty["video"]) != 0 {
		t.Errorf("dirty after drain = %v, want empty", dirty)
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
		{EntityType: model.EnrichEntityPerson, EntityID: ada, Missing: []repo.MissingFacet{{Canonical: "photo", Band: "critical"}}},
		{EntityType: model.EnrichEntityPerson, EntityID: bob},
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
}
