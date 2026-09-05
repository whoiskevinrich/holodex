package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// filmImageServer wires a films-enabled surface with self-hosted image storage
// (F56/HOLODEX-280, ADR-086) and returns the film id + the running server. tinyJPEG
// and uploadStudioImage's multipart-building style are shared from
// studio_images_test.go (same package).
func filmImageServer(t *testing.T, token string) (srv *httptest.Server, r *repo.Repo, fid int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r = repo.New(database)
	ctx := context.Background()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetFilmsEnabled(true)
	h.SetFilmImages(filepath.Join(dir, "film-images"), 5<<20, 1000)
	h.SetAuth(api.NewAuth(token), false)
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	fid, err = r.CreateFilm(ctx, "Spirited Away", 2001)
	if err != nil {
		t.Fatalf("seed film: %v", err)
	}
	return srv, r, fid
}

// uploadFilmImage POSTs a multipart image for a role and returns the response code.
func uploadFilmImage(t *testing.T, srv *httptest.Server, token string, fid int64, role string, raw []byte) int {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("image", "img.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(raw); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/films/"+itoa(fid)+"/images/"+role, &body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do upload: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// TestFilmImage_UploadServeDelete is the owner happy path for both roles: an upload
// normalizes and stores the bytes, the serve route streams them with an immutable
// cache, and delete removes the slot.
func TestFilmImage_UploadServeDelete(t *testing.T) {
	srv, _, fid := filmImageServer(t, "tok")
	jpg := tinyJPEG(t)

	for _, role := range []string{model.FilmImagePoster, model.FilmImageBanner} {
		// No image yet → 404.
		resp, err := http.Get(srv.URL + "/api/v1/films/" + itoa(fid) + "/images/" + role)
		if err != nil {
			t.Fatalf("get before upload (%s): %v", role, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s before upload = %d, want 404", role, resp.StatusCode)
		}

		if code := uploadFilmImage(t, srv, "tok", fid, role, jpg); code != http.StatusCreated {
			t.Fatalf("upload %s = %d, want 201", role, code)
		}

		resp, err = http.Get(srv.URL + "/api/v1/films/" + itoa(fid) + "/images/" + role)
		if err != nil {
			t.Fatalf("get after upload (%s): %v", role, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s after upload = %d, want 200", role, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
			t.Fatalf("content-type = %q", ct)
		}
		if cc := resp.Header.Get("Cache-Control"); !bytes.Contains([]byte(cc), []byte("immutable")) {
			t.Fatalf("cache-control = %q, want immutable", cc)
		}
		resp.Body.Close()

		// Delete via owner-gated request.
		req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/films/"+itoa(fid)+"/images/"+role, nil)
		req.Header.Set(api.AdminTokenHeader, "tok")
		delResp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete %s: %v", role, err)
		}
		delResp.Body.Close()
		if delResp.StatusCode != http.StatusNoContent {
			t.Fatalf("delete %s = %d, want 204", role, delResp.StatusCode)
		}
		resp, _ = http.Get(srv.URL + "/api/v1/films/" + itoa(fid) + "/images/" + role)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s after delete = %d, want 404", role, resp.StatusCode)
		}
	}
}

// TestFilmImage_ReplaceAdvancesVersion — a second upload for the same role replaces
// the row and busts the cache (new id → new ?v=), and the /films list carries the
// served poster_url.
func TestFilmImage_ReplaceAdvancesVersion(t *testing.T) {
	srv, r, fid := filmImageServer(t, "tok")
	jpg := tinyJPEG(t)

	if code := uploadFilmImage(t, srv, "tok", fid, model.FilmImagePoster, jpg); code != http.StatusCreated {
		t.Fatalf("upload 1 = %d", code)
	}
	before, _ := r.GetFilm(context.Background(), fid)
	if code := uploadFilmImage(t, srv, "tok", fid, model.FilmImagePoster, jpg); code != http.StatusCreated {
		t.Fatalf("upload 2 = %d", code)
	}
	after, _ := r.GetFilm(context.Background(), fid)
	if after.ImageVersions[model.FilmImagePoster] <= before.ImageVersions[model.FilmImagePoster] {
		t.Fatalf("version did not advance: %d then %d", before.ImageVersions[model.FilmImagePoster], after.ImageVersions[model.FilmImagePoster])
	}

	resp, err := http.Get(srv.URL + "/api/v1/films")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Items []struct {
			Name      string `json:"name"`
			PosterURL string `json:"poster_url"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].PosterURL == "" {
		t.Fatalf("list poster_url missing: %+v", body.Items)
	}
}

// TestFilmImage_MutationsRequireOwner — upload/delete are gated; the public serve
// read is not.
func TestFilmImage_MutationsRequireOwner(t *testing.T) {
	srv, _, fid := filmImageServer(t, "tok")
	jpg := tinyJPEG(t)

	if code := uploadFilmImage(t, srv, "", fid, model.FilmImagePoster, jpg); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated upload = %d, want 401", code)
	}
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/films/"+itoa(fid)+"/images/"+model.FilmImagePoster, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated delete = %d, want 401", resp.StatusCode)
	}
	// Public serve read still works with no token.
	get, err := http.Get(srv.URL + "/api/v1/films/" + itoa(fid) + "/images/" + model.FilmImagePoster)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	get.Body.Close()
	if get.StatusCode != http.StatusNotFound {
		t.Fatalf("public read = %d, want 404 (no auth error)", get.StatusCode)
	}
}

// TestFilmImage_InvalidRole — an unknown role is 400 on every verb.
//
// This used "banner" as its unknown role until F59/ADR-089 D4 made banner a real film
// role, at which point it correctly started failing. "thumb" replaces it, which is the
// stronger case anyway: thumb was a *former* role, retired in the same decision, so this
// now pins the retirement rather than just exercising the validator. "sidecar" keeps a
// never-valid value in the table so the test does not depend on thumb staying dead.
func TestFilmImage_InvalidRole(t *testing.T) {
	srv, _, fid := filmImageServer(t, "tok")
	for _, role := range []string{"thumb", "sidecar"} {
		if code := uploadFilmImage(t, srv, "tok", fid, role, tinyJPEG(t)); code != http.StatusBadRequest {
			t.Fatalf("upload role %q = %d, want 400", role, code)
		}
		resp, err := http.Get(srv.URL + "/api/v1/films/" + itoa(fid) + "/images/" + role)
		if err != nil {
			t.Fatalf("get %q: %v", role, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("get role %q = %d, want 400", role, resp.StatusCode)
		}
	}
}
