package repo_test

import (
	"context"
	"path/filepath"
	"slices"
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

func TestThumbnailStateLifecycle(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	b, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)

	// Fresh videos are backfill candidates (state NULL).
	cands, err := r.ThumbnailBackfillCandidates(ctx, 10)
	if err != nil || len(cands) != 2 {
		t.Fatalf("candidates = %d (%v), want 2", len(cands), err)
	}

	// Marking one generated removes it from the candidate set; a failed one stays.
	if err := r.SetThumbnailState(ctx, a, "generated"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetThumbnailState(ctx, b, "failed"); err != nil {
		t.Fatal(err)
	}
	cands, _ = r.ThumbnailBackfillCandidates(ctx, 10)
	if len(cands) != 1 || cands[0].ID != b {
		t.Fatalf("after marking: candidates = %v, want only %d (failed retried)", cands, b)
	}

	// The state surfaces on reads.
	got, _, _ := r.GetVideo(ctx, a)
	if got.ThumbnailState != "generated" {
		t.Errorf("GetVideo state = %q, want generated", got.ThumbnailState)
	}

	// Reset returns it to the candidate set.
	if err := r.ResetThumbnailState(ctx, a); err != nil {
		t.Fatal(err)
	}
	cands, _ = r.ThumbnailBackfillCandidates(ctx, 10)
	if len(cands) != 2 {
		t.Errorf("after reset: candidates = %d, want 2", len(cands))
	}

	// Candidate-by-id resolves path + duration, and reports missing ids.
	c, ok, err := r.ThumbnailCandidateByID(ctx, a)
	if err != nil || !ok || c.FilePath != "/m/a.mkv" {
		t.Errorf("candidate by id = %+v ok=%v err=%v", c, ok, err)
	}
	if _, ok, _ := r.ThumbnailCandidateByID(ctx, 99999); ok {
		t.Errorf("expected ok=false for missing id")
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

func TestMetadataFacetsKeysAndFilter(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	mk := func(path string, extra []model.ExtraMetadata) {
		v := &model.Video{FilePath: path, Title: "t", FileMtime: time.Now().UTC().Truncate(time.Second)}
		if _, err := r.UpsertVideo(ctx, v, extra); err != nil {
			t.Fatalf("seed %s: %v", path, err)
		}
	}
	mk("/m/a.mkv", []model.ExtraMetadata{{SourceKey: "Publisher", Value: "Acme"}, {SourceKey: "Comment", Value: "hi"}})
	mk("/m/b.mkv", []model.ExtraMetadata{{SourceKey: "Label", Value: "Acme"}, {SourceKey: "Publisher", Value: "Globex"}})

	// FacetValues over the studio sources [Publisher, Label]: Acme spans both
	// videos (a via Publisher, b via Label) → 2; Globex → 1.
	fv, err := r.FacetValues(ctx, []string{"Publisher", "Label"})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, f := range fv {
		counts[f.Value] = f.Count
	}
	if counts["Acme"] != 2 || counts["Globex"] != 1 {
		t.Errorf("facet values = %+v", fv)
	}

	// MetadataKeys: Publisher in 2 videos, Label + Comment in 1 each, with samples.
	keys, err := r.MetadataKeys(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	kc := map[string]int{}
	for _, k := range keys {
		kc[k.SourceKey] = k.Count
	}
	if kc["Publisher"] != 2 || kc["Label"] != 1 || kc["Comment"] != 1 {
		t.Errorf("metadata keys = %+v", keys)
	}

	// Mapped filter: studio=Acme matches both; studio=Globex matches only b.
	if _, total, _ := r.ListVideos(ctx, repo.VideoFilter{MappedFilters: []repo.MappedFilter{{SourceKeys: []string{"Publisher", "Label"}, Value: "Acme"}}}); total != 2 {
		t.Errorf("studio=Acme total = %d, want 2", total)
	}
	if _, total, _ := r.ListVideos(ctx, repo.VideoFilter{MappedFilters: []repo.MappedFilter{{SourceKeys: []string{"Publisher", "Label"}, Value: "Globex"}}}); total != 1 {
		t.Errorf("studio=Globex total = %d, want 1", total)
	}
}

func TestListVideosSort(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	mk := func(path, title string, dur, w, h int) {
		v := &model.Video{
			FilePath: path, Title: title, Duration: dur, Width: w, Height: h,
			FileMtime: time.Now().UTC().Truncate(time.Second),
		}
		if _, err := r.UpsertVideo(ctx, v, nil); err != nil {
			t.Fatalf("upsert %s: %v", path, err)
		}
	}
	// Distinct title / duration / width so each sort key has a unique order.
	mk("/m/c.mkv", "Charlie", 100, 1280, 720)
	mk("/m/a.mkv", "alpha", 300, 3840, 2160)
	mk("/m/b.mkv", "Bravo", 200, 1920, 1080)

	titlesFor := func(sort string) []string {
		items, _, err := r.ListVideos(ctx, repo.VideoFilter{Sort: sort, Limit: 50})
		if err != nil {
			t.Fatalf("list %s: %v", sort, err)
		}
		out := make([]string, len(items))
		for i, v := range items {
			out[i] = v.Title
		}
		return out
	}

	cases := map[string][]string{
		"title_asc":       {"alpha", "Bravo", "Charlie"}, // COLLATE NOCASE
		"title_desc":      {"Charlie", "Bravo", "alpha"},
		"duration_asc":    {"Charlie", "Bravo", "alpha"},
		"duration_desc":   {"alpha", "Bravo", "Charlie"},
		"resolution_asc":  {"Charlie", "Bravo", "alpha"},
		"resolution_desc": {"alpha", "Bravo", "Charlie"},
	}
	for sort, want := range cases {
		if got := titlesFor(sort); !slices.Equal(got, want) {
			t.Errorf("sort=%s order=%v, want %v", sort, got, want)
		}
	}
}

// Codec/container/bitrate (F12.4) survive a write and come back on both the
// detail getter and the list query.
func TestCodecRoundTrip(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	v := &model.Video{
		FilePath: "/m/x.mp4", Title: "X", FileMtime: time.Now().UTC().Truncate(time.Second),
		VideoCodec: "h264", AudioCodec: "aac", BitrateKbps: 8500, Container: "MP4",
	}
	id, err := r.UpsertVideo(ctx, v, nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, _, err := r.GetVideo(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.VideoCodec != "h264" || got.AudioCodec != "aac" || got.BitrateKbps != 8500 || got.Container != "MP4" {
		t.Errorf("detail codec round-trip = %+v", got)
	}
	items, _, err := r.ListVideos(ctx, repo.VideoFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].VideoCodec != "h264" || items[0].Container != "MP4" {
		t.Errorf("list codec = %+v", items)
	}
}
