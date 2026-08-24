package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"holodex/internal/repo"
)

// filmEntityServer wires a real repo + Handlers with films_enabled on, exposing
// the raw *sql.DB so tests can seed video_studios/video_tags/video_people
// directly -- those junction tables are DERIVED (RelinkVideoEntity, ADR-053/072)
// so seeding them straight avoids standing up the full mapping/enrichment
// machinery just to test the films-by-entity filter and video-candidates picker.
func filmEntityServer(t *testing.T) (*httptest.Server, *repo.Repo, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth("tok"), false)
	h.SetFilmsEnabled(true)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, database
}

func seedPlainVideo(t *testing.T, r *repo.Repo, title string) int64 {
	t.Helper()
	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/" + title + ".mkv", FileSize: 1, Title: title,
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video %s: %v", title, err)
	}
	return id
}

func seedStudio(t *testing.T, sqlDB *sql.DB, videoID int64, name string) int64 {
	t.Helper()
	res, err := sqlDB.Exec(`INSERT INTO studios (name) VALUES (?)`, name)
	if err != nil {
		t.Fatalf("seed studio: %v", err)
	}
	sid, _ := res.LastInsertId()
	if _, err := sqlDB.Exec(`INSERT INTO video_studios (video_id, studio_id) VALUES (?, ?)`, videoID, sid); err != nil {
		t.Fatalf("link video studio: %v", err)
	}
	return sid
}

func decodeFilmItems(t *testing.T, resp *http.Response) []model.Film {
	t.Helper()
	defer resp.Body.Close()
	var body struct {
		Items []model.Film `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode films list: %v", err)
	}
	return body.Items
}

// TestListFilmsForEntity covers GET /films?studio_id= (F56): a film matches when
// ANY of its attached videos carries the studio, and is absent from an unrelated
// studio's filter.
func TestListFilmsForEntity(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	ctx := context.Background()

	v1 := seedPlainVideo(t, r, "scene-with-studio")
	studioID := seedStudio(t, sqlDB, v1, "Acme Films")

	filmID, err := r.CreateFilm(ctx, "Studio Filter Test", 2023)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, v1, nil, false); err != nil {
		t.Fatalf("attach video: %v", err)
	}

	matched, err := http.Get(srv.URL + "/api/v1/films?studio_id=" + itoa(studioID))
	if err != nil {
		t.Fatalf("get films by studio: %v", err)
	}
	items := decodeFilmItems(t, matched)
	if len(items) != 1 || items[0].ID != filmID {
		t.Fatalf("films for matching studio: got %+v, want [film %d]", items, filmID)
	}

	missed, err := http.Get(srv.URL + "/api/v1/films?studio_id=999999")
	if err != nil {
		t.Fatalf("get films by unrelated studio: %v", err)
	}
	if items := decodeFilmItems(t, missed); len(items) != 0 {
		t.Fatalf("films for unrelated studio: got %+v, want none", items)
	}
}

// TestFilmVideoCandidates covers GET /films/{id}/video-candidates (F56, design
// handoff §4): default scope excludes videos already attached to ANY film;
// ?unattached=false includes them and flags already_attached; a video already
// attached to the film being edited is excluded in both scopes.
func TestFilmVideoCandidates(t *testing.T) {
	srv, r, _ := filmEntityServer(t)
	ctx := context.Background()

	unattached := seedPlainVideo(t, r, "never-attached")
	elsewhere := seedPlainVideo(t, r, "attached-elsewhere")
	ownScene := seedPlainVideo(t, r, "already-in-this-film")

	filmID, err := r.CreateFilm(ctx, "Candidates Test", 2024)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	otherFilmID, err := r.CreateFilm(ctx, "Other Film", 2024)
	if err != nil {
		t.Fatalf("create other film: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, otherFilmID, elsewhere, nil, false); err != nil {
		t.Fatalf("attach elsewhere: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, ownScene, nil, false); err != nil {
		t.Fatalf("attach own scene: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/films/"+itoa(filmID)+"/video-candidates", nil)
	req.Header.Set(api.AdminTokenHeader, "tok")
	defaultResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get video candidates: %v", err)
	}
	defer defaultResp.Body.Close()
	var defaultBody struct {
		Items []struct {
			Video           model.Video           `json:"video"`
			AlreadyAttached []repo.FilmAttachment `json:"already_attached"`
		} `json:"items"`
	}
	if err := json.NewDecoder(defaultResp.Body).Decode(&defaultBody); err != nil {
		t.Fatalf("decode default candidates: %v", err)
	}
	if len(defaultBody.Items) != 1 || defaultBody.Items[0].Video.ID != unattached {
		t.Fatalf("default-scope candidates: got %+v, want only video %d", defaultBody.Items, unattached)
	}

	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/films/"+itoa(filmID)+"/video-candidates?unattached=false", nil)
	req2.Header.Set(api.AdminTokenHeader, "tok")
	allResp, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("get all candidates: %v", err)
	}
	defer allResp.Body.Close()
	var allBody struct {
		Items []struct {
			Video           model.Video           `json:"video"`
			AlreadyAttached []repo.FilmAttachment `json:"already_attached"`
		} `json:"items"`
	}
	if err := json.NewDecoder(allResp.Body).Decode(&allBody); err != nil {
		t.Fatalf("decode all candidates: %v", err)
	}
	// ownScene must still be excluded (it's already attached to *this* film).
	byID := map[int64][]repo.FilmAttachment{}
	for _, it := range allBody.Items {
		byID[it.Video.ID] = it.AlreadyAttached
	}
	if _, present := byID[ownScene]; present {
		t.Fatalf("video already attached to this film must be excluded: got items %+v", allBody.Items)
	}
	if len(allBody.Items) != 2 {
		t.Fatalf("unattached=false candidates: got %d items, want 2 (unattached + elsewhere)", len(allBody.Items))
	}
	attached, ok := byID[elsewhere]
	if !ok || len(attached) != 1 || attached[0].FilmID != otherFilmID {
		t.Fatalf("already_attached for elsewhere video: got %+v", attached)
	}
}
