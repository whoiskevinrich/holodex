package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

// extractReviewServer wires the F48.6 review-queue endpoints over a real
// repo and (when wireQueue) a real write queue whose WriteFunc is a no-op —
// this suite only asserts a job was enqueued (Depth) and the review row's
// status flipped, never that a file was actually touched (writequeue has its
// own coverage for that).
func extractReviewServer(t *testing.T, wireQueue bool) (*httptest.Server, *repo.Repo, *writequeue.Queue) {
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

	var q *writequeue.Queue
	if wireQueue {
		q = writequeue.New(r, func(context.Context, string, []writeback.FieldWrite) error { return nil }, log, 1, "")
		h.SetWriteQueue(q)
	}
	h.SetAuth(api.NewAuth(""), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, q
}

func extractReviewPOST(t *testing.T, url string, body map[string]any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, _ := http.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func extractReviewGET(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestExtractionQueue_Empty(t *testing.T) {
	srv, _, _ := extractReviewServer(t, false)
	code, body := extractReviewGET(t, srv.URL+"/api/v1/owner/extraction-queue")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	rows, _ := body["rows"].([]any)
	if len(rows) != 0 {
		t.Fatalf("rows = %v, want empty", rows)
	}
}

func TestExtractionQueue_ListsPendingRowsVideoJoined(t *testing.T) {
	srv, r, _ := extractReviewServer(t, false)
	id := seedVideo(t, r, "/m/Big Movie.mkv", "Big Movie")
	if err := r.UpsertExtractionReview(context.Background(), id, "title", "New Title", "Old Title", 0.4, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	code, body := extractReviewGET(t, srv.URL+"/api/v1/owner/extraction-queue")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	rows, _ := body["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("rows = %v, want 1", rows)
	}
	row, _ := rows[0].(map[string]any)
	if row["video_title"] != "Big Movie" || row["file_path"] != "/m/Big Movie.mkv" {
		t.Fatalf("row missing video join: %v", row)
	}
}

func TestResolveExtractionReview_AcceptFilenameEnqueuesWrite(t *testing.T) {
	srv, r, q := extractReviewServer(t, true)
	ctx := context.Background()
	id := seedVideo(t, r, "/m/Big Movie.mkv", "Old Title")
	if err := r.UpsertExtractionReview(ctx, id, "title", "New Title", "Old Title", 0.4, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	pending, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)
	if err != nil || len(pending) != 1 {
		t.Fatalf("want 1 pending, got %d err=%v", len(pending), err)
	}

	url := srv.URL + "/api/v1/owner/extraction-review/" + itoa(pending[0].ID) + "/resolve"
	code, body := extractReviewPOST(t, url, map[string]any{"action": "filename"})
	// 200 + job_id, not 204: a resolution that enqueues a write hands the job id back so
	// the caller can wait on the real completion signal (the queue re-extracts before
	// marking a job done, ADR-073) instead of guessing when the value is readable again.
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d: %v", code, body)
	}
	if jobID, _ := body["job_id"].(float64); jobID <= 0 {
		t.Fatalf("response carried no usable job_id: %v", body)
	}

	depth, err := q.Depth(ctx)
	if err != nil || depth != 1 {
		t.Fatalf("queue depth = %d err=%v, want 1 (a write was enqueued)", depth, err)
	}
	if remaining, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending); err != nil || len(remaining) != 0 {
		t.Fatalf("want 0 pending after resolve, got %d err=%v", len(remaining), err)
	}
	resolved, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewResolved)
	if err != nil || len(resolved) != 1 {
		t.Fatalf("want 1 resolved row, got %d err=%v", len(resolved), err)
	}
}

func TestResolveExtractionReview_AcceptTagWritesNothing(t *testing.T) {
	srv, r, q := extractReviewServer(t, true)
	ctx := context.Background()
	id := seedVideo(t, r, "/m/Big Movie.mkv", "Old Title")
	if err := r.UpsertExtractionReview(ctx, id, "title", "New Title", "Old Title", 0.4, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	pending, _ := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)

	url := srv.URL + "/api/v1/owner/extraction-review/" + itoa(pending[0].ID) + "/resolve"
	code, body := extractReviewPOST(t, url, map[string]any{"action": "tag"})
	if code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %v", code, body)
	}
	// No write, so no job to wait on — a caller must not be told to poll for one.
	if _, ok := body["job_id"]; ok {
		t.Fatalf("tag resolve reported a job id: %v", body)
	}
	if depth, err := q.Depth(ctx); err != nil || depth != 0 {
		t.Fatalf("queue depth = %d err=%v, want 0 (tag needs no write)", depth, err)
	}
	if remaining, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending); err != nil || len(remaining) != 0 {
		t.Fatalf("want 0 pending after resolve, got %d err=%v", len(remaining), err)
	}
}

func TestResolveExtractionReview_ManualEditsWritesGivenValue(t *testing.T) {
	srv, r, q := extractReviewServer(t, true)
	ctx := context.Background()
	id := seedVideo(t, r, "/m/Big Movie.mkv", "Old Title")
	if err := r.UpsertExtractionReview(ctx, id, "title", "New Title", "Old Title", 0.4, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	pending, _ := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)

	url := srv.URL + "/api/v1/owner/extraction-review/" + itoa(pending[0].ID) + "/resolve"
	code, body := extractReviewPOST(t, url, map[string]any{"action": "manual", "value": "  Custom Title  "})
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d: %v", code, body)
	}
	if jobID, _ := body["job_id"].(float64); jobID <= 0 {
		t.Fatalf("response carried no usable job_id: %v", body)
	}
	if depth, err := q.Depth(ctx); err != nil || depth != 1 {
		t.Fatalf("queue depth = %d err=%v, want 1", depth, err)
	}
}

func TestResolveExtractionReview_ManualRequiresValue(t *testing.T) {
	srv, r, _ := extractReviewServer(t, true)
	ctx := context.Background()
	id := seedVideo(t, r, "/m/Big Movie.mkv", "Old Title")
	if err := r.UpsertExtractionReview(ctx, id, "title", "New Title", "Old Title", 0.4, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	pending, _ := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)

	url := srv.URL + "/api/v1/owner/extraction-review/" + itoa(pending[0].ID) + "/resolve"
	if code, _ := extractReviewPOST(t, url, map[string]any{"action": "manual", "value": "   "}); code != http.StatusBadRequest {
		t.Fatalf("blank value: want 400, got %d", code)
	}
	if code, _ := extractReviewPOST(t, url, map[string]any{"action": "bogus"}); code != http.StatusBadRequest {
		t.Fatalf("unknown action: want 400, got %d", code)
	}
}

func TestResolveExtractionReview_UnavailableWithoutQueue(t *testing.T) {
	srv, r, _ := extractReviewServer(t, false)
	ctx := context.Background()
	id := seedVideo(t, r, "/m/Big Movie.mkv", "Old Title")
	if err := r.UpsertExtractionReview(ctx, id, "title", "New Title", "Old Title", 0.4, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	pending, _ := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)

	url := srv.URL + "/api/v1/owner/extraction-review/" + itoa(pending[0].ID) + "/resolve"
	if code, _ := extractReviewPOST(t, url, map[string]any{"action": "filename"}); code != http.StatusServiceUnavailable {
		t.Fatalf("no queue wired: want 503, got %d", code)
	}
}

func TestResolveExtractionReview_NotFound(t *testing.T) {
	srv, _, _ := extractReviewServer(t, true)
	url := srv.URL + "/api/v1/owner/extraction-review/999999/resolve"
	if code, _ := extractReviewPOST(t, url, map[string]any{"action": "filename"}); code != http.StatusNotFound {
		t.Fatalf("unknown id: want 404, got %d", code)
	}
}

// TestResolveExtractionReview_StaleRowIsNoop proves the design handoff's
// "stale row → treat as already-handled" convention: resolving an
// already-resolved row a second time (e.g. a duplicate click, a second tab)
// is a no-op 204, not a duplicate write.
func TestResolveExtractionReview_StaleRowIsNoop(t *testing.T) {
	srv, r, q := extractReviewServer(t, true)
	ctx := context.Background()
	id := seedVideo(t, r, "/m/Big Movie.mkv", "Old Title")
	if err := r.UpsertExtractionReview(ctx, id, "title", "New Title", "Old Title", 0.4, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	pending, _ := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)
	reviewID := pending[0].ID

	url := srv.URL + "/api/v1/owner/extraction-review/" + itoa(reviewID) + "/resolve"
	if code, _ := extractReviewPOST(t, url, map[string]any{"action": "filename"}); code != http.StatusOK {
		t.Fatalf("first resolve: want 200, got %d", code)
	}
	// The stale repeat enqueues nothing, so it has no job to report and stays 204 — a
	// caller must not be handed a job id for work that never happened.
	code, body := extractReviewPOST(t, url, map[string]any{"action": "filename"})
	if code != http.StatusNoContent {
		t.Fatalf("second (stale) resolve: want 204, got %d", code)
	}
	if _, ok := body["job_id"]; ok {
		t.Fatalf("stale resolve reported a job id: %v", body)
	}
	if depth, err := q.Depth(ctx); err != nil || depth != 1 {
		t.Fatalf("queue depth = %d err=%v, want 1 (the stale resolve must not enqueue a second write)", depth, err)
	}
}

// TestResolveExtractionReview_PeopleFieldWritesToActorsTag is the writeback
// regression for the extract-package/writeback-layer field-key mismatch:
// "people" (the extract package's canonical field, from the {people}
// filename token) has no entry in internal/writeback's formatMap — only
// "actors" does (metadata-mappings.yaml.example: filename:people is a
// *source* of the actors field, not a canonical field of its own). Without
// translating row.FieldKey through extract.WritebackField before building
// the JobField, this resolves through the real writequeue.buildBatch /
// writeback.ResolveForContainer path as unmapped and silently drops the
// write — recorded as a successful no-op, never reaching the file's Artist
// tag. This test wires a real queue with a capturing WriteFunc (not the
// other tests' no-op) so it proves the field actually got mapped and
// written, not merely enqueued.
func TestResolveExtractionReview_PeopleFieldWritesToActorsTag(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: filepath.Join(dir, "v.mp4"), FileSize: 1, Title: "t",
		Container: "MP4", FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	var gotFields []writeback.FieldWrite
	write := func(_ context.Context, _ string, fields []writeback.FieldWrite) error {
		gotFields = fields
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := writequeue.New(r, write, log, 1, "")
	q.Start(ctx)

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetWriteQueue(q)
	h.SetAuth(api.NewAuth(""), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	if err := r.UpsertExtractionReview(ctx, id, "people", "Alice Smith", "", 0.7, 0); err != nil {
		t.Fatalf("upsert review: %v", err)
	}
	pending, _ := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)
	if len(pending) != 1 {
		t.Fatalf("want 1 pending, got %d", len(pending))
	}

	url := srv.URL + "/api/v1/owner/extraction-review/" + itoa(pending[0].ID) + "/resolve"
	if code, body := extractReviewPOST(t, url, map[string]any{"action": "filename"}); code != http.StatusOK {
		t.Fatalf("want 200, got %d: %v", code, body)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && gotFields == nil {
		time.Sleep(10 * time.Millisecond)
	}
	if len(gotFields) != 1 || gotFields[0].TagName != "QuickTime:Artist" {
		t.Fatalf("want the people field mapped and written to the QuickTime:Artist tag, got %+v", gotFields)
	}
}

func TestDismissExtractionReview(t *testing.T) {
	srv, r, _ := extractReviewServer(t, false)
	ctx := context.Background()
	id := seedVideo(t, r, "/m/Big Movie.mkv", "Old Title")
	if err := r.UpsertExtractionReview(ctx, id, "title", "New Title", "Old Title", 0.4, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	pending, _ := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)

	url := srv.URL + "/api/v1/owner/extraction-review/" + itoa(pending[0].ID) + "/dismiss"
	if code, body := extractReviewPOST(t, url, nil); code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %v", code, body)
	}
	if remaining, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending); err != nil || len(remaining) != 0 {
		t.Fatalf("want 0 pending after dismiss, got %d err=%v", len(remaining), err)
	}
	dismissed, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewDismissed)
	if err != nil || len(dismissed) != 1 {
		t.Fatalf("want 1 dismissed row, got %d err=%v", len(dismissed), err)
	}
}

// TestExtractionQueue_VideoIDFilter covers F48.6k: ?video_id= scopes the queue to
// one video for the media detail page's inline panel, while the unfiltered call
// still serves the owner tab. Same rows, same endpoint, same gate — just a
// narrower WHERE (ADR-090 D2).
func TestExtractionQueue_VideoIDFilter(t *testing.T) {
	srv, r, _ := extractReviewServer(t, false)
	ctx := context.Background()
	a := seedVideo(t, r, "/m/A.mkv", "Film A")
	b := seedVideo(t, r, "/m/B.mkv", "Film B")
	if err := r.UpsertExtractionReview(ctx, a, "title", "A New", "A Old", 0.4, 0); err != nil {
		t.Fatalf("upsert a: %v", err)
	}
	if err := r.UpsertExtractionReview(ctx, b, "title", "B New", "B Old", 0.4, 0); err != nil {
		t.Fatalf("upsert b: %v", err)
	}

	code, body := extractReviewGET(t, srv.URL+"/api/v1/owner/extraction-queue")
	rows, _ := body["rows"].([]any)
	if code != http.StatusOK || len(rows) != 2 {
		t.Fatalf("unfiltered: code=%d rows=%d, want 200/2", code, len(rows))
	}

	code, body = extractReviewGET(t, srv.URL+"/api/v1/owner/extraction-queue?video_id="+itoa(a))
	if code != http.StatusOK {
		t.Fatalf("filtered: want 200, got %d: %v", code, body)
	}
	rows, _ = body["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("filtered rows = %d, want 1: %v", len(rows), rows)
	}
	row, _ := rows[0].(map[string]any)
	if row["video_title"] != "Film A" {
		t.Fatalf("filter returned the wrong video: %v", row)
	}

	// A video with nothing pending is an empty list, not a 404 — the panel's
	// "nothing needs review" state (F48.6l) reads this, and must not treat it as
	// an error.
	c := seedVideo(t, r, "/m/C.mkv", "Film C")
	code, body = extractReviewGET(t, srv.URL+"/api/v1/owner/extraction-queue?video_id="+itoa(c))
	rows, _ = body["rows"].([]any)
	if code != http.StatusOK || len(rows) != 0 {
		t.Fatalf("empty scope: code=%d rows=%d, want 200/0", code, len(rows))
	}
}

// TestExtractionQueue_RejectsMalformedVideoID pins the deliberate choice that a bad
// video_id is a 400 rather than a silent fall-through to the whole library: a caller
// that meant to scope and got everything back would leak far more than it asked for
// while looking like it worked.
func TestExtractionQueue_RejectsMalformedVideoID(t *testing.T) {
	srv, r, _ := extractReviewServer(t, false)
	id := seedVideo(t, r, "/m/A.mkv", "Film A")
	if err := r.UpsertExtractionReview(context.Background(), id, "title", "New", "Old", 0.4, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	for _, bad := range []string{"abc", "-1", "0", "1;DROP TABLE videos", "1 OR 1=1"} {
		code, body := extractReviewGET(t, srv.URL+"/api/v1/owner/extraction-queue?video_id="+url.QueryEscape(bad))
		if code != http.StatusBadRequest {
			t.Fatalf("video_id=%q: want 400, got %d (%v) — must not fall back to the whole library", bad, code, body)
		}
		if rows, ok := body["rows"].([]any); ok && len(rows) > 0 {
			t.Fatalf("video_id=%q returned %d rows on a rejected request", bad, len(rows))
		}
	}
}

// TestExtractionQueue_FilteredStillRequiresOwner proves the new query parameter did
// not open a side door: the scoped read sits behind the same requireOwner gate as
// the unfiltered one (ADR-030).
func TestExtractionQueue_FilteredStillRequiresOwner(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth("secret"), false) // a token IS required here
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	id := seedVideo(t, r, "/m/A.mkv", "Film A")
	if err := r.UpsertExtractionReview(context.Background(), id, "title", "New", "Old", 0.4, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	for _, u := range []string{
		srv.URL + "/api/v1/owner/extraction-queue",
		srv.URL + "/api/v1/owner/extraction-queue?video_id=" + itoa(id),
	} {
		code, _ := extractReviewGET(t, u)
		if code != http.StatusUnauthorized {
			t.Fatalf("GET %s without a token: want 401, got %d", u, code)
		}
	}

	// With the token, the scoped read works — proving the 401s above were the gate,
	// not the filter.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/owner/extraction-queue?video_id="+itoa(id), nil)
	req.Header.Set(api.AdminTokenHeader, "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authorized GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authorized scoped GET: want 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if rows, _ := out["rows"].([]any); len(rows) != 1 {
		t.Fatalf("authorized scoped GET returned %d rows, want 1", len(rows))
	}
}
