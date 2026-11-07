package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"holodex/internal/api"
	"holodex/internal/maintenance"
	"holodex/internal/repo"
)

func trimJPEG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewGray(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// postTrim sends a trim request. fields overrides the good defaults; an empty value drops the field.
func postTrim(t *testing.T, url, token string, fields map[string]string, captures map[string][]byte) int {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	values := map[string]string{"in_time": "1.5", "out_time": "30", "mode": "replace"}
	for k, v := range fields {
		values[k] = v
	}
	for k, v := range values {
		if v != "" {
			if err := mw.WriteField(k, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	if captures == nil {
		captures = map[string][]byte{"capture_in": trimJPEG(t), "capture_out": trimJPEG(t)}
	}
	for k, v := range captures {
		w, err := mw.CreateFormFile(k, k+".jpg")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(v); err != nil {
			t.Fatal(err)
		}
	}
	mw.Close()
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func trimJob(t *testing.T, r *repo.Repo, id int64) maintenance.Payload {
	t.Helper()
	ms, err := r.GetVideoMaintenanceStatus(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var p maintenance.Payload
	if err := json.Unmarshal([]byte(ms.Payload), &p); err != nil {
		t.Fatalf("payload %q: %v", ms.Payload, err)
	}
	return p
}

func trimURL(srv *httptest.Server, id int64) string { return maintURL(srv, id, "/trim") }

// A1: a visitor can neither check, queue nor dismiss.
func TestTrimRoutes_OwnerGated(t *testing.T) {
	srv, _, id, fm := maintServer(t, "secret")
	if code := postTrim(t, trimURL(srv, id), "", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("POST trim as visitor: %d", code)
	}
	if code, _ := getTok(t, maintURL(srv, id, "/preflight?op=trim&mode=replace"), ""); code != http.StatusUnauthorized {
		t.Errorf("preflight as visitor: %d", code)
	}
	if fm.checks != 0 || fm.kicks != 0 {
		t.Errorf("the worker was reached: %+v", fm)
	}
}

func TestEnqueueTrim(t *testing.T) {
	srv, r, id, fm := maintServer(t, "secret")
	code := postTrim(t, trimURL(srv, id), "secret", map[string]string{"mode": "edition", "edition": "Trimmed"}, nil)
	if code != http.StatusAccepted {
		t.Fatalf("POST: %d", code)
	}
	if fm.kicks != 1 {
		t.Errorf("kicks = %d", fm.kicks)
	}
	if fm.mode != "edition" || fm.edition != "Trimmed" {
		t.Errorf("preflight saw mode %q name %q", fm.mode, fm.edition)
	}
	if math.Abs(fm.kept-28.5/60) > 1e-9 {
		t.Errorf("kept = %v, want the range's share of 60 s", fm.kept)
	}
	p := trimJob(t, r, id)
	if p.Op != string(maintenance.OpTrim) || p.Trim == nil || p.Trim.InTime != 1.5 || p.Trim.OutTime != 30 || p.Trim.Edition != "Trimmed" {
		t.Fatalf("payload = %+v", p)
	}
	// A3: the folder name is generated here, and both captures are in it.
	if len(p.Trim.Captures) != 32 || strings.ContainsAny(p.Trim.Captures, `/\.`) {
		t.Errorf("captures folder = %q", p.Trim.Captures)
	}
	for _, f := range []string{"in.jpg", "out.jpg"} {
		if _, err := os.Stat(fm.captureRoot + "/" + p.Trim.Captures + "/" + f); err != nil {
			t.Errorf("capture %s: %v", f, err)
		}
	}
}

// A2, A3: every way a request can be wrong is refused, and nothing is queued.
func TestEnqueueTrimRefusals(t *testing.T) {
	cases := []struct {
		name     string
		fields   map[string]string
		captures func(t *testing.T) map[string][]byte
	}{
		{"in after out", map[string]string{"in_time": "40", "out_time": "30"}, nil},
		{"in equal out", map[string]string{"in_time": "30", "out_time": "30"}, nil},
		{"negative in", map[string]string{"in_time": "-1"}, nil},
		{"not a number", map[string]string{"in_time": "NaN"}, nil},
		{"unbounded", map[string]string{"out_time": "Inf"}, nil},
		{"missing time", map[string]string{"in_time": ""}, nil},
		{"past the end", map[string]string{"in_time": "500", "out_time": "600"}, nil},
		{"unknown mode", map[string]string{"mode": "both"}, nil},
		{"missing capture", nil, func(t *testing.T) map[string][]byte { return map[string][]byte{"capture_in": trimJPEG(t)} }},
		{"not a JPEG", nil, func(t *testing.T) map[string][]byte {
			return map[string][]byte{"capture_in": []byte("<svg/>"), "capture_out": trimJPEG(t)}
		}},
		{"oversize capture", nil, func(t *testing.T) map[string][]byte {
			return map[string][]byte{"capture_in": append(trimJPEG(t), make([]byte, 9<<20)...), "capture_out": trimJPEG(t)}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, r, id, fm := maintServer(t, "secret")
			var captures map[string][]byte
			if c.captures != nil {
				captures = c.captures(t)
			}
			if code := postTrim(t, trimURL(srv, id), "secret", c.fields, captures); code != http.StatusBadRequest {
				t.Errorf("POST: %d, want 400", code)
			}
			ms, _ := r.GetVideoMaintenanceStatus(context.Background(), id)
			if ms.Pending || fm.kicks != 0 {
				t.Errorf("queued anyway: %+v", ms)
			}
			if entries, _ := os.ReadDir(fm.captureRoot); len(entries) != 0 {
				t.Errorf("%d capture folders left", len(entries))
			}
		})
	}
}

func TestEnqueueTrimRefusedByPreflightQueuesNothing(t *testing.T) {
	srv, r, id, fm := maintServer(t, "secret")
	fm.pf = maintenance.Preflight{Reason: maintenance.ReasonTargetExists}
	if code := postTrim(t, trimURL(srv, id), "secret", map[string]string{"mode": "edition", "edition": "x"}, nil); code != http.StatusConflict {
		t.Fatalf("POST: %d, want 409", code)
	}
	if ms, _ := r.GetVideoMaintenanceStatus(context.Background(), id); ms.Pending {
		t.Error("queued a refused trim")
	}
	if entries, _ := os.ReadDir(fm.captureRoot); len(entries) != 0 {
		t.Errorf("%d capture folders left", len(entries))
	}
}

// A4: a second job on the same video is refused, and its captures aren't kept.
func TestEnqueueTrimInFlight(t *testing.T) {
	srv, _, id, fm := maintServer(t, "secret")
	if code := postTrim(t, trimURL(srv, id), "secret", nil, nil); code != http.StatusAccepted {
		t.Fatalf("first: %d", code)
	}
	if code := postTrim(t, trimURL(srv, id), "secret", nil, nil); code != http.StatusConflict {
		t.Fatalf("second: %d, want 409", code)
	}
	if entries, _ := os.ReadDir(fm.captureRoot); len(entries) != 1 {
		t.Errorf("%d capture folders, want the first job's only", len(entries))
	}
}

func TestDismissFailedTrimRemovesItsCaptures(t *testing.T) {
	srv, r, id, fm := maintServer(t, "secret")
	ctx := context.Background()
	if code := postTrim(t, trimURL(srv, id), "secret", nil, nil); code != http.StatusAccepted {
		t.Fatalf("POST: %d", code)
	}
	job, err := r.ClaimNextMaintenance(ctx)
	if err != nil || job == nil {
		t.Fatalf("claim: %v, %v", job, err)
	}
	if err := r.FinishWriteback(ctx, job.ID, false, "verification failed (cut_check): first:2"); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(fm.captureRoot); len(entries) != 1 {
		t.Fatalf("a failed trim keeps its captures for Retry: %d folders", len(entries))
	}
	if code, _ := postTok(t, maintURL(srv, id, "/dismiss"), "secret", map[string]string{}); code != http.StatusOK {
		t.Fatalf("dismiss: %d", code)
	}
	if entries, _ := os.ReadDir(fm.captureRoot); len(entries) != 0 {
		t.Errorf("%d capture folders after dismissing", len(entries))
	}
}
