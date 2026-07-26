package repo_test

import (
	"context"
	"path/filepath"
	"testing"

	"holodex/internal/repo"
)

// GetWritebackJobStatus backs the SPA's poll of an enqueued write (HOLODEX-214).
// The three states a poller has to distinguish: still in flight, gave up, and
// succeeded — the last of which has no row at all, because FinishWriteback
// deletes it on success.
func TestGetWritebackJobStatus(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	videoID, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "v.mp4"), "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	jobID, err := r.EnqueueWriteback(ctx, videoID, `[{"field":"title","values":["X"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	status, errMsg, err := r.GetWritebackJobStatus(ctx, jobID)
	if err != nil {
		t.Fatalf("status of a fresh job: %v", err)
	}
	if status != repo.WritebackPending || errMsg != "" {
		t.Errorf("fresh job = %q/%q, want %q with no error", status, errMsg, repo.WritebackPending)
	}

	// A failure keeps its row so it is inspectable — and so a real failure is
	// never mistaken for the absent-row success below.
	if err := r.FinishWriteback(ctx, jobID, false, "exiftool exploded"); err != nil {
		t.Fatalf("fail job: %v", err)
	}
	status, errMsg, err = r.GetWritebackJobStatus(ctx, jobID)
	if err != nil {
		t.Fatalf("status of a failed job: %v", err)
	}
	if status != repo.WritebackFailed || errMsg != "exiftool exploded" {
		t.Errorf("failed job = %q/%q, want %q carrying the error", status, errMsg, repo.WritebackFailed)
	}

	// Success deletes the row; an absent row must read as done rather than as an
	// error, or the poller never terminates on the happy path.
	done, err := r.EnqueueWriteback(ctx, videoID, `[{"field":"title","values":["Y"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := r.FinishWriteback(ctx, done, true, ""); err != nil {
		t.Fatalf("finish job: %v", err)
	}
	status, _, err = r.GetWritebackJobStatus(ctx, done)
	if err != nil {
		t.Fatalf("status of a completed job: %v", err)
	}
	if status != repo.WritebackDone {
		t.Errorf("completed job = %q, want %q", status, repo.WritebackDone)
	}

	// An id that never existed is indistinguishable from a completed one. That is
	// the accepted cost of deleting on success; assert it so the contract is
	// explicit rather than incidental.
	status, _, err = r.GetWritebackJobStatus(ctx, 99999)
	if err != nil || status != repo.WritebackDone {
		t.Errorf("unknown id = %q/%v, want %q with no error", status, err, repo.WritebackDone)
	}
}
