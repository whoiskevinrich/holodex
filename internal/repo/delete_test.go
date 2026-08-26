package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// TestSoftDeleteHidesFromEverySurface is the cardinal F24.2 invariant: a
// soft-deleted item disappears from every read surface at once. The PR's burden of
// proof (spec non-functional seam) is exactly this enumeration.
func TestSoftDeleteHidesFromEverySurface(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx,
		sampleVideo("/m/gone.mkv", "Vanishing", []string{"Alice"}, []string{"documentary"}),
		[]model.ExtraMetadata{{SourceKey: "Studio", Value: "Acme"}})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	linkPeople(t, r, id, "Alice")
	// A second live item so the people/tags/facet surfaces still have content.
	liveID, err := r.UpsertVideo(ctx,
		sampleVideo("/m/live.mkv", "Surviving", []string{"Alice"}, []string{"documentary"}),
		[]model.ExtraMetadata{{SourceKey: "Studio", Value: "Acme"}})
	if err != nil {
		t.Fatalf("upsert live: %v", err)
	}
	linkPeople(t, r, liveID, "Alice")

	if err := r.SoftDelete(ctx, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// GetVideo → ErrNotFound
	if _, _, err := r.GetVideo(ctx, id); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("GetVideo after soft-delete = %v, want ErrNotFound", err)
	}
	// PathByID (stream) → ErrNotFound
	if _, err := r.PathByID(ctx, id); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("PathByID after soft-delete = %v, want ErrNotFound", err)
	}
	// VideoVisible → false
	if vis, err := r.VideoVisible(ctx, id); err != nil || vis {
		t.Errorf("VideoVisible = %v,%v want false,nil", vis, err)
	}
	// ListVideos → excludes it (only the live item remains)
	vids, total, err := r.ListVideos(ctx, repo.VideoFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(vids) != 1 || vids[0].ID == id {
		t.Errorf("ListVideos total=%d ids include soft-deleted? got %d rows", total, len(vids))
	}
	// Search → excludes it (title "Vanishing" must not surface)
	res, err := r.Search(ctx, "Vanishing", 10, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, v := range res.Videos {
		if v.ID == id {
			t.Errorf("Search returned soft-deleted video %d", id)
		}
	}
	// Related (as subject) → ErrNotFound
	if _, err := r.Related(ctx, id, 5, false); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("Related(subject) = %v, want ErrNotFound", err)
	}
	// Person/Tag counts → only the live item counts
	people, _ := r.ListPeople(ctx, false)
	if len(people) != 1 || people[0].VideoCount != 1 {
		t.Errorf("ListPeople count includes soft-deleted: %+v", people)
	}
	tags, _ := r.ListTags(ctx, false)
	if len(tags) != 1 || tags[0].VideoCount != 1 {
		t.Errorf("ListTags count includes soft-deleted: %+v", tags)
	}
	// FacetValues / MetadataKeys → count only the live item
	facets, _ := r.FacetValues(ctx, []string{"Studio"})
	if len(facets) != 1 || facets[0].Count != 1 {
		t.Errorf("FacetValues includes soft-deleted: %+v", facets)
	}
	keys, _ := r.MetadataKeys(ctx, 3)
	for _, k := range keys {
		if k.SourceKey == "Studio" && k.Count != 1 {
			t.Errorf("MetadataKeys count for Studio = %d, want 1", k.Count)
		}
	}
	// LibraryCounts → one active video, not two
	counts, _ := r.LibraryCounts(ctx)
	if counts.VideosActive != 1 {
		t.Errorf("LibraryCounts.VideosActive = %d, want 1", counts.VideosActive)
	}
	// Trash → exactly the soft-deleted item
	trash, err := r.Trash(ctx)
	if err != nil {
		t.Fatalf("trash: %v", err)
	}
	if len(trash) != 1 || trash[0].ID != id {
		t.Errorf("Trash = %+v, want only id %d", trash, id)
	}
}

func TestSoftDeleteIdempotentAndNotFound(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)

	if err := r.SoftDelete(ctx, id); err != nil {
		t.Fatalf("first soft-delete: %v", err)
	}
	// Capture the timestamp, then re-delete: idempotent, timestamp unchanged.
	first, err := r.Trash(ctx)
	if err != nil || len(first) != 1 {
		t.Fatalf("trash after delete: %v (%d)", err, len(first))
	}
	if err := r.SoftDelete(ctx, id); err != nil {
		t.Errorf("second soft-delete should be idempotent success, got %v", err)
	}
	second, _ := r.Trash(ctx)
	if len(second) != 1 || !second[0].DeletedAt.Equal(first[0].DeletedAt) {
		t.Errorf("idempotent delete changed deleted_at: %v -> %v", first[0].DeletedAt, second[0].DeletedAt)
	}
	// Unknown id → ErrNotFound.
	if err := r.SoftDelete(ctx, 99999); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("SoftDelete(unknown) = %v, want ErrNotFound", err)
	}
}

func TestRestore(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)

	// Restore of a live item → ErrNotFound (nothing to restore, F24.6).
	if err := r.Restore(ctx, id); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("Restore(live) = %v, want ErrNotFound", err)
	}
	if err := r.SoftDelete(ctx, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if err := r.Restore(ctx, id); err != nil {
		t.Fatalf("restore: %v", err)
	}
	// Back to visible.
	if _, _, err := r.GetVideo(ctx, id); err != nil {
		t.Errorf("GetVideo after restore = %v, want visible", err)
	}
	if trash, _ := r.Trash(ctx); len(trash) != 0 {
		t.Errorf("Trash after restore = %d items, want 0", len(trash))
	}
}

func TestExpiredSoftDeletedAndHardDelete(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, _ := r.UpsertVideo(ctx,
		sampleVideo("/m/a.mkv", "A", []string{"Alice"}, []string{"x"}),
		[]model.ExtraMetadata{{SourceKey: "K", Value: "V"}})
	if err := r.SoftDelete(ctx, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// A cutoff in the past excludes the just-deleted item; a future cutoff includes it.
	if exp, _ := r.ExpiredSoftDeleted(ctx, time.Now().Add(-time.Hour)); len(exp) != 0 {
		t.Errorf("ExpiredSoftDeleted(past cutoff) = %d, want 0", len(exp))
	}
	exp, err := r.ExpiredSoftDeleted(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("expired: %v", err)
	}
	if len(exp) != 1 || exp[0].ID != id || exp[0].FilePath != "/m/a.mkv" {
		t.Fatalf("ExpiredSoftDeleted = %+v, want id %d with path", exp, id)
	}

	// PurgePath sees the soft-deleted row (the one read that ignores deleted_at).
	if p, err := r.PurgePath(ctx, id); err != nil || p != "/m/a.mkv" {
		t.Errorf("PurgePath = %q,%v", p, err)
	}

	// HardDelete removes the row; junctions cascade (no orphan video_metadata).
	if err := r.HardDelete(ctx, id); err != nil {
		t.Fatalf("hard delete: %v", err)
	}
	if _, err := r.PurgePath(ctx, id); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("PurgePath after hard-delete = %v, want ErrNotFound", err)
	}
	if exp, _ := r.ExpiredSoftDeleted(ctx, time.Now().Add(time.Hour)); len(exp) != 0 {
		t.Errorf("ExpiredSoftDeleted after hard-delete = %d, want 0", len(exp))
	}
}

// TestStatByPathSurfacesDeleted: the scanner reads Deleted via StatByPath to
// short-circuit a soft-deleted row (F24.3).
func TestStatByPathSurfacesDeleted(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)

	if st, _, _ := r.StatByPath(ctx, "/m/a.mkv"); st.Deleted {
		t.Errorf("Deleted=true before any delete")
	}
	if err := r.SoftDelete(ctx, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	st, ok, err := r.StatByPath(ctx, "/m/a.mkv")
	if err != nil || !ok {
		t.Fatalf("stat: %v ok=%v", err, ok)
	}
	if !st.Deleted {
		t.Errorf("Deleted=false after soft-delete")
	}
}
