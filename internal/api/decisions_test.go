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

// decisionServer wires the owner-gated F36 decision surface over a real repo with a
// title (replace) + genres (merge) mapping and a matched `tmdb` provider. token=""
// leaves the gate open (single-user default).
func decisionServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "File Title",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	// A matched tmdb provider supplying both a replace and a merge field.
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, id, "tmdb", "ext-1", map[string][]string{
		"title":  {"TMDB Title"},
		"genres": {"Action", "Drama"},
	}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [tmdb:title, file:title]\n" +
		"  - canonical: genres\n    label: Genres\n    merge: true\n    sources: [tmdb:genres, file:genres]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, id
}

// sendDecision issues a PUT/DELETE with an optional owner token + JSON body and
// returns the status code.
func sendDecision(t *testing.T, method, url, token string, body any) int {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		rdr = strings.NewReader(string(buf))
	}
	req, _ := http.NewRequest(method, url, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// resolvedField pulls one canonical field out of GET /media/{id}'s resolved array.
func resolvedField(t *testing.T, srv *httptest.Server, id int64, canonical string) map[string]any {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/v1/media/" + itoa(id))
	if err != nil {
		t.Fatalf("get media: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Resolved []map[string]any `json:"resolved"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode media: %v", err)
	}
	for _, f := range body.Resolved {
		if f["canonical"] == canonical {
			return f
		}
	}
	t.Fatalf("resolved field %q not found in %v", canonical, body.Resolved)
	return nil
}


func TestDecisionAPI_AdoptProviderThenClear(t *testing.T) {
	srv, _, id := decisionServer(t, "")
	base := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/title/decision"

	// Undecided: file-first default shows the file value.
	f := resolvedField(t, srv, id, "title")
	if vals := f["values"].([]any); vals[0] != "File Title" {
		t.Fatalf("undecided should show file value, got %v", vals)
	}

	// Adopt the provider → 204, then the resolved field flips + reports out of sync.
	if code := sendDecision(t, http.MethodPut, base, "", map[string]string{"source": "provider:tmdb"}); code != 204 {
		t.Fatalf("adopt provider: want 204, got %d", code)
	}
	f = resolvedField(t, srv, id, "title")
	if f["values"].([]any)[0] != "TMDB Title" {
		t.Errorf("adopted: want TMDB Title, got %v", f["values"])
	}
	dec := f["decision"].(map[string]any)
	if dec["source"] != "provider:tmdb" || dec["standing"] != true {
		t.Errorf("decision marker = %v", dec)
	}
	if f["in_sync"] != false {
		t.Errorf("adopted provider differing from file must be out of sync, got %v", f["in_sync"])
	}

	// Clear → 204, back to the file default (in sync).
	if code := sendDecision(t, http.MethodDelete, base, "", nil); code != 204 {
		t.Fatalf("clear: want 204, got %d", code)
	}
	f = resolvedField(t, srv, id, "title")
	if f["values"].([]any)[0] != "File Title" || f["in_sync"] != true {
		t.Errorf("after clear: want File Title in sync, got values=%v in_sync=%v", f["values"], f["in_sync"])
	}
}

func TestDecisionAPI_ManualLiteral(t *testing.T) {
	srv, _, id := decisionServer(t, "")
	base := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/title/decision"

	if code := sendDecision(t, http.MethodPut, base, "", map[string]string{"source": "manual", "manual_value": "My Cut"}); code != 204 {
		t.Fatalf("manual: want 204, got %d", code)
	}
	f := resolvedField(t, srv, id, "title")
	if f["values"].([]any)[0] != "My Cut" {
		t.Errorf("manual: want My Cut, got %v", f["values"])
	}
	if dec := f["decision"].(map[string]any); dec["manual_value"] != "My Cut" {
		t.Errorf("decision should carry the manual literal, got %v", dec)
	}
}

func TestDecisionAPI_Validation(t *testing.T) {
	srv, r, id := decisionServer(t, "")
	mediaFields := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/"

	// Bad source shape → 400.
	if code := sendDecision(t, http.MethodPut, mediaFields+"title/decision", "", map[string]string{"source": "bogus"}); code != 400 {
		t.Errorf("bad source: want 400, got %d", code)
	}
	// Manual without a value → 400.
	if code := sendDecision(t, http.MethodPut, mediaFields+"title/decision", "", map[string]string{"source": "manual"}); code != 400 {
		t.Errorf("manual w/o value: want 400, got %d", code)
	}
	// Unmatched provider → 400.
	if code := sendDecision(t, http.MethodPut, mediaFields+"title/decision", "", map[string]string{"source": "provider:imdb"}); code != 400 {
		t.Errorf("unmatched provider: want 400, got %d", code)
	}
	// Merge field is replace-only → 400.
	if code := sendDecision(t, http.MethodPut, mediaFields+"genres/decision", "", map[string]string{"source": "provider:tmdb"}); code != 400 {
		t.Errorf("merge field: want 400, got %d", code)
	}
	// Unknown field → 404.
	if code := sendDecision(t, http.MethodPut, mediaFields+"nope/decision", "", map[string]string{"source": "file"}); code != 404 {
		t.Errorf("unknown field: want 404, got %d", code)
	}
	// Unknown id → 404.
	if code := sendDecision(t, http.MethodPut, srv.URL+"/api/v1/media/99999/fields/title/decision", "", map[string]string{"source": "file"}); code != 404 {
		t.Errorf("unknown id: want 404, got %d", code)
	}
	// Soft-deleted id → 409.
	if err := r.SoftDelete(context.Background(), id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if code := sendDecision(t, http.MethodPut, mediaFields+"title/decision", "", map[string]string{"source": "file"}); code != 409 {
		t.Errorf("soft-deleted: want 409, got %d", code)
	}
}

func TestDecisionAPI_OwnerGated(t *testing.T) {
	srv, _, id := decisionServer(t, "secret")
	base := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/title/decision"

	// No token → 401 for both verbs.
	if code := sendDecision(t, http.MethodPut, base, "", map[string]string{"source": "file"}); code != 401 {
		t.Errorf("PUT without token: want 401, got %d", code)
	}
	if code := sendDecision(t, http.MethodDelete, base, "", nil); code != 401 {
		t.Errorf("DELETE without token: want 401, got %d", code)
	}
	// With the token → 204.
	if code := sendDecision(t, http.MethodPut, base, "secret", map[string]string{"source": "file"}); code != 204 {
		t.Errorf("PUT with token: want 204, got %d", code)
	}
}
