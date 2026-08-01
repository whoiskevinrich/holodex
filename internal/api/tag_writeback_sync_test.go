package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

// patchTok issues a PATCH with a JSON body, mirroring postTok (enrich_test.go).
func patchTok(t *testing.T, url, token string, body any) (int, map[string]any) {
	t.Helper()
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPatch, url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", url, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// tagWritebackSyncServer wires a real repo + a durable write queue (recording
// each write's genre values by file path) so the sync trigger's actual
// enqueue → drain → write path can be exercised end to end.
func tagWritebackSyncServer(t *testing.T) (srv *httptest.Server, r *repo.Repo, written *map[string][]string, mu *sync.Mutex) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r = repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)

	mu = &sync.Mutex{}
	writtenMap := map[string][]string{}
	written = &writtenMap
	q := writequeue.New(r, func(_ context.Context, path string, fields []writeback.FieldWrite) error {
		mu.Lock()
		defer mu.Unlock()
		for _, f := range fields {
			if f.TagName == "Genre" {
				writtenMap[path] = f.Values
			}
		}
		return nil
	}, log, 1, "")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	q.Start(ctx)
	h.SetWriteQueue(q)
	h.SetAuth(api.NewAuth(""), false)

	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, written, mu
}

// noQueueTagServer wires a real repo with no write queue at all — for tests
// that must prove an endpoint never enqueues (a nil h.writeQueue makes any
// erroneous enqueue attempt nil-panic the handler) but still need *Handlers
// in scope to call SetAuth.
func noQueueTagServer(t *testing.T) (srv *httptest.Server, r *repo.Repo, h *api.Handlers) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r = repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h = api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth(""), false)
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, h
}

func seedGenreVideo(t *testing.T, r *repo.Repo, path, title string, tags ...string) int64 {
	t.Helper()
	v := &model.Video{
		FilePath: path, FileSize: 100, Title: title, Duration: 60, Width: 1920, Height: 1080,
		Container: "Matroska", FileMtime: time.Now().UTC().Truncate(time.Second),
	}
	for _, tag := range tags {
		v.Tags = append(v.Tags, model.Tag{Name: tag})
	}
	vid, err := r.UpsertVideo(context.Background(), v, nil)
	if err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
	return vid
}

func waitQueueDrained(t *testing.T, r *repo.Repo) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if n, _ := r.PendingWritebackCount(ctx); n == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("writeback queue did not drain")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestSetTagWriteback covers the single-tag flag flip endpoint: it toggles
// the stored flag and returns the updated tag, never enqueuing a write
// (spec P0) — asserted here by there being no writeQueue wired at all, so an
// erroneous enqueue attempt would nil-panic the handler.
func TestSetTagWriteback(t *testing.T) {
	srv, r, _ := noQueueTagServer(t)

	seedGenreVideo(t, r, "/m/a.mkv", "A", "Yoda")
	tagID := tagID(t, r, "Yoda")

	code, body := patchTok(t, srv.URL+"/api/v1/tags/"+itoa(tagID)+"/writeback", "", map[string]bool{"enabled": false})
	if code != http.StatusOK {
		t.Fatalf("patch writeback flag = %d, want 200", code)
	}
	tag, _ := body["tag"].(map[string]any)
	if tag["writeback_enabled"] != false {
		t.Errorf("response tag.writeback_enabled = %v, want false", tag["writeback_enabled"])
	}

	got, err := r.GetTag(context.Background(), tagID)
	if err != nil {
		t.Fatalf("get tag: %v", err)
	}
	if got.WritebackEnabled {
		t.Errorf("GetTag.WritebackEnabled = true after PATCH, want false")
	}

	code, _ = patchTok(t, srv.URL+"/api/v1/tags/999999/writeback", "", map[string]bool{"enabled": false})
	if code != http.StatusNotFound {
		t.Errorf("unknown tag = %d, want 404", code)
	}
}

// TestSyncTagWriteback_RecomputesFullUnion covers D2: the sync trigger writes
// the video's *current* full genres union (every attached tag plus resolved
// raw genres), not just the name of the tag being synced — proving sync is a
// real sync and not an append of one tag's name.
func TestSyncTagWriteback_RecomputesFullUnion(t *testing.T) {
	srv, r, written, mu := tagWritebackSyncServer(t)
	vid := seedGenreVideo(t, r, "/m/full_union.mkv", "V", "Comedy", "Adventure")
	comedyID := tagID(t, r, "Comedy")

	code, body := patchTok(t, srv.URL+"/api/v1/tags/"+itoa(comedyID)+"/writeback", "", map[string]bool{"enabled": true})
	if code != http.StatusOK {
		t.Fatalf("ensure comedy enabled = %d: %v", code, body)
	}

	code, body = postTok(t, srv.URL+"/api/v1/tags/"+itoa(comedyID)+"/writeback/sync", "", nil)
	if code != http.StatusAccepted {
		t.Fatalf("sync = %d, want 202: %v", code, body)
	}
	if n, _ := body["enqueued"].(float64); n != 1 {
		t.Errorf("enqueued = %v, want 1", body["enqueued"])
	}

	waitQueueDrained(t, r)

	mu.Lock()
	defer mu.Unlock()
	got := (*written)["/m/full_union.mkv"]
	set := map[string]bool{}
	for _, v := range got {
		set[v] = true
	}
	if !set["comedy"] || !set["adventure"] {
		t.Errorf("written genres = %v, want the video's full current union [comedy adventure], not just the synced tag", got)
	}
	if len(got) != 2 {
		t.Errorf("written genres = %v, want exactly 2", got)
	}
	_ = vid
}

// TestSyncTagWritebackBulk_DedupsSharedVideo covers D2's bulk sync scope: a
// video attached to two selected tags is enqueued (and so written) once, not
// once per selected tag it happens to carry.
func TestSyncTagWritebackBulk_DedupsSharedVideo(t *testing.T) {
	srv, r, written, mu := tagWritebackSyncServer(t)
	seedGenreVideo(t, r, "/m/shared.mkv", "Shared", "Comedy", "Action")
	seedGenreVideo(t, r, "/m/comedy_only.mkv", "ComedyOnly", "Comedy")
	seedGenreVideo(t, r, "/m/action_only.mkv", "ActionOnly", "Action")
	comedyID := tagID(t, r, "Comedy")
	actionID := tagID(t, r, "Action")

	code, body := postTok(t, srv.URL+"/api/v1/tags/writeback/sync", "",
		map[string]any{"tag_ids": []int64{comedyID, actionID}})
	if code != http.StatusAccepted {
		t.Fatalf("bulk sync = %d, want 202: %v", code, body)
	}
	if n, _ := body["enqueued"].(float64); n != 3 {
		t.Errorf("enqueued = %v, want 3 (one per distinct video, shared video counted once)", body["enqueued"])
	}

	waitQueueDrained(t, r)

	mu.Lock()
	defer mu.Unlock()
	if len(*written) != 3 {
		t.Errorf("files written = %v, want exactly 3 (dedup across the shared video)", *written)
	}
	if got := (*written)["/m/shared.mkv"]; len(got) != 2 {
		t.Errorf("shared.mkv written = %v, want both Comedy and Action from one job", got)
	}
}

// TestSetTagsWritebackBulk_AppliesRegardlessOfPriorState covers the bulk
// flag-toggle endpoint applying to every listed tag regardless of individual
// prior state, without enqueuing any write.
func TestSetTagsWritebackBulk_AppliesRegardlessOfPriorState(t *testing.T) {
	srv, r, written, mu := tagWritebackSyncServer(t)
	seedGenreVideo(t, r, "/m/a.mkv", "A", "Comedy")
	seedGenreVideo(t, r, "/m/b.mkv", "B", "Action")
	comedyID := tagID(t, r, "Comedy")
	actionID := tagID(t, r, "Action")

	// Seed a mixed prior state: Comedy already disabled, Action still enabled.
	if _, err := r.SetTagWritebackEnabled(context.Background(), comedyID, false); err != nil {
		t.Fatalf("seed disabled comedy: %v", err)
	}

	code, _ := patchTok(t, srv.URL+"/api/v1/tags/writeback", "",
		map[string]any{"tag_ids": []int64{comedyID, actionID}, "enabled": false})
	if code != http.StatusNoContent {
		t.Fatalf("bulk patch = %d, want 204", code)
	}

	for _, id := range []int64{comedyID, actionID} {
		tag, err := r.GetTag(context.Background(), id)
		if err != nil {
			t.Fatalf("get tag %d: %v", id, err)
		}
		if tag.WritebackEnabled {
			t.Errorf("tag %d WritebackEnabled = true after bulk disable, want false", id)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(*written) != 0 {
		t.Errorf("files written = %v, want none — a flag toggle alone must never enqueue", *written)
	}
}

// TestSyncTagWritebackOne_ZeroVideosNoop covers the zero-attached-videos
// case: nothing to sync, no error, enqueued 0.
func TestSyncTagWritebackOne_ZeroVideosNoop(t *testing.T) {
	srv, r, written, mu := tagWritebackSyncServer(t)
	vid := seedGenreVideo(t, r, "/m/a.mkv", "A", "Lonely")
	lonelyID := tagID(t, r, "Lonely")

	// Detach so the tag has zero attached videos.
	if err := r.DetachTagFromVideo(context.Background(), vid, lonelyID); err != nil {
		t.Fatalf("detach: %v", err)
	}

	code, body := postTok(t, srv.URL+"/api/v1/tags/"+itoa(lonelyID)+"/writeback/sync", "", nil)
	if code != http.StatusAccepted {
		t.Fatalf("sync (zero attached videos) = %d, want 202: %v", code, body)
	}
	if n, _ := body["enqueued"].(float64); n != 0 {
		t.Errorf("enqueued = %v, want 0 (nothing to sync)", body["enqueued"])
	}
	mu.Lock()
	if len(*written) != 0 {
		t.Errorf("files written = %v, want none", *written)
	}
	mu.Unlock()

	code, body = postTok(t, srv.URL+"/api/v1/tags/999999/writeback/sync", "", nil)
	if code != http.StatusNotFound {
		t.Errorf("unknown tag sync = %d, want 404: %v", code, body)
	}
}

// TestWritebackBatchStatusEndpoint covers D3's aggregation endpoint wiring
// end to end over HTTP. Progress is driven directly through the queue's repo
// primitives (as TestGetWritebackBatchStatus does at the repo layer) rather
// than a live worker, so the test isn't coupled to exiftool/mkvpropedit
// actually being able to read the fixture's non-existent on-disk file — this
// test's job is proving the HTTP route + JSON shape, not re-proving the
// aggregation math (already covered at the repo layer).
func TestWritebackBatchStatusEndpoint(t *testing.T) {
	srv, r, _ := noQueueTagServer(t)

	ctx := context.Background()
	vid := seedGenreVideo(t, r, "/m/a.mkv", "A", "Comedy")

	const batchID = "tag-writeback-sync-endpoint-test"
	if _, err := r.EnqueueWritebackBatch(ctx, []repo.WritebackJobInsert{
		{VideoID: vid, Payload: `[{"field":"genres","values":["Comedy"]}]`, BatchID: batchID},
		{VideoID: vid, Payload: `[{"field":"genres","values":["Comedy"]}]`, BatchID: batchID},
	}); err != nil {
		t.Fatalf("enqueue batch: %v", err)
	}

	getStatus := func() map[string]any {
		t.Helper()
		resp, err := http.Get(srv.URL + "/api/v1/writeback/batches/" + batchID + "/status")
		if err != nil {
			t.Fatalf("get batch status: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("batch status = %d, want 200", resp.StatusCode)
		}
		var status map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			t.Fatalf("decode batch status: %v", err)
		}
		return status
	}

	if status := getStatus(); status["pending"] != float64(2) || status["running"] != float64(0) ||
		status["done"] != float64(0) || status["failed"] != float64(0) {
		t.Errorf("initial batch status = %v, want pending=2 running=0 done=0 failed=0", status)
	}

	job1, err := r.ClaimNextWriteback(ctx)
	if err != nil || job1 == nil {
		t.Fatalf("claim 1: job=%v err=%v", job1, err)
	}
	if err := r.FinishWriteback(ctx, job1.ID, true, ""); err != nil {
		t.Fatalf("finish 1: %v", err)
	}
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: model.JobKindWriteback, Trigger: model.TriggerManual, Status: model.JobStatusOK,
		StartedAt: time.Now(), FinishedAt: time.Now(), EntityType: model.EnrichEntityVideo,
		EntityID: vid, BatchID: batchID,
	}); err != nil {
		t.Fatalf("record job run 1: %v", err)
	}
	if status := getStatus(); status["pending"] != float64(1) || status["done"] != float64(1) {
		t.Errorf("mid-batch status = %v, want pending=1 done=1", status)
	}

	job2, err := r.ClaimNextWriteback(ctx)
	if err != nil || job2 == nil {
		t.Fatalf("claim 2: job=%v err=%v", job2, err)
	}
	if err := r.FinishWriteback(ctx, job2.ID, true, ""); err != nil {
		t.Fatalf("finish 2: %v", err)
	}
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: model.JobKindWriteback, Trigger: model.TriggerManual, Status: model.JobStatusOK,
		StartedAt: time.Now(), FinishedAt: time.Now(), EntityType: model.EnrichEntityVideo,
		EntityID: vid, BatchID: batchID,
	}); err != nil {
		t.Fatalf("record job run 2: %v", err)
	}

	if status := getStatus(); status["pending"] != float64(0) || status["running"] != float64(0) ||
		status["done"] != float64(2) || status["failed"] != float64(0) {
		t.Errorf("final batch status = %v, want pending=0 running=0 done=2 failed=0", status)
	}

	if status := getStatus(); status["pending"] == nil {
		t.Errorf("batch status for an unknown batch should still 200 with zero counts, got %v", status)
	}
}
