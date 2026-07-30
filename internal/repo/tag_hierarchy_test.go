package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/repo"
)

// seedTagTree builds a 4-level tag tree — Animal > Mammal > Dog > GermanShepherd
// — plus an unrelated root "Vehicle", via one video per node so each tag exists.
// Returns ids keyed by name.
func seedTagTree(t *testing.T, r *repo.Repo) map[string]int64 {
	t.Helper()
	ctx := context.Background()
	names := []string{"Animal", "Mammal", "Dog", "GermanShepherd", "Vehicle"}
	for _, name := range names {
		if _, err := r.UpsertVideo(ctx, sampleVideo("/m/tree_"+name+".mkv", name, nil, []string{name}), nil); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	ids := make(map[string]int64, len(names))
	for _, name := range names {
		ids[name] = tagIDByName(t, r, name)
	}
	link := func(child, parent string) {
		if _, err := r.SetTagParent(ctx, ids[child], ptr(ids[parent])); err != nil {
			t.Fatalf("link %s -> %s: %v", child, parent, err)
		}
	}
	link("Mammal", "Animal")
	link("Dog", "Mammal")
	link("GermanShepherd", "Dog")
	return ids
}

func ptr(id int64) *int64 { return &id }

// TestSetTagParent_CycleGuard covers the four boundary cases the testing
// strategy calls out (F50 S3, ADR-075 D1): self, direct parent (one-level
// descendant), deep ancestor (multi-level descendant), and an unrelated
// sibling (must succeed — not every reparent is a cycle).
func TestSetTagParent_CycleGuard(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	// Self: a tag cannot be its own parent.
	if _, err := r.SetTagParent(ctx, ids["Dog"], ptr(ids["Dog"])); !errors.Is(err, repo.ErrTagCycle) {
		t.Errorf("self-parent = %v, want ErrTagCycle", err)
	}

	// Direct: Dog is Mammal's direct child; Mammal's parent cannot become Dog.
	if _, err := r.SetTagParent(ctx, ids["Mammal"], ptr(ids["Dog"])); !errors.Is(err, repo.ErrTagCycle) {
		t.Errorf("direct-child-as-parent = %v, want ErrTagCycle", err)
	}

	// Deep: GermanShepherd is a grandchild-of-a-grandchild of Animal; Animal's
	// parent cannot become GermanShepherd.
	if _, err := r.SetTagParent(ctx, ids["Animal"], ptr(ids["GermanShepherd"])); !errors.Is(err, repo.ErrTagCycle) {
		t.Errorf("deep-descendant-as-parent = %v, want ErrTagCycle", err)
	}

	// Unrelated: Vehicle shares no ancestry with Dog — a legitimate reparent.
	tag, err := r.SetTagParent(ctx, ids["Dog"], ptr(ids["Vehicle"]))
	if err != nil {
		t.Fatalf("unrelated reparent: %v", err)
	}
	if tag.ParentTagID == nil || *tag.ParentTagID != ids["Vehicle"] {
		t.Errorf("Dog.ParentTagID = %v, want %d", tag.ParentTagID, ids["Vehicle"])
	}

	// Clearing (nil) always succeeds and drops the tag to root.
	tag, err = r.SetTagParent(ctx, ids["Dog"], nil)
	if err != nil {
		t.Fatalf("clear parent: %v", err)
	}
	if tag.ParentTagID != nil {
		t.Errorf("Dog.ParentTagID after clear = %v, want nil", tag.ParentTagID)
	}
}

// TestTagNamesForVideo covers P0-10's tag-side input (F50 S6, ADR-075 RD9):
// a video tagged only with the deepest leaf of a hierarchy expands to every
// ancestor's name too, and a video with no tags at all is an empty, not nil-panic, result.
func TestTagNamesForVideo(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	seedTagTree(t, r)

	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/ancestor_expand.mkv", "V", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if _, err := r.AttachTagToVideo(ctx, vid, "GermanShepherd"); err != nil {
		t.Fatalf("attach leaf tag: %v", err)
	}

	names, err := r.TagNamesForVideo(ctx, vid)
	if err != nil {
		t.Fatalf("tag names for video: %v", err)
	}
	got := make(map[string]bool, len(names))
	for _, n := range names {
		got[n] = true
	}
	want := []string{"Animal", "Mammal", "Dog", "GermanShepherd"}
	for _, w := range want {
		if !got[w] {
			t.Errorf("ancestor-expanded names = %v, missing %q", names, w)
		}
	}
	if len(names) != len(want) {
		t.Errorf("ancestor-expanded names = %v, want exactly %v", names, want)
	}

	empty, err := r.UpsertVideo(ctx, sampleVideo("/m/no_tags.mkv", "V2", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed untagged video: %v", err)
	}
	if names, err := r.TagNamesForVideo(ctx, empty); err != nil || len(names) != 0 {
		t.Errorf("untagged video names = %v, %v, want empty", names, err)
	}
}

func TestSetTagParent_NotFound(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	if _, err := r.SetTagParent(ctx, 99999, nil); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("unknown tag = %v, want ErrNotFound", err)
	}
	if _, err := r.SetTagParent(ctx, ids["Dog"], ptr(99999)); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("unknown parent = %v, want ErrNotFound", err)
	}
}

// TestListVideos_TagFilterIsDescendantInclusive covers P0-6 (ADR-075 D1):
// filtering by a broader tag also matches videos tagged only with a
// descendant, at any depth, but not the reverse. seedTagTree seeds one video
// per node, each tagged only with that node's own name, so a filter's match
// set is exactly {seed videos of the filtered tag's subtree} plus Rex.
func TestListVideos_TagFilterIsDescendantInclusive(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	// A video tagged only with the deepest leaf.
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/rex.mkv", "Rex", nil, []string{"GermanShepherd"}), nil); err != nil {
		t.Fatalf("seed rex: %v", err)
	}

	// Filtering by the root (Animal) reaches three levels down: its own seed
	// video plus Mammal's, Dog's, GermanShepherd's, and Rex's.
	_, total, err := r.ListVideos(ctx, repo.VideoFilter{TagIDs: []int64{ids["Animal"]}})
	if err != nil {
		t.Fatalf("list by Animal: %v", err)
	}
	if total != 5 {
		t.Errorf("filter by Animal: total=%d, want 5", total)
	}

	// Filtering by an unrelated sibling (Vehicle, a childless root) matches
	// only its own seed video — nothing from the Animal subtree leaks in.
	_, total, err = r.ListVideos(ctx, repo.VideoFilter{TagIDs: []int64{ids["Vehicle"]}})
	if err != nil {
		t.Fatalf("list by Vehicle: %v", err)
	}
	if total != 1 {
		t.Errorf("filter by Vehicle: total=%d, want 1", total)
	}

	// Not the reverse: filtering by the leaf (GermanShepherd, no children of
	// its own) matches only its own seed video and Rex — not the videos
	// seeded under its ancestors (Animal/Mammal/Dog).
	_, total, err = r.ListVideos(ctx, repo.VideoFilter{TagIDs: []int64{ids["GermanShepherd"]}})
	if err != nil {
		t.Fatalf("list by GermanShepherd: %v", err)
	}
	if total != 2 {
		t.Errorf("filter by GermanShepherd: total=%d, want 2 (GermanShepherd + Rex)", total)
	}
}
