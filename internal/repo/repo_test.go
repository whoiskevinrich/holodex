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

// linkPeople reconciles video_people (F40, ADR-072) for videoID to the given
// names, each with role "actor" — the resolved-derivation path that replaced
// UpsertVideo's old synchronous write. Tests that care about a specific role
// call r.ReconcileVideoPeople directly instead.
func linkPeople(t *testing.T, r *repo.Repo, videoID int64, names ...string) {
	t.Helper()
	links := make([]repo.PersonRoleName, len(names))
	for i, n := range names {
		links[i] = repo.PersonRoleName{Name: n, Role: "actor"}
	}
	if err := r.ReconcileVideoPeople(context.Background(), videoID, links, nil); err != nil {
		t.Fatalf("link people: %v", err)
	}
}

// seedVideoAndPerson seeds one video ("T", /m/a.mkv) linked to a single person
// ("Hayao Miyazaki") and returns both ids — the shared setup for the F55 D4
// *ForEntities batch-loader tests, which prove a person id and a video id
// sharing the same numeric value don't cross-contaminate.
func seedVideoAndPerson(t *testing.T, r *repo.Repo) (vid, pid int64) {
	t.Helper()
	vid, err := r.UpsertVideo(context.Background(), sampleVideo("/m/a.mkv", "T", []string{"Hayao Miyazaki"}, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	linkPeople(t, r, vid, "Hayao Miyazaki")
	pid, ok, err := r.PersonIDByName(context.Background(), "Hayao Miyazaki")
	if err != nil || !ok {
		t.Fatalf("person id: ok=%v err=%v", ok, err)
	}
	return vid, pid
}

func TestUpsertAndGet(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	v := sampleVideo("/m/a.mkv", "Amélie", []string{"Alice", "Bob"}, []string{"documentary"})
	id, err := r.UpsertVideo(ctx, v, []model.ExtraMetadata{{SourceKey: "Publisher", Value: "Acme"}})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	linkPeople(t, r, id, "Alice", "Bob")

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
	linkPeople(t, r, id1, "Alice")
	// Re-extract with changed cast — associations should be replaced, not duplicated.
	v.People = []model.Person{{Name: "Carol"}}
	id2, err := r.UpsertVideo(ctx, v, nil)
	if err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("expected same id, got %d then %d", id1, id2)
	}
	linkPeople(t, r, id2, "Carol")
	got, _, _ := r.GetVideo(ctx, id2)
	if len(got.People) != 1 || got.People[0].Name != "Carol" {
		t.Errorf("people not replaced: %+v", got.People)
	}
}

// TestUpsertPreservesNonFileTags is the highest-priority regression test in
// ADR-075 D3 (HOLODEX-225): a rescan must only ever replace source='file'
// video_tags rows. Before this fix, replaceAssociations() unconditionally
// deleted and rebuilt every video_tags row on each scan, which would have
// silently destroyed a manually-attached or enrichment-materialized tag on the
// very next rescan. No attach/materialization endpoint exists yet (S4/S5), so
// this seeds those rows directly, mirroring what those future callers will do.
func TestUpsertPreservesNonFileTags(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()

	v := sampleVideo("/m/a.mkv", "Title", nil, []string{"documentary"})
	id, err := r.UpsertVideo(ctx, v, nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if _, err := database.ExecContext(ctx,
		`INSERT INTO tags (name) VALUES ('curated'), ('genre-x')`); err != nil {
		t.Fatalf("seed tags: %v", err)
	}
	var manualID, providerID int64
	if err := database.QueryRowContext(ctx, `SELECT id FROM tags WHERE name = 'curated'`).Scan(&manualID); err != nil {
		t.Fatalf("lookup manual tag: %v", err)
	}
	if err := database.QueryRowContext(ctx, `SELECT id FROM tags WHERE name = 'genre-x'`).Scan(&providerID); err != nil {
		t.Fatalf("lookup provider tag: %v", err)
	}
	if _, err := database.ExecContext(ctx,
		`INSERT INTO video_tags (video_id, tag_id, source) VALUES (?, ?, 'manual'), (?, ?, 'provider:tmdb')`,
		id, manualID, id, providerID); err != nil {
		t.Fatalf("seed video_tags: %v", err)
	}

	// Re-extract with a changed file-embedded tag -- only the file-derived
	// association may be replaced; the manual and provider rows must survive.
	v.Tags = []model.Tag{{Name: "short film"}}
	if _, err := r.UpsertVideo(ctx, v, nil); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}

	got, _, err := r.GetVideo(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	names := make(map[string]bool, len(got.Tags))
	for _, tg := range got.Tags {
		names[tg.Name] = true
	}
	if !names["curated"] || !names["genre-x"] {
		t.Errorf("non-file tags lost across rescan: %+v", got.Tags)
	}
	if names["documentary"] {
		t.Errorf("stale file-derived tag survived rescan: %+v", got.Tags)
	}
	if !names["short film"] {
		t.Errorf("new file-derived tag missing: %+v", got.Tags)
	}
	if len(got.Tags) != 3 {
		t.Errorf("want exactly 3 tags (1 file + 1 manual + 1 provider), got %d: %+v", len(got.Tags), got.Tags)
	}
}

// TestPeopleForVideos mirrors StudiosForVideos (studios_test.go): a bulk,
// video-id-keyed people lookup used by merge-writeback propagation (F48.8) so
// it doesn't need one GetVideo call per affected video.
func TestPeopleForVideos(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Bob", "Alice"}, nil), nil)
	if err != nil {
		t.Fatalf("seed video a: %v", err)
	}
	linkPeople(t, r, a, "Bob", "Alice")
	b, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Carol"}, nil), nil)
	if err != nil {
		t.Fatalf("seed video b: %v", err)
	}
	linkPeople(t, r, b, "Carol")
	c, err := r.UpsertVideo(ctx, sampleVideo("/m/c.mkv", "C", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video c (no people): %v", err)
	}

	byVideo, err := r.PeopleForVideos(ctx, []int64{a, b, c})
	if err != nil {
		t.Fatalf("people for videos: %v", err)
	}
	if names := namesOf(byVideo[a]); len(names) != 2 || names[0] != "Alice" || names[1] != "Bob" {
		t.Errorf("video a people = %v, want [Alice Bob] (name order)", names)
	}
	if names := namesOf(byVideo[b]); len(names) != 1 || names[0] != "Carol" {
		t.Errorf("video b people = %v, want [Carol]", names)
	}
	if _, ok := byVideo[c]; ok {
		t.Errorf("video c has no people, want absent from the map, got %+v", byVideo[c])
	}
}

func namesOf(people []model.Person) []string {
	names := make([]string, len(people))
	for i, p := range people {
		names[i] = p.Name
	}
	return names
}

func TestListFilterByPersonAndSearch(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	sunID, _ := r.UpsertVideo(ctx, sampleVideo("/m/sun.mkv", "Sunrise", []string{"Alice"}, []string{"nature"}), nil)
	linkPeople(t, r, sunID, "Alice")
	moonID, _ := r.UpsertVideo(ctx, sampleVideo("/m/moon.mkv", "Moonset", []string{"Bob"}, []string{"nature"}), nil)
	linkPeople(t, r, moonID, "Bob")

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

// Issue #26: Reactivate flips a deactivated row back to active without touching
// its metadata; StatByPath surfaces the active flag the scanner's fast-path needs.
func TestReactivate(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)

	if _, err := r.DeactivateExcept(ctx, nil); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	st, ok, err := r.StatByPath(ctx, "/m/a.mkv")
	if err != nil || !ok {
		t.Fatalf("stat: ok=%v err=%v", ok, err)
	}
	if st.Active {
		t.Fatal("row should be inactive after deactivation")
	}

	if err := r.Reactivate(ctx, id); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if st, _, _ := r.StatByPath(ctx, "/m/a.mkv"); !st.Active {
		t.Error("row should be active after reactivate")
	}
	if _, total, _ := r.ListVideos(ctx, repo.VideoFilter{}); total != 1 {
		t.Errorf("reactivated row should be listed: total=%d, want 1", total)
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

// Random sort (ADR-045) must be a seeded, deterministic shuffle: under one seed a
// paginated walk visits every row exactly once (no duplicate, no gap), the same
// seed reproduces the order, and a different seed generally reorders.
func TestListVideosRandomSeeded(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	const n = 30
	for i := 0; i < n; i++ {
		v := &model.Video{
			FilePath:  "/m/v" + strconv.Itoa(i) + ".mkv",
			Title:     "v" + strconv.Itoa(i),
			FileMtime: time.Now().UTC().Truncate(time.Second),
		}
		if _, err := r.UpsertVideo(ctx, v, nil); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	// Walk the whole set in pages of 7 under a fixed seed and collect ids in order.
	pagedIDs := func(seed int64) []int64 {
		var ids []int64
		for off := 0; off < n; off += 7 {
			items, total, err := r.ListVideos(ctx, repo.VideoFilter{
				Sort: "random", Seed: seed, Limit: 7, Offset: off,
			})
			if err != nil {
				t.Fatalf("list random off=%d: %v", off, err)
			}
			if total != n {
				t.Fatalf("total = %d, want %d", total, n)
			}
			for _, v := range items {
				ids = append(ids, v.ID)
			}
		}
		return ids
	}

	seedA := pagedIDs(12345)
	// Tiling: every id appears exactly once across all pages — no dup, no gap.
	if len(seedA) != n {
		t.Fatalf("paged walk yielded %d ids, want %d", len(seedA), n)
	}
	seen := make(map[int64]bool, n)
	for _, id := range seedA {
		if seen[id] {
			t.Fatalf("id %d appeared twice across pages (shuffle did not tile)", id)
		}
		seen[id] = true
	}

	// Stability: the same seed reproduces the same order.
	if got := pagedIDs(12345); !slices.Equal(got, seedA) {
		t.Errorf("same seed gave a different order:\n %v\n %v", got, seedA)
	}

	// A different seed should generally produce a different order (n=30 makes an
	// accidental identical permutation astronomically unlikely).
	if seedB := pagedIDs(67890); slices.Equal(seedB, seedA) {
		t.Errorf("different seed produced an identical order: %v", seedB)
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

// TestNilSliceRegressions confirms every list-shaped repo read that a JSON
// handler serializes directly returns a non-nil (possibly empty) slice, never
// nil — a nil slice marshals to JSON `null`, and several frontend components
// read `.length` on these fields unconditionally. Caught live for
// FacetValues (MappedFacets.svelte crashed app-wide on hydration); this test
// also covers ListVideos, ListPeople, and Search, which shared the same bug
// class (HOLODEX-275).
func TestNilSliceRegressions(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if fv, err := r.FacetValues(ctx, nil); err != nil {
		t.Fatalf("facet values (no sources): %v", err)
	} else if fv == nil || len(fv) != 0 {
		t.Errorf("facet values (no sources) = %#v, want empty non-nil slice", fv)
	}
	if fv, err := r.FacetValues(ctx, []string{"Publisher"}); err != nil {
		t.Fatalf("facet values (no matches): %v", err)
	} else if fv == nil || len(fv) != 0 {
		t.Errorf("facet values (no matches) = %#v, want empty non-nil slice", fv)
	}

	if items, total, err := r.ListVideos(ctx, repo.VideoFilter{Limit: 10}); err != nil {
		t.Fatalf("list videos: %v", err)
	} else if items == nil || len(items) != 0 || total != 0 {
		t.Errorf("list videos = %#v (total %d), want empty non-nil slice", items, total)
	}

	if people, err := r.ListPeople(ctx, false); err != nil {
		t.Fatalf("list people: %v", err)
	} else if people == nil || len(people) != 0 {
		t.Errorf("list people = %#v, want empty non-nil slice", people)
	}

	if res, err := r.Search(ctx, "", 10); err != nil {
		t.Fatalf("search (empty query): %v", err)
	} else if res.Videos == nil || res.People == nil || res.Tags == nil || res.Studios == nil {
		t.Errorf("search (empty query) = %#v, want every field non-nil", res)
	}
	if res, err := r.Search(ctx, "no-such-thing-anywhere", 10); err != nil {
		t.Fatalf("search (no matches): %v", err)
	} else if res.Videos == nil || res.People == nil || res.Tags == nil || res.Studios == nil {
		t.Errorf("search (no matches) = %#v, want every field non-nil", res)
	}
}
