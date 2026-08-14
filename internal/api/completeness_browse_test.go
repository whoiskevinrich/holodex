package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// completenessBrowseServer seeds three videos at three distinct completeness
// levels (title/poster_url/actors/studio, the four Critical video facets) so
// browse sort/filter tests have something to distinguish:
//   - "Bare": title only (file baseline) — lowest score.
//   - "Half": title + studio — mid score, missing poster_url and actors.
//   - "Full": title + studio + poster_url + actors — highest score.
func completenessBrowseServer(t *testing.T, token string) *httptest.Server {
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

	seed := func(path, title string, extra ...model.ExtraMetadata) int64 {
		id, err := r.UpsertVideo(ctx, &model.Video{
			FilePath: path, FileSize: 1, Title: title,
			FileMtime: time.Now().UTC().Truncate(time.Second),
		}, extra)
		if err != nil {
			t.Fatalf("seed video %s: %v", path, err)
		}
		return id
	}
	seed("/m/bare.mkv", "Bare")
	seed("/m/half.mkv", "Half", model.ExtraMetadata{SourceKey: "Publisher", Value: "Acme"})
	fullID := seed("/m/full.mkv", "Full", model.ExtraMetadata{SourceKey: "Publisher", Value: "Acme"})
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, fullID, "tmdb", "ext-1", map[string][]string{
		"poster_url": {"https://cdn.example/poster.jpg"},
		"actors":     {"Someone"},
	}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [file:title]\n" +
		"  - canonical: poster_url\n    label: Poster\n    sources: [tmdb:poster_url]\n" +
		"  - canonical: actors\n    label: Actors\n    merge: true\n    sources: [tmdb:actors]\n" +
		"  - canonical: studio\n    label: Studio\n    sources: [Publisher]\n"
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
	return srv
}

// mediaTitles extracts the ordered titles from a /media list response body.
func mediaTitles(t *testing.T, body map[string]any) []string {
	t.Helper()
	items, _ := body["items"].([]any)
	titles := make([]string, len(items))
	for i, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			t.Fatalf("item %d not an object: %v", i, it)
		}
		titles[i], _ = m["title"].(string)
	}
	return titles
}

// TestListMedia_CompletenessSort_OwnerGated covers the spec's "Access control
// & security" requirement verbatim: score/actionability must not be reachable
// by a non-owner, even via the otherwise-public /media list endpoint.
func TestListMedia_CompletenessSort_OwnerGated(t *testing.T) {
	srv := completenessBrowseServer(t, "secret")

	if code, _ := getJSONTok(t, srv.URL+"/api/v1/media?sort=completeness_desc", ""); code != http.StatusUnauthorized {
		t.Errorf("sort=completeness_desc without token: want 401, got %d", code)
	}
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/media?missing_facet=poster_url", ""); code != http.StatusUnauthorized {
		t.Errorf("missing_facet without token: want 401, got %d", code)
	}
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/media?sort=completeness_desc", "secret"); code != http.StatusOK {
		t.Errorf("sort=completeness_desc with token: want 200, got %d", code)
	}
}

// TestListMedia_CompletenessSort_Orders covers F55.5: completeness_desc/asc
// rank videos by score, highest/lowest first.
func TestListMedia_CompletenessSort_Orders(t *testing.T) {
	srv := completenessBrowseServer(t, "")

	_, body := getJSONTok(t, srv.URL+"/api/v1/media?sort=completeness_desc", "")
	if got, want := mediaTitles(t, body), []string{"Full", "Half", "Bare"}; !slices.Equal(got, want) {
		t.Errorf("completeness_desc order = %v, want %v", got, want)
	}

	_, body = getJSONTok(t, srv.URL+"/api/v1/media?sort=completeness_asc", "")
	if got, want := mediaTitles(t, body), []string{"Bare", "Half", "Full"}; !slices.Equal(got, want) {
		t.Errorf("completeness_asc order = %v, want %v", got, want)
	}
}

// TestListMedia_MissingFacetFilter covers F55.6: missing_facet returns only
// videos currently missing that facet, and multiple selections AND together.
// With no completeness sort requested, order falls back to the default SQL
// order (added_desc — newest-indexed first): Half was seeded after Bare.
func TestListMedia_MissingFacetFilter(t *testing.T) {
	srv := completenessBrowseServer(t, "")

	_, body := getJSONTok(t, srv.URL+"/api/v1/media?missing_facet=poster_url", "")
	if got, want := mediaTitles(t, body), []string{"Half", "Bare"}; !slices.Equal(got, want) {
		t.Errorf("missing poster_url = %v, want %v", got, want)
	}

	_, body = getJSONTok(t, srv.URL+"/api/v1/media?missing_facet=poster_url&missing_facet=studio", "")
	if got, want := mediaTitles(t, body), []string{"Bare"}; !slices.Equal(got, want) {
		t.Errorf("missing poster_url AND studio = %v, want %v", got, want)
	}
}

// TestCompletenessFacets covers F55.6's "counts never disagree" requirement:
// the facet summary's missing_count for poster_url must equal how many videos
// the equivalent missing_facet=poster_url filter returns, and the endpoint is
// owner-gated like the sort/filter params.
func TestCompletenessFacets(t *testing.T) {
	srv := completenessBrowseServer(t, "secret")

	if code, _ := getJSONTok(t, srv.URL+"/api/v1/completeness/facets?entity_type=video", ""); code != http.StatusUnauthorized {
		t.Fatalf("facets without token: want 401, got %d", code)
	}

	code, body := getJSONTok(t, srv.URL+"/api/v1/completeness/facets?entity_type=video", "secret")
	if code != http.StatusOK {
		t.Fatalf("facets with token: want 200, got %d", code)
	}
	facets, _ := body["facets"].([]any)
	var posterMissing float64
	found := false
	for _, f := range facets {
		m, _ := f.(map[string]any)
		if m["canonical"] == "poster_url" {
			posterMissing, found = m["missing_count"].(float64), true
		}
	}
	if !found {
		t.Fatalf("poster_url not in facet summary: %v", facets)
	}
	if posterMissing != 2 {
		t.Errorf("poster_url missing_count = %v, want 2 (Bare, Half)", posterMissing)
	}
}

// TestListPeople_CompletenessSort_OwnerGated and TestListStudios_..._OwnerGated
// cover the same F55.5/F55.6 owner-gate requirement on the other two browse
// endpoints — completenessForVideos' HTTP-level coverage above already
// exercises the shared isMissingAll/sortByScore/summarizeFacets helpers, so
// these stay scoped to the security-critical gating behavior.
func TestListPeople_CompletenessSort_OwnerGated(t *testing.T) {
	srv, r := identityServer(t, "secret")
	vid := seedStudioVideo(t, r, "/m/p.mkv", "Acme")
	linkPeople(t, r, vid, "Hayao Miyazaki")

	if code, _ := getJSONTok(t, srv.URL+"/api/v1/people?sort=completeness_desc", ""); code != http.StatusUnauthorized {
		t.Errorf("sort=completeness_desc without token: want 401, got %d", code)
	}
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/people?missing_facet=photo", ""); code != http.StatusUnauthorized {
		t.Errorf("missing_facet without token: want 401, got %d", code)
	}
	code, body := getJSONTok(t, srv.URL+"/api/v1/people?sort=completeness_desc", "secret")
	if code != http.StatusOK {
		t.Errorf("sort=completeness_desc with token: want 200, got %d", code)
	}
	if items, _ := body["items"].([]any); len(items) != 1 {
		t.Errorf("items = %v, want 1 person", items)
	}
}

func TestListStudios_CompletenessSort_OwnerGated(t *testing.T) {
	srv, r := identityServer(t, "secret")
	seedStudioVideo(t, r, "/m/s.mkv", "Acme")

	if code, _ := getJSONTok(t, srv.URL+"/api/v1/studios?sort=completeness_desc", ""); code != http.StatusUnauthorized {
		t.Errorf("sort=completeness_desc without token: want 401, got %d", code)
	}
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/studios?missing_facet=branding_image", ""); code != http.StatusUnauthorized {
		t.Errorf("missing_facet without token: want 401, got %d", code)
	}
	code, body := getJSONTok(t, srv.URL+"/api/v1/studios?sort=completeness_desc", "secret")
	if code != http.StatusOK {
		t.Errorf("sort=completeness_desc with token: want 200, got %d", code)
	}
	if items, _ := body["items"].([]any); len(items) != 1 {
		t.Errorf("items = %v, want 1 studio", items)
	}
}
