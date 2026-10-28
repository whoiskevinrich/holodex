package api_test

import (
	"context"
	"io"
	"log/slog"
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
	"holodex/internal/thumbnail"
)

// stubThumbs implements the API's thumbnailer seam.
type stubThumbs struct {
	enabled    bool
	depth      int
	enqueued   []int64
	extractIDs []int64 // ids passed to ExtractEmbedded
	extractOK  bool    // value ExtractEmbedded returns for ok
}

func (s *stubThumbs) EnqueueHigh(ids []int64) { s.enqueued = append(s.enqueued, ids...) }
func (s *stubThumbs) QueueDepth() int         { return s.depth }
func (s *stubThumbs) QueueStats() thumbnail.QueueStats {
	return thumbnail.QueueStats{Depth: s.depth, Normal: s.depth}
}
func (s *stubThumbs) Enabled() bool { return s.enabled }
func (s *stubThumbs) ExtractEmbedded(_ context.Context, id int64, _ string) (bool, error) {
	s.extractIDs = append(s.extractIDs, id)
	return s.extractOK, nil
}
func (s *stubThumbs) SetOwnerImage(context.Context, int64, string, []byte, *float64) error {
	return nil
}
func (s *stubThumbs) RemoveOwnerImage(context.Context, int64, string) (bool, error) {
	return false, nil
}
func (s *stubThumbs) Present(context.Context, int64) error { return nil }

func thumbServer(t *testing.T, thumbs *stubThumbs) (*httptest.Server, *repo.Repo, string) {
	t.Helper()
	return thumbServerTok(t, thumbs, "")
}

// thumbServerTok is thumbServer with the owner gate closed by token ("" = open).
func thumbServerTok(t *testing.T, thumbs *stubThumbs, token string) (*httptest.Server, *repo.Repo, string) {
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
	h := api.NewHandlers(r, log, thumbs, thumbDir, nil, nil)
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, thumbDir
}

func seedThumbVideo(t *testing.T, r *repo.Repo, path string) int64 {
	t.Helper()
	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: path, FileSize: 1, Title: "x", Duration: 60,
		Width: 1920, Height: 1080, FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return id
}

func TestServeThumbnailNotReadyThenReady(t *testing.T) {
	srv, r, thumbDir := thumbServer(t, &stubThumbs{enabled: true})
	id := seedThumbVideo(t, r, "/m/a.mkv")

	// No file on disk yet -> 404 (the contract the frontend retry relies on).
	code, _ := getJSON(t, srv.URL+"/api/v1/media/"+itoa(id)+"/thumbnail")
	if code != http.StatusNotFound {
		t.Fatalf("missing thumbnail code = %d, want 404", code)
	}

	// Write the file -> 200 with the bytes.
	if err := os.WriteFile(filepath.Join(thumbDir, itoa(id)+".jpg"), []byte("JPEGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(srv.URL + "/api/v1/media/" + itoa(id) + "/thumbnail")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ready thumbnail code = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Cache-Control") == "" {
		t.Errorf("missing Cache-Control header")
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "JPEGDATA" {
		t.Errorf("thumbnail body = %q", body)
	}
}

func TestRegenerateThumbnail(t *testing.T) {
	thumbs := &stubThumbs{enabled: true}
	srv, r, _ := thumbServer(t, thumbs)
	id := seedThumbVideo(t, r, "/m/a.mkv")
	if err := r.SetThumbnailState(context.Background(), id, "generated"); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Post(srv.URL+"/api/v1/media/"+itoa(id)+"/thumbnail", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("regenerate code = %d, want 202", resp.StatusCode)
	}
	if len(thumbs.enqueued) != 1 || thumbs.enqueued[0] != id {
		t.Errorf("enqueued = %v, want [%d]", thumbs.enqueued, id)
	}
	// State was reset to NULL so the worker reprocesses it.
	got, _, _ := r.GetVideo(context.Background(), id)
	if got.ThumbnailState != "" {
		t.Errorf("state after regenerate = %q, want empty", got.ThumbnailState)
	}
}

// "Reset to automatic" (F85) drops an owner frame/upload choice so the page falls
// back, and leaves any other poster decision alone.
func TestRegenerateThumbnailClearsLocalPosterChoice(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct{ decided, want string }{
		{"local:frame", ""},
		{"file", "file"},
	} {
		srv, r, _ := thumbServer(t, &stubThumbs{enabled: true})
		id := seedThumbVideo(t, r, "/m/a.mkv")
		if err := r.SetDecision(ctx, "video", id, "poster_url", tc.decided, ""); err != nil {
			t.Fatal(err)
		}
		resp, err := http.Post(srv.URL+"/api/v1/media/"+itoa(id)+"/thumbnail", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if got, _ := r.PosterDecision(ctx, id); got != tc.want {
			t.Errorf("decided %s: after reset = %q, want %q", tc.decided, got, tc.want)
		}
	}
}

// Regenerate resets state and queues ffmpeg work, so it sits under requireOwner:
// a visitor gets 401 and nothing moves; the owner's token still gets 202. The
// GET stays public.
func TestRegenerateThumbnailOwnerGated(t *testing.T) {
	thumbs := &stubThumbs{enabled: true}
	srv, r, thumbDir := thumbServerTok(t, thumbs, "secret")
	id := seedThumbVideo(t, r, "/m/a.mkv")
	if err := r.SetThumbnailState(context.Background(), id, "generated"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(thumbDir, itoa(id)+".jpg"), []byte("JPEGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	url := srv.URL + "/api/v1/media/" + itoa(id) + "/thumbnail"

	if code, _ := postTok(t, url, "", nil); code != http.StatusUnauthorized {
		t.Fatalf("visitor regenerate code = %d, want 401", code)
	}
	if len(thumbs.enqueued) != 0 || len(thumbs.extractIDs) != 0 {
		t.Errorf("visitor regenerate reached the pipeline: enqueued=%v extract=%v", thumbs.enqueued, thumbs.extractIDs)
	}
	if got, _, _ := r.GetVideo(context.Background(), id); got.ThumbnailState != "generated" {
		t.Errorf("visitor regenerate changed state to %q", got.ThumbnailState)
	}
	if code, _ := getJSON(t, url); code != http.StatusOK {
		t.Errorf("visitor thumbnail GET code = %d, want 200 (reads stay public)", code)
	}

	if code, _ := postTok(t, url, "secret", nil); code != http.StatusAccepted {
		t.Fatalf("owner regenerate code = %d, want 202", code)
	}
	if len(thumbs.enqueued) != 1 || thumbs.enqueued[0] != id {
		t.Errorf("owner enqueued = %v, want [%d]", thumbs.enqueued, id)
	}
}

func TestRegenerateDisabled(t *testing.T) {
	srv, r, _ := thumbServer(t, &stubThumbs{enabled: false})
	id := seedThumbVideo(t, r, "/m/a.mkv")
	resp, err := http.Post(srv.URL+"/api/v1/media/"+itoa(id)+"/thumbnail", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("disabled regenerate code = %d, want 503", resp.StatusCode)
	}
}

func TestListEnqueuesVisibleAndExposesURL(t *testing.T) {
	thumbs := &stubThumbs{enabled: true}
	srv, r, _ := thumbServer(t, thumbs)
	pending := seedThumbVideo(t, r, "/m/pending.mkv") // state NULL
	ready := seedThumbVideo(t, r, "/m/ready.mkv")
	if err := r.SetThumbnailState(context.Background(), ready, "generated"); err != nil {
		t.Fatal(err)
	}

	code, body := getJSON(t, srv.URL+"/api/v1/media")
	if code != 200 {
		t.Fatalf("list code = %d", code)
	}
	items, _ := body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	// The ready video exposes a thumbnail_url; the pending one does not.
	for _, it := range items {
		m := it.(map[string]any)
		idF := m["id"].(float64)
		url, hasURL := m["thumbnail_url"].(string)
		if int64(idF) == ready {
			if !hasURL {
				t.Errorf("ready video missing thumbnail_url")
			}
			// The URL must carry a ?v= cache-bust token (file mtime) so a writeback
			// that rewrites the file's cover art is not masked by a stale browser cache.
			if !strings.Contains(url, "?v=") {
				t.Errorf("thumbnail_url missing ?v= version token: %q", url)
			}
		}
		if int64(idF) == pending && hasURL {
			t.Errorf("pending video should not expose thumbnail_url yet")
		}
	}
	// Only the never-attempted (pending) video is enqueued for Tier 3.
	if len(thumbs.enqueued) != 1 || thumbs.enqueued[0] != pending {
		t.Errorf("enqueued = %v, want [%d]", thumbs.enqueued, pending)
	}
}

func TestAdminStatus(t *testing.T) {
	srv, _, _ := thumbServer(t, &stubThumbs{enabled: true, depth: 7})
	code, body := getJSON(t, srv.URL+"/api/v1/admin/status")
	if code != 200 || body["thumbnail_queue_depth"].(float64) != 7 {
		t.Fatalf("admin status: code=%d body=%v", code, body)
	}
}
