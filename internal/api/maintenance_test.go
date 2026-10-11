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
	"holodex/internal/maintenance"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// fakeMaint stands in for the maintenance worker: the preflight answer is set per test, and
// kicks are counted. The real CheckFlip has its own tests (internal/maintenance).
type fakeMaint struct {
	pf     maintenance.Preflight
	checks int
	kicks  int
}

func (f *fakeMaint) CheckFlip(context.Context, string) (maintenance.Preflight, error) {
	f.checks++
	return f.pf, nil
}
func (f *fakeMaint) Hold() time.Duration { return 7 * 24 * time.Hour }
func (f *fakeMaint) Kick()               { f.kicks++ }

func maintServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64, *fakeMaint) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth(token), false)
	fm := &fakeMaint{pf: maintenance.Preflight{OK: true, NeedBytes: 150, FreeBytes: 1 << 30}}
	h.SetMaintenance(fm)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/media/library/2019/file.mkv", Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return srv, r, id, fm
}

func maintURL(srv *httptest.Server, id int64, tail string) string {
	return srv.URL + "/api/v1/media/" + itoa(id) + "/maintenance" + tail
}

var flipBody = map[string]string{"op": "hflip"}

func TestMaintenanceRoutes_OwnerGated(t *testing.T) {
	srv, _, id, fm := maintServer(t, "secret")
	if code, _ := getTok(t, maintURL(srv, id, "/preflight?op=hflip"), ""); code != http.StatusUnauthorized {
		t.Errorf("visitor preflight = %d, want 401", code)
	}
	for _, tail := range []string{"", "/restore", "/discard", "/retry", "/dismiss"} {
		if code, _ := postTok(t, maintURL(srv, id, tail), "", flipBody); code != http.StatusUnauthorized {
			t.Errorf("visitor POST maintenance%s = %d, want 401", tail, code)
		}
	}
	if fm.checks != 0 || fm.kicks != 0 {
		t.Errorf("an unauthenticated request reached the worker (checks %d, kicks %d)", fm.checks, fm.kicks)
	}
}

func TestMaintenancePreflight_ReportsAndQuotesExpiry(t *testing.T) {
	srv, _, id, fm := maintServer(t, "secret")
	code, body := getTok(t, maintURL(srv, id, "/preflight?op=hflip"), "secret")
	if code != http.StatusOK || body["ok"] != true || body["need_bytes"] != float64(150) {
		t.Fatalf("preflight = %d %v", code, body)
	}
	exp, err := time.Parse(time.RFC3339, body["expires_at"].(string))
	if err != nil || time.Until(exp) < 6*24*time.Hour {
		t.Errorf("expires_at = %v (%v), want about a week out", body["expires_at"], err)
	}

	fm.pf = maintenance.Preflight{Reason: maintenance.ReasonReadOnly}
	if _, body := getTok(t, maintURL(srv, id, "/preflight?op=hflip"), "secret"); body["ok"] != false || body["reason"] != "read_only" {
		t.Errorf("refused preflight = %v", body)
	}
	if code, _ := getTok(t, maintURL(srv, id, "/preflight?op=rotate"), "secret"); code != http.StatusBadRequest {
		t.Errorf("unknown op = %d, want 400", code)
	}
	if code, _ := getTok(t, srv.URL+"/api/v1/media/99999/maintenance/preflight?op=hflip", "secret"); code != http.StatusNotFound {
		t.Errorf("missing video = %d, want 404", code)
	}
}

func TestEnqueueFlip_ReRunsPreflightAndRefusesASecond(t *testing.T) {
	srv, r, id, fm := maintServer(t, "secret")
	ctx := context.Background()

	// A dialog opened while things were fine, submitted after the library went read-only.
	fm.pf = maintenance.Preflight{Reason: maintenance.ReasonReadOnly}
	if code, body := postTok(t, maintURL(srv, id, ""), "secret", flipBody); code != http.StatusConflict || body["reason"] != "read_only" {
		t.Fatalf("enqueue past a refusal = %d %v, want 409 read_only", code, body)
	}
	if ms, _ := r.GetVideoMaintenanceStatus(ctx, id); ms.Pending {
		t.Fatal("a refused enqueue queued a job")
	}

	fm.pf = maintenance.Preflight{OK: true}
	if code, body := postTok(t, maintURL(srv, id, ""), "secret", flipBody); code != http.StatusAccepted || body["job_id"] == nil {
		t.Fatalf("enqueue = %d %v, want 202 with a job id", code, body)
	}
	if fm.kicks != 1 {
		t.Errorf("worker kicks = %d, want 1", fm.kicks)
	}

	checks := fm.checks
	if _, body := getTok(t, maintURL(srv, id, "/preflight?op=hflip"), "secret"); body["ok"] != false || body["reason"] != "in_flight" {
		t.Errorf("preflight while queued = %v, want in_flight", body)
	}
	if fm.checks != checks {
		t.Error("preflight ran the file checks for a video that already has a job")
	}
	if code, body := postTok(t, maintURL(srv, id, ""), "secret", flipBody); code != http.StatusConflict || body["reason"] != "in_flight" {
		t.Errorf("second enqueue = %d %v, want 409 in_flight", code, body)
	}
	if code, _ := postTok(t, maintURL(srv, id, ""), "secret", map[string]string{"op": "restore"}); code != http.StatusBadRequest {
		t.Errorf("restore through the flip route = %d, want 400", code)
	}
}

func TestEnqueueHeld_NeedsAStandingHold(t *testing.T) {
	srv, r, id, _ := maintServer(t, "secret")
	ctx := context.Background()
	for _, tail := range []string{"/restore", "/discard"} {
		if code, _ := postTok(t, maintURL(srv, id, tail), "secret", nil); code != http.StatusConflict {
			t.Errorf("%s with no hold = %d, want 409", tail, code)
		}
	}
	now := time.Now()
	if err := r.BeginSwap(ctx, id, "/media/library/2019/file.mkv.holodex-orig", "hflip", now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if code, _ := postTok(t, maintURL(srv, id, "/restore"), "secret", nil); code != http.StatusConflict {
		t.Errorf("restore mid-swap = %d, want 409", code)
	}
	if err := r.SetHeldState(ctx, id, repo.HeldHeld); err != nil {
		t.Fatal(err)
	}
	if code, _ := postTok(t, maintURL(srv, id, "/restore"), "secret", nil); code != http.StatusAccepted {
		t.Errorf("restore = %d, want 202", code)
	}
	ms, _ := r.GetVideoMaintenanceStatus(ctx, id)
	if !ms.Pending || ms.Payload != `{"op":"restore"}` {
		t.Errorf("queued = %+v", ms)
	}
}

// The tag-write status and the maintenance block never mix, and a visitor's payload carries
// no maintenance key at all, not even a redacted one.
func TestGetMedia_MaintenanceBlockIsOwnerOnly(t *testing.T) {
	srv, r, id, _ := maintServer(t, "secret")
	ctx := context.Background()
	const pathErr = `verification failed (tags): rename /media/library/2019/file.mkv.holodex-new: permission denied`
	if _, err := r.EnqueueMaintenance(ctx, id, `{"op":"hflip"}`); err != nil {
		t.Fatal(err)
	}
	job, _ := r.ClaimNextMaintenance(ctx)
	if err := r.FinishWriteback(ctx, job.ID, false, pathErr); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	_ = r.BeginSwap(ctx, id, "/media/library/2019/file.mkv.holodex-orig", "hflip", now, now.Add(time.Hour))
	_ = r.SetHeldState(ctx, id, repo.HeldHeld)

	_, owner := getTok(t, srv.URL+"/api/v1/media/"+itoa(id), "secret")
	m, ok := owner["maintenance"].(map[string]any)
	if !ok || m["failed"] != true || m["error"] != pathErr || m["op"] != "hflip" || m["held"] == nil {
		t.Errorf("owner maintenance block = %v", owner["maintenance"])
	}
	if wb, _ := owner["writeback_status"].(map[string]any); wb["failed"] == true || wb["pending"] == true {
		t.Errorf("a maintenance failure leaked into the tag-write status: %v", wb)
	}

	_, visitor := getTok(t, srv.URL+"/api/v1/media/"+itoa(id), "")
	if _, has := visitor["maintenance"]; has {
		t.Errorf("visitor payload carries a maintenance key: %v", visitor["maintenance"])
	}
}

// Retry and Dismiss act on their own lane only.
func TestRetryDismiss_StayInTheirLane(t *testing.T) {
	srv, r, id, fm := maintServer(t, "secret")
	ctx := context.Background()
	_, _ = r.EnqueueWriteback(ctx, id, `[]`, "")
	w, _ := r.ClaimNextWriteback(ctx)
	_ = r.FinishWriteback(ctx, w.ID, false, "tag write failed")
	_, _ = r.EnqueueMaintenance(ctx, id, `{"op":"hflip"}`)
	m, _ := r.ClaimNextMaintenance(ctx)
	_ = r.FinishWriteback(ctx, m.ID, false, "flip failed")

	if code, body := postTok(t, maintURL(srv, id, "/retry"), "secret", nil); code != http.StatusOK || body["retried"] != true {
		t.Fatalf("maintenance retry = %d %v", code, body)
	}
	if fm.kicks != 1 {
		t.Errorf("kicks = %d", fm.kicks)
	}
	if wb, _ := r.GetVideoWritebackStatus(ctx, id); !wb.Failed || wb.Pending {
		t.Errorf("maintenance retry touched the tag write: %+v", wb)
	}
	if code, body := postTok(t, srv.URL+"/api/v1/media/"+itoa(id)+"/writeback/dismiss", "secret", nil); code != http.StatusOK || body["dismissed"] != true {
		t.Fatalf("writeback dismiss = %d %v", code, body)
	}
	if ms, _ := r.GetVideoMaintenanceStatus(ctx, id); !ms.Pending {
		t.Errorf("tag-write dismiss removed the maintenance job: %+v", ms)
	}
}
