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

// studioEvidenceServer seeds five videos whose studio field reads file:Publisher, then
// provider-1:studio, and relinks them, so two near-miss studios exist:
//
//	v1 file "North Pictures"
//	v2 file "North Pictures", provider-1 "North Pictures"
//	v3                         provider-1 "North Pictures"
//	v4 file "NorthPictures",  provider-1 "North Pictures"   (file wins → the other studio)
//	v5                         provider-1 "North Pictures"
func studioEvidenceServer(t *testing.T) (srv *httptest.Server, r *repo.Repo, videos []int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	r = repo.New(database)
	ctx := context.Background()
	seeds := []struct{ file, provider string }{
		{"North Pictures", ""}, {"North Pictures", "North Pictures"}, {"", "North Pictures"},
		{"NorthPictures", "North Pictures"}, {"", "North Pictures"},
	}
	for i, s := range seeds {
		var extra []model.ExtraMetadata
		if s.file != "" {
			extra = []model.ExtraMetadata{{SourceKey: "Publisher", Value: s.file}}
		}
		id, err := r.UpsertVideo(ctx, &model.Video{
			FilePath: "/m/" + itoa(int64(i)) + ".mkv", FileSize: 1, Title: "V",
			FileMtime: time.Now().UTC().Truncate(time.Second),
		}, extra)
		if err != nil {
			t.Fatal(err)
		}
		if s.provider != "" {
			if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, id, "provider-1", "ext-"+itoa(id), map[string][]string{
				"studio": {s.provider},
			}); err != nil {
				t.Fatal(err)
			}
		}
		videos = append(videos, id)
	}
	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n  - canonical: studio\n    label: Studio\n    sources: [file:Publisher, provider-1:studio]\n"
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
	h.SetAuth(api.NewAuth("s3cret"), false)
	for _, id := range videos {
		if err := h.RelinkVideoStudios(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, videos
}

// TestStudioEvidence: a side lists the sources that carry its own name now, most videos
// first, with up to three posters (one per source, then the newest) captioned by the
// source that won each video, and its aliases with their origin (F70 P0-10, OQ7).
func TestStudioEvidence(t *testing.T) {
	srv, r, v := studioEvidenceServer(t)
	ctx := context.Background()
	studios, err := r.ListStudios(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, s := range studios {
		ids[s.Name] = s.ID
	}
	spaced, joined := ids["North Pictures"], ids["NorthPictures"]
	if spaced == 0 || joined == 0 {
		t.Fatalf("studios = %+v, want both spellings", studios)
	}
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityStudio, spaced, "N Pictures"); err != nil {
		t.Fatal(err)
	}
	url := srv.URL + "/api/v1/owner/duplicates/studios/"

	if code := sendTok(t, http.MethodGet, url+itoa(spaced), ""); code != http.StatusUnauthorized {
		t.Errorf("no-token evidence = %d, want 401", code)
	}
	if code, _ := getJSONTok(t, url+"999999", "s3cret"); code != http.StatusNotFound {
		t.Errorf("missing studio = %d, want 404", code)
	}

	code, body := getJSONTok(t, url+itoa(spaced), "s3cret")
	if code != http.StatusOK {
		t.Fatalf("evidence = %d, want 200", code)
	}
	if num(body, "video_count") != 4 {
		t.Errorf("video_count = %v, want 4", body["video_count"])
	}
	wantSources := []string{"provider-1:3", "file:2"}
	if got := sourcesOf(body); !slices.Equal(got, wantSources) {
		t.Errorf("sources = %v, want %v", got, wantSources)
	}
	// One poster per source (provider-1's newest is v5, file's is v2), then the newest left.
	wantPosters := []string{itoa(v[4]) + ":provider-1", itoa(v[1]) + ":file", itoa(v[2]) + ":provider-1"}
	if got := postersOf(body); !slices.Equal(got, wantPosters) {
		t.Errorf("posters = %v, want %v", got, wantPosters)
	}
	aliases, _ := body["aliases"].([]any)
	// An owner alias carries no source (omitted, as everywhere aliases are served).
	if len(aliases) != 1 || aliases[0].(map[string]any)["source"] != nil {
		t.Errorf("aliases = %v, want one owner alias", body["aliases"])
	}
	if links, ok := body["external_links"].([]any); !ok || len(links) != 0 {
		t.Errorf("external_links = %v, want []", body["external_links"])
	}

	// The joined spelling comes from the file tags alone: provider-1 names v4 too, but
	// with the other spelling, so it does not count here.
	_, body = getJSONTok(t, url+itoa(joined), "s3cret")
	if got := sourcesOf(body); !slices.Equal(got, []string{"file:1"}) {
		t.Errorf("joined sources = %v, want [file:1]", got)
	}
	if got := postersOf(body); !slices.Equal(got, []string{itoa(v[3]) + ":file"}) {
		t.Errorf("joined posters = %v", got)
	}
	if aliases, _ := body["aliases"].([]any); aliases == nil || len(aliases) != 0 {
		t.Errorf("joined aliases = %v, want []", body["aliases"])
	}
}

func sourcesOf(body map[string]any) []string {
	var out []string
	for _, s := range body["sources"].([]any) {
		m := s.(map[string]any)
		out = append(out, m["source"].(string)+":"+itoa(int64(m["video_count"].(float64))))
	}
	return out
}

func postersOf(body map[string]any) []string {
	var out []string
	for _, p := range body["posters"].([]any) {
		m := p.(map[string]any)
		out = append(out, itoa(int64(m["video_id"].(float64)))+":"+m["source"].(string))
	}
	return out
}
