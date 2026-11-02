package api_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"holodex/internal/api"
	"holodex/internal/rename"
)

// kickCounter stands in for the rename queue: the API only wakes it. The worker has its own tests
// (internal/rename).
type kickCounter struct{ kicks int }

func (k *kickCounter) Kick() { k.kicks++ }

func (f *renameFixture) queued(t *testing.T) int {
	t.Helper()
	jobs, err := f.r.BatchRenameJobs(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	n := len(jobs)
	for _, id := range f.ids {
		if ok, _ := f.r.HasRenameJob(context.Background(), id); ok {
			n++
		}
	}
	return n
}

func (f *renameFixture) apply(t *testing.T, body any) (int, map[string]any) {
	return doJSON(t, http.MethodPost, f.srv.URL+"/api/v1/rename/apply", "secret", body)
}

// A4: Off and Plan enqueue nothing and touch no file; Manual enqueues and wakes the queue.
func TestRenameApplyModeGating(t *testing.T) {
	f := renameServer(t, "secret")
	f.put(t, rename.KeyTemplates, `["[{studio}] {title}", "{title}"]`)
	before := treeOf(t, f.lib)
	ids := []int64{f.ids["junk1.mkv"], f.ids["junk2.mkv"]}
	for _, mode := range []string{rename.ModeOff, rename.ModePlan} {
		f.put(t, rename.KeyMode, mode)
		if code, _ := f.apply(t, map[string]any{"video_ids": ids}); code != http.StatusConflict {
			t.Errorf("%s: apply = %d, want 409", mode, code)
		}
	}
	if f.queued(t) != 0 || f.kick.kicks != 0 || !bytes.Equal(before, treeOf(t, f.lib)) {
		t.Fatal("Off or Plan queued work or touched a file")
	}
	f.put(t, rename.KeyMode, rename.ModeManual)
	code, body := f.apply(t, map[string]any{"video_ids": append(ids, f.ids["junk4.mkv"])})
	if code != http.StatusAccepted || body["queued"] != float64(2) || f.kick.kicks != 1 {
		t.Fatalf("manual apply = %d %v (kicks %d)", code, body, f.kick.kicks)
	}
	if nr := body["not_renamed"].(map[string]any); nr[rename.ReasonClash] != float64(1) {
		t.Errorf("not renamed = %v", nr)
	}
	// R24: the apply writes the batch's one summary row, counts only: queued, refused by reason.
	runs, err := f.r.ListJobRunsByBatch(context.Background(), body["batch_id"].(string))
	if err != nil || len(runs) != 1 {
		t.Fatalf("batch runs = %v, %v", runs, err)
	}
	if s := runs[0]; s.EntityID != 0 || s.Seen != 2 || s.Skipped != 1 || s.Detail != rename.ReasonClash+" 1" {
		t.Errorf("apply summary = %+v", s)
	}
	if !bytes.Equal(before, treeOf(t, f.lib)) {
		t.Error("the API renamed a file itself; only the worker may")
	}
}

// A2: the apply body carries video ids only; a path, folder or name is ignored, and the payload's
// name is the server's own plan.
func TestRenameApplyIgnoresPaths(t *testing.T) {
	f := renameServer(t, "secret")
	f.put(t, rename.KeyTemplates, `["{title}"]`)
	f.put(t, rename.KeyMode, rename.ModeManual)
	id := f.ids["junk2.mkv"]
	code, _ := f.apply(t, map[string]any{"video_ids": []int64{id}, "path": "/etc", "folder": "/tmp", "name": "../x.mkv"})
	if code != http.StatusAccepted {
		t.Fatalf("apply = %d", code)
	}
	var payload string
	if err := f.sqldb.QueryRow(`SELECT payload FROM writeback_queue WHERE video_id = ? AND kind = 'rename'`, id).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload, `"name":"Bob.mkv"`) || strings.Contains(payload, "etc") || strings.Contains(payload, "../") {
		t.Errorf("payload = %s", payload)
	}
}

// A9: an oversized apply body is refused before any row is queued.
func TestRenameApplyBodyBounded(t *testing.T) {
	f := renameServer(t, "secret")
	f.put(t, rename.KeyTemplates, `["{title}"]`)
	f.put(t, rename.KeyMode, rename.ModeManual)
	big := `{"video_ids":[` + strings.Repeat("1,", 1<<20) + `1]}`
	req, _ := http.NewRequest(http.MethodPost, f.srv.URL+"/api/v1/rename/apply", strings.NewReader(big))
	req.Header.Set(api.AdminTokenHeader, "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || f.queued(t) != 0 {
		t.Errorf("oversized body = %d, queued %d", resp.StatusCode, f.queued(t))
	}
}

// A8: a failed rename's path-bearing error never reaches a visitor's media payload.
func TestRenameErrorNotInVisitorPayload(t *testing.T) {
	f := renameServer(t, "secret")
	id := f.ids["junk1.mkv"]
	secretErr := "rename /very/secret/library/junk1.mkv: denied"
	if _, err := f.sqldb.Exec(`INSERT INTO writeback_queue (video_id, kind, payload, status, error, enqueued_at, updated_at)
		VALUES (?, 'rename', '{"name":"x.mkv"}', 'failed', ?, 'x', 'x')`, id, secretErr); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/media/" + itoa(id), "/api/v1/media"} {
		resp, err := http.Get(f.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if strings.Contains(string(b), "very/secret") || strings.Contains(string(b), "x.mkv") {
			t.Errorf("visitor %s leaks rename state: %s", path, b)
		}
	}
	if code, body := getTok(t, f.srv.URL+"/api/v1/media/"+itoa(id)+"/rename/status", "secret"); code != http.StatusOK || body["error"] != secretErr {
		t.Errorf("owner status = %d %v", code, body)
	}
}
