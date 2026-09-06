package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

// syncWritebackServer wires the legacy synchronous writeback path (no queue) over
// a real repo with a "title" (mapped for every container) and "director" (mapped
// for none — see internal/writeback/tags.go's formatMap) canonical field, plus a
// capturing WriteBatchFunc, so tests can drive the actual POST /media/{id}/writeback
// mixed-batch behavior (HOLODEX-216) and the GET /media/{id} write_target stamping
// end to end.
func syncWritebackServer(t *testing.T) (srv *httptest.Server, vid int64, r *repo.Repo, written *[]writeback.FieldWrite) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r = repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx := context.Background()
	vid, err = r.UpsertVideo(ctx, &model.Video{
		FilePath: filepath.Join(dir, "v.mp4"), FileSize: 1, Title: "A", Container: "MP4",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [file:title]\n" +
		"  - canonical: director\n    label: Director\n    sources: [tmdb:director]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	written = &[]writeback.FieldWrite{}
	h.SetWriteback(func(_ context.Context, _ string, fields []writeback.FieldWrite) error {
		*written = append(*written, fields...)
		return nil
	})
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, vid, r, written
}

// TestWritebackEndpoint_MixedBatchReportsWrittenAndSkipped covers HOLODEX-216: a
// batch mixing a mapped field (title) and an unmapped one (director — mapped for
// no container) must write only the mappable field and report exactly that in the
// response, rather than the old bare 204 that let the unmapped field vanish
// without a trace.
func TestWritebackEndpoint_MixedBatchReportsWrittenAndSkipped(t *testing.T) {
	srv, vid, _, written := syncWritebackServer(t)

	body, _ := json.Marshal(map[string]any{
		"fields": []map[string]any{
			{"field": "title", "values": []string{"New Title"}, "source": "file:title"},
			{"field": "director", "values": []string{"Someone"}, "source": "tmdb:director"},
		},
	})
	resp, err := http.Post(srv.URL+"/api/v1/media/"+strconv.FormatInt(vid, 10)+"/writeback",
		"application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("POST writeback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("writeback status = %d, want 200", resp.StatusCode)
	}
	var out struct {
		Written []string `json:"written"`
		Skipped []string `json:"skipped"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode writeback response: %v", err)
	}
	if len(out.Written) != 1 || out.Written[0] != "title" {
		t.Errorf("written = %v, want [title]", out.Written)
	}
	if len(out.Skipped) != 1 || out.Skipped[0] != "director" {
		t.Errorf("skipped = %v, want [director]", out.Skipped)
	}
	if len(*written) != 1 || (*written)[0].TagName != "QuickTime:Title" {
		t.Fatalf("actually-written fields = %+v, want only QuickTime:Title", *written)
	}
}

// TestGetMedia_WriteTarget covers HOLODEX-216's other half: the resolved[] payload
// names each field's destination file tag for the video's actual container, empty
// for a canonical with no mapping there, so the dialog can show the target and
// disable a field that can never be written instead of offering it and silently
// dropping it on write.
func TestGetMedia_WriteTarget(t *testing.T) {
	srv, vid, r, _ := syncWritebackServer(t)

	// director needs a resolved value to appear in resolved[] at all; seeding it
	// via enrichment is what exercises "has a value, but no mapping for MP4".
	if err := r.UpsertEnrichment(context.Background(), model.EnrichEntityVideo, vid, "tmdb", "ext-1",
		map[string][]string{"director": {"Someone"}}); err != nil {
		t.Fatalf("seed director enrichment: %v", err)
	}

	resp, err := http.Get(srv.URL + "/api/v1/media/" + strconv.FormatInt(vid, 10))
	if err != nil {
		t.Fatalf("GET media: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Resolved []struct {
			Canonical   string `json:"canonical"`
			WriteTarget string `json:"write_target"`
		} `json:"resolved"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode media: %v", err)
	}
	targets := make(map[string]string, len(out.Resolved))
	for _, f := range out.Resolved {
		targets[f.Canonical] = f.WriteTarget
	}
	if targets["title"] != "QuickTime:Title" {
		t.Errorf("title write_target = %q, want QuickTime:Title", targets["title"])
	}
	if got, ok := targets["director"]; !ok || got != "" {
		t.Errorf("director write_target = %q (present=%v), want empty (no mapping for MP4)", got, ok)
	}
}

// jobStatusURL builds the status URL for one queued write.
func jobStatusURL(base string, jobID int64) string {
	return base + "/api/v1/writeback/jobs/" + strconv.FormatInt(jobID, 10)
}

// The job-status endpoint is what lets the SPA wait for a queued write before
// refetching (HOLODEX-214) — the writeback POST's 202 only means "accepted", and
// refetching on that answer is what produced the stale "N out of sync" header.
func TestWritebackJobStatus(t *testing.T) {
	srv, r, videoID := decisionServer(t, "")
	ctx := context.Background()

	jobID, err := r.EnqueueWriteback(ctx, videoID, `[{"field":"title","values":["X"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	code, body := getJSONTok(t, jobStatusURL(srv.URL, jobID), "")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if body["status"] != "pending" {
		t.Errorf("in-flight job = %v, want pending", body)
	}
	if _, ok := body["error"]; ok {
		t.Errorf("in-flight job must carry no error, got %v", body)
	}

	// A failed job reports the queue's own error so the dialog can show that
	// rather than a generic failure.
	if err := r.FinishWriteback(ctx, jobID, false, "mkvpropedit exploded"); err != nil {
		t.Fatalf("fail job: %v", err)
	}
	code, body = getJSONTok(t, jobStatusURL(srv.URL, jobID), "")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if body["status"] != "failed" || body["error"] != "mkvpropedit exploded" {
		t.Errorf("failed job = %v, want failed carrying the error", body)
	}

	// Success deletes the row, so an absent one reads as done — without that the
	// poller would never terminate on the happy path.
	done, err := r.EnqueueWriteback(ctx, videoID, `[{"field":"title","values":["Y"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := r.FinishWriteback(ctx, done, true, ""); err != nil {
		t.Fatalf("finish job: %v", err)
	}
	if _, body = getJSONTok(t, jobStatusURL(srv.URL, done), ""); body["status"] != "done" {
		t.Errorf("completed job = %v, want done", body)
	}
}

// A non-numeric or non-positive id is rejected outright rather than reaching the repo.
func TestWritebackJobStatus_RejectsBadID(t *testing.T) {
	srv, _, _ := decisionServer(t, "")
	for _, id := range []string{"not-a-number", "0", "-3"} {
		if code, _ := getJSONTok(t, srv.URL+"/api/v1/writeback/jobs/"+id, ""); code != http.StatusBadRequest {
			t.Errorf("id %q = %d, want 400", id, code)
		}
	}
}

// The endpoint sits behind requireOwner with the rest of the writeback surface:
// queue state is owner-only, like the writes that produce it.
func TestWritebackJobStatus_OwnerGated(t *testing.T) {
	srv, r, videoID := decisionServer(t, "secret")
	jobID, err := r.EnqueueWriteback(context.Background(), videoID, `[]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if code, _ := getJSONTok(t, jobStatusURL(srv.URL, jobID), ""); code != http.StatusUnauthorized {
		t.Errorf("unauthenticated = %d, want 401", code)
	}
	if code, _ := getJSONTok(t, jobStatusURL(srv.URL, jobID), "secret"); code != http.StatusOK {
		t.Errorf("owner = %d, want 200", code)
	}
}

// writebackStatusServer wires GET /media/{id} plus the Retry/Dismiss endpoints
// (ADR-091, HOLODEX-323) over a real repo and a real write queue whose WriteFunc
// is a no-op — this suite drives job state directly via r.EnqueueWriteback /
// r.FinishWriteback, the same minimal-mocking approach TestWritebackJobStatus
// above uses, rather than running the queue's own worker loop (which has its own
// coverage in internal/writequeue). No mapping store is wired: these tests only
// assert on the writeback_status field, which getMedia computes independently of
// h.mappings being set.
func writebackStatusServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	videoID, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: filepath.Join(dir, "v.mp4"), FileSize: 1, Title: "T", Container: "MP4",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	q := writequeue.New(r, func(context.Context, string, []writeback.FieldWrite) error { return nil }, log, 1, "")
	h.SetWriteQueue(q)
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, videoID
}

// getMediaWritebackStatus decodes just the field this suite cares about out of
// GET /media/{id}, tolerating its absence (a video with no writeback activity, or
// the redacted-for-visitor case) as the zero value rather than a decode error.
func getMediaWritebackStatus(t *testing.T, base string, videoID int64, token string) (int, struct {
	Pending bool   `json:"pending"`
	Failed  bool   `json:"failed"`
	Error   string `json:"error"`
}) {
	t.Helper()
	var out struct {
		WritebackStatus struct {
			Pending bool   `json:"pending"`
			Failed  bool   `json:"failed"`
			Error   string `json:"error"`
		} `json:"writeback_status"`
	}
	req, _ := http.NewRequest(http.MethodGet, base+"/api/v1/media/"+strconv.FormatInt(videoID, 10), nil)
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET media: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode media: %v", err)
		}
	}
	return resp.StatusCode, out.WritebackStatus
}

// TestGetMedia_WritebackStatusRedactedForVisitor is the security-review finding
// closed in the spec (R2.1a) before this code existed: every failure path in
// internal/writeback/writeback.go embeds absolute filesystem paths, and
// GET /media/{id} is not owner-gated — a visitor's browser reaches this response
// regardless of what the SPA chooses to render. redactFileMetadataForVisitor
// already strips FilePath/codecs/container from this exact payload for the same
// reason; the writeback error must get the same treatment, or a new field would
// reopen the leak that redaction exists to close. Booleans carry no such risk and
// must remain for both — R2.1a only redacts the message.
func TestGetMedia_WritebackStatusRedactedForVisitor(t *testing.T) {
	srv, r, videoID := writebackStatusServer(t, "secret")
	jobID, err := r.EnqueueWriteback(context.Background(), videoID, `[{"field":"title","values":["X"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	const pathBearingError = `writeback rename: rename /media/library/2019/file.mkv.holodex-tmp /media/library/2019/file.mkv: permission denied`
	if err := r.FinishWriteback(context.Background(), jobID, false, pathBearingError); err != nil {
		t.Fatalf("fail job: %v", err)
	}

	code, owner := getMediaWritebackStatus(t, srv.URL, videoID, "secret")
	if code != http.StatusOK {
		t.Fatalf("owner GET media = %d, want 200", code)
	}
	if !owner.Failed || owner.Error != pathBearingError {
		t.Errorf("owner writeback_status = %+v, want Failed=true carrying the full message", owner)
	}

	code, visitor := getMediaWritebackStatus(t, srv.URL, videoID, "")
	if code != http.StatusOK {
		t.Fatalf("visitor GET media = %d, want 200 (the route itself is not owner-gated)", code)
	}
	if !visitor.Failed {
		t.Errorf("visitor writeback_status.failed = %v, want true — the boolean discloses nothing and must survive redaction", visitor.Failed)
	}
	if visitor.Error != "" {
		t.Errorf("visitor writeback_status.error = %q, want empty — this is the exact leak R2.1a exists to close", visitor.Error)
	}
}

// TestRetryDismissWriteback_OwnerGated: both are new mutation routes and Dismiss
// deletes a row, so they get the same requireOwner scrutiny as every other
// writeback endpoint (spec R3.6).
func TestRetryDismissWriteback_OwnerGated(t *testing.T) {
	srv, r, videoID := writebackStatusServer(t, "secret")
	jobID, err := r.EnqueueWriteback(context.Background(), videoID, `[{"field":"title","values":["X"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := r.FinishWriteback(context.Background(), jobID, false, "exiftool exploded"); err != nil {
		t.Fatalf("fail job: %v", err)
	}

	retryURL := srv.URL + "/api/v1/media/" + strconv.FormatInt(videoID, 10) + "/writeback/retry"
	dismissURL := srv.URL + "/api/v1/media/" + strconv.FormatInt(videoID, 10) + "/writeback/dismiss"

	if code, _ := postTok(t, retryURL, "", nil); code != http.StatusUnauthorized {
		t.Errorf("unauthenticated retry = %d, want 401", code)
	}
	if code, _ := postTok(t, dismissURL, "", nil); code != http.StatusUnauthorized {
		t.Errorf("unauthenticated dismiss = %d, want 401", code)
	}
	if code, body := postTok(t, retryURL, "secret", nil); code != http.StatusOK || body["retried"] != true {
		t.Errorf("owner retry = %d/%v, want 200 with retried:true", code, body)
	}
}

// TestRetryWriteback_ResetsFailedToPending covers spec R3.3 end to end through
// the HTTP layer: the failed row goes back to pending and the poll-facing
// GET /media/{id} reflects it — Retry is a status reset plus a queue kick, not a
// re-submission of the original write.
func TestRetryWriteback_ResetsFailedToPending(t *testing.T) {
	srv, r, videoID := writebackStatusServer(t, "")
	jobID, err := r.EnqueueWriteback(context.Background(), videoID, `[{"field":"title","values":["X"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := r.FinishWriteback(context.Background(), jobID, false, "exiftool exploded"); err != nil {
		t.Fatalf("fail job: %v", err)
	}

	retryURL := srv.URL + "/api/v1/media/" + strconv.FormatInt(videoID, 10) + "/writeback/retry"
	if code, body := postTok(t, retryURL, "", nil); code != http.StatusOK || body["retried"] != true {
		t.Fatalf("retry = %d/%v, want 200 with retried:true", code, body)
	}

	_, status := getMediaWritebackStatus(t, srv.URL, videoID, "")
	if !status.Pending || status.Failed {
		t.Errorf("status after retry = %+v, want Pending=true Failed=false", status)
	}

	// A safe no-op — never a 404 — when there is nothing left to retry.
	if code, body := postTok(t, retryURL, "", nil); code != http.StatusOK || body["retried"] != false {
		t.Errorf("second retry (nothing failed) = %d/%v, want 200 with retried:false", code, body)
	}
}

// TestDismissWriteback_DeletesRow covers spec R3.4/RD2: Dismiss clears the badge
// without retrying — the job_runs audit trail (recorded by the real queue worker,
// not exercised by this repo-level drive) is what's meant to survive, not this row.
func TestDismissWriteback_DeletesRow(t *testing.T) {
	srv, r, videoID := writebackStatusServer(t, "")
	jobID, err := r.EnqueueWriteback(context.Background(), videoID, `[{"field":"title","values":["X"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := r.FinishWriteback(context.Background(), jobID, false, "exiftool exploded"); err != nil {
		t.Fatalf("fail job: %v", err)
	}

	dismissURL := srv.URL + "/api/v1/media/" + strconv.FormatInt(videoID, 10) + "/writeback/dismiss"
	if code, body := postTok(t, dismissURL, "", nil); code != http.StatusOK || body["dismissed"] != true {
		t.Fatalf("dismiss = %d/%v, want 200 with dismissed:true", code, body)
	}

	_, status := getMediaWritebackStatus(t, srv.URL, videoID, "")
	if status.Pending || status.Failed {
		t.Errorf("status after dismiss = %+v, want the zero value", status)
	}
}

// TestEnqueueWriteback_ClearsPriorFailedForVideo covers spec R3.5/RD5: submitting
// a new write for a video is an implicit acknowledgment of that video's own prior
// failure, scoped to it alone — a second video's failed row must survive
// untouched, or one owner action would silently erase an unrelated warning.
func TestEnqueueWriteback_ClearsPriorFailedForVideo(t *testing.T) {
	srv, r, videoID := writebackStatusServer(t, "")
	failedJob, err := r.EnqueueWriteback(context.Background(), videoID, `[{"field":"title","values":["X"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := r.FinishWriteback(context.Background(), failedJob, false, "exiftool exploded"); err != nil {
		t.Fatalf("fail job: %v", err)
	}

	otherID, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/other.mp4", FileSize: 1, Title: "Other", Container: "MP4",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed second video: %v", err)
	}
	otherJob, err := r.EnqueueWriteback(context.Background(), otherID, `[{"field":"title","values":["Y"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue for second video: %v", err)
	}
	if err := r.FinishWriteback(context.Background(), otherJob, false, "exiftool exploded"); err != nil {
		t.Fatalf("fail job for second video: %v", err)
	}

	body := map[string]any{"fields": []map[string]any{
		{"field": "title", "values": []string{"New Title"}, "source": "file:title"},
	}}
	writebackURL := srv.URL + "/api/v1/media/" + strconv.FormatInt(videoID, 10) + "/writeback"
	if code, respBody := postTok(t, writebackURL, "", body); code != http.StatusAccepted {
		t.Fatalf("writeback enqueue = %d/%v, want 202", code, respBody)
	}

	_, status := getMediaWritebackStatus(t, srv.URL, videoID, "")
	if !status.Pending || status.Failed {
		t.Errorf("this video's status = %+v, want Pending=true Failed=false (the old failure cleared)", status)
	}
	_, otherStatus := getMediaWritebackStatus(t, srv.URL, otherID, "")
	if !otherStatus.Failed {
		t.Errorf("other video's status = %+v, want Failed=true — untouched by this video's enqueue", otherStatus)
	}
}
