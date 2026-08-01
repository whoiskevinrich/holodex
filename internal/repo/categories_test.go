package repo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"holodex/internal/repo"
)

// TestCategoryCRUD covers create/list/get/rename/delete and the same-table
// duplicate-name conflict (ErrNameTaken, ADR-078 D1).
func TestCategoryCRUD(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	c, err := r.CreateCategory(ctx, "Holiday")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.Name != "Holiday" || len(c.Tags) != 0 {
		t.Fatalf("created category = %+v", c)
	}

	if _, err := r.CreateCategory(ctx, "holiday"); !errors.Is(err, repo.ErrNameTaken) {
		t.Errorf("duplicate create (case-fold) = %v, want ErrNameTaken", err)
	}

	list, err := r.ListCategories(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v, %v, want exactly one category", list, err)
	}

	got, err := r.GetCategory(ctx, c.ID)
	if err != nil || got.Name != "Holiday" {
		t.Fatalf("get = %+v, %v", got, err)
	}

	renamed, err := r.RenameCategory(ctx, c.ID, "Holidays")
	if err != nil || renamed.Name != "Holidays" {
		t.Fatalf("rename = %+v, %v", renamed, err)
	}
	// Renaming to the current exact name is a no-op success.
	if _, err := r.RenameCategory(ctx, c.ID, "Holidays"); err != nil {
		t.Errorf("no-op rename: %v", err)
	}

	if err := r.DeleteCategory(ctx, c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.GetCategory(ctx, c.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("get after delete = %v, want ErrNotFound", err)
	}
	if err := r.DeleteCategory(ctx, c.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("delete already-deleted = %v, want ErrNotFound", err)
	}
}

// TestCategoryCrossTableCollision covers ADR-078 D3 in both directions: a
// category can't take an existing tag's name, and a tag can't take an
// existing category's name — using the tag-style fold ("Sci Fi" == "SciFi")
// on both sides.
func TestCategoryCrossTableCollision(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", nil, []string{"Sci Fi"}), nil); err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if _, err := r.CreateCategory(ctx, "SciFi"); !errors.Is(err, repo.ErrCategoryNameCollidesWithTag) {
		t.Errorf("category colliding with tag (fold) = %v, want ErrCategoryNameCollidesWithTag", err)
	}

	holiday, err := r.CreateCategory(ctx, "Holiday")
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	if _, err := r.RenameCategory(ctx, holiday.ID, "Sci Fi"); !errors.Is(err, repo.ErrCategoryNameCollidesWithTag) {
		t.Errorf("category rename colliding with tag = %v, want ErrCategoryNameCollidesWithTag", err)
	}

	if _, err := r.AttachTagToVideo(ctx, mustVideoID(t, r, ctx), "Holiday"); !errors.Is(err, repo.ErrTagNameCollidesWithCategory) {
		t.Errorf("tag colliding with category = %v, want ErrTagNameCollidesWithCategory", err)
	}

	// The scanner path (UpsertVideo -> replaceAssociations) skips a
	// category-colliding tag silently (ADR-078 D3, mirroring ADR-075 D2's
	// deny-list precedent) rather than failing the whole upsert.
	id, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "T2", nil, []string{"Holiday", "Winter"}), nil)
	if err != nil {
		t.Fatalf("upsert with category-colliding tag: %v", err)
	}
	got, _, err := r.GetVideo(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	names := make(map[string]bool, len(got.Tags))
	for _, tg := range got.Tags {
		names[tg.Name] = true
	}
	if names["holiday"] {
		t.Errorf("category-colliding tag was created via scanner: %+v", got.Tags)
	}
	if !names["winter"] {
		t.Errorf("sibling non-colliding tag missing: %+v", got.Tags)
	}
}

// mustVideoID seeds a bare video with no tags and returns its id, for tests
// that only need a valid video to attach a tag to.
func mustVideoID(t *testing.T, r *repo.Repo, ctx context.Context) int64 {
	t.Helper()
	id, err := r.UpsertVideo(ctx, sampleVideo("/m/attach-target.mkv", "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	return id
}

// TestCategoryTagAssignment covers assign/unassign (idempotent, bulk,
// transactional) and delete's cascade to category_tags (ADR-078 D2): deleting
// a category unassigns it from every tag without deleting the tags.
func TestCategoryTagAssignment(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", nil, []string{"Beach", "Mountain", "Forest"}), nil)
	if err != nil {
		t.Fatalf("seed tags: %v", err)
	}
	video, _, err := r.GetVideo(ctx, id)
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	tagID := func(name string) int64 {
		for _, tg := range video.Tags {
			if tg.Name == name {
				return tg.ID
			}
		}
		t.Fatalf("no such tag %q", name)
		return 0
	}
	beach, mountain, forest := tagID("beach"), tagID("mountain"), tagID("forest")

	cat, err := r.CreateCategory(ctx, "Nature")
	if err != nil {
		t.Fatalf("create category: %v", err)
	}

	if _, err := r.AssignTagsToCategory(ctx, cat.ID, []int64{beach, mountain}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	// Idempotent: re-assigning an already-member tag alongside a new one is a no-op + add.
	got, err := r.AssignTagsToCategory(ctx, cat.ID, []int64{beach, forest})
	if err != nil || len(got.Tags) != 3 {
		t.Fatalf("category after re-assign = %+v, %v, want 3 member tags", got, err)
	}

	got, err = r.UnassignTagsFromCategory(ctx, cat.ID, []int64{mountain})
	if err != nil || len(got.Tags) != 2 {
		t.Fatalf("category after unassign = %+v, %v, want 2 member tags", got, err)
	}

	if _, err := r.AssignTagsToCategory(ctx, 999999, []int64{beach}); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("assign to missing category = %v, want ErrNotFound", err)
	}

	// Delete cascades to category_tags (D2) but leaves the tags themselves intact.
	if err := r.DeleteCategory(ctx, cat.ID); err != nil {
		t.Fatalf("delete category: %v", err)
	}
	tags, err := r.ListTags(ctx, false)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) != 3 {
		t.Errorf("tags after category delete = %+v, want all 3 still present", tags)
	}
}

// TestCategoryVideoFilterFacet covers the browse-page category facet
// (ADR-078 D2/Consequences): selecting a category matches every video tagged
// with any of its member tags, with no new filtering primitive.
func TestCategoryVideoFilterFacet(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	beachVideo, err := r.UpsertVideo(ctx, sampleVideo("/m/beach.mkv", "Beach Day", nil, []string{"Beach"}), nil)
	if err != nil {
		t.Fatalf("seed beach video: %v", err)
	}
	mountainVideo, err := r.UpsertVideo(ctx, sampleVideo("/m/mountain.mkv", "Mountain Day", nil, []string{"Mountain"}), nil)
	if err != nil {
		t.Fatalf("seed mountain video: %v", err)
	}
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/city.mkv", "City Day", nil, []string{"City"}), nil); err != nil {
		t.Fatalf("seed city video: %v", err)
	}

	video, _, err := r.GetVideo(ctx, beachVideo)
	if err != nil {
		t.Fatalf("get beach video: %v", err)
	}
	beachTagID := video.Tags[0].ID
	video, _, err = r.GetVideo(ctx, mountainVideo)
	if err != nil {
		t.Fatalf("get mountain video: %v", err)
	}
	mountainTagID := video.Tags[0].ID

	cat, err := r.CreateCategory(ctx, "Nature")
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	if _, err := r.AssignTagsToCategory(ctx, cat.ID, []int64{beachTagID, mountainTagID}); err != nil {
		t.Fatalf("assign: %v", err)
	}

	items, total, err := r.ListVideos(ctx, repo.VideoFilter{CategoryIDs: []int64{cat.ID}})
	if err != nil {
		t.Fatalf("list videos by category: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("category facet matched %d videos, want 2: %+v", total, items)
	}
	got := map[int64]bool{}
	for _, v := range items {
		got[v.ID] = true
	}
	if !got[beachVideo] || !got[mountainVideo] {
		t.Errorf("category facet results = %+v, want beach+mountain videos", items)
	}
}

// TestListCategoriesTagFields covers the S5 addition to ListCategories: the
// TagCount/TagIDs fields the /tags pill's count badge and the "Remove from
// category…" picker's client-side membership filter both read (HOLODEX-240) —
// derived from a single pass over category_tags, not a per-category query.
func TestListCategoriesTagFields(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", nil, []string{"Beach", "Mountain"}), nil)
	if err != nil {
		t.Fatalf("seed tags: %v", err)
	}
	video, _, err := r.GetVideo(ctx, id)
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	var beach, mountain int64
	for _, tg := range video.Tags {
		switch tg.Name {
		case "beach":
			beach = tg.ID
		case "mountain":
			mountain = tg.ID
		}
	}

	populated, err := r.CreateCategory(ctx, "Nature")
	if err != nil {
		t.Fatalf("create populated category: %v", err)
	}
	if _, err := r.AssignTagsToCategory(ctx, populated.ID, []int64{beach, mountain}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	empty, err := r.CreateCategory(ctx, "Empty")
	if err != nil {
		t.Fatalf("create empty category: %v", err)
	}

	list, err := r.ListCategories(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	find := func(id int64) (tagCount int, tagIDs []int64, ok bool) {
		for _, c := range list {
			if c.ID == id {
				return c.TagCount, c.TagIDs, true
			}
		}
		return 0, nil, false
	}

	tc, ids, ok := find(populated.ID)
	if !ok || tc != 2 || len(ids) != 2 {
		t.Fatalf("populated category = TagCount:%d TagIDs:%v ok:%v, want TagCount=2, 2 TagIDs", tc, ids, ok)
	}
	got := map[int64]bool{ids[0]: true, ids[1]: true}
	if !got[beach] || !got[mountain] {
		t.Errorf("populated category TagIDs = %v, want {%d,%d}", ids, beach, mountain)
	}

	tc, ids, ok = find(empty.ID)
	if !ok || tc != 0 || len(ids) != 0 {
		t.Errorf("empty category = TagCount:%d TagIDs:%v ok:%v, want TagCount=0, no TagIDs", tc, ids, ok)
	}
}

// TestResolveOrCreateTag covers the S5 addition backing /categories/{id}'s
// "+ Add tag" control (HOLODEX-240): resolve-or-create with no video attach,
// sharing resolveOrCreateByName's deny-list/length-cap/category-collision
// checks (ADR-078 D3) with every other tag-creation path (mirrors
// TestAttachTagToVideo in video_tags_test.go, minus the video-link concern).
func TestResolveOrCreateTag(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	tag, err := r.ResolveOrCreateTag(ctx, "Documentary")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tag.Name != "documentary" {
		t.Errorf("tag.Name = %q, want documentary", tag.Name)
	}
	// ListTags now left-joins video_tags (HOLODEX-243: namedCountQuery switched
	// from an inner join) so a zero-video tag appears immediately with
	// VideoCount=0, instead of being invisible until some video is tagged with
	// it -- this is the resolution of the UX edge case a previous version of
	// this test flagged rather than fixed.
	if tags, err := r.ListTags(ctx, false); err != nil || len(tags) != 1 || tags[0].VideoCount != 0 {
		t.Errorf("ListTags with only a zero-video tag = %+v, %v, want one tag with VideoCount=0", tags, err)
	}
	if id, ok, err := r.TagIDByName(ctx, "Documentary"); err != nil || !ok || id != tag.ID {
		t.Fatalf("TagIDByName after resolve-or-create = %d, %v, %v, want ok with id %d", id, ok, err, tag.ID)
	}

	// Idempotent: resolving the same (case/whitespace-variant) name returns
	// the same tag, not a duplicate.
	again, err := r.ResolveOrCreateTag(ctx, "  documentary ")
	if err != nil {
		t.Fatalf("re-resolve: %v", err)
	}
	if again.ID != tag.ID {
		t.Errorf("re-resolve id = %d, want %d (same tag)", again.ID, tag.ID)
	}

	// Deny-list: refused, not silently created.
	if _, err := r.DenyTag(ctx, "Gnome"); err != nil {
		t.Fatalf("deny: %v", err)
	}
	if _, err := r.ResolveOrCreateTag(ctx, "Gnome"); !errors.Is(err, repo.ErrTagDenied) {
		t.Errorf("resolve denied term = %v, want ErrTagDenied", err)
	}

	// Length cap (ADR-078 item 8 — category names inherit the same cap the
	// tag-creation choke point already enforces).
	if _, err := r.ResolveOrCreateTag(ctx, strings.Repeat("a", 201)); !errors.Is(err, repo.ErrTagNameTooLong) {
		t.Errorf("resolve over-long name = %v, want ErrTagNameTooLong", err)
	}

	// Cross-table collision (ADR-078 D3): a name already claimed by a category.
	if _, err := r.CreateCategory(ctx, "Holiday"); err != nil {
		t.Fatalf("create category: %v", err)
	}
	if _, err := r.ResolveOrCreateTag(ctx, "Holiday"); !errors.Is(err, repo.ErrTagNameCollidesWithCategory) {
		t.Errorf("resolve category-colliding name = %v, want ErrTagNameCollidesWithCategory", err)
	}
}
