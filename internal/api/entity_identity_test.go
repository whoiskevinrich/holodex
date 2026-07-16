package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

// identityServer wires the full API with an optional owner token, for the studio/tag
// name-identity endpoints (F43 S2). token="" leaves the gate open.
func identityServer(t *testing.T, token string) (*httptest.Server, *repo.Repo) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r
}

// seedStudioVideo links a studio name to a fresh video and returns the studio id.
func seedStudioVideo(t *testing.T, r *repo.Repo, path, name string) int64 {
	t.Helper()
	ctx := context.Background()
	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: path, FileSize: 100, Title: "V", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, vid, []string{name}, nil); err != nil {
		t.Fatalf("reconcile studio: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil {
		t.Fatalf("list studios: %v", err)
	}
	for _, s := range studios {
		if s.Name == name {
			return s.ID
		}
	}
	t.Fatalf("studio %q not created", name)
	return 0
}

func seedTagVideo(t *testing.T, r *repo.Repo, path string, tags ...string) {
	t.Helper()
	v := &model.Video{
		FilePath: path, FileSize: 100, Title: "V", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}
	for _, tag := range tags {
		v.Tags = append(v.Tags, model.Tag{Name: tag})
	}
	if _, err := r.UpsertVideo(context.Background(), v, nil); err != nil {
		t.Fatalf("seed tag video: %v", err)
	}
}

func tagID(t *testing.T, r *repo.Repo, name string) int64 {
	t.Helper()
	id, ok, err := r.TagIDByName(context.Background(), name)
	if err != nil || !ok {
		t.Fatalf("tag %q id: ok=%v err=%v", name, ok, err)
	}
	return id
}

func TestStudioIdentityEndpointsGatedAndValidated(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	wb := seedStudioVideo(t, r, "/m/a.mkv", "Warner Bros.")
	aliases := srv.URL + "/api/v1/studios/" + itoa(wb) + "/aliases"

	// Gating.
	if code, _ := postTok(t, aliases, "", map[string]string{"alias": "WB"}); code != http.StatusUnauthorized {
		t.Errorf("no-token add = %d, want 401", code)
	}
	// Validation.
	if code, _ := postTok(t, aliases, "s3cret", map[string]string{"alias": "   "}); code != http.StatusBadRequest {
		t.Errorf("empty alias = %d, want 400", code)
	}
	if code, _ := postTok(t, aliases, "s3cret", map[string]string{"alias": strings.Repeat("a", 201)}); code != http.StatusBadRequest {
		t.Errorf("over-long alias = %d, want 400", code)
	}
	// Happy path: trimmed + returned.
	code, body := postTok(t, aliases, "s3cret", map[string]string{"alias": "  WB  "})
	if code != http.StatusOK {
		t.Fatalf("add = %d, want 200", code)
	}
	list := aliasList(t, body)
	if len(list) != 1 || list[0]["alias"] != "WB" {
		t.Fatalf("after add = %v, want one trimmed 'WB'", body["aliases"])
	}
	aliasID := int64(list[0]["id"].(float64))
	// Idempotent, folded.
	_, body = postTok(t, aliases, "s3cret", map[string]string{"alias": "wb"})
	if l := aliasList(t, body); len(l) != 1 {
		t.Errorf("idempotent re-add grew the list: %v", body["aliases"])
	}
	// Delete gating + 404 + happy path.
	del := aliases + "/" + itoa(aliasID)
	if code := sendTok(t, http.MethodDelete, del, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token delete = %d, want 401", code)
	}
	if code := sendTok(t, http.MethodDelete, aliases+"/99999", "s3cret"); code != http.StatusNotFound {
		t.Errorf("unknown delete = %d, want 404", code)
	}
	if code := sendTok(t, http.MethodDelete, del, "s3cret"); code != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", code)
	}
}

func TestStudioMergeAndRenameEndpoints(t *testing.T) {
	srv, r := identityServer(t, "")
	warner := seedStudioVideo(t, r, "/m/a.mkv", "Warner Bros.")
	wb := seedStudioVideo(t, r, "/m/b.mkv", "WB")
	mergeURL := srv.URL + "/api/v1/studios/" + itoa(warner) + "/merge"

	// Validation.
	if code, _ := postTok(t, mergeURL, "", map[string]int64{"from_id": warner}); code != http.StatusBadRequest {
		t.Errorf("self-merge = %d, want 400", code)
	}
	if code, _ := postTok(t, mergeURL, "", map[string]int64{"from_id": 99999}); code != http.StatusNotFound {
		t.Errorf("unknown from_id = %d, want 404", code)
	}
	// Happy path: returns the survivor with the merged name as an alias.
	code, body := postTok(t, mergeURL, "", map[string]int64{"from_id": wb})
	if code != http.StatusOK {
		t.Fatalf("merge = %d, want 200", code)
	}
	studio, _ := body["studio"].(map[string]any)
	if studio["name"] != "Warner Bros." {
		t.Fatalf("merged studio = %v, want survivor 'Warner Bros.'", body["studio"])
	}
	if aliases, _ := studio["aliases"].([]any); len(aliases) != 1 {
		t.Errorf("survivor aliases = %v, want [WB]", studio["aliases"])
	}

	// Rename to a fresh name → 204; then a collision → 409 carrying the conflict.
	renameURL := srv.URL + "/api/v1/studios/" + itoa(warner) + "/rename"
	if code, _ := postTok(t, renameURL, "", map[string]string{"name": "WarnerMedia"}); code != http.StatusNoContent {
		t.Fatalf("rename = %d, want 204", code)
	}
	disney := seedStudioVideo(t, r, "/m/c.mkv", "Disney")
	code, body = postTok(t, srv.URL+"/api/v1/studios/"+itoa(disney)+"/rename", "", map[string]string{"name": "warnermedia"})
	if code != http.StatusConflict {
		t.Fatalf("colliding rename = %d, want 409", code)
	}
	if _, ok := body["conflict"]; !ok {
		t.Errorf("409 body missing conflict: %v", body)
	}
}

// TestStudioMergeEndpoint_PropagatesWritebackToAffectedVideos is F48.8b end
// to end: merging "WB" into "Warner Bros." enqueues one writeback job per
// video WB was linked to, and a video co-tagged with another studio keeps
// that studio's name alongside the survivor's.
func TestStudioMergeEndpoint_PropagatesWritebackToAffectedVideos(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)

	var mu sync.Mutex
	written := map[string][]string{} // file path -> Publisher tag values written
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

	seed := func(path, title string, studios ...string) {
		vid, err := r.UpsertVideo(ctx, &model.Video{
			FilePath: path, Title: title, Duration: 60, Width: 1920, Height: 1080,
			Container: "Matroska", FileMtime: time.Now().UTC().Truncate(time.Second),
		}, nil)
		if err != nil {
			t.Fatalf("seed %s: %v", path, err)
		}
		if err := r.ReconcileVideoStudios(ctx, vid, studios, nil); err != nil {
			t.Fatalf("reconcile studios for %s: %v", path, err)
		}
	}
	seed("/m/solo.mkv", "Solo", "WB")
	seed("/m/together.mkv", "Together", "WB", "A24")
	seed("/m/warner.mkv", "Warner", "Warner Bros.")

	warner := studioIDFromList(t, r, "Warner Bros.")
	wb := studioIDFromList(t, r, "WB")

	code, _ := postTok(t, srv.URL+"/api/v1/studios/"+itoa(warner)+"/merge", "", map[string]int64{"from_id": wb})
	if code != http.StatusOK {
		t.Fatalf("merge = %d, want 200", code)
	}

	if depth, err := q.Depth(ctx); err != nil || depth != 2 {
		t.Fatalf("queue depth = %d err=%v, want 2 (one writeback job per affected video)", depth, err)
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
	if got := written["/m/solo.mkv"]; len(got) != 1 || got[0] != "Warner Bros." {
		t.Errorf("solo.mkv written = %v, want [Warner Bros.]", got)
	}
	if got := written["/m/together.mkv"]; len(got) != 2 || got[0] != "A24" || got[1] != "Warner Bros." {
		t.Errorf("together.mkv written = %v, want [A24 Warner Bros.] (A24 preserved, WB → Warner Bros.)", got)
	}
	if _, wrote := written["/m/warner.mkv"]; wrote {
		t.Error("warner.mkv was never linked to WB, should not have been written")
	}
}

func studioIDFromList(t *testing.T, r *repo.Repo, name string) int64 {
	t.Helper()
	studios, err := r.ListStudios(context.Background(), false)
	if err != nil {
		t.Fatalf("list studios: %v", err)
	}
	for _, s := range studios {
		if s.Name == name {
			return s.ID
		}
	}
	t.Fatalf("studio %q not found", name)
	return 0
}

func TestTagIdentityEndpoints(t *testing.T) {
	srv, r := identityServer(t, "")
	seedTagVideo(t, r, "/m/a.mkv", "sci-fi")
	seedTagVideo(t, r, "/m/b.mkv", "science fiction")
	scifi := tagID(t, r, "sci-fi")
	sf := tagID(t, r, "science fiction")

	// Add alias.
	code, body := postTok(t, srv.URL+"/api/v1/tags/"+itoa(scifi)+"/aliases", "", map[string]string{"alias": "SF"})
	if code != http.StatusOK || len(aliasList(t, body)) != 1 {
		t.Fatalf("tag add alias = %d, body %v", code, body)
	}

	// Merge "science fiction" into "sci-fi".
	code, body = postTok(t, srv.URL+"/api/v1/tags/"+itoa(scifi)+"/merge", "", map[string]int64{"from_id": sf})
	if code != http.StatusOK {
		t.Fatalf("tag merge = %d, want 200", code)
	}
	tag, _ := body["tag"].(map[string]any)
	if tag["name"] != "sci-fi" {
		t.Errorf("merged tag = %v, want survivor 'sci-fi'", body["tag"])
	}
	if _, ok, _ := r.TagIDByName(context.Background(), "science fiction"); ok {
		t.Error("merge left the loser tag behind")
	}

	// Rename conflict: another tag renamed to the survivor's key → 409.
	seedTagVideo(t, r, "/m/c.mkv", "drama")
	drama := tagID(t, r, "drama")
	code, body = postTok(t, srv.URL+"/api/v1/tags/"+itoa(drama)+"/rename", "", map[string]string{"name": "Sci-Fi"})
	if code != http.StatusConflict {
		t.Fatalf("colliding tag rename = %d, want 409", code)
	}
	if _, ok := body["conflict"]; !ok {
		t.Errorf("409 body missing conflict: %v", body)
	}
}
