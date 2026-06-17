package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
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

// personImageServer wires the person-image surface with an on-disk store. token=""
// leaves the owner gate open (single-user default).
func personImageServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
	return personImageServerCfg(t, token, 10<<20)
}

// personImageServerCfg is personImageServer with a configurable upload byte cap (so
// the size-cap path can be exercised with a small limit).
func personImageServerCfg(t *testing.T, token string, maxBytes int64) (*httptest.Server, *repo.Repo, int64) {
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
	h.SetPersonImages(filepath.Join(dir, "person-images"), maxBytes, 2000, "cinematheque")
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	if _, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/x.mkv", Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Alice"}},
	}, nil); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	pid, _, err := r.PersonIDByName(context.Background(), "Alice")
	if err != nil {
		t.Fatalf("person id: %v", err)
	}
	return srv, r, pid
}

func pngUpload(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0x80, 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// uploadImage posts a multipart image+role to the upload endpoint.
func uploadImage(t *testing.T, url, token, role string, fileBytes []byte) (int, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if role != "" {
		_ = mw.WriteField("role", role)
	}
	if fileBytes != nil {
		fw, _ := mw.CreateFormFile("image", "upload.png")
		_, _ = fw.Write(fileBytes)
	}
	_ = mw.Close()

	req, _ := http.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// TestServePersonImage covers real-or-placeholder serving, the ?v= cache buster,
// and the 400/404 paths.
func TestServePersonImage(t *testing.T) {
	srv, _, pid := personImageServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid)

	// Empty slot → themed placeholder SVG (200, svg content type).
	resp, err := http.Get(base + "/image/headshot")
	if err != nil {
		t.Fatalf("get placeholder: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("placeholder code = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct[:9] != "image/svg" {
		t.Errorf("placeholder content-type = %q, want svg", ct)
	}

	// Upload a headshot → the role now serves the real JPEG.
	code, out := uploadImage(t, base+"/image", "", "headshot", pngUpload(t, 64, 64))
	if code != http.StatusCreated {
		t.Fatalf("upload code = %d body=%v", code, out)
	}
	version, _ := out["version"].(float64)
	if version == 0 {
		t.Fatalf("upload returned no version: %v", out)
	}

	resp2, err := http.Get(base + "/image/headshot")
	if err != nil {
		t.Fatalf("get real: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("real image code = %d", resp2.StatusCode)
	}
	if ct := resp2.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("real content-type = %q, want image/jpeg", ct)
	}
	if cc := resp2.Header.Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("cache-control = %q", cc)
	}
	if resp2.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff header on real image")
	}

	// Unknown role → 400.
	if c, _ := getStatus(t, base+"/image/avatar"); c != http.StatusBadRequest {
		t.Errorf("unknown role = %d, want 400", c)
	}
	// Unknown person → 404.
	if c, _ := getStatus(t, srv.URL+"/api/v1/people/99999/image/headshot"); c != http.StatusNotFound {
		t.Errorf("unknown person = %d, want 404", c)
	}
}

func TestUploadValidated(t *testing.T) {
	srv, r, pid := personImageServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid)

	// Missing role → 400.
	if c, _ := uploadImage(t, base+"/image", "", "", pngUpload(t, 16, 16)); c != http.StatusBadRequest {
		t.Errorf("missing role = %d, want 400", c)
	}
	// Bad role → 400.
	if c, _ := uploadImage(t, base+"/image", "", "avatar", pngUpload(t, 16, 16)); c != http.StatusBadRequest {
		t.Errorf("bad role = %d, want 400", c)
	}
	// Missing file → 400.
	if c, _ := uploadImage(t, base+"/image", "", "headshot", nil); c != http.StatusBadRequest {
		t.Errorf("missing file = %d, want 400", c)
	}
	// Non-image bytes → 400 (decode fails).
	if c, _ := uploadImage(t, base+"/image", "", "headshot", []byte("not an image")); c != http.StatusBadRequest {
		t.Errorf("non-image = %d, want 400", c)
	}
	// A valid gallery upload over the cap → 409.
	ctx := context.Background()
	for i := 0; i < repo.GalleryCap; i++ {
		if _, err := r.InsertPersonImage(ctx, pid, model.PersonImageExtra, model.PersonImageSourceUpload, "", "", 1, 1, 1); err != nil {
			t.Fatalf("seed gallery: %v", err)
		}
	}
	if c, _ := uploadImage(t, base+"/image", "", "extra", pngUpload(t, 16, 16)); c != http.StatusConflict {
		t.Errorf("over-cap upload = %d, want 409", c)
	}
}

// TestUploadRejectsOversize: a body over the configured byte cap is rejected by the
// MaxBytesReader at parse time (400) — the disk/memory-exhaustion backstop — before
// any decode. Uses a tiny 256 B cap so a normal PNG exceeds it.
func TestUploadRejectsOversize(t *testing.T) {
	const cap = 256
	srv, _, pid := personImageServerCfg(t, "", cap)
	big := pngUpload(t, 200, 200)
	if len(big) <= cap {
		t.Fatalf("test png is %d bytes; need > %d to exceed the cap", len(big), cap)
	}
	url := srv.URL + "/api/v1/people/" + itoa(pid) + "/image"
	if c, _ := uploadImage(t, url, "", model.PersonImageHeadshot, big); c != http.StatusBadRequest {
		t.Errorf("oversize upload = %d, want 400", c)
	}
}

// TestPersonImageEndpointsGated: with a token set, all mutations require it; reads
// stay public.
func TestPersonImageEndpointsGated(t *testing.T) {
	srv, _, pid := personImageServer(t, "s3cret")
	base := srv.URL + "/api/v1/people/" + itoa(pid)

	// Upload without token → 401.
	if c, _ := uploadImage(t, base+"/image", "", "headshot", pngUpload(t, 16, 16)); c != http.StatusUnauthorized {
		t.Errorf("no-token upload = %d, want 401", c)
	}
	// Upload with token → 201.
	if c, _ := uploadImage(t, base+"/image", "s3cret", "headshot", pngUpload(t, 16, 16)); c != http.StatusCreated {
		t.Errorf("token upload = %d, want 201", c)
	}
	// Public read still works without a token.
	if c, _ := getStatus(t, base+"/image/headshot"); c != http.StatusOK {
		t.Errorf("public read = %d, want 200", c)
	}
}

// TestPromoteAndReorder covers promote (gallery → core) and reorder.
func TestPromoteAndReorder(t *testing.T) {
	srv, _, pid := personImageServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid)

	// Upload two gallery items.
	_, o1 := uploadImage(t, base+"/image", "", "extra", pngUpload(t, 20, 20))
	_, o2 := uploadImage(t, base+"/image", "", "extra", pngUpload(t, 30, 30))
	id1 := int64(o1["id"].(float64))
	id2 := int64(o2["id"].(float64))

	// Promote the first gallery item into the poster slot.
	code, _ := postTok(t, base+"/images/"+itoa(id1)+"/promote", "", map[string]string{"role": "poster"})
	if code != http.StatusCreated {
		t.Fatalf("promote code = %d", code)
	}
	// The poster slot now serves a real image.
	if c, _ := getStatus(t, base+"/image/poster"); c != http.StatusOK {
		t.Errorf("poster after promote = %d, want 200", c)
	}

	// Reorder: id2 first.
	code, body := postTok(t, base+"/images/reorder", "", map[string]any{"order": []int64{id2, id1}})
	if code != http.StatusOK {
		t.Fatalf("reorder code = %d", code)
	}
	images, _ := body["images"].(map[string]any)
	gallery, _ := images["gallery"].([]any)
	if len(gallery) != 2 {
		t.Fatalf("gallery len = %d", len(gallery))
	}
	first, _ := gallery[0].(map[string]any)
	if int64(first["id"].(float64)) != id2 {
		t.Errorf("reordered first = %v, want %d", first["id"], id2)
	}

	// Delete a gallery item → 204; then 404 on re-delete.
	if c := deleteReq(t, base+"/images/"+itoa(id1), ""); c != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", c)
	}
	if c := deleteReq(t, base+"/images/"+itoa(id1), ""); c != http.StatusNotFound {
		t.Errorf("re-delete = %d, want 404", c)
	}
}

// TestPersonDetailImageSet: GET /people/{id} carries the image set.
func TestPersonDetailImageSet(t *testing.T) {
	srv, _, pid := personImageServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid)
	_, _ = uploadImage(t, base+"/image", "", "headshot", pngUpload(t, 16, 16))

	code, body := getJSON(t, base)
	if code != http.StatusOK {
		t.Fatalf("person code = %d", code)
	}
	images, ok := body["images"].(map[string]any)
	if !ok {
		t.Fatalf("person detail missing images: %v", body)
	}
	roles, _ := images["roles"].(map[string]any)
	hs, _ := roles["headshot"].(map[string]any)
	if hs == nil || hs["present"] != true {
		t.Errorf("headshot slot not present in image set: %v", roles)
	}
}

func getStatus(t *testing.T, url string) (int, *http.Response) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	resp.Body.Close()
	return resp.StatusCode, resp
}

func deleteReq(t *testing.T, url, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}
