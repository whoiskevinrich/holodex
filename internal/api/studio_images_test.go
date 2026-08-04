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
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// tinyJPEG encodes a real 8×8 JPEG so the ingest normalizer (decode →
// re-encode) accepts it — a placeholder byte string would be rejected.
func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 32), G: uint8(y * 32), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	return buf.Bytes()
}

// studioImageServer wires a studio surface with self-hosted image storage (F51,
// ADR-079) and returns the studio id + the running server.
func studioImageServer(t *testing.T, token string) (srv *httptest.Server, r *repo.Repo, sid int64) {
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
	h.SetStudioImages(filepath.Join(dir, "studio-images"), 5<<20, 1000)
	h.SetAuth(api.NewAuth(token), false)
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/s.mkv", Title: "Clip",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, vid, []string{"Studio Ghibli"}, nil); err != nil {
		t.Fatalf("seed studio: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil || len(studios) != 1 {
		t.Fatalf("list studios: %v (%d)", err, len(studios))
	}
	return srv, r, studios[0].ID
}

// uploadStudioImage POSTs a multipart image for a role and returns the response code.
func uploadStudioImage(t *testing.T, srv *httptest.Server, token string, sid int64, role string, raw []byte) int {
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
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/studios/"+itoa(sid)+"/images/"+role, &body)
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

// TestStudioImage_UploadServeDelete is the owner happy path for all three roles: an
// upload normalizes and stores the bytes, the serve route streams them with an
// immutable cache, and delete removes the slot.
func TestStudioImage_UploadServeDelete(t *testing.T) {
	srv, _, sid := studioImageServer(t, "tok")
	jpg := tinyJPEG(t)

	for _, role := range []string{model.StudioImageIcon, model.StudioImageLogo, model.StudioImagePoster} {
		// No image yet → 404.
		resp, err := http.Get(srv.URL + "/api/v1/studios/" + itoa(sid) + "/images/" + role)
		if err != nil {
			t.Fatalf("get before upload (%s): %v", role, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s before upload = %d, want 404", role, resp.StatusCode)
		}

		if code := uploadStudioImage(t, srv, "tok", sid, role, jpg); code != http.StatusCreated {
			t.Fatalf("upload %s = %d, want 201", role, code)
		}

		resp, err = http.Get(srv.URL + "/api/v1/studios/" + itoa(sid) + "/images/" + role)
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
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if _, _, err := image.Decode(bytes.NewReader(body)); err != nil {
			t.Fatalf("served bytes for %s are not a decodable image: %v", role, err)
		}

		// Delete via owner-gated request.
		req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/studios/"+itoa(sid)+"/images/"+role, nil)
		req.Header.Set(api.AdminTokenHeader, "tok")
		delResp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete %s: %v", role, err)
		}
		delResp.Body.Close()
		if delResp.StatusCode != http.StatusNoContent {
			t.Fatalf("delete %s = %d, want 204", role, delResp.StatusCode)
		}
		resp, _ = http.Get(srv.URL + "/api/v1/studios/" + itoa(sid) + "/images/" + role)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s after delete = %d, want 404", role, resp.StatusCode)
		}
	}
}

// TestStudioImage_ReplaceAdvancesVersion — a second upload for the same role
// replaces the row and busts the cache (new id → new ?v=), and the /studios list
// carries the served icon_url.
func TestStudioImage_ReplaceAdvancesVersion(t *testing.T) {
	srv, r, sid := studioImageServer(t, "tok")
	jpg := tinyJPEG(t)

	if code := uploadStudioImage(t, srv, "tok", sid, model.StudioImageIcon, jpg); code != http.StatusCreated {
		t.Fatalf("upload 1 = %d", code)
	}
	before, _ := r.GetStudio(context.Background(), sid)
	if code := uploadStudioImage(t, srv, "tok", sid, model.StudioImageIcon, jpg); code != http.StatusCreated {
		t.Fatalf("upload 2 = %d", code)
	}
	after, _ := r.GetStudio(context.Background(), sid)
	if after.ImageVersions[model.StudioImageIcon] <= before.ImageVersions[model.StudioImageIcon] {
		t.Fatalf("version did not advance: %d then %d", before.ImageVersions[model.StudioImageIcon], after.ImageVersions[model.StudioImageIcon])
	}

	resp, err := http.Get(srv.URL + "/api/v1/studios")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Items []struct {
			Name    string `json:"name"`
			IconURL string `json:"icon_url"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].IconURL == "" {
		t.Fatalf("list icon_url missing: %+v", body.Items)
	}
}

// TestStudioImage_MutationsRequireOwner — upload/delete are gated; the public serve
// read is not.
func TestStudioImage_MutationsRequireOwner(t *testing.T) {
	srv, _, sid := studioImageServer(t, "tok")
	jpg := tinyJPEG(t)

	if code := uploadStudioImage(t, srv, "", sid, model.StudioImageLogo, jpg); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated upload = %d, want 401", code)
	}
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/studios/"+itoa(sid)+"/images/"+model.StudioImageLogo, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated delete = %d, want 401", resp.StatusCode)
	}
	// Public serve read still works with no token.
	get, err := http.Get(srv.URL + "/api/v1/studios/" + itoa(sid) + "/images/" + model.StudioImageLogo)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	get.Body.Close()
	if get.StatusCode != http.StatusNotFound {
		t.Fatalf("public read = %d, want 404 (no auth error)", get.StatusCode)
	}
}

// TestStudioImage_InvalidRole404s — an unknown role is 400 on every verb.
func TestStudioImage_InvalidRole(t *testing.T) {
	srv, _, sid := studioImageServer(t, "tok")
	if code := uploadStudioImage(t, srv, "tok", sid, "banner", tinyJPEG(t)); code != http.StatusBadRequest {
		t.Fatalf("upload invalid role = %d, want 400", code)
	}
	resp, err := http.Get(srv.URL + "/api/v1/studios/" + itoa(sid) + "/images/banner")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("get invalid role = %d, want 400", resp.StatusCode)
	}
}
