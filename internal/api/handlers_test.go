package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

func newServer(t *testing.T) (*httptest.Server, *repo.Repo, string) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"))))
	t.Cleanup(srv.Close)
	return srv, r, dir
}

func seedVideo(t *testing.T, r *repo.Repo, path, title string) int64 {
	t.Helper()
	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: path, FileSize: 100, Title: title, Duration: 60,
		Width: 1920, Height: 1080, FileMtime: time.Now().UTC().Truncate(time.Second),
		People: []model.Person{{Name: "Alice"}}, Tags: []model.Tag{{Name: "nature"}},
	}, []model.ExtraMetadata{{SourceKey: "Publisher", Value: "Acme"}})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return id
}

func getJSON(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if resp.Header.Get("Content-Type") != "" {
		_ = json.NewDecoder(resp.Body).Decode(&body)
	}
	return resp.StatusCode, body
}

func TestListAndGetMedia(t *testing.T) {
	srv, r, _ := newServer(t)
	id := seedVideo(t, r, "/m/sun.mkv", "Sunrise")

	code, body := getJSON(t, srv.URL+"/api/v1/media")
	if code != 200 || body["total"].(float64) != 1 {
		t.Fatalf("list: code=%d body=%v", code, body)
	}

	code, body = getJSON(t, srv.URL+"/api/v1/media/"+itoa(id))
	if code != 200 {
		t.Fatalf("detail: code=%d", code)
	}
	if body["video"].(map[string]any)["title"] != "Sunrise" {
		t.Errorf("detail title = %v", body["video"])
	}

	code, _ = getJSON(t, srv.URL+"/api/v1/media/99999")
	if code != http.StatusNotFound {
		t.Errorf("missing media code = %d, want 404", code)
	}
}

func TestSearchAndNav(t *testing.T) {
	srv, r, _ := newServer(t)
	seedVideo(t, r, "/m/sun.mkv", "Sunrise")

	code, body := getJSON(t, srv.URL+"/api/v1/search?q=sun")
	if code != 200 {
		t.Fatalf("search code=%d", code)
	}
	if vids, _ := body["videos"].([]any); len(vids) != 1 {
		t.Errorf("search videos = %v", body["videos"])
	}

	code, body = getJSON(t, srv.URL+"/api/v1/people")
	if code != 200 || len(body["items"].([]any)) != 1 {
		t.Errorf("people: code=%d body=%v", code, body)
	}
	code, body = getJSON(t, srv.URL+"/api/v1/tags")
	if code != 200 || len(body["items"].([]any)) != 1 {
		t.Errorf("tags: code=%d body=%v", code, body)
	}
}

func TestStreamRange(t *testing.T) {
	srv, r, dir := newServer(t)
	// Create a real file and index it so the stream handler can serve it.
	path := filepath.Join(dir, "clip.mp4")
	content := []byte("0123456789abcdef")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	id := seedVideo(t, r, path, "Clip")

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/media/"+itoa(id)+"/stream", nil)
	req.Header.Set("Range", "bytes=0-3")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("stream status = %d, want 206", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != "0123" {
		t.Errorf("range body = %q, want %q", got, "0123")
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
