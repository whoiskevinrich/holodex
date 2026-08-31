package api_test

import (
	"bytes"
	"context"
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

// filmServer wires a real repo with films_enabled on and two seeded videos, for
// exercising the film API layer end-to-end (F56, ADR-085).
func filmServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64, int64) {
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
	v1, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/scene1.mkv", FileSize: 1, Title: "Scene 1",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video 1: %v", err)
	}
	v2, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/scene2.mkv", FileSize: 1, Title: "Scene 2",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video 2: %v", err)
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth(token), false)
	h.SetFilmsEnabled(true)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, v1, v2
}

func filmPost(t *testing.T, srv *httptest.Server, token, path string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1"+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do post %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func filmDelete(t *testing.T, srv *httptest.Server, token, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1"+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do delete %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// TestGetFilm_RedactsFileMetadataForVisitor guards against a regression found
// in security review: GET /films/{id} is public (no requireOwner), so its
// scenes/full_films arrays -- []repo.FilmVideo wrapping model.Video -- must
// redact file_path the same way every other []model.Video-serializing handler
// does (redactFileMetadataForVisitor(s), handlers.go), or an unauthenticated
// caller learns the server's absolute on-disk file layout.
func TestGetFilm_RedactsFileMetadataForVisitor(t *testing.T) {
	srv, r, v1, _ := filmServer(t, "tok")
	filmID, err := r.CreateFilm(t.Context(), "Redaction Test", 2024)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	if _, err := r.AttachFilmVideo(t.Context(), filmID, v1, nil, false); err != nil {
		t.Fatalf("attach video: %v", err)
	}

	type filmDetail struct {
		Scenes []struct {
			Video struct {
				FilePath string `json:"file_path"`
			} `json:"video"`
		} `json:"scenes"`
	}

	visitorResp, err := http.Get(srv.URL + "/api/v1/films/" + itoa(filmID))
	if err != nil {
		t.Fatalf("get film as visitor: %v", err)
	}
	defer visitorResp.Body.Close()
	var visitorBody filmDetail
	if err := json.NewDecoder(visitorResp.Body).Decode(&visitorBody); err != nil {
		t.Fatalf("decode visitor film detail: %v", err)
	}
	if len(visitorBody.Scenes) != 1 || visitorBody.Scenes[0].Video.FilePath != "" {
		t.Fatalf("visitor scenes: got %+v, want file_path redacted (empty)", visitorBody.Scenes)
	}

	ownerReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/films/"+itoa(filmID), nil)
	ownerReq.Header.Set(api.AdminTokenHeader, "tok")
	ownerResp, err := http.DefaultClient.Do(ownerReq)
	if err != nil {
		t.Fatalf("get film as owner: %v", err)
	}
	defer ownerResp.Body.Close()
	var ownerBody filmDetail
	if err := json.NewDecoder(ownerResp.Body).Decode(&ownerBody); err != nil {
		t.Fatalf("decode owner film detail: %v", err)
	}
	if len(ownerBody.Scenes) != 1 || ownerBody.Scenes[0].Video.FilePath != "/m/scene1.mkv" {
		t.Fatalf("owner scenes: got %+v, want unredacted file_path", ownerBody.Scenes)
	}
}

// TestFilmsDisabled_RoutesUnregistered confirms films_enabled=false doesn't just
// hide films -- the routes don't exist at all (404, not 403), per spec.
func TestFilmsDisabled_RoutesUnregistered(t *testing.T) {
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
	// films_enabled left at its zero value (false) -- SetFilmsEnabled not called.
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/api/v1/films")
	if err != nil {
		t.Fatalf("get films: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /films with films_enabled=false: got %d, want 404", resp.StatusCode)
	}
}

// TestCreateFilm_GetOrCreate covers the create endpoint's 201-then-200
// get-or-create semantics (films.go's createFilm doc comment).
func TestCreateFilm_GetOrCreate(t *testing.T) {
	srv, _, _, _ := filmServer(t, "tok")

	resp := filmPost(t, srv, "tok", "/films", map[string]any{"name": "The Sun", "year": 2020})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create film: got %d, want 201", resp.StatusCode)
	}
	var created struct {
		Film model.Film `json:"film"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Film.Name != "The Sun" || created.Film.Year != 2020 {
		t.Fatalf("unexpected created film: %+v", created.Film)
	}

	// Re-submitting the same name+year is get-or-create: 200 with the existing film.
	dupResp := filmPost(t, srv, "tok", "/films", map[string]any{"name": "The Sun", "year": 2020})
	if dupResp.StatusCode != http.StatusOK {
		t.Fatalf("duplicate create film: got %d, want 200", dupResp.StatusCode)
	}
	var dup struct {
		Film model.Film `json:"film"`
	}
	if err := json.NewDecoder(dupResp.Body).Decode(&dup); err != nil {
		t.Fatalf("decode dup response: %v", err)
	}
	if dup.Film.ID != created.Film.ID {
		t.Fatalf("duplicate create returned a different film: got id %d, want %d", dup.Film.ID, created.Film.ID)
	}

	// Missing name is a 400.
	badResp := filmPost(t, srv, "tok", "/films", map[string]any{"year": 2020})
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create film with no name: got %d, want 400", badResp.StatusCode)
	}

	// Unauthenticated create is rejected.
	unauthResp := filmPost(t, srv, "", "/films", map[string]any{"name": "Nope"})
	if unauthResp.StatusCode != http.StatusUnauthorized && unauthResp.StatusCode != http.StatusForbidden {
		t.Fatalf("unauthenticated create film: got %d, want 401/403", unauthResp.StatusCode)
	}
}

// TestFilmVideoAttachDetach covers the attach/detach happy path and getFilm's
// scene/full-film split, cast/tags/studios (empty here since the seeded videos
// carry none), and list/search including a zero-attachment film.
func TestFilmVideoAttachDetach(t *testing.T) {
	srv, r, v1, v2 := filmServer(t, "tok")
	ctx := context.Background()

	id, err := r.CreateFilm(ctx, "Attach Test", 2021)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}

	scene := int64(1)
	attachResp := filmPost(t, srv, "tok", "/films/"+itoa(id)+"/videos", map[string]any{
		"video_id": v1, "scene_number": scene, "is_full_film": false,
	})
	if attachResp.StatusCode != http.StatusNoContent {
		t.Fatalf("attach scene: got %d, want 204", attachResp.StatusCode)
	}
	fullResp := filmPost(t, srv, "tok", "/films/"+itoa(id)+"/videos", map[string]any{
		"video_id": v2, "is_full_film": true,
	})
	if fullResp.StatusCode != http.StatusNoContent {
		t.Fatalf("attach full film: got %d, want 204", fullResp.StatusCode)
	}

	getResp, err := http.Get(srv.URL + "/api/v1/films/" + itoa(id))
	if err != nil {
		t.Fatalf("get film: %v", err)
	}
	defer getResp.Body.Close()
	var detail struct {
		Film      model.Film       `json:"film"`
		Scenes    []repo.FilmVideo `json:"scenes"`
		FullFilms []repo.FilmVideo `json:"full_films"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode film detail: %v", err)
	}
	if len(detail.Scenes) != 1 || detail.Scenes[0].Video.ID != v1 {
		t.Fatalf("unexpected scenes: %+v", detail.Scenes)
	}
	if len(detail.FullFilms) != 1 || detail.FullFilms[0].Video.ID != v2 {
		t.Fatalf("unexpected full films: %+v", detail.FullFilms)
	}

	// Re-attaching the same pair is a 409 (non-idempotent by design).
	reAttach := filmPost(t, srv, "tok", "/films/"+itoa(id)+"/videos", map[string]any{"video_id": v1})
	if reAttach.StatusCode != http.StatusConflict {
		t.Fatalf("re-attach same pair: got %d, want 409", reAttach.StatusCode)
	}

	detachResp := filmDelete(t, srv, "tok", "/films/"+itoa(id)+"/videos/"+itoa(v1))
	if detachResp.StatusCode != http.StatusNoContent {
		t.Fatalf("detach: got %d, want 204", detachResp.StatusCode)
	}
	// Detaching an already-detached pair is a 404 (not idempotent).
	reDetach := filmDelete(t, srv, "tok", "/films/"+itoa(id)+"/videos/"+itoa(v1))
	if reDetach.StatusCode != http.StatusNotFound {
		t.Fatalf("re-detach: got %d, want 404", reDetach.StatusCode)
	}
}

// TestFilmVideoSceneCollision covers the 409 collision response naming the
// current occupant, for both the single-attach and bulk-attach paths.
func TestFilmVideoSceneCollision(t *testing.T) {
	srv, r, v1, v2 := filmServer(t, "tok")
	ctx := context.Background()
	v3, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/scene3.mkv", FileSize: 1, Title: "Scene 3",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video 3: %v", err)
	}

	id, err := r.CreateFilm(ctx, "Collision Test", 2022)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}

	scene := int64(5)
	attachResp := filmPost(t, srv, "tok", "/films/"+itoa(id)+"/videos", map[string]any{
		"video_id": v1, "scene_number": scene,
	})
	if attachResp.StatusCode != http.StatusNoContent {
		t.Fatalf("attach scene 5: got %d, want 204", attachResp.StatusCode)
	}

	collideResp := filmPost(t, srv, "tok", "/films/"+itoa(id)+"/videos", map[string]any{
		"video_id": v2, "scene_number": scene,
	})
	if collideResp.StatusCode != http.StatusConflict {
		t.Fatalf("colliding attach: got %d, want 409", collideResp.StatusCode)
	}
	var conflict struct {
		Conflict repo.FilmSceneCollision `json:"conflict"`
	}
	if err := json.NewDecoder(collideResp.Body).Decode(&conflict); err != nil {
		t.Fatalf("decode conflict: %v", err)
	}
	if conflict.Conflict.VideoID != v1 {
		t.Fatalf("collision occupant: got video %d, want %d", conflict.Conflict.VideoID, v1)
	}

	// Bulk-attach starting at scene 5 collides on the first video -- all-or-nothing,
	// so v3 must NOT end up attached either.
	bulkResp := filmPost(t, srv, "tok", "/films/"+itoa(id)+"/videos/bulk", map[string]any{
		"video_ids": []int64{v2, v3}, "starting_scene_number": scene,
	})
	if bulkResp.StatusCode != http.StatusConflict {
		t.Fatalf("bulk attach collision: got %d, want 409", bulkResp.StatusCode)
	}
	fvs, err := r.FilmVideos(ctx, id)
	if err != nil {
		t.Fatalf("film videos: %v", err)
	}
	if len(fvs) != 1 {
		t.Fatalf("bulk attach should have rolled back entirely: got %d attachments, want 1", len(fvs))
	}
}

// TestBulkAttachFilmVideosUnnumbered covers the omitted-starting_scene_number case
// (design handoff §4c): the whole selection attaches unnumbered instead of a 400.
func TestBulkAttachFilmVideosUnnumbered(t *testing.T) {
	srv, r, v1, v2 := filmServer(t, "tok")
	ctx := context.Background()

	id, err := r.CreateFilm(ctx, "Unnumbered Bulk Test", 2022)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}

	resp := filmPost(t, srv, "tok", "/films/"+itoa(id)+"/videos/bulk", map[string]any{
		"video_ids": []int64{v1, v2},
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("bulk attach without starting_scene_number: got %d, want 204", resp.StatusCode)
	}
	fvs, err := r.FilmVideos(ctx, id)
	if err != nil {
		t.Fatalf("film videos: %v", err)
	}
	if len(fvs) != 2 {
		t.Fatalf("film videos = %+v, want 2", fvs)
	}
	for _, fv := range fvs {
		if fv.SceneNumber != nil {
			t.Errorf("scene number = %v, want nil (unnumbered)", *fv.SceneNumber)
		}
	}
}
