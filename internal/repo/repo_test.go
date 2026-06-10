package repo_test

import (
	"context"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

func newRepo(t *testing.T) *repo.Repo {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return repo.New(database)
}

func sampleVideo(path, title string, people, tags []string) *model.Video {
	v := &model.Video{
		FilePath:  path,
		FileSize:  1000,
		Title:     title,
		Duration:  3600,
		Width:     3840,
		Height:    2160,
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}
	for _, p := range people {
		v.People = append(v.People, model.Person{Name: p})
	}
	for _, tg := range tags {
		v.Tags = append(v.Tags, model.Tag{Name: tg})
	}
	return v
}

func TestUpsertAndGet(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "Amélie", []string{"Alice", "Bob"}, []string{"documentary"}),
		[]model.ExtraMetadata{{SourceKey: "Publisher", Value: "Acme"}})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, extra, err := r.GetVideo(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "Amélie" {
		t.Errorf("title = %q", got.Title)
	}
	if len(got.People) != 2 || len(got.Tags) != 1 {
		t.Errorf("associations: people=%d tags=%d", len(got.People), len(got.Tags))
	}
	if len(extra) != 1 || extra[0].SourceKey != "Publisher" {
		t.Errorf("extra metadata = %+v", extra)
	}
}

func TestUpsertIsIdempotent(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	v := sampleVideo("/m/a.mkv", "Title", []string{"Alice"}, []string{"x"})

	id1, _ := r.UpsertVideo(ctx, v, nil)
	// Re-extract with changed cast — associations should be replaced, not duplicated.
	v.People = []model.Person{{Name: "Carol"}}
	id2, err := r.UpsertVideo(ctx, v, nil)
	if err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("expected same id, got %d then %d", id1, id2)
	}
	got, _, _ := r.GetVideo(ctx, id2)
	if len(got.People) != 1 || got.People[0].Name != "Carol" {
		t.Errorf("people not replaced: %+v", got.People)
	}
}

func TestListFilterByPersonAndSearch(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/sun.mkv", "Sunrise", []string{"Alice"}, []string{"nature"}), nil)
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/moon.mkv", "Moonset", []string{"Bob"}, []string{"nature"}), nil)

	alice, err := r.ListPeople(ctx, false)
	if err != nil || len(alice) != 2 {
		t.Fatalf("list people: %v (n=%d)", err, len(alice))
	}
	var aliceID int64
	for _, p := range alice {
		if p.Name == "Alice" {
			aliceID = p.ID
		}
	}

	vids, total, err := r.ListVideos(ctx, repo.VideoFilter{PersonIDs: []int64{aliceID}})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(vids) != 1 || vids[0].Title != "Sunrise" {
		t.Errorf("person filter: total=%d vids=%v", total, vids)
	}

	// Prefix FTS: "sun" should match "Sunrise".
	res, err := r.Search(ctx, "sun", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Videos) != 1 || res.Videos[0].Title != "Sunrise" {
		t.Errorf("search videos = %v", res.Videos)
	}
}

func TestConcurrentUpsertsNoLock(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	// Mirror the scanner's parallel workers: many concurrent writers must all
	// succeed (no SQLITE_BUSY) thanks to the repo write lock.
	const n = 40
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := "/m/clip" + strconv.Itoa(i) + ".mkv"
			if _, err := r.UpsertVideo(ctx, sampleVideo(path, "Clip", []string{"Alice"}, []string{"x"}), nil); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent upsert failed: %v", err)
	}

	_, total, err := r.ListVideos(ctx, repo.VideoFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if total != n {
		t.Errorf("indexed %d videos, want %d", total, n)
	}
}

func TestDeactivateExcept(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	keep, _ := r.UpsertVideo(ctx, sampleVideo("/m/keep.mkv", "Keep", nil, nil), nil)
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/gone.mkv", "Gone", nil, nil), nil)

	n, err := r.DeactivateExcept(ctx, []int64{keep})
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if n != 1 {
		t.Errorf("deactivated = %d, want 1", n)
	}
	vids, total, _ := r.ListVideos(ctx, repo.VideoFilter{})
	if total != 1 || len(vids) != 1 || vids[0].ID != keep {
		t.Errorf("after deactivate: total=%d", total)
	}
}
