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

type actorsItem struct {
	Value   string   `json:"value"`
	Sources []string `json:"sources"`
}

// personAliasServer seeds a video linked to "John Doe" (alias "Johnny D") whose
// tvdb credits read ["Johnny D", "Jane Roe"] and tmdb credits ["John Doe"]. The alias
// source is first in the mapping, so the alias is the first-seen spelling.
func personAliasServer(t *testing.T) (srv *httptest.Server, r *repo.Repo, vid int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r = repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	vid, err = r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "A", Container: "Matroska",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: actors\n    label: Cast\n    multi: true\n    sources: [tvdb:actors, tmdb:actors]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	if err := r.ReconcileVideoPeople(ctx, vid, []repo.PersonRoleName{{Name: "John Doe", Role: "actor"}}, nil); err != nil {
		t.Fatalf("link person: %v", err)
	}
	people, err := r.PeopleForVideos(ctx, []int64{vid})
	if err != nil || len(people[vid]) != 1 {
		t.Fatalf("linked people = %v, err %v", people[vid], err)
	}
	if _, err := r.AddPersonAlias(ctx, people[vid][0].ID, "Johnny D"); err != nil {
		t.Fatalf("add alias: %v", err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, vid, "tvdb", "ext-1", map[string][]string{
		"actors": {"Johnny D", "Jane Roe"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, vid, "tmdb", "ext-2", map[string][]string{
		"actors": {"John Doe"},
	}); err != nil {
		t.Fatal(err)
	}
	return srv, r, vid
}

// fetchActors returns the media detail's actors row (values, items), or ok=false.
func fetchActors(t *testing.T, srv *httptest.Server, vid int64) ([]string, []actorsItem, bool) {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/v1/media/" + itoa(vid))
	if err != nil {
		t.Fatalf("GET media: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Resolved []struct {
			Canonical string       `json:"canonical"`
			Values    []string     `json:"values"`
			Items     []actorsItem `json:"items"`
		} `json:"resolved"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode media: %v", err)
	}
	for _, f := range body.Resolved {
		if f.Canonical == "actors" {
			return f.Values, f.Items, true
		}
	}
	return nil, nil, false
}

// HOLODEX-554: a credit that is an alias of a person linked to the video folds into
// that person's canonical name in the detail's actors row (what the writeback dialog
// shows) — never listed beside it.
func TestGetMedia_PersonAliasCollapsesIntoLinkedPerson(t *testing.T) {
	srv, _, vid := personAliasServer(t)
	values, items, ok := fetchActors(t, srv, vid)
	if !ok {
		t.Fatalf("actors row missing")
	}
	if !slices.Equal(values, []string{"John Doe", "Jane Roe"}) {
		t.Fatalf("actors values = %v, want [John Doe Jane Roe] — the alias must not be listed beside its person", values)
	}
	if got := items[0].Sources; !slices.Equal(got, []string{"tvdb", "tmdb"}) {
		t.Errorf("collapsed item sources = %v, want [tvdb tmdb]", got)
	}
}

// HOLODEX-555: removing a person from the cast suppresses every spelling of them, so
// the alias can't bring them back; clearing that removal restores them.
func TestCuration_SuppressPersonCoversAliases(t *testing.T) {
	srv, r, vid := personAliasServer(t)
	ctx := context.Background()
	curation := srv.URL + "/api/v1/media/" + itoa(vid) + "/curation"
	suppress := map[string]any{"field": "actors", "value": "John Doe", "action": "suppress"}

	if code := sendDecision(t, http.MethodPost, curation, "", suppress); code != http.StatusNoContent {
		t.Fatalf("suppress: want 204, got %d", code)
	}
	if values, _, _ := fetchActors(t, srv, vid); !slices.Equal(values, []string{"Jane Roe"}) {
		t.Fatalf("actors after removing John Doe = %v, want [Jane Roe] — the alias brought him back", values)
	}
	people, err := r.PeopleForVideos(ctx, []int64{vid})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range people[vid] {
		if p.Name == "John Doe" {
			t.Fatalf("John Doe still linked after removal: %v", people[vid])
		}
	}

	if code := sendDecision(t, http.MethodPost, curation+"/clear", "", suppress); code != http.StatusNoContent {
		t.Fatalf("clear: want 204, got %d", code)
	}
	if values, _, _ := fetchActors(t, srv, vid); !slices.Equal(values, []string{"John Doe", "Jane Roe"}) {
		t.Fatalf("actors after clearing the removal = %v, want [John Doe Jane Roe]", values)
	}
}
