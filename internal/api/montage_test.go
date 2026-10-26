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

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/montage"
	"holodex/internal/repo"
)

// stubMontages implements the API's montager seam.
type stubMontages struct {
	mode      montage.Mode
	requested []int64
	visible   [][]int64
	status    montage.Status
}

func (s *stubMontages) Mode() montage.Mode { return s.mode }
func (s *stubMontages) Request(id int64) error {
	if s.mode == montage.ModeOff {
		return montage.ErrDisabled
	}
	s.requested = append(s.requested, id)
	return nil
}
func (s *stubMontages) EnqueueVisible(_ context.Context, ids []int64) {
	s.visible = append(s.visible, ids)
}
func (s *stubMontages) Status(context.Context, int64) (montage.Status, error) { return s.status, nil }
func (s *stubMontages) QueueStats() montage.Stats                             { return montage.Stats{Mode: s.mode} }

func montageServer(t *testing.T, m *stubMontages, token string) (*httptest.Server, *repo.Repo, string) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	montageDir := filepath.Join(dir, "montages")
	if err := os.MkdirAll(montageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth(token), false)
	h.SetMontages(m, montageDir)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, montageDir
}

// A1: the request route is owner-only, and refused in off mode.
func TestRequestMontage_OwnerGated(t *testing.T) {
	m := &stubMontages{mode: montage.ModeManual}
	srv, r, _ := montageServer(t, m, "secret")
	id := seedThumbVideo(t, r, "/m/a.mkv")
	url := srv.URL + "/api/v1/media/" + itoa(id) + "/montage"

	if code, _ := postTok(t, url, "", nil); code != http.StatusUnauthorized {
		t.Fatalf("visitor request code = %d, want 401", code)
	}
	if len(m.requested) != 0 {
		t.Fatalf("visitor request reached the pipeline: %v", m.requested)
	}
	if code, _ := postTok(t, url, "secret", nil); code != http.StatusAccepted {
		t.Fatalf("owner request code = %d, want 202", code)
	}
	if len(m.requested) != 1 || m.requested[0] != id {
		t.Errorf("requested = %v, want [%d]", m.requested, id)
	}
	if code, _ := postTok(t, srv.URL+"/api/v1/media/999999/montage", "secret", nil); code != http.StatusNotFound {
		t.Errorf("unknown id code = %d, want 404", code)
	}
	m.mode = montage.ModeOff
	if code, _ := postTok(t, url, "secret", nil); code != http.StatusConflict {
		t.Errorf("off mode code = %d, want 409", code)
	}
}

// A2: serving — 404 until the file exists, then video/mp4 with Range support.
func TestServeMontage(t *testing.T) {
	srv, r, dir := montageServer(t, &stubMontages{mode: montage.ModeManual}, "")
	id := seedThumbVideo(t, r, "/m/a.mkv")
	url := srv.URL + "/api/v1/media/" + itoa(id) + "/montage"

	if code, _ := getJSON(t, url); code != http.StatusNotFound {
		t.Fatalf("missing montage code = %d, want 404", code)
	}
	body := strings.Repeat("M", 1000)
	if err := os.WriteFile(montage.Path(dir, id), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "video/mp4" {
		t.Fatalf("GET = %d %q, want 200 video/mp4", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Range", "bytes=0-99")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Errorf("Range GET = %d, want 206", resp.StatusCode)
	}
	if err := r.SoftDelete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if code, _ := getJSON(t, url); code != http.StatusNotFound {
		t.Errorf("soft-deleted montage code = %d, want 404", code)
	}
}

// A3 + A5: montage_url only when the file exists; a browse bumps the manager
// only in automatic mode.
func TestMontageURLAndBrowseBump(t *testing.T) {
	m := &stubMontages{mode: montage.ModeManual}
	srv, r, dir := montageServer(t, m, "")
	withFile := seedThumbVideo(t, r, "/m/a.mkv")
	without := seedThumbVideo(t, r, "/m/b.mkv")
	if err := os.WriteFile(montage.Path(dir, withFile), []byte("M"), 0o644); err != nil {
		t.Fatal(err)
	}

	urls := func() map[int64]string {
		t.Helper()
		code, body := getJSON(t, srv.URL+"/api/v1/media")
		if code != http.StatusOK {
			t.Fatalf("list code = %d", code)
		}
		items, _ := body["items"].([]any)
		out := map[int64]string{}
		for _, it := range items {
			row, _ := it.(map[string]any)
			id, _ := row["id"].(float64)
			u, _ := row["montage_url"].(string)
			out[int64(id)] = u
		}
		return out
	}
	got := urls()
	if !strings.HasPrefix(got[withFile], "/api/v1/media/"+itoa(withFile)+"/montage?v=") {
		t.Errorf("montage_url = %q, want a versioned URL", got[withFile])
	}
	if got[without] != "" {
		t.Errorf("montage_url for a video with no file = %q, want empty", got[without])
	}
	if len(m.visible) != 0 {
		t.Errorf("manual-mode browse reached the manager: %v", m.visible)
	}

	m.mode = montage.ModeAutomatic
	urls()
	if len(m.visible) != 1 || len(m.visible[0]) != 1 || m.visible[0][0] != without {
		t.Errorf("automatic browse bumped %v, want [[%d]]", m.visible, without)
	}
}

// A4: the status route is owner-only.
func TestMontageStatus_OwnerOnly(t *testing.T) {
	m := &stubMontages{mode: montage.ModeManual, status: montage.Status{State: "ready", SizeBytes: 248}}
	srv, r, _ := montageServer(t, m, "secret")
	id := seedThumbVideo(t, r, "/m/a.mkv")
	url := srv.URL + "/api/v1/media/" + itoa(id) + "/montage/status"
	if code, _ := getTok(t, url, ""); code != http.StatusUnauthorized {
		t.Fatalf("visitor status code = %d, want 401", code)
	}
	code, st := getTok(t, url, "secret")
	if code != http.StatusOK {
		t.Fatalf("owner status code = %d", code)
	}
	if st["mode"] != "manual" || st["state"] != "ready" || st["size_bytes"] != float64(248) {
		t.Errorf("status = %v", st)
	}
}
