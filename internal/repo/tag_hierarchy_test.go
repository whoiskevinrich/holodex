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
	want := []string{"animal", "mammal", "dog", "germanshepherd"}
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

// TestTagNamesForVideo_WritebackFlagFlat covers D1's flat per-name filter
// (HOLODEX-239, ADR-077): disabling a tag's writeback flag drops only its own
// name from TagNamesForVideo's output — the ancestor walk still climbs
// through it (a further ancestor beyond the disabled tag still appears) and a
// descendant, attached independently with its own flag, is unaffected.
func TestTagNamesForVideo_WritebackFlagFlat(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r) // Animal > Mammal > Dog > GermanShepherd

	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/flag_flat.mkv", "V", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if _, err := r.AttachTagToVideo(ctx, vid, "GermanShepherd"); err != nil {
		t.Fatalf("attach leaf tag: %v", err)
	}

	// Sanity: flag on (the default) behaves identically to current behavior.
	names, err := r.TagNamesForVideo(ctx, vid)
	if err != nil {
		t.Fatalf("tag names (flag on): %v", err)
	}
	if len(names) != 4 {
		t.Fatalf("tag names (flag on) = %v, want all 4 ancestors", names)
	}

	// Disable "Dog" (a mid-chain ancestor). Its own name must disappear, but
	// "Animal"/"Mammal" (further ancestors, reached by climbing through Dog)
	// and "GermanShepherd" (a descendant, its own directly-attached row) must
	// both still appear.
	if _, err := r.SetTagWritebackEnabled(ctx, ids["Dog"], false); err != nil {
		t.Fatalf("disable Dog writeback: %v", err)
	}
	names, err = r.TagNamesForVideo(ctx, vid)
	if err != nil {
		t.Fatalf("tag names (Dog disabled): %v", err)
	}
	got := make(map[string]bool, len(names))
	for _, n := range names {
		got[n] = true
	}
	if got["dog"] {
		t.Errorf("tag names = %v, Dog must be excluded once disabled", names)
	}
	for _, want := range []string{"animal", "mammal", "germanshepherd"} {
		if !got[want] {
			t.Errorf("tag names = %v, missing %q (must survive a disabled ancestor elsewhere in the chain)", names, want)
		}
	}
	if len(names) != 3 {
		t.Errorf("tag names = %v, want exactly 3 (Dog excluded, nothing else)", names)
	}

	// Re-enabling restores identical-to-default behavior.
	if _, err := r.SetTagWritebackEnabled(ctx, ids["Dog"], true); err != nil {
		t.Fatalf("re-enable Dog writeback: %v", err)
	}
	names, err = r.TagNamesForVideo(ctx, vid)
	if err != nil {
		t.Fatalf("tag names (Dog re-enabled): %v", err)
	}
	if len(names) != 4 {
		t.Errorf("tag names (re-enabled) = %v, want all 4 ancestors again", names)
	}
}

// TestTagNamesForVideo_WritebackFlagFlat_DirectAndAncestorOverlap covers the
// case a code-review pass flagged as untested: a tag reachable BOTH as a
// video's own directly-attached tag AND as an ancestor of a different
// directly-attached tag on the same video. The ancestor CTE (videoTagAncestorsQuery)
// is a UNION-deduplicated set filtered once per unique id, so this should
// collapse to one row and obey its own flag regardless of which path found
// it — this only proves that in practice.
func TestTagNamesForVideo_WritebackFlagFlat_DirectAndAncestorOverlap(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r) // Animal > Mammal > Dog > GermanShepherd

	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/flag_flat_overlap.mkv", "V", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	// Dog is attached directly AND is an ancestor of the separately-attached
	// GermanShepherd.
	if _, err := r.AttachTagToVideo(ctx, vid, "Dog"); err != nil {
		t.Fatalf("attach Dog: %v", err)
	}
	if _, err := r.AttachTagToVideo(ctx, vid, "GermanShepherd"); err != nil {
		t.Fatalf("attach GermanShepherd: %v", err)
	}

	if _, err := r.SetTagWritebackEnabled(ctx, ids["Dog"], false); err != nil {
		t.Fatalf("disable Dog writeback: %v", err)
	}
	names, err := r.TagNamesForVideo(ctx, vid)
	if err != nil {
		t.Fatalf("tag names: %v", err)
	}
	got := make(map[string]int, len(names))
	for _, n := range names {
		got[n]++
	}
	if got["dog"] != 0 {
		t.Errorf("tag names = %v, Dog must be excluded once disabled (reached via both a direct attach and an ancestor walk)", names)
	}
	for _, want := range []string{"animal", "mammal", "germanshepherd"} {
		if got[want] != 1 {
			t.Errorf("tag names = %v, want %q exactly once, got %d", names, want, got[want])
		}
	}
	if len(names) != 3 {
		t.Errorf("tag names = %v, want exactly 3 (no duplicate row for the doubly-reached Dog)", names)
	}
}

// TestSetTagWritebackEnabled covers the single-tag flag flip: it never
// enqueues anything itself (there is no writeQueue wired into this repo-only
// test at all, so any enqueue attempt would panic/nil-deref), only updates
// the stored value and returns the updated tag; an unknown id is ErrNotFound.
func TestSetTagWritebackEnabled(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	tag, err := r.SetTagWritebackEnabled(ctx, ids["Dog"], false)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if tag.WritebackEnabled {
		t.Errorf("tag.WritebackEnabled = true after disabling, want false")
	}
	got, err := r.GetTag(ctx, ids["Dog"])
	if err != nil {
		t.Fatalf("get tag: %v", err)
	}
	if got.WritebackEnabled {
		t.Errorf("GetTag.WritebackEnabled = true after disabling, want false")
	}

	if _, err := r.SetTagWritebackEnabled(ctx, 99999, false); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("unknown tag = %v, want ErrNotFound", err)
	}
}

// TestSetTagsWritebackEnabled covers the bulk flag flip: it applies to every
// listed tag regardless of each one's individual prior state.
func TestSetTagsWritebackEnabled(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	// Dog starts enabled (default); Mammal starts disabled — a mixed
	// selection, so the bulk action must not just toggle each one.
	if _, err := r.SetTagWritebackEnabled(ctx, ids["Mammal"], false); err != nil {
		t.Fatalf("seed disabled Mammal: %v", err)
	}

	if err := r.SetTagsWritebackEnabled(ctx, []int64{ids["Dog"], ids["Mammal"]}, false); err != nil {
		t.Fatalf("bulk disable: %v", err)
	}
	for _, name := range []string{"Dog", "Mammal"} {
		tag, err := r.GetTag(ctx, ids[name])
		if err != nil {
			t.Fatalf("get %s: %v", name, err)
		}
		if tag.WritebackEnabled {
			t.Errorf("%s.WritebackEnabled = true after bulk disable, want false", name)
		}
	}
	// Untouched sibling keeps the default.
	animal, err := r.GetTag(ctx, ids["Animal"])
	if err != nil {
		t.Fatalf("get Animal: %v", err)
	}
	if !animal.WritebackEnabled {
		t.Errorf("Animal.WritebackEnabled = false, want true (untouched by the bulk call)")
	}

	if err := r.SetTagsWritebackEnabled(ctx, []int64{ids["Dog"], ids["Mammal"]}, true); err != nil {
		t.Fatalf("bulk re-enable: %v", err)
	}
	for _, name := range []string{"Dog", "Mammal"} {
		tag, err := r.GetTag(ctx, ids[name])
		if err != nil {
			t.Fatalf("get %s: %v", name, err)
		}
		if !tag.WritebackEnabled {
			t.Errorf("%s.WritebackEnabled = false after bulk re-enable, want true", name)
		}
	}

	// A bad id in the set must 404 the whole call, not silently update the
	// good ids while ignoring the bad one (a code-review fix: the IN (...)
	// UPDATE alone can't tell "some ids didn't match" from "nothing to do").
	// Dog is currently true (re-enabled above); flip toward false so a leaked
	// side effect from the rejected call would actually show up.
	if err := r.SetTagsWritebackEnabled(ctx, []int64{ids["Dog"], 99999}, false); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("bulk with unknown id = %v, want ErrNotFound", err)
	}
	if tag, err := r.GetTag(ctx, ids["Dog"]); err != nil || !tag.WritebackEnabled {
		t.Errorf("Dog.WritebackEnabled = %v, %v, want unchanged (true) after a rejected bulk call", tag, err)
	}
}

// TestTagsExist covers the bulk existence check backing SetTagsWritebackEnabled
// and the bulk sync trigger's 404 (a code-review fix: bulk endpoints
// previously accepted a stale/bad tag id silently).
func TestTagsExist(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	if err := r.TagsExist(ctx, []int64{ids["Dog"], ids["Mammal"]}); err != nil {
		t.Errorf("all-valid ids = %v, want nil", err)
	}
	if err := r.TagsExist(ctx, []int64{ids["Dog"], 99999}); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("one bad id = %v, want ErrNotFound", err)
	}
	// A duplicated valid id must not be mistaken for a bad one (COUNT(*) over
	// IN (...) doesn't grow with repeated values the way len(ids) does).
	if err := r.TagsExist(ctx, []int64{ids["Dog"], ids["Dog"]}); err != nil {
		t.Errorf("duplicated valid id = %v, want nil", err)
	}
	if err := r.TagsExist(ctx, nil); err != nil {
		t.Errorf("nil ids = %v, want nil (no-op)", err)
	}
}

// TestListTagsWritebackEnabled guards against ListTags' batch-attach query
// (separate from GetTag's) silently dropping the column — namedCountQuery is
// shared with ListStudios and has no writeback_enabled select, so this has to
// come from its own attach step, same as parent_tag_id.
func TestListTagsWritebackEnabled(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	if _, err := r.SetTagWritebackEnabled(ctx, ids["Dog"], false); err != nil {
		t.Fatalf("disable Dog: %v", err)
	}

	tags, err := r.ListTags(ctx, false)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	byID := make(map[int64]bool, len(tags))
	for _, tag := range tags {
		byID[tag.ID] = tag.WritebackEnabled
	}
	if byID[ids["Dog"]] {
		t.Errorf("Dog.WritebackEnabled = true from ListTags, want false")
	}
	if !byID[ids["Animal"]] {
		t.Errorf("Animal.WritebackEnabled = false from ListTags, want true (default, untouched)")
	}
}

// TestVideoIDsForTags covers D2's bulk sync scope: the deduplicated union of
// active/non-deleted video ids across every listed tag, so a video attached
// to two selected tags is returned once, not twice.
func TestVideoIDsForTags(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	// "shared" carries both Comedy and Action; the other two videos carry one
	// tag each.
	shared, err := r.UpsertVideo(ctx, sampleVideo("/m/shared.mkv", "Shared", nil, []string{"Comedy", "Action"}), nil)
	if err != nil {
		t.Fatalf("seed shared: %v", err)
	}
	comedyOnly, err := r.UpsertVideo(ctx, sampleVideo("/m/comedy.mkv", "Comedy Only", nil, []string{"Comedy"}), nil)
	if err != nil {
		t.Fatalf("seed comedy-only: %v", err)
	}
	actionOnly, err := r.UpsertVideo(ctx, sampleVideo("/m/action.mkv", "Action Only", nil, []string{"Action"}), nil)
	if err != nil {
		t.Fatalf("seed action-only: %v", err)
	}
	comedyID := tagIDByName(t, r, "Comedy")
	actionID := tagIDByName(t, r, "Action")

	single, err := r.VideoIDsForTag(ctx, comedyID)
	if err != nil {
		t.Fatalf("video ids for Comedy: %v", err)
	}
	if got := toSet(single); len(got) != 2 || !got[shared] || !got[comedyOnly] {
		t.Errorf("VideoIDsForTag(Comedy) = %v, want {shared, comedyOnly}", single)
	}

	union, err := r.VideoIDsForTags(ctx, []int64{comedyID, actionID})
	if err != nil {
		t.Fatalf("video ids for Comedy+Action: %v", err)
	}
	got := toSet(union)
	if len(got) != 3 || !got[shared] || !got[comedyOnly] || !got[actionOnly] {
		t.Errorf("VideoIDsForTags(Comedy,Action) = %v, want {shared, comedyOnly, actionOnly} deduplicated", union)
	}

	empty, err := r.VideoIDsForTags(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("VideoIDsForTags(nil) = %v, %v, want empty no-op", empty, err)
	}
}

func toSet(ids []int64) map[int64]bool {
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
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
	if len(got) != 1 || got[0] != "dog" {
		t.Errorf("GermanShepherd ancestors = %v, want [dog]", got)
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
	want := []string{"animal", "mammal", "dog"}
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

// TestChildrenForTag covers HOLODEX-259's direct-children query: a tag with
// multiple children returns them name-ordered, a leaf tag returns empty (not
// the full subtree — Dog's grandchild GermanShepherd must not appear under
// Mammal), and GetTag surfaces the same result on the tag-detail read.
func TestChildrenForTag(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	ids := seedTagTree(t, r)

	got, err := r.ChildrenForTag(ctx, ids["Mammal"])
	if err != nil {
		t.Fatalf("children for tag: %v", err)
	}
	if len(got) != 1 || got[0].ID != ids["Dog"] || got[0].Name != "dog" {
		t.Errorf("Mammal children = %+v, want [{%d dog}]", got, ids["Dog"])
	}

	leaf, err := r.ChildrenForTag(ctx, ids["GermanShepherd"])
	if err != nil {
		t.Fatalf("children for tag (leaf): %v", err)
	}
	if len(leaf) != 0 {
		t.Errorf("GermanShepherd children = %v, want empty", leaf)
	}

	tag, err := r.GetTag(ctx, ids["Mammal"])
	if err != nil {
		t.Fatalf("get tag: %v", err)
	}
	if len(tag.Children) != 1 || tag.Children[0].ID != ids["Dog"] {
		t.Errorf("GetTag(Mammal).Children = %+v, want [{%d dog}]", tag.Children, ids["Dog"])
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
