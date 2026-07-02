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
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// studioServer wires a real repo with a video whose `studio` field resolves
// file-first from a Publisher tag ("Acme") with a matched tmdb candidate
// ("Acme Films"), plus the studio entity surface. The studio field is a replace
// field so a media decision can move the resolved value — and thus the derived
// video_studios link (F38, ADR-053).
func studioServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
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
		FilePath: "/m/a.mkv", FileSize: 1, Title: "A",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, []model.ExtraMetadata{{SourceKey: "Publisher", Value: "Acme"}})
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, id, "tmdb", "ext-1", map[string][]string{
		"studio": {"Acme Films"},
	}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: studio\n    label: Studio\n    filterable: true\n    sources: [file:Publisher, tmdb:studio]\n"
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

// listStudioNames returns the names in GET /studios (name-sorted).
func listStudioNames(t *testing.T, srv *httptest.Server) []string {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/v1/studios")
	if err != nil {
		t.Fatalf("list studios: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Items []model.Studio `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode studios: %v", err)
	}
	names := make([]string, len(body.Items))
	for i, s := range body.Items {
		names[i] = s.Name
	}
	return names
}

// TestStudioLinkFollowsResolvedValue is the RD1 loop through real endpoints: adopting
// the provider studio moves the derived link (no rescan), and clearing the decision
// reverts to the file value while the adopted studio is pruned.
func TestStudioLinkFollowsResolvedValue(t *testing.T) {
	srv, _, id := studioServer(t, "")
	dec := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/studio/decision"

	// Adopt tmdb → relink → the link (and /studios) shows the adopted value.
	if code := sendDecision(t, http.MethodPut, dec, "", map[string]string{"source": "provider:tmdb"}); code != http.StatusNoContent {
		t.Fatalf("adopt tmdb studio: %d", code)
	}
	if got := listStudioNames(t, srv); len(got) != 1 || got[0] != "Acme Films" {
		t.Fatalf("after adopt: %v, want [Acme Films]", got)
	}

	// Clear → file-first default → the file value; the adopted studio is pruned.
	if code := sendDecision(t, http.MethodDelete, dec, "", nil); code != http.StatusNoContent {
		t.Fatalf("clear studio decision: %d", code)
	}
	if got := listStudioNames(t, srv); len(got) != 1 || got[0] != "Acme" {
		t.Fatalf("after clear: %v, want [Acme] (Acme Films pruned)", got)
	}
}

// TestStudioFacetFilter confirms ?studio_id filters media by the derived link.
func TestStudioFacetFilter(t *testing.T) {
	srv, _, id := studioServer(t, "")
	dec := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/studio/decision"
	if code := sendDecision(t, http.MethodPut, dec, "", map[string]string{"source": "file"}); code != http.StatusNoContent {
		// Pin the file baseline value (Acme) — establishes a link (media vocab = "file").
		t.Fatalf("pin file: %d", code)
	}
	// Find the studio id from the list.
	resp, _ := http.Get(srv.URL + "/api/v1/studios")
	var lst struct {
		Items []model.Studio `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&lst)
	resp.Body.Close()
	if len(lst.Items) != 1 {
		t.Fatalf("want one studio, got %v", lst.Items)
	}
	sid := lst.Items[0].ID

	// ?studio_id matches the video; a bogus id matches nothing.
	if total := mediaTotal(t, srv, "studio_id="+itoa(sid)); total != 1 {
		t.Errorf("studio_id=%d total = %d, want 1", sid, total)
	}
	if total := mediaTotal(t, srv, "studio_id=99999"); total != 0 {
		t.Errorf("studio_id=miss total = %d, want 0", total)
	}
	_ = id
}

// mediaTotal returns the "total" from GET /media?<query>.
func mediaTotal(t *testing.T, srv *httptest.Server, query string) int {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/v1/media?" + query)
	if err != nil {
		t.Fatalf("list media: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Total int `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	return body.Total
}

// TestStudioDetailPayload checks GET /studios/{id} carries resolved[] in the record
// vocabulary with no in_sync (F38 RD5), and its videos.
func TestStudioDetailPayload(t *testing.T) {
	srv, _, id := studioServer(t, "")
	dec := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/studio/decision"
	sendDecision(t, http.MethodPut, dec, "", map[string]string{"source": "file"})

	resp, _ := http.Get(srv.URL + "/api/v1/studios")
	var lst struct {
		Items []model.Studio `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&lst)
	resp.Body.Close()
	sid := lst.Items[0].ID

	r2, err := http.Get(srv.URL + "/api/v1/studios/" + itoa(sid))
	if err != nil {
		t.Fatalf("get studio: %v", err)
	}
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("get studio status %d", r2.StatusCode)
	}
	var body struct {
		Studio   model.Studio     `json:"studio"`
		Items    []model.Video    `json:"items"`
		Resolved []map[string]any `json:"resolved"`
	}
	if err := json.NewDecoder(r2.Body).Decode(&body); err != nil {
		t.Fatalf("decode studio detail: %v", err)
	}
	if body.Studio.Name != "Acme" {
		t.Errorf("studio name = %q, want Acme", body.Studio.Name)
	}
	if len(body.Items) != 1 {
		t.Errorf("studio videos = %d, want 1", len(body.Items))
	}
	// No field carries in_sync (record entity has no file), and the name field
	// resolves in the record vocabulary.
	for _, f := range body.Resolved {
		if _, ok := f["in_sync"]; ok {
			t.Errorf("studio resolved field must omit in_sync: %v", f)
		}
	}
}

// TestStudioDecisionAuth covers the studio ENTITY decision endpoint: name is
// rejected, unknown fields 404, and a visitor is gated.
func TestStudioDecisionAuth(t *testing.T) {
	srv, _, _ := studioServer(t, "")
	// Establish a studio to target.
	vid := srv.URL + "/api/v1/media/1/fields/studio/decision"
	sendDecision(t, http.MethodPut, vid, "", map[string]string{"source": "file"})
	resp, _ := http.Get(srv.URL + "/api/v1/studios")
	var lst struct {
		Items []model.Studio `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&lst)
	resp.Body.Close()
	if len(lst.Items) == 0 {
		t.Fatal("no studio to target")
	}
	sid := itoa(lst.Items[0].ID)

	// name is read-only → 400.
	if code := sendDecision(t, http.MethodPut, srv.URL+"/api/v1/studios/"+sid+"/fields/name/decision", "", map[string]string{"source": "record"}); code != http.StatusBadRequest {
		t.Errorf("name decision = %d, want 400", code)
	}
	// unknown field → 404.
	if code := sendDecision(t, http.MethodPut, srv.URL+"/api/v1/studios/"+sid+"/fields/nope/decision", "", map[string]string{"source": "record"}); code != http.StatusNotFound {
		t.Errorf("unknown field = %d, want 404", code)
	}
	// unknown studio → 404.
	if code := sendDecision(t, http.MethodPut, srv.URL+"/api/v1/studios/99999/fields/description/decision", "", map[string]string{"source": "record"}); code != http.StatusNotFound {
		t.Errorf("unknown studio = %d, want 404", code)
	}
}

func TestStudioDecisionVisitorGated(t *testing.T) {
	srv, _, _ := studioServer(t, "secret") // token set → gate closed for visitors
	vid := srv.URL + "/api/v1/media/1/fields/studio/decision"
	sendDecision(t, http.MethodPut, vid, "secret", map[string]string{"source": "file"})
	resp, _ := http.Get(srv.URL + "/api/v1/studios")
	var lst struct {
		Items []model.Studio `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&lst)
	resp.Body.Close()
	sid := itoa(lst.Items[0].ID)

	// Visitor (no token) is refused on the owner-gated studio decision endpoint.
	url := srv.URL + "/api/v1/studios/" + sid + "/fields/description/decision"
	if code := sendDecision(t, http.MethodPut, url, "", map[string]string{"source": "manual", "manual_value": "x"}); code != http.StatusUnauthorized && code != http.StatusForbidden {
		t.Errorf("visitor decision = %d, want 401/403", code)
	}
}
