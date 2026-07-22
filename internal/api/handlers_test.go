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
	"strings"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
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
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil), nil))
	t.Cleanup(srv.Close)
	return srv, r, dir
}

// fakeRescanner records TriggerRescan calls for the admin-rescan handler test.
type fakeRescanner struct {
	calls   int
	started bool
}

func (f *fakeRescanner) TriggerRescan() bool { f.calls++; return f.started }

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

func TestAdminActivity(t *testing.T) {
	srv, r, _ := newServer(t)
	_ = seedVideo(t, r, "/secret/library/clip.mp4", "Clip")

	code, body := getJSON(t, srv.URL+"/api/v1/admin/activity")
	if code != 200 {
		t.Fatalf("activity code = %d", code)
	}
	scan, _ := body["scan"].(map[string]any)
	if scan["state"] != "idle" {
		t.Errorf("scan.state = %v, want idle", scan["state"])
	}
	lib, _ := body["library"].(map[string]any)
	if lib["videos_active"].(float64) != 1 {
		t.Errorf("library.videos_active = %v, want 1", lib["videos_active"])
	}
	sys, _ := body["system"].(map[string]any)
	if _, ok := sys["media_path_present"]; !ok {
		t.Error("system.media_path_present missing")
	}
	if sys["controls_unauthenticated"] != false {
		t.Errorf("system.controls_unauthenticated = %v, want false", sys["controls_unauthenticated"])
	}
}

// The activity read-model must not leak filesystem paths (no-secrets invariant,
// ADR-028): media_path_present is a boolean, not the path.
func TestAdminActivityNoSecrets(t *testing.T) {
	srv, r, _ := newServer(t)
	_ = seedVideo(t, r, "/secret/library/clip.mp4", "Clip")

	resp, err := http.Get(srv.URL + "/api/v1/admin/activity")
	if err != nil {
		t.Fatalf("GET activity: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(raw), "/secret/library") {
		t.Errorf("activity payload leaked a filesystem path: %s", raw)
	}
}

func TestAdminActivityHistory(t *testing.T) {
	srv, r, _ := newServer(t)
	now := time.Now().UTC()
	if err := r.RecordJobRun(context.Background(), model.JobRun{
		Kind: model.JobKindScan, Trigger: model.TriggerManual, Status: model.JobStatusOK,
		StartedAt: now, FinishedAt: now, DurationMs: 5, Added: 3,
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	code, body := getJSON(t, srv.URL+"/api/v1/admin/activity/history")
	if code != 200 {
		t.Fatalf("history code = %d", code)
	}
	runs, _ := body["runs"].([]any)
	if len(runs) != 1 {
		t.Fatalf("history runs = %d, want 1", len(runs))
	}
}

// The digest endpoint (ADR-071 D3) rolls up per kind and lists the window's
// failures; a clean window has an empty failures list and no error counted.
func TestAdminActivityDigest(t *testing.T) {
	srv, r, _ := newServer(t)
	now := time.Now().UTC()
	rec := func(kind, status string, ago time.Duration) {
		at := now.Add(-ago)
		if err := r.RecordJobRun(context.Background(), model.JobRun{
			Kind: kind, Trigger: model.TriggerManual, Status: status,
			StartedAt: at, FinishedAt: at,
		}); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
	rec(model.JobKindScan, model.JobStatusOK, 2*time.Minute)
	rec(model.JobKindEnrich, model.JobStatusErr, time.Minute)

	code, body := getJSON(t, srv.URL+"/api/v1/admin/activity/digest")
	if code != 200 {
		t.Fatalf("digest code = %d", code)
	}
	kinds, _ := body["kinds"].([]any)
	if len(kinds) != 2 {
		t.Fatalf("digest kinds = %d, want 2", len(kinds))
	}
	failures, _ := body["failures"].([]any)
	if len(failures) != 1 {
		t.Fatalf("digest failures = %d, want 1 (the enrich error)", len(failures))
	}
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

func TestAdminRescan(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	fake := &fakeRescanner{started: true}
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), fake, nil), nil))
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/api/v1/admin/rescan", "application/json", nil)
	if err != nil {
		t.Fatalf("POST rescan: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("rescan status = %d, want 202", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["started"] != true {
		t.Errorf("started = %v, want true", body["started"])
	}
	if fake.calls != 1 {
		t.Errorf("TriggerRescan calls = %d, want 1", fake.calls)
	}
}

// Without a wired scanner (health-only mode), rescan reports 503 rather than
// pretending to have queued work.
func TestAdminRescanUnavailable(t *testing.T) {
	srv, _, _ := newServer(t)
	resp, err := http.Post(srv.URL+"/api/v1/admin/rescan", "application/json", nil)
	if err != nil {
		t.Fatalf("POST rescan: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("rescan without scanner = %d, want 503", resp.StatusCode)
	}
}

func TestMetadataFields(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "Studio Film",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, []model.ExtraMetadata{{SourceKey: "Publisher", Value: "Acme"}})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	if err := os.WriteFile(mpath, []byte("fields:\n  - canonical: studio\n    label: Studio\n    sources: [Publisher, Label]\n    filterable: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	// Facets: one filterable field with value "Acme".
	code, body := getJSON(t, srv.URL+"/api/v1/facets")
	facets, _ := body["facets"].([]any)
	if code != 200 || len(facets) != 1 {
		t.Fatalf("facets: code=%d body=%v", code, body)
	}
	f0 := facets[0].(map[string]any)
	vals := f0["values"].([]any)
	if f0["canonical"] != "studio" || len(vals) != 1 || vals[0].(map[string]any)["value"] != "Acme" {
		t.Errorf("facet = %v", f0)
	}

	// Mapped filter ?studio=Acme matches; a miss is empty.
	if _, b := getJSON(t, srv.URL+"/api/v1/media?studio=Acme"); b["total"].(float64) != 1 {
		t.Errorf("studio=Acme total = %v", b["total"])
	}
	if _, b := getJSON(t, srv.URL+"/api/v1/media?studio=Nope"); b["total"].(float64) != 0 {
		t.Errorf("studio=Nope total = %v", b["total"])
	}

	// Detail includes resolved fields.
	_, body = getJSON(t, srv.URL+"/api/v1/media/"+itoa(id))
	fields, _ := body["fields"].([]any)
	if len(fields) != 1 || fields[0].(map[string]any)["label"] != "Studio" {
		t.Errorf("detail fields = %v", body["fields"])
	}

	// Metadata keys flag the mapped Publisher key.
	_, body = getJSON(t, srv.URL+"/api/v1/metadata-keys")
	keys, _ := body["keys"].([]any)
	if len(keys) < 1 || keys[0].(map[string]any)["source_key"] != "Publisher" || keys[0].(map[string]any)["mapped"] != true {
		t.Errorf("metadata-keys = %v", body["keys"])
	}

	// Reload endpoint succeeds.
	resp, err := http.Post(srv.URL+"/api/v1/admin/reload-config", "application/json", nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("reload code = %d", resp.StatusCode)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
