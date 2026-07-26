package api_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
)

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
