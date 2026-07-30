package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/model"
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

// assertTagParent fetches tagID and checks its ParentTagID equals want (0
// meaning root/nil), naming the tag as label in any failure.
func assertTagParent(t *testing.T, ctx context.Context, r *repo.Repo, tagID int64, label string, want int64) {
	t.Helper()
	tag, err := r.GetTag(ctx, tagID)
	if err != nil {
		t.Fatalf("get %s: %v", label, err)
	}
	switch {
	case want == 0:
		if tag.ParentTagID != nil {
			t.Errorf("%s.ParentTagID = %v, want nil (root)", label, tag.ParentTagID)
		}
	case tag.ParentTagID == nil || *tag.ParentTagID != want:
		t.Errorf("%s.ParentTagID = %v, want %d", label, tag.ParentTagID, want)
	}
}

// TestMergeReparentsChildren covers P0-11 (ADR-075 D1 RD-M, the spec's own
// example): merging away a tag with children repoints those children onto the
// survivor in the same transaction, rather than orphaning the subtree to root.
func TestMergeReparentsChildren(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	// Mammal is a child of Animal and parent of Dog; Vehicle is an unrelated
	// root. Merging Mammal into Vehicle must repoint Dog onto Vehicle.
	if err := r.MergeEntities(ctx, model.EntityTag, ids["Vehicle"], ids["Mammal"]); err != nil {
		t.Fatalf("merge: %v", err)
	}
	assertTagParent(t, ctx, r, ids["Dog"], "Dog", ids["Vehicle"])
	// GermanShepherd's own parent (Dog) was untouched by the merge.
	assertTagParent(t, ctx, r, ids["GermanShepherd"], "GermanShepherd", ids["Dog"])
}

// TestMergeReparentsChildren_SurvivorWasChildOfLoser covers the edge case
// where the merge survivor was itself a child of the tag being merged away
// (Dog's parent is Mammal; Mammal merges into Dog). The reparent step must
// exclude the survivor's own row — otherwise it would set Dog.ParentTagID to
// Dog. Left alone, Dog's stale reference to the now-deleted Mammal row falls
// to the migration-0032 FK's ON DELETE SET NULL, promoting Dog to root.
func TestMergeReparentsChildren_SurvivorWasChildOfLoser(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	if err := r.MergeEntities(ctx, model.EntityTag, ids["Dog"], ids["Mammal"]); err != nil {
		t.Fatalf("merge: %v", err)
	}
	assertTagParent(t, ctx, r, ids["Dog"], "Dog", 0)
	// GermanShepherd (child of Dog, unrelated to the merge) is untouched.
	assertTagParent(t, ctx, r, ids["GermanShepherd"], "GermanShepherd", ids["Dog"])
}

// TestMergeReparentsChildren_DeepDescendantSurvivor covers the cycle bug found
// alongside P0-11: merging a tag into a non-direct descendant of itself must
// not create a cycle in parent_tag_id. Mammal (loser) merges into
// GermanShepherd, a survivor two levels down (Mammal > Dog > GermanShepherd).
// A naive repoint of Mammal's direct children would set Dog's parent to
// GermanShepherd while GermanShepherd's own parent stays Dog — a live cycle.
// The fix excludes GermanShepherd's whole ancestor chain (Dog, Mammal, Animal)
// from the repoint, so Dog instead falls to the merged row's
// ON DELETE SET NULL and is promoted to root, matching the one-hop case's
// existing precedent.
func TestMergeReparentsChildren_DeepDescendantSurvivor(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	if err := r.MergeEntities(ctx, model.EntityTag, ids["GermanShepherd"], ids["Mammal"]); err != nil {
		t.Fatalf("merge: %v", err)
	}
	// Dog (Mammal's only direct child, and on GermanShepherd's ancestor path)
	// is promoted to root rather than repointed onto GermanShepherd.
	assertTagParent(t, ctx, r, ids["Dog"], "Dog", 0)
	// GermanShepherd's own parent (Dog) is untouched.
	assertTagParent(t, ctx, r, ids["GermanShepherd"], "GermanShepherd", ids["Dog"])

	// No cycle: the ancestor walk must terminate with a bounded chain, not hang.
	got, err := r.AncestorNamesForTag(ctx, ids["GermanShepherd"])
	if err != nil {
		t.Fatalf("ancestor names: %v", err)
	}
	if len(got) != 1 || got[0] != "Dog" {
		t.Errorf("GermanShepherd ancestors = %v, want [Dog]", got)
	}
}

// TestAncestorNamesForTag covers P1-3's breadcrumb query: root-first order
// for a deep tag, an empty (non-nil) slice for a root tag, and that an
// unrelated subtree (Vehicle) doesn't leak in.
func TestAncestorNamesForTag(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	got, err := r.AncestorNamesForTag(ctx, ids["GermanShepherd"])
	if err != nil {
		t.Fatalf("ancestor names: %v", err)
	}
	want := []string{"Animal", "Mammal", "Dog"}
	if len(got) != len(want) {
		t.Fatalf("ancestor names = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("ancestor names[%d] = %q, want %q (order matters: root-first)", i, got[i], w)
		}
	}

	root, err := r.AncestorNamesForTag(ctx, ids["Animal"])
	if err != nil {
		t.Fatalf("ancestor names (root): %v", err)
	}
	if len(root) != 0 {
		t.Errorf("root tag ancestor names = %v, want empty", root)
	}

	unrelated, err := r.AncestorNamesForTag(ctx, ids["Vehicle"])
	if err != nil {
		t.Fatalf("ancestor names (unrelated root): %v", err)
	}
	if len(unrelated) != 0 {
		t.Errorf("Vehicle ancestor names = %v, want empty", unrelated)
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
