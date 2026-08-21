package api_test

import (
	"context"
	"database/sql"
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

// filmInjectionServer wires a real httptest server with a "collection"/"title" field
// mapping and returns the underlying Handlers (so films_enabled can be flipped between
// requests against the same server) alongside the repo and raw *sql.DB (film_videos has
// no repo/API write path yet -- seeded directly, mirroring film_links_test.go).
func filmInjectionServer(t *testing.T) (srv *httptest.Server, h *api.Handlers, r *repo.Repo, sqlDB *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	sqlDB, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	r = repo.New(sqlDB)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: collection\n    label: Film\n    sources: [file:Album]\n" +
		"  - canonical: title\n    sources: [file:title]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	h = api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, h, r, sqlDB
}

func seedFilm(t *testing.T, sqlDB *sql.DB, name string) int64 {
	t.Helper()
	res, err := sqlDB.ExecContext(context.Background(), `INSERT INTO films (name, year) VALUES (?, 2020)`, name)
	if err != nil {
		t.Fatalf("seed film: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("film id: %v", err)
	}
	return id
}

func attachFilmVideo(t *testing.T, sqlDB *sql.DB, filmID, videoID int64, isFullFilm bool) {
	t.Helper()
	full := 0
	if isFullFilm {
		full = 1
	}
	if _, err := sqlDB.ExecContext(context.Background(),
		`INSERT INTO film_videos (film_id, video_id, scene_number, is_full_film, created_at) VALUES (?, ?, NULL, ?, ?)`,
		filmID, videoID, full, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		t.Fatalf("attach film video: %v", err)
	}
}

func resolvedValue(t *testing.T, body map[string]any, canonical string) (string, bool) {
	t.Helper()
	rows, _ := body["resolved"].([]any)
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok || row["canonical"] != canonical {
			continue
		}
		values, _ := row["values"].([]any)
		if len(values) == 0 {
			return "", true
		}
		s, _ := values[0].(string)
		return s, true
	}
	return "", false
}

// TestFilmSourceInjection_ScenevsFullFilm proves the getMedia call-site injection
// (F56, ADR-085 §4) end-to-end: a decided film source resolves "collection" for every
// attachment and "title" only when the file represents the entire film.
func TestFilmSourceInjection_SceneVsFullFilm(t *testing.T) {
	srv, h, r, sqlDB := filmInjectionServer(t)
	h.SetFilmsEnabled(true)
	ctx := context.Background()

	sceneVid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/scene.mkv", FileSize: 1, Title: "Scene Baseline Title",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed scene video: %v", err)
	}
	fullVid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/full.mkv", FileSize: 1, Title: "Full Baseline Title",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed full-film video: %v", err)
	}

	filmID := seedFilm(t, sqlDB, "Test Film")
	attachFilmVideo(t, sqlDB, filmID, sceneVid, false)
	attachFilmVideo(t, sqlDB, filmID, fullVid, true)

	filmSource := "provider:film:" + itoa(filmID)
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, sceneVid, "collection", filmSource, ""); err != nil {
		t.Fatalf("decide scene collection: %v", err)
	}
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, fullVid, "collection", filmSource, ""); err != nil {
		t.Fatalf("decide full collection: %v", err)
	}
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, fullVid, "title", filmSource, ""); err != nil {
		t.Fatalf("decide full title: %v", err)
	}

	// Scene file: collection resolves to the film; title is untouched (no decision was
	// made on it), so it keeps the file baseline -- the film source never offered a
	// title candidate for a non-full-film attachment.
	_, body := getJSON(t, srv.URL+"/api/v1/media/"+itoa(sceneVid))
	if got, ok := resolvedValue(t, body, "collection"); !ok || got != "Test Film" {
		t.Errorf("scene collection = %q (ok=%v), want %q", got, ok, "Test Film")
	}
	if got, ok := resolvedValue(t, body, "title"); !ok || got != "Scene Baseline Title" {
		t.Errorf("scene title = %q (ok=%v), want baseline %q", got, ok, "Scene Baseline Title")
	}

	// Full-film file: both collection and title resolve to the film.
	_, body = getJSON(t, srv.URL+"/api/v1/media/"+itoa(fullVid))
	if got, ok := resolvedValue(t, body, "collection"); !ok || got != "Test Film" {
		t.Errorf("full collection = %q (ok=%v), want %q", got, ok, "Test Film")
	}
	if got, ok := resolvedValue(t, body, "title"); !ok || got != "Test Film" {
		t.Errorf("full title = %q (ok=%v), want %q", got, ok, "Test Film")
	}

	// films_enabled=false suspends the source (ADR-085 §5): the decision still points
	// at the film, but with no candidate injected it resolves to no value at all --
	// it must NOT silently fall back to the file baseline.
	h.SetFilmsEnabled(false)
	_, body = getJSON(t, srv.URL+"/api/v1/media/"+itoa(sceneVid))
	if got, ok := resolvedValue(t, body, "collection"); ok && got != "" {
		t.Errorf("suspended scene collection = %q, want empty (not silently reverted to file)", got)
	}
	_, body = getJSON(t, srv.URL+"/api/v1/media/"+itoa(fullVid))
	if got, ok := resolvedValue(t, body, "title"); ok && got != "" {
		t.Errorf("suspended full title = %q, want empty (not silently reverted to file)", got)
	}
}
