package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/filmimage"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// filmImageServer wires a films-enabled surface with self-hosted image storage
// (F56/HOLODEX-280, ADR-086) and returns the film id + the running server. tinyJPEG
// and uploadStudioImage's multipart-building style are shared from
// studio_images_test.go (same package).
func filmImageServer(t *testing.T, token string) (srv *httptest.Server, r *repo.Repo, fid int64) {
	t.Helper()
	srv, r, fid, _ = filmImageServerDir(t, token)
	return srv, r, fid
}

// filmImageServerDir is filmImageServer plus the on-disk image directory, for tests
// that seed a provider-sourced file directly instead of going through the upload route.
func filmImageServerDir(t *testing.T, token string) (srv *httptest.Server, r *repo.Repo, fid int64, imageDir string) {
	t.Helper()
	dir := t.TempDir()
	imageDir = filepath.Join(dir, "film-images")
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
	h.SetFilmImages(imageDir, 5<<20, 1000)
	h.SetAuth(api.NewAuth(token), false)
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	fid, err = r.CreateFilm(ctx, "Spirited Away", 2001)
	if err != nil {
		t.Fatalf("seed film: %v", err)
	}
	return srv, r, fid, imageDir
}

// uploadFilmImage POSTs a multipart image for a role and returns the response code
// and body (the body carries the plain-language reason on a 400).
func uploadFilmImage(t *testing.T, srv *httptest.Server, token string, fid int64, role string, raw []byte) (int, string) {
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
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out)
}

// solidJPEG encodes a solid w×h JPEG — tinyJPEG with the dimensions chosen by the
// caller, so a test can build the landscape the banner role requires and the
// portrait it refuses (HOLODEX-386) from one helper.
func solidJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 16), G: uint8(y * 32), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	return buf.Bytes()
}

// TestFilmImage_UploadServeDelete is the owner happy path for both roles: an upload
// normalizes and stores the bytes, the serve route streams them with an immutable
// cache, and delete removes the slot. The banner gets a landscape file because the
// role refuses anything else (HOLODEX-386, pinned separately below).
func TestFilmImage_UploadServeDelete(t *testing.T) {
	srv, _, fid := filmImageServer(t, "tok")
	files := map[string][]byte{
		model.FilmImagePoster: tinyJPEG(t),
		model.FilmImageBanner: solidJPEG(t, 16, 6),
	}

	for _, role := range []string{model.FilmImagePoster, model.FilmImageBanner} {
		jpg := files[role]
		// No image yet → 404.
		resp, err := http.Get(srv.URL + "/api/v1/films/" + itoa(fid) + "/images/" + role)
		if err != nil {
			t.Fatalf("get before upload (%s): %v", role, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s before upload = %d, want 404", role, resp.StatusCode)
		}

		if code, _ := uploadFilmImage(t, srv, "tok", fid, role, jpg); code != http.StatusCreated {
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

// TestFilmImage_DeleteClearsProviderRow reproduces HOLODEX-388: the owner's Remove on
// a banner that came from enrichment (source "provider:tmdb", no upload row) must
// actually clear the slot. Before the fix the handler deleted only the upload row, so
// the DELETE was a 204 no-op and the provider banner kept serving. The second half
// covers the upload-over-provider pair — one Remove empties the role, it does not peel
// the upload back to reveal the provider image.
func TestFilmImage_DeleteClearsProviderRow(t *testing.T) {
	srv, r, fid, dir := filmImageServerDir(t, "tok")
	ctx := context.Background()
	role := model.FilmImageBanner
	url := srv.URL + "/api/v1/films/" + itoa(fid) + "/images/" + role

	insertProvider := func() int64 {
		t.Helper()
		id, err := r.ReplaceFilmImage(ctx, repo.FilmImageInsert{
			FilmID: fid, Role: role, Source: "provider:tmdb", Provider: "tmdb", ExternalID: "129",
			Width: 16, Height: 6,
		})
		if err != nil {
			t.Fatalf("insert provider image: %v", err)
		}
		if err := filmimage.Store(dir, fid, id, solidJPEG(t, 16, 6)); err != nil {
			t.Fatalf("store provider file: %v", err)
		}
		return id
	}
	del := func() {
		t.Helper()
		req, _ := http.NewRequest(http.MethodDelete, url, nil)
		req.Header.Set(api.AdminTokenHeader, "tok")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("delete = %d, want 204", resp.StatusCode)
		}
	}
	served := func() int {
		t.Helper()
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	// Provider-only slot — the reported case.
	providerID := insertProvider()
	if served() != http.StatusOK {
		t.Fatal("provider banner should serve before delete")
	}
	del()
	if code := served(); code != http.StatusNotFound {
		t.Fatalf("banner after delete = %d, want 404 (provider row survived)", code)
	}
	if _, err := os.Stat(filmimage.ImagePath(dir, fid, providerID)); !os.IsNotExist(err) {
		t.Fatalf("provider file still on disk: %v", err)
	}
	f, _ := r.GetFilm(ctx, fid)
	if _, ok := f.ImageVersions[role]; ok {
		t.Fatal("ImageVersions still carries the banner role")
	}

	// Upload + provider pair — one Remove clears both.
	insertProvider()
	if code, _ := uploadFilmImage(t, srv, "tok", fid, role, solidJPEG(t, 16, 6)); code != http.StatusCreated {
		t.Fatalf("upload = %d, want 201", code)
	}
	del()
	if code := served(); code != http.StatusNotFound {
		t.Fatalf("banner after paired delete = %d, want 404", code)
	}
}

// TestFilmImage_ReplaceAdvancesVersion — a second upload for the same role replaces
// the row and busts the cache (new id → new ?v=), and the /films list carries the
// served poster_url.
func TestFilmImage_ReplaceAdvancesVersion(t *testing.T) {
	srv, r, fid := filmImageServer(t, "tok")
	jpg := tinyJPEG(t)

	if code, _ := uploadFilmImage(t, srv, "tok", fid, model.FilmImagePoster, jpg); code != http.StatusCreated {
		t.Fatalf("upload 1 = %d", code)
	}
	before, _ := r.GetFilm(context.Background(), fid)
	if code, _ := uploadFilmImage(t, srv, "tok", fid, model.FilmImagePoster, jpg); code != http.StatusCreated {
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

	if code, _ := uploadFilmImage(t, srv, "", fid, model.FilmImagePoster, jpg); code != http.StatusUnauthorized {
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
		if code, _ := uploadFilmImage(t, srv, "tok", fid, role, tinyJPEG(t)); code != http.StatusBadRequest {
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

// TestFilmImage_BannerRequiresLandscape — the banner role renders cover-fit into an
// 8:3 band, so a square or portrait upload is refused with the dimensions in the
// message and nothing stored (HOLODEX-386). The poster role is untouched: it still
// takes the same square file.
func TestFilmImage_BannerRequiresLandscape(t *testing.T) {
	srv, r, fid := filmImageServer(t, "tok")

	for _, tc := range []struct {
		name string
		raw  []byte
		want string
	}{
		{"square", tinyJPEG(t), "8×8 is portrait"},
		{"portrait", solidJPEG(t, 10, 15), "10×15 is portrait"},
	} {
		code, body := uploadFilmImage(t, srv, "tok", fid, model.FilmImageBanner, tc.raw)
		if code != http.StatusBadRequest {
			t.Fatalf("%s banner upload = %d, want 400", tc.name, code)
		}
		if !strings.Contains(body, "banner refused: "+tc.want) {
			t.Errorf("%s banner 400 body = %q, want the dimensions in a plain-language reason", tc.name, body)
		}
	}
	film, err := r.GetFilm(context.Background(), fid)
	if err != nil {
		t.Fatalf("get film: %v", err)
	}
	if _, stored := film.ImageVersions[model.FilmImageBanner]; stored {
		t.Fatalf("a refused banner was stored: %+v", film.ImageVersions)
	}

	if code, body := uploadFilmImage(t, srv, "tok", fid, model.FilmImageBanner, solidJPEG(t, 16, 6)); code != http.StatusCreated {
		t.Fatalf("landscape banner upload = %d (%s), want 201", code, body)
	}
	if code, body := uploadFilmImage(t, srv, "tok", fid, model.FilmImagePoster, tinyJPEG(t)); code != http.StatusCreated {
		t.Fatalf("square poster upload = %d (%s), want 201 — the rule is banner-only", code, body)
	}
}
