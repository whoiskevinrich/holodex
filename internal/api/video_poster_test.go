package api_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
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
	"holodex/internal/repo"
)

// videoPosterServer wires the poster upload/delete surface (owner gate open,
// no token set) with the person-image config uploadVideoPoster reuses for its
// size/dimension caps — thumbServer alone leaves those at their zero value,
// which would reject every upload as over the (zero) byte cap.
func videoPosterServer(t *testing.T) (*httptest.Server, *repo.Repo, string) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	thumbDir := filepath.Join(dir, "thumbnails")
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	h := api.NewHandlers(r, log, &stubThumbs{enabled: false}, thumbDir, nil, nil)
	h.SetPersonImages(filepath.Join(dir, "person-images"), 10<<20, 2000, "cinematheque")
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, thumbDir
}

func postersPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0x80, A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func postVideoPoster(t *testing.T, url string, fileBytes []byte) int {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", "poster.png")
	_, _ = fw.Write(fileBytes)
	_ = mw.Close()

	req, _ := http.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// TestUploadVideoPosterWritesBothTiers guards the fix for a bug the F53
// dual-tier serving priority introduced: servePoster prefers the poster-tier
// file when present, so an upload that only touched the thumbnail tier would
// be silently shadowed by a stale, previously auto-extracted poster. The
// upload must overwrite both files.
func TestUploadVideoPosterWritesBothTiers(t *testing.T) {
	srv, r, thumbDir := videoPosterServer(t)
	id := seedThumbVideo(t, r, "/m/a.mkv")

	// Simulate a prior auto-extraction that already produced distinct tiers.
	if err := os.WriteFile(filepath.Join(thumbDir, itoa(id)+".jpg"), []byte("OLDTHUMB"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(thumbDir, itoa(id)+"-poster.jpg"), []byte("OLDPOSTER"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := postVideoPoster(t, srv.URL+"/api/v1/media/"+itoa(id)+"/poster", postersPNG(t, 40, 40))
	if code != http.StatusCreated {
		t.Fatalf("upload code = %d, want 201", code)
	}

	thumbBytes, err := os.ReadFile(filepath.Join(thumbDir, itoa(id)+".jpg"))
	if err != nil {
		t.Fatalf("read thumb: %v", err)
	}
	posterBytes, err := os.ReadFile(filepath.Join(thumbDir, itoa(id)+"-poster.jpg"))
	if err != nil {
		t.Fatalf("read poster: %v", err)
	}
	if string(thumbBytes) == "OLDTHUMB" || string(posterBytes) == "OLDPOSTER" {
		t.Fatalf("upload did not overwrite stale tiers: thumb stale=%v poster stale=%v",
			string(thumbBytes) == "OLDTHUMB", string(posterBytes) == "OLDPOSTER")
	}
	if !bytes.Equal(thumbBytes, posterBytes) {
		t.Errorf("expected both tiers to hold the same freshly-uploaded bytes")
	}

	// GET .../poster must now serve the fresh upload, not the stale poster tier.
	resp, err := http.Get(srv.URL + "/api/v1/media/" + itoa(id) + "/poster")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.Equal(body, posterBytes) {
		t.Errorf("GET .../poster did not serve the freshly-uploaded bytes")
	}
}

// TestDeleteVideoPosterRemovesBothTiers is the mirror of the upload fix: a
// remove must clear both tiers, not just the thumbnail one.
func TestDeleteVideoPosterRemovesBothTiers(t *testing.T) {
	srv, r, thumbDir := videoPosterServer(t)
	id := seedThumbVideo(t, r, "/m/a.mkv")
	if err := r.SetThumbnailState(context.Background(), id, "uploaded"); err != nil {
		t.Fatal(err)
	}
	thumbPath := filepath.Join(thumbDir, itoa(id)+".jpg")
	posterPath := filepath.Join(thumbDir, itoa(id)+"-poster.jpg")
	if err := os.WriteFile(thumbPath, []byte("UPLOADEDTHUMB"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(posterPath, []byte("UPLOADEDPOSTER"), 0o644); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/media/"+itoa(id)+"/poster", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete code = %d, want 204", resp.StatusCode)
	}
	for _, p := range []string{thumbPath, posterPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s removed, stat err = %v", p, err)
		}
	}
}

// TestServePosterFallsBackToThumbnail exercises P0-6/RD6 (F53, HOLODEX-253):
// before a video's poster-tier derivative has been generated, GET .../poster
// serves the existing thumbnail bytes (200, not 404) so Video.PosterURL
// always resolves. Once the poster-tier file appears (the next natural
// trigger), the route switches to serving it.
func TestServePosterFallsBackToThumbnail(t *testing.T) {
	srv, r, thumbDir := thumbServer(t, &stubThumbs{enabled: true})
	id := seedThumbVideo(t, r, "/m/a.mkv")
	if err := r.SetThumbnailState(context.Background(), id, "generated"); err != nil {
		t.Fatal(err)
	}

	// Poster tier not generated yet -> falls back to the thumbnail bytes.
	if err := os.WriteFile(filepath.Join(thumbDir, itoa(id)+".jpg"), []byte("THUMBBYTES"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(srv.URL + "/api/v1/media/" + itoa(id) + "/poster")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fallback poster code = %d, want 200", resp.StatusCode)
	}
	if string(body) != "THUMBBYTES" {
		t.Errorf("fallback poster body = %q, want thumbnail bytes", body)
	}

	// Poster tier now exists -> the route prefers it over the thumbnail.
	if err := os.WriteFile(filepath.Join(thumbDir, itoa(id)+"-poster.jpg"), []byte("POSTERBYTES"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp2, err := http.Get(srv.URL + "/api/v1/media/" + itoa(id) + "/poster")
	if err != nil {
		t.Fatal(err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("poster code = %d, want 200", resp2.StatusCode)
	}
	if string(body2) != "POSTERBYTES" {
		t.Errorf("poster body = %q, want poster-tier bytes", body2)
	}
}

// TestServePosterNotVisible mirrors serveThumbnail's not-ready/not-visible
// posture: no file on disk yet (neither tier) returns 404.
func TestServePosterNotReady(t *testing.T) {
	srv, r, _ := thumbServer(t, &stubThumbs{enabled: true})
	id := seedThumbVideo(t, r, "/m/a.mkv")

	code, _ := getJSON(t, srv.URL+"/api/v1/media/"+itoa(id)+"/poster")
	if code != http.StatusNotFound {
		t.Fatalf("missing poster code = %d, want 404", code)
	}
}

// TestGetMediaExposesPosterURL confirms P0-5: Video.PosterURL is computed
// alongside ThumbnailURL, by the same mtime-cache-busting convention, once an
// image exists.
func TestGetMediaExposesPosterURL(t *testing.T) {
	srv, r, _ := thumbServer(t, &stubThumbs{enabled: true})
	id := seedThumbVideo(t, r, "/m/a.mkv")
	if err := r.SetThumbnailState(context.Background(), id, "generated"); err != nil {
		t.Fatal(err)
	}

	code, body := getJSON(t, srv.URL+"/api/v1/media/"+itoa(id))
	if code != http.StatusOK {
		t.Fatalf("get media code = %d, want 200", code)
	}
	video, ok := body["video"].(map[string]any)
	if !ok {
		t.Fatalf("expected a video object in response, got %v", body["video"])
	}
	posterURL, ok := video["poster_url"].(string)
	if !ok || posterURL == "" {
		t.Fatalf("expected poster_url in response, got %v", video["poster_url"])
	}
	if !strings.Contains(posterURL, "/media/"+itoa(id)+"/poster") || !strings.Contains(posterURL, "?v=") {
		t.Errorf("unexpected poster_url %q", posterURL)
	}
}
