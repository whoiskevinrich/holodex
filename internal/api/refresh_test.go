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
	"holodex/internal/refresh"
	"holodex/internal/repo"
)

// stubFileExtractor returns a canned video without touching disk — the API test
// exercises the HTTP contract (auth + status mapping) against a real repo, not
// real exiftool/ffprobe extraction (covered by the refresh + scanner unit tests).
type stubFileExtractor struct{}

func (stubFileExtractor) BuildVideoFromFile(_ context.Context, path string) (*model.Video, []model.ExtraMetadata, error) {
	return &model.Video{
		FilePath:  path,
		Title:     "Refreshed",
		Width:     1920,
		Height:    1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil, nil
}

// refreshServer wires the owner-gated refresh endpoint over a real repo + a stub
// file extractor and no providers (file-only). token="" leaves the gate open;
// wire=false leaves the service unset (to exercise the 503 path).
func refreshServer(t *testing.T, token string, wire bool) (*httptest.Server, *repo.Repo) {
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
	if wire {
		h.SetRefresh(refresh.NewService(stubFileExtractor{}, r, nil, log))
	}
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r
}

func seedRefreshVideo(t *testing.T, r *repo.Repo, path string) int64 {
	t.Helper()
	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: path, Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	return id
}

func refreshPOST(t *testing.T, url, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, nil)
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// The endpoint maps each case to the documented status (ADR-047): live → 202,
// unknown → 404, soft-deleted → 409, bad id → 400.
func TestRefreshEndpointStatuses(t *testing.T) {
	srv, r := refreshServer(t, "", true)
	id := seedRefreshVideo(t, r, "/m/clip.mp4")
	base := srv.URL + "/api/v1/media/"

	if code := refreshPOST(t, base+itoa(id)+"/refresh", ""); code != http.StatusAccepted {
		t.Fatalf("live item: want 202, got %d", code)
	}
	if code := refreshPOST(t, base+"999999/refresh", ""); code != http.StatusNotFound {
		t.Fatalf("unknown id: want 404, got %d", code)
	}
	if code := refreshPOST(t, base+"abc/refresh", ""); code != http.StatusBadRequest {
		t.Fatalf("invalid id: want 400, got %d", code)
	}

	if err := r.SoftDelete(context.Background(), id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if code := refreshPOST(t, base+itoa(id)+"/refresh", ""); code != http.StatusConflict {
		t.Fatalf("soft-deleted item: want 409, got %d", code)
	}
}

// With a token configured the endpoint is owner-gated: rejected without it,
// accepted with it (ADR-030).
func TestRefreshEndpointRequiresOwner(t *testing.T) {
	srv, r := refreshServer(t, "s3cret", true)
	url := srv.URL + "/api/v1/media/" + itoa(seedRefreshVideo(t, r, "/m/x.mp4")) + "/refresh"

	if code := refreshPOST(t, url, ""); code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", code)
	}
	if code := refreshPOST(t, url, "s3cret"); code != http.StatusAccepted {
		t.Fatalf("valid token: want 202, got %d", code)
	}
}

// With no refresh service wired the endpoint reports unavailable rather than
// panicking on a nil service.
func TestRefreshEndpointDisabled(t *testing.T) {
	srv, r := refreshServer(t, "", false)
	url := srv.URL + "/api/v1/media/" + itoa(seedRefreshVideo(t, r, "/m/y.mp4")) + "/refresh"
	if code := refreshPOST(t, url, ""); code != http.StatusServiceUnavailable {
		t.Fatalf("unwired service: want 503, got %d", code)
	}
}
