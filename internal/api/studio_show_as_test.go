package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

// TestStudioMergeShowAs is Show as (F70 P0-11, RD14): merging with show_as "merged" keeps
// the survivor's identity, gives it the merged spelling, keeps its former name and every
// alias of both, and rewrites the Studio tag on every linked video as a rename does.
// A bad show_as merges nothing.
func TestStudioMergeShowAs(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	var mu sync.Mutex
	written := map[string][]string{}
	q := writequeue.New(r, func(_ context.Context, path string, fields []writeback.FieldWrite) error {
		mu.Lock()
		defer mu.Unlock()
		for _, f := range fields {
			if f.TagName == "Publisher" {
				written[path] = f.Values
			}
		}
		return nil
	}, log, 1, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)
	h.SetWriteQueue(q)
	h.SetAuth(api.NewAuth(""), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	for _, s := range []struct{ path, studio string }{
		{"/m/a.mkv", "North Pictures"}, {"/m/b.mkv", "North Pictures"}, {"/m/c.mkv", "NorthPictures"},
	} {
		vid, err := r.UpsertVideo(ctx, &model.Video{FilePath: s.path, Title: "V", Container: "Matroska", FileMtime: time.Now().UTC().Truncate(time.Second)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.ReconcileVideoStudios(ctx, vid, []string{s.studio}, nil); err != nil {
			t.Fatal(err)
		}
	}
	survivor := studioIDFromList(t, r, "North Pictures")
	merged := studioIDFromList(t, r, "NorthPictures")
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityStudio, merged, "N Pictures"); err != nil {
		t.Fatal(err)
	}
	mergeURL := srv.URL + "/api/v1/studios/" + itoa(survivor) + "/merge"

	if code, _ := postTok(t, mergeURL, "", map[string]any{"from_id": merged, "show_as": "bogus"}); code != http.StatusBadRequest {
		t.Fatalf("show_as bogus = %d, want 400", code)
	}
	if studioIDFromList(t, r, "NorthPictures") != merged {
		t.Fatal("a refused show_as must merge nothing")
	}
	if code, _ := postTok(t, srv.URL+"/api/v1/tags/1/merge", "", map[string]any{"from_id": 2, "show_as": "merged"}); code != http.StatusBadRequest {
		t.Errorf("tag show_as = %d, want 400", code)
	}

	code, body := postTok(t, mergeURL, "", map[string]any{"from_id": merged, "show_as": "merged"})
	if code != http.StatusOK {
		t.Fatalf("merge show_as merged = %d, want 200", code)
	}
	studio, _ := body["studio"].(map[string]any)
	if studio["name"] != "NorthPictures" || int64(studio["id"].(float64)) != survivor {
		t.Fatalf("survivor = %v, want id %d named NorthPictures", studio, survivor)
	}
	var aliases []string
	for _, a := range studio["aliases"].([]any) {
		aliases = append(aliases, a.(map[string]any)["alias"].(string))
	}
	slices.Sort(aliases)
	if !slices.Equal(aliases, []string{"N Pictures", "North Pictures"}) {
		t.Errorf("aliases = %v, want the former name and the merged studio's alias", aliases)
	}
	ids, err := r.VideoIDsForStudio(ctx, survivor)
	if err != nil || len(ids) != 3 {
		t.Errorf("survivor videos = %v (%v), want all 3", ids, err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		if n, _ := r.PendingWritebackCount(ctx); n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("writeback queue did not drain")
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, path := range []string{"/m/a.mkv", "/m/b.mkv", "/m/c.mkv"} {
		if got := written[path]; !slices.Equal(got, []string{"NorthPictures"}) {
			t.Errorf("%s written = %v, want [NorthPictures]", path, got)
		}
	}
}
