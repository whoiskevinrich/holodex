package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// activityServer is newServer with an owner token, for the mutation routes under
// /admin/activity that must answer 401 without one.
func activityServer(t *testing.T, token string) (*httptest.Server, *repo.Repo) {
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

// TestDismissJobRuns_OwnerGatedAndIdempotent covers spec P0-7's HTTP contract:
// both dismiss routes sit under requireOwner (401 without the token), a row
// dismiss answers {dismissed:true} once and {dismissed:false} after, the bulk
// dismiss reports its count, and the digest read afterwards excludes what was
// dismissed while the history still returns it with dismissed_at.
func TestDismissJobRuns_OwnerGatedAndIdempotent(t *testing.T) {
	srv, r := activityServer(t, "secret")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	rec := func(kind string, ago time.Duration) {
		t.Helper()
		at := now.Add(-ago)
		if err := r.RecordJobRun(ctx, model.JobRun{
			Kind: kind, Trigger: model.TriggerManual, Status: model.JobStatusErr,
			StartedAt: at, FinishedAt: at, ErrorMessage: "boom",
		}); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
	rec(model.JobKindWriteback, 2*time.Hour)
	rec(model.JobKindEnrich, time.Hour)
	runs, err := r.ListJobRuns(ctx, 30)
	if err != nil || len(runs) != 2 {
		t.Fatalf("seed: %v / %d runs", err, len(runs))
	}
	newest := runs[0].ID // enrich, newest-first

	rowURL := srv.URL + "/api/v1/admin/activity/runs/" + itoa(newest) + "/dismiss"
	allURL := srv.URL + "/api/v1/admin/activity/failures/dismiss"

	if code, _ := postTok(t, rowURL, "", nil); code != http.StatusUnauthorized {
		t.Errorf("unauthenticated row dismiss = %d, want 401", code)
	}
	if code, _ := postTok(t, allURL, "", map[string]int{"days": 30}); code != http.StatusUnauthorized {
		t.Errorf("unauthenticated dismiss all = %d, want 401", code)
	}

	if code, body := postTok(t, rowURL, "secret", nil); code != http.StatusOK || body["dismissed"] != true {
		t.Errorf("row dismiss = %d/%v, want 200 dismissed:true", code, body)
	}
	if code, body := postTok(t, rowURL, "secret", nil); code != http.StatusOK || body["dismissed"] != false {
		t.Errorf("second row dismiss = %d/%v, want 200 dismissed:false (no-op, never 409)", code, body)
	}

	code, body := getJSONTok(t, srv.URL+"/api/v1/admin/activity/digest", "secret")
	if code != http.StatusOK {
		t.Fatalf("digest = %d", code)
	}
	if failures, _ := body["failures"].([]any); len(failures) != 1 {
		t.Errorf("digest failures after row dismiss = %d, want 1 (the writeback error)", len(failures))
	}
	for _, k := range body["kinds"].([]any) {
		kind := k.(map[string]any)
		if kind["kind"] == model.JobKindEnrich && (kind["errors"] != float64(0) || kind["last_dismissed"] != true) {
			t.Errorf("enrich digest row = %v, want errors:0 last_dismissed:true", kind)
		}
	}

	if code, body := postTok(t, allURL, "secret", map[string]int{"days": 30}); code != http.StatusOK || body["dismissed"] != float64(1) {
		t.Errorf("dismiss all = %d/%v, want 200 dismissed:1 (the writeback error; enrich already dismissed)", code, body)
	}

	code, body = getJSONTok(t, srv.URL+"/api/v1/admin/activity/history", "secret")
	if code != http.StatusOK {
		t.Fatalf("history = %d", code)
	}
	hist, _ := body["runs"].([]any)
	if len(hist) != 2 {
		t.Fatalf("history runs = %d, want 2 — dismissing never removes a run", len(hist))
	}
	for _, run := range hist {
		if _, ok := run.(map[string]any)["dismissed_at"]; !ok {
			t.Errorf("history run %v lacks dismissed_at after dismiss all", run)
		}
	}
}
