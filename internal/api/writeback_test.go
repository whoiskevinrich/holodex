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
