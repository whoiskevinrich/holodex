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
	"testing"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/extract"
	"holodex/internal/mapping"
	"holodex/internal/repo"
)

// extractServer wires the F48.5 extraction triggers over a real repo. wire=false
// leaves both the orchestrator and batch runner unset (to exercise the 503
// path). The pattern list matches seedVideo's convention "<title> (Acme).mkv"
// so a seeded video's filename always matches, exercising the full
// match -> store -> route pipeline end to end.
func extractServer(t *testing.T, token string, wire bool) (*httptest.Server, *repo.Repo) {
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
	if wire {
		patternPath := filepath.Join(dir, "metadata-patterns.yaml")
		if err := os.WriteFile(patternPath, []byte("patterns:\n  - \"{title} (Acme)\"\n"), 0o644); err != nil {
			t.Fatalf("write patterns: %v", err)
		}
		patterns, err := extract.NewPatternStore(patternPath)
		if err != nil {
			t.Fatalf("load patterns: %v", err)
		}
		mappingPath := filepath.Join(dir, "metadata-mappings.yaml")
		if err := os.WriteFile(mappingPath, []byte("fields:\n  - canonical: title\n    sources: [\"filename:title\"]\n"), 0o644); err != nil {
			t.Fatalf("write mappings: %v", err)
		}
		mappings, err := mapping.NewStore(mappingPath)
		if err != nil {
			t.Fatalf("load mappings: %v", err)
		}
		orch := &extract.Orchestrator{
			Videos:   r,
			Mappings: mappings,
			Patterns: patterns,
			Store:    r,
			Deps: extract.Deps{
				Resolver:     r,
				ManualSource: r,
				Reviews:      r,
				Log:          log,
			},
		}
		batch := &extract.BatchRunner{Orchestrator: orch, Videos: r, Recorder: r, Log: log}
		h.SetExtraction(orch, batch)
	}
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r
}

func extractPOST(t *testing.T, url, token string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, nil)
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func TestExtractMediaUnavailable(t *testing.T) {
	srv, r := extractServer(t, "", false)
	id := seedVideo(t, r, "/m/Big Movie (Acme).mkv", "Big Movie")
	if code, _ := extractPOST(t, srv.URL+"/api/v1/media/"+itoa(id)+"/extract", ""); code != http.StatusServiceUnavailable {
		t.Fatalf("unwired: want 503, got %d", code)
	}
}

func TestExtractMediaNotFound(t *testing.T) {
	srv, _ := extractServer(t, "", true)
	if code, _ := extractPOST(t, srv.URL+"/api/v1/media/999999/extract", ""); code != http.StatusNotFound {
		t.Fatalf("unknown id: want 404, got %d", code)
	}
}

func TestExtractMediaRequiresOwner(t *testing.T) {
	srv, r := extractServer(t, "s3cret", true)
	id := seedVideo(t, r, "/m/Big Movie (Acme).mkv", "Big Movie")
	url := srv.URL + "/api/v1/media/" + itoa(id) + "/extract"

	if code, _ := extractPOST(t, url, ""); code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", code)
	}
	if code, _ := extractPOST(t, url, "s3cret"); code != http.StatusOK {
		t.Fatalf("valid token: want 200, got %d", code)
	}
}

// TestExtractMediaMatch proves the full F48.5a pipeline end to end: a
// filename matching the configured pattern is parsed, its value stored into
// the filename shadow provider, and the field routed — reflected immediately
// in the synchronous response (no queue, no preview).
func TestExtractMediaMatch(t *testing.T) {
	srv, r := extractServer(t, "", true)
	id := seedVideo(t, r, "/m/Big Movie (Acme).mkv", "Something Else")
	url := srv.URL + "/api/v1/media/" + itoa(id) + "/extract"

	code, body := extractPOST(t, url, "")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d: %v", code, body)
	}
	if matched, _ := body["matched"].(bool); !matched {
		t.Fatalf("matched = %v, want true (body=%v)", body["matched"], body)
	}
	fields, _ := body["fields"].([]any)
	if len(fields) != 1 {
		t.Fatalf("fields = %v, want exactly one (title)", fields)
	}

	rows, err := r.EnrichmentForEntity(context.Background(), "video", id)
	if err != nil {
		t.Fatalf("EnrichmentForEntity: %v", err)
	}
	if len(rows) != 1 || rows[0].Provider != "filename" || rows[0].FieldKey != "title" {
		t.Fatalf("filename shadow rows = %+v, want a single title row", rows)
	}
}

func TestAdminExtractAllUnavailable(t *testing.T) {
	srv, _ := extractServer(t, "", false)
	if code, _ := extractPOST(t, srv.URL+"/api/v1/admin/extract-all", ""); code != http.StatusServiceUnavailable {
		t.Fatalf("unwired: want 503, got %d", code)
	}
}

// TestAdminExtractAllAccepted proves the batch trigger returns 202 with
// started:true immediately, matching adminRescan's contract (F48.5b).
func TestAdminExtractAllAccepted(t *testing.T) {
	srv, _ := extractServer(t, "", true)
	code, body := extractPOST(t, srv.URL+"/api/v1/admin/extract-all", "")
	if code != http.StatusAccepted {
		t.Fatalf("want 202, got %d", code)
	}
	if started, _ := body["started"].(bool); !started {
		t.Fatalf("started = %v, want true", body["started"])
	}
}
