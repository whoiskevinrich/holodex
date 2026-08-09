package api_test

import (
	"context"
	"io"
	"log/slog"
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

// facetMap indexes a decoded completeness["facets"] array by canonical, for
// concise per-facet assertions below.
func facetMap(t *testing.T, completeness map[string]any) map[string]map[string]any {
	t.Helper()
	facets, _ := completeness["facets"].([]any)
	out := make(map[string]map[string]any, len(facets))
	for _, f := range facets {
		fm, _ := f.(map[string]any)
		out[fm["canonical"].(string)] = fm
	}
	return out
}

// TestGetMedia_Completeness covers F55.13: getMedia's owner-gated completeness
// field scores the same resolve pass the response's own resolved[] came from
// (title curated from file baseline, poster_url left missing since no
// provider/mapping value was seeded), and is omitted entirely for a visitor.
func TestGetMedia_Completeness(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "A",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [file:title]\n" +
		"  - canonical: poster_url\n    label: Poster\n    sources: [tmdb:poster_url]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	// Owner (open gate, no SetAuth call): completeness present with the
	// expected per-facet tiers.
	_, body := getJSON(t, srv.URL+"/api/v1/media/"+itoa(id))
	completeness, ok := body["completeness"].(map[string]any)
	if !ok {
		t.Fatalf("completeness missing or wrong shape: %v", body["completeness"])
	}
	facets := facetMap(t, completeness)
	if f := facets["title"]; f == nil || f["tier"] != "curated" {
		t.Errorf("title facet = %v, want curated", f)
	}
	if f := facets["poster_url"]; f == nil || f["tier"] != "missing" {
		t.Errorf("poster_url facet = %v, want missing", f)
	}

	// Visitor: completeness must be entirely absent, mirroring enrich_queries.
	h.SetAuth(api.NewAuth("secret"), false)
	_, body = getJSON(t, srv.URL+"/api/v1/media/"+itoa(id))
	if v, present := body["completeness"]; present && v != nil {
		t.Errorf("completeness should be omitted/null for an unauthenticated request, got %v", v)
	}
}

// TestGetPerson_Completeness covers F55.13's asset-facet injection on the
// single-entity path: photo has no row in the normal resolve pipeline, so
// getPerson must synthesize it (missing here — no headshot uploaded), the
// same signal completenessForPeople uses.
func TestGetPerson_Completeness(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	ctx := context.Background()

	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/p.mkv", Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Alice"}},
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	linkPeople(t, r, vid, "Alice")
	pid, _, err := r.PersonIDByName(ctx, "Alice")
	if err != nil {
		t.Fatalf("lookup person: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	_, body := getJSON(t, srv.URL+"/api/v1/people/"+itoa(pid))
	completeness, ok := body["completeness"].(map[string]any)
	if !ok {
		t.Fatalf("completeness missing or wrong shape: %v", body["completeness"])
	}
	facets := facetMap(t, completeness)
	if f := facets["photo"]; f == nil || f["tier"] != "missing" {
		t.Errorf("photo facet = %v, want missing (no headshot uploaded)", f)
	}

	h.SetAuth(api.NewAuth("secret"), false)
	_, body = getJSON(t, srv.URL+"/api/v1/people/"+itoa(pid))
	if v, present := body["completeness"]; present && v != nil {
		t.Errorf("completeness should be omitted/null for an unauthenticated request, got %v", v)
	}
}

// TestGetStudio_Completeness mirrors TestGetPerson_Completeness for the
// branding_image asset facet (F55.13/F55.15).
func TestGetStudio_Completeness(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	ctx := context.Background()

	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/s.mkv", Title: "Clip",
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
	sid := studios[0].ID

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	_, body := getJSON(t, srv.URL+"/api/v1/studios/"+itoa(sid))
	completeness, ok := body["completeness"].(map[string]any)
	if !ok {
		t.Fatalf("completeness missing or wrong shape: %v", body["completeness"])
	}
	facets := facetMap(t, completeness)
	if f := facets["branding_image"]; f == nil || f["tier"] != "missing" {
		t.Errorf("branding_image facet = %v, want missing (no image uploaded)", f)
	}

	h.SetAuth(api.NewAuth("secret"), false)
	_, body = getJSON(t, srv.URL+"/api/v1/studios/"+itoa(sid))
	if v, present := body["completeness"]; present && v != nil {
		t.Errorf("completeness should be omitted/null for an unauthenticated request, got %v", v)
	}
}
