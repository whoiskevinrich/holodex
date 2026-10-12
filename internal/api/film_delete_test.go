package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/purge"
	"holodex/internal/repo"
)

// filmDeleteServer wires films on with the real purger. One film, "Heat" (1995), is
// seeded with one attached video; their ids are returned.
func filmDeleteServer(t *testing.T, token string) (srv *httptest.Server, r *repo.Repo, filmID, videoID int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r = repo.New(database)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth(token), false)
	h.SetFilmsEnabled(true)
	grace := 7 * 24 * time.Hour
	h.SetDelete(purge.New(r, purge.Config{Grace: grace, FilmImageDir: filepath.Join(dir, "film-images")}, log), grace)
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	if filmID, err = r.CreateFilm(ctx, "Heat", 1995); err != nil {
		t.Fatalf("create film: %v", err)
	}
	if videoID, err = r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/heat.mkv", Title: "Heat scene", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil); err != nil {
		t.Fatalf("seed video: %v", err)
	}
	one := int64(1)
	if _, err := r.AttachFilmVideo(ctx, filmID, videoID, &one, false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	return srv, r, filmID, videoID
}

func TestFilmDeleteEndpointsGated(t *testing.T) {
	srv, _, filmID, _ := filmDeleteServer(t, "s3cret")
	film := srv.URL + "/api/v1/films/" + itoa(filmID)

	if code := sendTok(t, http.MethodDelete, film, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token delete = %d, want 401", code)
	}
	if code := sendTok(t, http.MethodDelete, film+"?purge=true", ""); code != http.StatusUnauthorized {
		t.Errorf("no-token purge = %d, want 401", code)
	}
	if code := sendTok(t, http.MethodPost, film+"/restore", ""); code != http.StatusUnauthorized {
		t.Errorf("no-token restore = %d, want 401", code)
	}
	if code, _ := getJSON(t, film); code != http.StatusOK {
		t.Errorf("film after rejected deletes = %d, want 200", code)
	}
}

// P0-2/P0-5/P0-6: trash → the page 404s and the film sits in Trash labelled as a
// film; restore → back.
func TestFilmTrashRestoreFlow(t *testing.T) {
	srv, _, filmID, videoID := filmDeleteServer(t, "")
	film := srv.URL + "/api/v1/films/" + itoa(filmID)

	if code := sendTok(t, http.MethodDelete, film, ""); code != http.StatusNoContent {
		t.Fatalf("trash film = %d, want 204", code)
	}
	if code, _ := getJSON(t, film); code != http.StatusNotFound {
		t.Errorf("film page after trash = %d, want 404", code)
	}
	if code := sendTok(t, http.MethodDelete, film, ""); code != http.StatusNoContent {
		t.Errorf("second trash = %d, want 204 (idempotent)", code)
	}
	if code, _ := getJSON(t, srv.URL+"/api/v1/media/"+itoa(videoID)); code != http.StatusOK {
		t.Errorf("linked video after film trash = %d, want 200 (RD2)", code)
	}
	if code := sendTok(t, http.MethodPut, film+"/year", ""); code != http.StatusNotFound {
		t.Errorf("year edit on a trashed film = %d, want 404", code)
	}

	_, trash := getJSON(t, srv.URL+"/api/v1/admin/trash")
	items, _ := trash["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("trash = %v, want one film row", trash)
	}
	row, _ := items[0].(map[string]any)
	if row["kind"] != "film" || row["title"] != "Heat (1995)" || row["purge_at"] == nil {
		t.Errorf("film trash row = %v", row)
	}
	if _, hasPath := row["path"]; hasPath {
		t.Errorf("a film row carries no file path: %v", row)
	}

	if code, body := rawRequest(t, http.MethodPost, film+"/restore", nil); code != http.StatusOK || body["film"] == nil {
		t.Errorf("restore = %d %v, want 200 with the film", code, body)
	}
	if code, _ := getJSON(t, film); code != http.StatusOK {
		t.Errorf("film after restore = %d, want 200", code)
	}
	if code := sendTok(t, http.MethodPost, film+"/restore", ""); code != http.StatusNotFound {
		t.Errorf("restore of a live film = %d, want 404", code)
	}
}

// P0-3: purge works on a live film and on one already in Trash; the video stays.
func TestFilmPurgeNowEndpoint(t *testing.T) {
	srv, r, filmID, videoID := filmDeleteServer(t, "")
	film := srv.URL + "/api/v1/films/" + itoa(filmID)

	if code := sendTok(t, http.MethodDelete, film+"?purge=true", ""); code != http.StatusNoContent {
		t.Fatalf("purge film = %d, want 204", code)
	}
	if err := r.FilmExistsAny(context.Background(), filmID); err != repo.ErrNotFound {
		t.Errorf("film row survived purge: %v", err)
	}
	if code, _ := getJSON(t, srv.URL+"/api/v1/media/"+itoa(videoID)); code != http.StatusOK {
		t.Errorf("linked video after film purge = %d, want 200", code)
	}
	if code := sendTok(t, http.MethodDelete, film+"?purge=true", ""); code != http.StatusNotFound {
		t.Errorf("purge of a gone film = %d, want 404", code)
	}
}

// RD6: create and rename onto a trashed film's name and year answer 409 with the
// trashed film under "trashed", so the SPA can offer its restore.
func TestTrashedFilmConflicts(t *testing.T) {
	srv, r, filmID, _ := filmDeleteServer(t, "")
	if code := sendTok(t, http.MethodDelete, srv.URL+"/api/v1/films/"+itoa(filmID), ""); code != http.StatusNoContent {
		t.Fatalf("trash = %d", code)
	}

	code, body := rawRequest(t, http.MethodPost, srv.URL+"/api/v1/films", map[string]any{"name": "Heat", "year": 1995})
	trashed, _ := body["trashed"].(map[string]any)
	if code != http.StatusConflict || trashed == nil || trashed["film_id"] != float64(filmID) || trashed["trashed"] != true {
		t.Errorf("create onto trashed key = %d %v, want 409 naming film %d", code, body, filmID)
	}

	fire, err := r.CreateFilm(context.Background(), "Fire", 1995)
	if err != nil {
		t.Fatal(err)
	}
	code, body = rawRequest(t, http.MethodPost, srv.URL+"/api/v1/films/"+itoa(fire)+"/rename", map[string]any{"name": "Heat"})
	trashed, _ = body["trashed"].(map[string]any)
	if code != http.StatusConflict || trashed == nil || trashed["film_id"] != float64(filmID) {
		t.Errorf("rename onto trashed key = %d %v, want 409 naming film %d", code, body, filmID)
	}
}
