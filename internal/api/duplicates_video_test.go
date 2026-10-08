package api_test

import (
	"context"
	"fmt"
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
	"holodex/internal/enrich"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// Duplicate videos API (F76): owner gating, list, compare, keep one, keep both, labels.

func videoDupServer(t *testing.T) (*httptest.Server, *repo.Repo) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    browse: true\n    sources: [fake:title, file:title]\n" +
		"  - canonical: edition\n    label: Edition\n    sources: [Edition, filename:edition]\n" +
		"  - canonical: part\n    label: Part\n    sources: [PartNumber, filename:part]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}
	sp := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sp, []byte("sources:\n  - name: fake\n    base_url: http://fake:9100\n    entity_types: [video]\n    enabled: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	estore, err := enrich.NewStore(sp, log)
	if err != nil {
		t.Fatal(err)
	}
	svc := enrich.NewServiceWithClient(estore, r, log, func(enrich.Source) enrich.ProviderClient { return enrich.NewFake("fake") })
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	h.SetEnrichment(svc)
	h.SetAuth(api.NewAuth("tok"), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r
}

func seedMatchedPair(t *testing.T, r *repo.Repo, prefix string) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	var ids [2]int64
	for i, res := range []int{3840, 1920} {
		id, err := r.UpsertVideo(ctx, &model.Video{
			FilePath: fmt.Sprintf("/m/%s/%d.mkv", prefix, res), FileSize: int64(res), Title: "file title",
			Duration: 2054, Width: res, Height: res * 9 / 16,
			FileMtime: time.Now().UTC().Truncate(time.Second),
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, id, "fake", "fake:"+prefix,
			map[string][]string{"title": {"Harbor Lights"}}); err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}
	return ids[0], ids[1]
}

func TestVideoDuplicates_OwnerGated(t *testing.T) {
	srv, _ := videoDupServer(t)
	base := srv.URL + "/api/v1/owner/duplicates/videos"
	for _, c := range []struct{ method, url string }{
		{http.MethodGet, base},
		{http.MethodGet, base + "/1/2"},
		{http.MethodPost, base + "/keep"},
		{http.MethodPost, base + "/keep-both"},
		{http.MethodPost, base + "/label"},
	} {
		if code := sendTok(t, c.method, c.url, ""); code != http.StatusUnauthorized {
			t.Errorf("%s %s without token = %d, want 401", c.method, c.url, code)
		}
	}
}

func TestVideoDuplicates_ListCompareKeepOne(t *testing.T) {
	srv, r := videoDupServer(t)
	a, b := seedMatchedPair(t, r, "one")
	// Pin a's title to the provider, so the list must show the resolved title (what the
	// media page shows), not the raw file title.
	if err := r.SetDecision(context.Background(), model.EnrichEntityVideo, a, "title", "provider:fake", ""); err != nil {
		t.Fatal(err)
	}
	base := srv.URL + "/api/v1/owner/duplicates/videos"

	code, body := getJSONTok(t, base, "tok")
	pairs, _ := body["pairs"].([]any)
	if code != http.StatusOK || len(pairs) != 1 {
		t.Fatalf("list = %d %v, want one pair", code, body)
	}
	row := pairs[0].(map[string]any)
	if row["title"] != "Harbor Lights" {
		t.Errorf("row title = %v, want the shared provider item's title", row["title"])
	}
	sideA := row["a"].(map[string]any)
	if int64(sideA["id"].(float64)) != a || sideA["title"] != "Harbor Lights" || sideA["width"] != float64(3840) {
		t.Fatalf("side a = %v, want resolved title and facts", sideA)
	}

	code, cmp := getJSONTok(t, fmt.Sprintf("%s/%d/%d", base, a, b), "tok")
	if code != http.StatusOK || cmp["can_label_editions"] != true || cmp["can_label_parts"] != true {
		t.Fatalf("compare = %d %v", code, cmp)
	}
	ca := cmp["a"].(map[string]any)
	if ca["file_name"] != "3840.mkv" || ca["if_kept"] == nil || ca["work"] == nil {
		t.Fatalf("compare side a = %v", ca)
	}
	if code, _ := getJSONTok(t, fmt.Sprintf("%s/%d/%d", base, a, a+100), "tok"); code != http.StatusNotFound {
		t.Errorf("compare of a non-pair = %d, want 404", code)
	}

	if code, _ := postTok(t, base+"/keep", "tok", map[string]any{"keep_id": a, "trash_id": a}); code != http.StatusBadRequest {
		t.Errorf("keep same ids = %d, want 400", code)
	}
	code, kept := postTok(t, base+"/keep", "tok", map[string]any{"keep_id": a, "trash_id": b})
	if code != http.StatusOK || kept["carried"] == nil {
		t.Fatalf("keep = %d %v", code, kept)
	}
	if code, _ := postTok(t, base+"/keep", "tok", map[string]any{"keep_id": a, "trash_id": b}); code != http.StatusConflict {
		t.Errorf("keep on a resolved pair = %d, want 409", code)
	}
	if _, body := getJSONTok(t, base, "tok"); len(body["pairs"].([]any)) != 0 {
		t.Errorf("pair still listed after keep one: %v", body)
	}
}

func TestVideoDuplicates_KeepBoth(t *testing.T) {
	srv, r := videoDupServer(t)
	a, b := seedMatchedPair(t, r, "two")
	base := srv.URL + "/api/v1/owner/duplicates/videos"
	if code, _ := postTok(t, base+"/keep-both", "tok", map[string]any{"id_a": a, "id_b": b}); code != http.StatusNoContent {
		t.Fatalf("keep both = %d, want 204", code)
	}
	if _, body := getJSONTok(t, base, "tok"); len(body["pairs"].([]any)) != 0 {
		t.Errorf("pair still listed after keep both: %v", body)
	}
}

func TestVideoDuplicates_LabelValidation(t *testing.T) {
	srv, r := videoDupServer(t)
	a, b := seedMatchedPair(t, r, "three")
	label := srv.URL + "/api/v1/owner/duplicates/videos/label"
	labels := func(va, vb string) []map[string]any {
		return []map[string]any{{"id": a, "value": va}, {"id": b, "value": vb}}
	}
	for _, c := range []struct {
		name string
		body map[string]any
		code int
	}{
		{"unknown field", map[string]any{"field": "title", "labels": labels("x", "y")}, http.StatusBadRequest},
		{"editions both empty", map[string]any{"field": "edition", "labels": labels(" ", "")}, http.StatusBadRequest},
		{"parts equal", map[string]any{"field": "part", "labels": labels("1", "1")}, http.StatusBadRequest},
		{"part missing", map[string]any{"field": "part", "labels": labels("1", "")}, http.StatusBadRequest},
		{"part not a number", map[string]any{"field": "part", "labels": labels("one", "2")}, http.StatusBadRequest},
		{"part zero", map[string]any{"field": "part", "labels": labels("0", "2")}, http.StatusBadRequest},
	} {
		if code, _ := postTok(t, label, "tok", c.body); code != c.code {
			t.Errorf("%s = %d, want %d", c.name, code, c.code)
		}
	}
	if code, _ := postTok(t, label, "tok", map[string]any{"field": "part", "labels": labels("2", "1")}); code != http.StatusNoContent {
		t.Fatalf("label parts = %d, want 204", code)
	}
	if _, body := getJSONTok(t, srv.URL+"/api/v1/owner/duplicates/videos", "tok"); len(body["pairs"].([]any)) != 0 {
		t.Errorf("labeled pair still listed: %v", body)
	}
}

// Clearing a side that shows an edition stores it as cleared, so the file tag or
// filename can't bring it back; the other side gets its label.
func TestVideoDuplicates_LabelClearsPrefilledEdition(t *testing.T) {
	srv, r := videoDupServer(t)
	a, b := seedMatchedPair(t, r, "five")
	ctx := context.Background()
	for _, id := range []int64{a, b} {
		if err := r.SetDecision(ctx, model.EnrichEntityVideo, id, "edition", "manual", "Extended"); err != nil {
			t.Fatal(err)
		}
	}
	code, _ := postTok(t, srv.URL+"/api/v1/owner/duplicates/videos/label", "tok", map[string]any{
		"field": "edition", "labels": []map[string]any{{"id": a, "value": "Extended"}, {"id": b, "value": ""}},
	})
	if code != http.StatusNoContent {
		t.Fatalf("label = %d, want 204", code)
	}
	decs, err := r.DecisionsForEntity(ctx, model.EnrichEntityVideo, b)
	if err != nil {
		t.Fatal(err)
	}
	cleared := false
	for _, d := range decs {
		if d.FieldKey == "edition" && d.Source == "manual" && d.ManualValue == "" {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("b's edition was not stored as cleared: %+v", decs)
	}
}

// F76 RD9 admitted edition to the clearable allowlist, so the media page's decision API
// takes an explicit clear on it too (title stays refused).
func TestVideoDuplicates_EditionClearableEverywhere(t *testing.T) {
	srv, r := videoDupServer(t)
	a, _ := seedMatchedPair(t, r, "six")
	url := func(c string) string { return fmt.Sprintf("%s/api/v1/media/%d/fields/%s/decision", srv.URL, a, c) }
	if code := sendDecision(t, http.MethodPut, url("edition"), "tok", map[string]any{"source": "manual", "clear": true}); code != http.StatusNoContent {
		t.Fatalf("clear edition = %d, want 204", code)
	}
	if code := sendDecision(t, http.MethodPut, url("title"), "tok", map[string]any{"source": "manual", "clear": true}); code != http.StatusBadRequest {
		t.Fatalf("clear title = %d, want 400 (identity field)", code)
	}
}

// A library whose mapping doesn't declare edition can't label editions.
func TestVideoDuplicates_LabelNotSettable(t *testing.T) {
	srv, r := identityServer(t, "tok")
	a, b := seedMatchedPair(t, r, "four")
	code, _ := postTok(t, srv.URL+"/api/v1/owner/duplicates/videos/label", "tok", map[string]any{
		"field": "edition", "labels": []map[string]any{{"id": a, "value": "Cut"}, {"id": b, "value": ""}},
	})
	if code != http.StatusConflict {
		t.Fatalf("label on an unmapped field = %d, want 409", code)
	}
}
