package api_test

import (
	"bytes"
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
	"holodex/internal/db"
	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// enrichServer wires the owner-gated enrichment surface over the in-process fake
// provider (no network). token="" leaves the gate open (single-user default).
func enrichServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)

	sp := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sp, []byte("sources:\n  - name: fake\n    base_url: http://fake:9100\n    entity_types: [person]\n    enabled: true\n"), 0o644); err != nil {
		t.Fatalf("write sources: %v", err)
	}
	store, err := enrich.NewStore(sp)
	if err != nil {
		t.Fatalf("sources store: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := enrich.NewServiceWithClient(store, r, log, func(enrich.Source) enrich.ProviderClient { return enrich.NewFake("fake") })

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetEnrichment(svc)
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	// Seed a person matching the fake's canned record.
	ctx := context.Background()
	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/x.mkv", Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Hayao Miyazaki"}},
	}, nil)
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	linkPeopleAs(t, r, vid, "director", "Hayao Miyazaki")
	pid, _, err := r.PersonIDByName(context.Background(), "Hayao Miyazaki")
	if err != nil {
		t.Fatalf("person id: %v", err)
	}
	return srv, r, pid
}

// studioEnrichServer wires the owner-gated studio enrichment surface over the fake
// provider (studio-capable) and seeds one studio entity ("Studio Ghibli") via the
// link-reconcile path. Returns the studio id. Mirrors enrichServer (F38 S3).
func studioEnrichServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	ctx := context.Background()

	sp := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sp, []byte("sources:\n  - name: fake\n    base_url: http://fake:9100\n    entity_types: [person, studio]\n    enabled: true\n"), 0o644); err != nil {
		t.Fatalf("write sources: %v", err)
	}
	store, err := enrich.NewStore(sp)
	if err != nil {
		t.Fatalf("sources store: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := enrich.NewServiceWithClient(store, r, log, func(enrich.Source) enrich.ProviderClient { return enrich.NewFake("fake") })

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetEnrichment(svc)
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	// Seed a video and derive a studio entity from it (the link-reconcile path).
	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/s.mkv", Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, vid, []string{"Studio Ghibli"}, nil); err != nil {
		t.Fatalf("seed studio: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil || len(studios) != 1 {
		t.Fatalf("list studios: %v (%d)", err, len(studios))
	}
	return srv, r, studios[0].ID
}

// Full owner studio flow (open gate): resolve → apply → studio detail shows the
// enriched fields with record-vocabulary provenance (F38 S3).
func TestStudioEnrichFlow(t *testing.T) {
	srv, _, sid := studioEnrichServer(t, "")
	base := srv.URL + "/api/v1/studios/" + itoa(sid)

	code, body := postTok(t, base+"/enrich/resolve", "", map[string]string{"provider": "fake", "query": "ghibli"})
	if code != http.StatusOK {
		t.Fatalf("resolve code = %d", code)
	}
	if cands, _ := body["candidates"].([]any); len(cands) != 1 {
		t.Fatalf("candidates = %v", body["candidates"])
	}

	code, body = postTok(t, base+"/enrich", "", map[string]string{"provider": "fake", "external_id": "tmdb:10342"})
	if code != http.StatusOK {
		t.Fatalf("apply code = %d", code)
	}
	if enriched, _ := body["enriched"].([]any); len(enriched) == 0 {
		t.Fatalf("enriched empty: %v", body["enriched"])
	}

	// Studio detail carries the resolved fields with provenance (record vocabulary).
	code, sbody := getJSON(t, base)
	if code != http.StatusOK {
		t.Fatalf("studio code = %d", code)
	}
	resolved, _ := sbody["resolved"].([]any)
	var desc map[string]any
	for _, f := range resolved {
		if m, _ := f.(map[string]any); m["canonical"] == "description" {
			desc = m
		}
	}
	if desc == nil {
		t.Fatalf("resolved description missing: %v", sbody["resolved"])
	}
	if desc["winning_source"] != "fake:description" {
		t.Errorf("provenance = %v, want fake:description", desc["winning_source"])
	}
}

// Owner gate (F38 S3): with a token set, studio enrichment controls require it.
func TestStudioEnrichGated(t *testing.T) {
	srv, _, sid := studioEnrichServer(t, "s3cret")
	url := srv.URL + "/api/v1/studios/" + itoa(sid) + "/enrich/resolve"
	if code, _ := postTok(t, url, "", map[string]string{"provider": "fake", "query": "x"}); code != http.StatusUnauthorized {
		t.Errorf("no-token studio resolve = %d, want 401", code)
	}
	if code, _ := postTok(t, url, "s3cret", map[string]string{"provider": "fake", "query": "ghibli"}); code != http.StatusOK {
		t.Errorf("token studio resolve = %d, want 200", code)
	}
}

func postTok(t *testing.T, url, token string, body any) (int, map[string]any) {
	t.Helper()
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// Full owner flow (open gate): resolve → apply → person detail shows provenance.
func TestEnrichFlow(t *testing.T) {
	srv, _, pid := enrichServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid)

	code, body := postTok(t, base+"/enrich/resolve", "", map[string]string{"provider": "fake", "query": "miyazaki"})
	if code != http.StatusOK {
		t.Fatalf("resolve code = %d", code)
	}
	cands, _ := body["candidates"].([]any)
	if len(cands) != 1 {
		t.Fatalf("candidates = %v", body["candidates"])
	}

	code, body = postTok(t, base+"/enrich", "", map[string]string{"provider": "fake", "external_id": "tmdb:608"})
	if code != http.StatusOK {
		t.Fatalf("apply code = %d", code)
	}
	if enriched, _ := body["enriched"].([]any); len(enriched) == 0 {
		t.Fatalf("enriched empty: %v", body["enriched"])
	}

	// Person detail carries the resolved fields with provenance (F37: the raw
	// enriched[] block is retired; resolved[] is the unified payload).
	code, pbody := getJSON(t, base)
	if code != http.StatusOK {
		t.Fatalf("person code = %d", code)
	}
	if _, ok := pbody["enriched"]; ok {
		t.Error("person payload must not carry enriched[] (retired by F37)")
	}
	resolved, _ := pbody["resolved"].([]any)
	var bio map[string]any
	for _, f := range resolved {
		if m, _ := f.(map[string]any); m["canonical"] == "bio" {
			bio = m
		}
	}
	if bio == nil {
		t.Fatalf("resolved bio missing: %v", pbody["resolved"])
	}
	if bio["winning_source"] != "fake:bio" {
		t.Errorf("provenance = %v, want fake:bio", bio["winning_source"])
	}
}

// Owner gate (F22.9a): with a token set, enrichment controls require it.
func TestEnrichGated(t *testing.T) {
	srv, _, pid := enrichServer(t, "s3cret")
	url := srv.URL + "/api/v1/people/" + itoa(pid) + "/enrich/resolve"

	if code, _ := postTok(t, url, "", map[string]string{"provider": "fake", "query": "x"}); code != http.StatusUnauthorized {
		t.Errorf("no-token resolve = %d, want 401", code)
	}
	if code, _ := postTok(t, url, "s3cret", map[string]string{"provider": "fake", "query": "miyazaki"}); code != http.StatusOK {
		t.Errorf("token resolve = %d, want 200", code)
	}
	if code, _ := getJSON(t, srv.URL+"/api/v1/enrich/sources"); code != http.StatusUnauthorized {
		t.Errorf("ungated sources read = %d, want 401 (owner-only)", code)
	}
}
