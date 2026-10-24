package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// Tag poster images (F82, testing-strategy §33.2): owner gate, the shared image gate,
// the serve route, the list/detail URL, and the merge lifecycle.

type tagImageEnv struct {
	srv *httptest.Server
	r   *repo.Repo
	dir string // the tag image root
	ids map[string]int64
}

// tagImageServer wires tag image storage with an upload cap of maxBytes and seeds the
// tags noir, rain and piers.
func tagImageServer(t *testing.T, token string, maxBytes int64) tagImageEnv {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	ctx := context.Background()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	imgDir := filepath.Join(dir, "tag-images")
	h.SetTagImages(imgDir, maxBytes, 1000)
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	if _, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/t.mkv", Title: "Clip", FileMtime: time.Now().UTC().Truncate(time.Second),
		Tags: []model.Tag{{Name: "noir"}, {Name: "rain"}, {Name: "piers"}},
	}, nil); err != nil {
		t.Fatalf("seed video: %v", err)
	}
	tags, err := r.ListTagsFiltered(ctx, repo.NamedListFilter{})
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	ids := map[string]int64{}
	for _, tg := range tags {
		ids[tg.Name] = tg.ID
	}
	return tagImageEnv{srv: srv, r: r, dir: imgDir, ids: ids}
}

func (e tagImageEnv) url(tag string) string {
	return e.srv.URL + "/api/v1/tags/" + itoa(e.ids[tag]) + "/image"
}

// upload POSTs raw as the `image` field and returns the status and the new version.
func (e tagImageEnv) upload(t *testing.T, token, tag string, raw []byte) (int, int64) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", "img.jpg")
	_, _ = fw.Write(raw)
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, e.url(tag), &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Version int64 `json:"version"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out.Version
}

func (e tagImageEnv) del(t *testing.T, token, tag string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, e.url(tag), nil)
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// serve GETs the image and returns the response (body closed by the caller).
func (e tagImageEnv) serve(t *testing.T, tag string) *http.Response {
	t.Helper()
	resp, err := http.Get(e.url(tag))
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// imageURLs reads image_url for every tag on the list, keyed by name.
func (e tagImageEnv) imageURLs(t *testing.T) map[string]string {
	t.Helper()
	resp, err := http.Get(e.srv.URL + "/api/v1/tags?limit=50")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Items []struct {
			Name     string `json:"name"`
			ImageURL string `json:"image_url"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	urls := map[string]string{}
	for _, it := range out.Items {
		urls[it.Name] = it.ImageURL
	}
	return urls
}

// imageFiles lists the files stored under one tag's directory.
func (e tagImageEnv) imageFiles(t *testing.T, tag string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(e.dir, itoa(e.ids[tag])))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var names []string
	for _, en := range entries {
		names = append(names, en.Name())
	}
	return names
}

func TestTagImage_UploadServeDelete(t *testing.T) {
	e := tagImageServer(t, "tok", 5<<20)

	if code := e.serve(t, "noir").StatusCode; code != http.StatusNotFound {
		t.Fatalf("GET before upload = %d, want 404", code)
	}
	if u := e.imageURLs(t)["noir"]; u != "" {
		t.Fatalf("image_url before upload = %q, want none", u)
	}
	code, v := e.upload(t, "tok", "noir", tinyJPEG(t))
	if code != http.StatusCreated || v == 0 {
		t.Fatalf("upload = %d v%d, want 201 with a version", code, v)
	}

	resp := e.serve(t, "noir")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET after upload = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable", cc)
	}
	if ns := resp.Header.Get("X-Content-Type-Options"); ns != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", ns)
	}
	if _, _, err := image.Decode(resp.Body); err != nil {
		t.Errorf("served bytes don't decode: %v", err)
	}

	want := "/api/v1/tags/" + itoa(e.ids["noir"]) + "/image?v=" + itoa(v)
	if u := e.imageURLs(t)["noir"]; u != want {
		t.Errorf("list image_url = %q, want %q", u, want)
	}
	if u := e.imageURLs(t)["rain"]; u != "" {
		t.Errorf("a tag without an image has image_url %q", u)
	}
	detail, err := http.Get(e.srv.URL + "/api/v1/tags/" + itoa(e.ids["noir"]))
	if err != nil {
		t.Fatal(err)
	}
	defer detail.Body.Close()
	var d struct {
		Tag struct {
			ImageURL string `json:"image_url"`
		} `json:"tag"`
	}
	_ = json.NewDecoder(detail.Body).Decode(&d)
	if d.Tag.ImageURL != want {
		t.Errorf("detail image_url = %q, want %q", d.Tag.ImageURL, want)
	}

	if code := e.del(t, "tok", "noir"); code != http.StatusNoContent {
		t.Fatalf("DELETE = %d, want 204", code)
	}
	if code := e.serve(t, "noir").StatusCode; code != http.StatusNotFound {
		t.Errorf("GET after delete = %d, want 404", code)
	}
	if files := e.imageFiles(t, "noir"); len(files) != 0 {
		t.Errorf("files left after delete: %v", files)
	}
	if code := e.del(t, "tok", "noir"); code != http.StatusNoContent {
		t.Errorf("second DELETE = %d, want 204 (idempotent)", code)
	}
}

func TestTagImage_ReplaceAdvancesVersionAndDropsOldFile(t *testing.T) {
	e := tagImageServer(t, "tok", 5<<20)
	_, v1 := e.upload(t, "tok", "noir", tinyJPEG(t))
	code, v2 := e.upload(t, "tok", "noir", tinyJPEG(t))
	if code != http.StatusCreated || v2 <= v1 {
		t.Fatalf("replace = %d v%d, want 201 with a version past %d", code, v2, v1)
	}
	if files := e.imageFiles(t, "noir"); len(files) != 1 || files[0] != itoa(v2)+".jpg" {
		t.Errorf("files after replace = %v, want only %d.jpg", files, v2)
	}
}

func TestTagImage_MutationsRequireOwner(t *testing.T) {
	e := tagImageServer(t, "tok", 5<<20)
	if code, _ := e.upload(t, "", "noir", tinyJPEG(t)); code != http.StatusUnauthorized && code != http.StatusForbidden {
		t.Fatalf("visitor upload = %d, want 401/403", code)
	}
	if code, _ := e.upload(t, "wrong", "noir", tinyJPEG(t)); code != http.StatusUnauthorized && code != http.StatusForbidden {
		t.Fatalf("wrong-token upload = %d, want 401/403", code)
	}
	if code := e.serve(t, "noir").StatusCode; code != http.StatusNotFound {
		t.Fatalf("a refused upload stored an image (GET = %d)", code)
	}
	if code, _ := e.upload(t, "tok", "noir", tinyJPEG(t)); code != http.StatusCreated {
		t.Fatalf("owner upload = %d", code)
	}
	if code := e.del(t, "", "noir"); code != http.StatusUnauthorized && code != http.StatusForbidden {
		t.Fatalf("visitor delete = %d, want 401/403", code)
	}
	if code := e.serve(t, "noir").StatusCode; code != http.StatusOK {
		t.Fatalf("a refused delete removed the image (GET = %d)", code)
	}
}

func TestTagImage_ImageGate(t *testing.T) {
	// A noisy 300×300 JPEG is well over a 4 KiB cap.
	noisy := func() []byte {
		img := image.NewRGBA(image.Rect(0, 0, 300, 300))
		rng := rand.New(rand.NewSource(1))
		for i := range img.Pix {
			img.Pix[i] = uint8(rng.Intn(256))
		}
		var buf bytes.Buffer
		_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
		return buf.Bytes()
	}()
	e := tagImageServer(t, "tok", 4<<10)
	if code, _ := e.upload(t, "tok", "noir", noisy); code != http.StatusBadRequest {
		t.Errorf("over-cap upload = %d, want 400", code)
	}
	if code, _ := e.upload(t, "tok", "noir", []byte("not an image at all")); code != http.StatusBadRequest {
		t.Errorf("non-image upload = %d, want 400", code)
	}
	if code := e.serve(t, "noir").StatusCode; code != http.StatusNotFound {
		t.Errorf("a rejected upload stored an image (GET = %d)", code)
	}
	if code, _ := e.upload(t, "tok", "noir", tinyTransparentPNG(t)); code != http.StatusCreated {
		t.Fatalf("transparent PNG upload = %d", code)
	}
	if ct := e.serve(t, "noir").Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("transparent upload served as %q, want image/png (alpha kept)", ct)
	}
}

// merge folds loser into survivor and returns the response body.
func (e tagImageEnv) merge(t *testing.T, survivor, loser string) string {
	t.Helper()
	code, body := postTok(t, e.srv.URL+"/api/v1/tags/"+itoa(e.ids[survivor])+"/merge", "tok",
		map[string]int64{"from_id": e.ids[loser]})
	if code != http.StatusOK {
		t.Fatalf("merge %s into %s = %d: %s", loser, survivor, code, body)
	}
	return fmt.Sprint(body)
}

func TestTagImage_MergeCarriesImageToASurvivorWithout(t *testing.T) {
	e := tagImageServer(t, "tok", 5<<20)
	_, v := e.upload(t, "tok", "rain", tinyJPEG(t))
	loserDir := filepath.Join(e.dir, itoa(e.ids["rain"]))

	mergeBody := e.merge(t, "noir", "rain")

	resp := e.serve(t, "noir")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("survivor GET after merge = %d, want 200 (the image carried over)", resp.StatusCode)
	}
	if u := e.imageURLs(t)["noir"]; !strings.HasSuffix(u, "?v="+itoa(v)) {
		t.Errorf("survivor image_url = %q, want the carried version %d", u, v)
	}
	if _, err := os.Stat(loserDir); !os.IsNotExist(err) {
		t.Errorf("the merged tag's image directory is still there (err %v)", err)
	}
	// The merge response itself names the carried image, so the page needn't refetch.
	if !strings.Contains(mergeBody, "?v="+itoa(v)) {
		t.Errorf("merge response lacks the carried image_url: %s", mergeBody)
	}
}

func TestTagImage_MergeKeepsTheSurvivorsOwnImage(t *testing.T) {
	e := tagImageServer(t, "tok", 5<<20)
	_, keep := e.upload(t, "tok", "noir", tinyJPEG(t))
	_, _ = e.upload(t, "tok", "rain", tinyJPEG(t))
	loserDir := filepath.Join(e.dir, itoa(e.ids["rain"]))

	e.merge(t, "noir", "rain")

	if u := e.imageURLs(t)["noir"]; !strings.HasSuffix(u, "?v="+itoa(keep)) {
		t.Errorf("survivor image_url = %q, want its own version %d", u, keep)
	}
	if code := e.serve(t, "noir").StatusCode; code != http.StatusOK {
		t.Errorf("survivor GET = %d, want 200", code)
	}
	if _, err := os.Stat(loserDir); !os.IsNotExist(err) {
		t.Errorf("the merged tag's image file was left behind (err %v)", err)
	}
	if files := e.imageFiles(t, "noir"); len(files) != 1 {
		t.Errorf("survivor files = %v, want its one image", files)
	}
}

func TestTagImage_RenameKeepsImage(t *testing.T) {
	e := tagImageServer(t, "tok", 5<<20)
	_, v := e.upload(t, "tok", "noir", tinyJPEG(t))
	code, body := postTok(t, e.srv.URL+"/api/v1/tags/"+itoa(e.ids["noir"])+"/rename", "tok",
		map[string]string{"name": "neo-noir"})
	if code != http.StatusOK && code != http.StatusNoContent {
		t.Fatalf("rename = %d: %s", code, body)
	}
	if u := e.imageURLs(t)["neo-noir"]; !strings.HasSuffix(u, "?v="+itoa(v)) {
		t.Errorf("renamed tag image_url = %q, want version %d", u, v)
	}
}
