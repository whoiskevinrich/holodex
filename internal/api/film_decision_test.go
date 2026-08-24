package api_test

import (
	"context"
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

// filmDecisionServer wires a films_enabled server with a "collection" (Album,
// F56's Film field) replace mapping and one video attached to a film — the
// fixture for TestDecisionAPI_FilmSourceMatched below.
func filmDecisionServer(t *testing.T) (*httptest.Server, int64, int64) {
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
	videoID, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/scene.mkv", FileSize: 1, Title: "Scene",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	filmID, err := r.CreateFilm(ctx, "Dune", 2021)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, videoID, nil, false); err != nil {
		t.Fatalf("attach film video: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: collection\n    label: Film\n    sources: [file:Album]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	h.SetAuth(api.NewAuth(""), false)
	h.SetFilmsEnabled(true)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, videoID, filmID
}

// TestDecisionAPI_FilmSourceMatched is a regression guard (F56/ADR-085 §4): a
// film source is injected as synthetic "film:<id>" candidates at read time
// (injectFilmSources) and never persisted to entity_enrichment, so
// providerMatched's default entity_enrichment scan could never find one —
// pinning any film-source decision on a video 400'd with "provider is not
// matched to this item" until providerMatched grew a film_videos-backed
// special case for the "film:" namespace.
func TestDecisionAPI_FilmSourceMatched(t *testing.T) {
	srv, videoID, filmID := filmDecisionServer(t)
	base := srv.URL + "/api/v1/media/" + itoa(videoID) + "/fields/collection/decision"

	source := "provider:film:" + itoa(filmID)
	if code := sendDecision(t, http.MethodPut, base, "", map[string]string{"source": source}); code != 204 {
		t.Fatalf("pin film source: want 204, got %d", code)
	}
	f := resolvedField(t, srv, videoID, "collection")
	if vals, _ := f["values"].([]any); len(vals) != 1 || vals[0] != "Dune" {
		t.Fatalf("resolved collection after film decision: got %v, want [Dune]", f["values"])
	}
	dec := f["decision"].(map[string]any)
	if dec["source"] != source || dec["standing"] != true {
		t.Errorf("decision marker = %v", dec)
	}

	// A film id the video isn't actually attached to must still be rejected.
	if code := sendDecision(t, http.MethodPut, base, "", map[string]string{"source": "provider:film:99999"}); code != 400 {
		t.Fatalf("pin unattached film: want 400, got %d", code)
	}
}
