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
	"holodex/internal/purge"
	"holodex/internal/repo"
)

// deleteServer wires the soft-delete/purge surface with the real purger against a
// test DB. A single video ("Clip") is seeded; its id is returned. RemoveFiles is
// true but the seeded file path doesn't exist on disk, so purge-now treats the
// missing file as success and removes the row (F24.8).
func deleteServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
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
	h.SetAuth(api.NewAuth(token), false)
	grace := 7 * 24 * time.Hour
	h.SetDelete(purge.New(r, purge.Config{Grace: grace, RemoveFiles: true}, log), grace)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/x.mkv", Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	return srv, r, id
}

func TestDeleteEndpointsGated(t *testing.T) {
	srv, _, id := deleteServer(t, "s3cret")
	media := srv.URL + "/api/v1/media/" + itoa(id)

	// All mutating endpoints + the Trash list reject a missing token (401).
	if code := sendTok(t, http.MethodDelete, media, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token delete = %d, want 401", code)
	}
	if code := sendTok(t, http.MethodPost, media+"/restore", ""); code != http.StatusUnauthorized {
		t.Errorf("no-token restore = %d, want 401", code)
	}
	if code := sendTok(t, http.MethodGet, srv.URL+"/api/v1/admin/trash", ""); code != http.StatusUnauthorized {
		t.Errorf("no-token trash = %d, want 401", code)
	}
	// The item is still visible — nothing was deleted by the rejected calls.
	if code, _ := getJSON(t, media); code != http.StatusOK {
		t.Errorf("media after rejected deletes = %d, want 200", code)
	}
}

func TestSoftDeleteRestoreFlow(t *testing.T) {
	srv, _, id := deleteServer(t, "")
	media := srv.URL + "/api/v1/media/" + itoa(id)

	// Soft-delete → 204, then the item 404s and is in Trash.
	if code := sendTok(t, http.MethodDelete, media, ""); code != http.StatusNoContent {
		t.Fatalf("soft delete = %d, want 204", code)
	}
	if code, _ := getJSON(t, media); code != http.StatusNotFound {
		t.Errorf("media after soft-delete = %d, want 404", code)
	}
	// Idempotent: a second soft-delete is still 204.
	if code := sendTok(t, http.MethodDelete, media, ""); code != http.StatusNoContent {
		t.Errorf("second soft delete = %d, want 204 (idempotent)", code)
	}

	code, trash := getJSON(t, srv.URL+"/api/v1/admin/trash")
	if code != http.StatusOK {
		t.Fatalf("trash = %d, want 200", code)
	}
	items, _ := trash["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("trash items = %d, want 1", len(items))
	}
	if first, _ := items[0].(map[string]any); first["purge_at"] == nil {
		t.Errorf("trash item missing purge_at (grace > 0): %v", first)
	}

	// Restore → 200, item visible again, Trash empty.
	if code := sendTok(t, http.MethodPost, media+"/restore", ""); code != http.StatusOK {
		t.Errorf("restore = %d, want 200", code)
	}
	if code, _ := getJSON(t, media); code != http.StatusOK {
		t.Errorf("media after restore = %d, want 200", code)
	}
	if _, trash := getJSON(t, srv.URL+"/api/v1/admin/trash"); len(trash["items"].([]any)) != 0 {
		t.Errorf("trash after restore not empty: %v", trash["items"])
	}
}

func TestDeleteNotFoundPaths(t *testing.T) {
	srv, _, _ := deleteServer(t, "")
	unknown := srv.URL + "/api/v1/media/99999"

	if code := sendTok(t, http.MethodDelete, unknown, ""); code != http.StatusNotFound {
		t.Errorf("delete unknown = %d, want 404", code)
	}
	if code := sendTok(t, http.MethodPost, unknown+"/restore", ""); code != http.StatusNotFound {
		t.Errorf("restore unknown = %d, want 404", code)
	}
	// Restore of a live (never-deleted) item → 404 (nothing to restore, F24.6).
	if code := sendTok(t, http.MethodGet, srv.URL+"/api/v1/admin/trash", ""); code != http.StatusOK {
		t.Fatalf("trash precondition = %d", code)
	}
}

func TestPurgeNowEndpoint(t *testing.T) {
	srv, r, id := deleteServer(t, "")
	media := srv.URL + "/api/v1/media/" + itoa(id)

	// purge=true hard-deletes immediately (the missing seed file is treated as
	// already-gone success), bypassing the grace period.
	if code := sendTok(t, http.MethodDelete, media+"?purge=true", ""); code != http.StatusNoContent {
		t.Fatalf("purge now = %d, want 204", code)
	}
	if _, err := r.PurgePath(context.Background(), id); err != repo.ErrNotFound {
		t.Errorf("row still present after purge: %v", err)
	}
	// A second purge of the now-gone id → 404.
	if code := sendTok(t, http.MethodDelete, media+"?purge=true", ""); code != http.StatusNotFound {
		t.Errorf("purge gone id = %d, want 404", code)
	}
}
