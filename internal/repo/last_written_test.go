package repo_test

import (
	"context"
	"path/filepath"
	"testing"
)

// TestLastWrittenValues pins ADR-101 D2's query: the newest successful write per
// field_key for one video, nothing from another video, and an empty map (not an
// error) for a video with no writes.
func TestLastWrittenValues(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "v.mp4"), "A", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	other, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "w.mp4"), "B", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed other: %v", err)
	}

	empty, err := r.LastWrittenValues(ctx, id)
	if err != nil || len(empty) != 0 {
		t.Fatalf("no writes: want empty map, got %v %v", empty, err)
	}

	must := func(vid int64, field, value string) {
		t.Helper()
		if err := r.InsertWriteback(ctx, vid, field, "cover.jpg", value, "provider:tmdb"); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	must(id, "poster_url", "https://image.tmdb.org/old.jpg")
	must(id, "poster_url", "https://image.tmdb.org/new.jpg")
	must(id, "title", "Dune")
	must(other, "poster_url", "https://image.tmdb.org/other.jpg")

	got, err := r.LastWrittenValues(ctx, id)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if got["poster_url"] != "https://image.tmdb.org/new.jpg" {
		t.Errorf("newest poster write: got %q", got["poster_url"])
	}
	if got["title"] != "Dune" {
		t.Errorf("title write: got %q", got["title"])
	}
	if len(got) != 2 {
		t.Errorf("want exactly this video's two fields, got %v", got)
	}
}
