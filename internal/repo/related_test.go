package repo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"holodex/internal/repo"
)

// up upserts an active video and returns its id.
func up(t *testing.T, r *repo.Repo, path, title string, people, tags []string) int64 {
	t.Helper()
	id, err := r.UpsertVideo(context.Background(), sampleVideo(path, title, people, tags), nil)
	if err != nil {
		t.Fatalf("upsert %s: %v", path, err)
	}
	return id
}

func personID(t *testing.T, r *repo.Repo, name string) int64 {
	t.Helper()
	people, err := r.ListPeople(context.Background(), false)
	if err != nil {
		t.Fatalf("list people: %v", err)
	}
	for _, p := range people {
		if p.Name == name {
			return p.ID
		}
	}
	t.Fatalf("person %q not found", name)
	return 0
}

// tagID looks up a fixture tag by name, case-insensitively — tag names are
// lower-cased in storage, but fixtures here use mixed-case labels for readability.
func tagID(t *testing.T, r *repo.Repo, name string) int64 {
	t.Helper()
	tags, err := r.ListTags(context.Background(), false)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	for _, tg := range tags {
		if strings.EqualFold(tg.Name, name) {
			return tg.ID
		}
	}
	t.Fatalf("tag %q not found", name)
	return 0
}

func itemIDs(shelf *repo.RelatedShelf) map[int64]bool {
	set := map[int64]bool{}
	if shelf != nil {
		for _, v := range shelf.Items {
			set[v.ID] = true
		}
	}
	return set
}

// TestRelatedSelectionAndExclusion exercises the core ADR-031 rules: person chosen by
// highest global count, tag chosen by the distinctiveness score (a near-universal tag
// loses to a mid-frequency one), items exclude the current item, and associations are
// attached. RANDOM() order is irrelevant — we assert set membership only.
func TestRelatedSelectionAndExclusion(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	// N = 10 active videos. A on 4, B on 2; Tuniv on 8 (near-universal), Ttheme on 4.
	//   score(Tuniv) = 8·(1−8/10) = 1.6  <  score(Ttheme) = 4·(1−4/10) = 2.4  → Ttheme.
	v := up(t, r, "/m/v.mkv", "V", []string{"A", "B"}, []string{"Tuniv", "Ttheme"})
	a1 := up(t, r, "/m/a1.mkv", "A1", []string{"A"}, []string{"Tuniv"})
	a2 := up(t, r, "/m/a2.mkv", "A2", []string{"A"}, []string{"Tuniv"})
	a3 := up(t, r, "/m/a3.mkv", "A3", []string{"A"}, []string{"Ttheme"})
	up(t, r, "/m/b1.mkv", "B1", []string{"B"}, []string{"Tuniv"})
	c1 := up(t, r, "/m/c1.mkv", "C1", []string{"C"}, []string{"Tuniv", "Ttheme"})
	up(t, r, "/m/c2.mkv", "C2", []string{"C"}, []string{"Tuniv"})
	d1 := up(t, r, "/m/d1.mkv", "D1", nil, []string{"Ttheme"})
	up(t, r, "/m/e1.mkv", "E1", nil, []string{"Tuniv"})
	up(t, r, "/m/e2.mkv", "E2", nil, []string{"Tuniv"})

	related, err := r.Related(ctx, v, 5)
	if err != nil {
		t.Fatalf("related: %v", err)
	}

	// Person: A (count 4) over B (count 2).
	if related.Person == nil || related.Person.ID != personID(t, r, "A") {
		t.Fatalf("person block = %+v, want A", related.Person)
	}
	wantPeople := map[int64]bool{a1: true, a2: true, a3: true}
	if got := itemIDs(related.Person); !sameSet(got, wantPeople) {
		t.Errorf("person items = %v, want {a1,a2,a3}", got)
	}
	if itemIDs(related.Person)[v] {
		t.Error("person items must exclude the current item")
	}

	// Tag: the DISTINCTIVE Ttheme, not the near-universal Tuniv.
	if related.Tag == nil || related.Tag.ID != tagID(t, r, "Ttheme") {
		t.Fatalf("tag block = %+v, want Ttheme", related.Tag)
	}
	wantTags := map[int64]bool{a3: true, c1: true, d1: true}
	if got := itemIDs(related.Tag); !sameSet(got, wantTags) {
		t.Errorf("tag items = %v, want {a3,c1,d1}", got)
	}

	// Associations attached (no N+1): every returned item carries its people/tags.
	for _, it := range related.Person.Items {
		if len(it.People) == 0 {
			t.Errorf("item %d missing attached people", it.ID)
		}
	}
}

func sameSet(a, b map[int64]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// TestRelatedEmptyAndNullBlocks covers the present-but-empty and null cases.
func TestRelatedEmptyAndNullBlocks(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	// Item whose only person is unique to it, and which has no tags.
	solo := up(t, r, "/m/solo.mkv", "Solo", []string{"Loner"}, nil)
	related, err := r.Related(ctx, solo, 5)
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	if related.Person == nil {
		t.Fatal("person block should be present (item has a person)")
	}
	if len(related.Person.Items) != 0 {
		t.Errorf("person items = %v, want empty (no siblings)", related.Person.Items)
	}
	if related.Tag != nil {
		t.Errorf("tag block = %+v, want nil (item has no tags)", related.Tag)
	}

	// Item with no people but a tag → person nil, tag block present.
	noPeople := up(t, r, "/m/np.mkv", "NoPeople", nil, []string{"OnlyTag"})
	related2, err := r.Related(ctx, noPeople, 5)
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	if related2.Person != nil {
		t.Errorf("person block = %+v, want nil (no people)", related2.Person)
	}
	if related2.Tag == nil {
		t.Error("tag block should be present (item has a tag)")
	}
}

// TestRelatedActiveOnly verifies deactivated siblings are excluded.
func TestRelatedActiveOnly(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	v := up(t, r, "/m/v.mkv", "V", []string{"A"}, []string{"T"})
	a1 := up(t, r, "/m/a1.mkv", "A1", []string{"A"}, []string{"T"})
	a2 := up(t, r, "/m/a2.mkv", "A2", []string{"A"}, []string{"T"})

	// Deactivate A2 (keep V and A1 active).
	if _, err := r.DeactivateExcept(ctx, []int64{v, a1}); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	related, err := r.Related(ctx, v, 5)
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	got := itemIDs(related.Person)
	if got[a2] {
		t.Error("deactivated sibling A2 must not appear")
	}
	if !got[a1] || len(got) != 1 {
		t.Errorf("person items = %v, want {a1} only", got)
	}
}

// TestRelatedNotFound covers missing and inactive items.
func TestRelatedNotFound(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if _, err := r.Related(ctx, 99999, 5); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("missing id err = %v, want ErrNotFound", err)
	}

	v := up(t, r, "/m/v.mkv", "V", []string{"A"}, []string{"T"})
	if _, err := r.DeactivateExcept(ctx, nil); err != nil { // deactivate all
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := r.Related(ctx, v, 5); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("inactive id err = %v, want ErrNotFound", err)
	}
}

// TestRelatedLimit caps the number of siblings.
func TestRelatedLimit(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	v := up(t, r, "/m/v.mkv", "V", []string{"A"}, nil)
	for i := 0; i < 8; i++ {
		up(t, r, "/m/a"+string(rune('a'+i))+".mkv", "Sib", []string{"A"}, nil)
	}
	related, err := r.Related(ctx, v, 5)
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	if related.Person == nil || len(related.Person.Items) != 5 {
		t.Errorf("want 5 items (capped), got %d", len(related.Person.Items))
	}
}
