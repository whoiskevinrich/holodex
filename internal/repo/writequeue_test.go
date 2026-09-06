package repo_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/model"
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

// TestGetWritebackBatchStatus covers D3's aggregation across a shared
// batchID (HOLODEX-239, ADR-077): pending/running come from still-live
// writeback_queue rows, done/failed from job_runs — driven directly through
// the queue's own repo primitives (claim/finish/record) rather than a live
// worker, so the counts at each step are deterministic.
func TestGetWritebackBatchStatus(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	videoID, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "v.mp4"), "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	const batchID = "tag-writeback-sync-test"
	jobs := []repo.WritebackJobInsert{
		{VideoID: videoID, Payload: `[{"field":"genres","values":["A"]}]`, BatchID: batchID},
		{VideoID: videoID, Payload: `[{"field":"genres","values":["B"]}]`, BatchID: batchID},
		{VideoID: videoID, Payload: `[{"field":"genres","values":["C"]}]`, BatchID: batchID},
	}
	if _, err := r.EnqueueWritebackBatch(ctx, jobs); err != nil {
		t.Fatalf("enqueue batch: %v", err)
	}

	assertStatus := func(label string, wantPending, wantRunning, wantDone, wantFailed int) {
		t.Helper()
		pending, running, done, failed, err := r.GetWritebackBatchStatus(ctx, batchID)
		if err != nil {
			t.Fatalf("%s: get batch status: %v", label, err)
		}
		if pending != wantPending || running != wantRunning || done != wantDone || failed != wantFailed {
			t.Errorf("%s: (pending,running,done,failed) = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
				label, pending, running, done, failed, wantPending, wantRunning, wantDone, wantFailed)
		}
	}
	assertStatus("all pending", 3, 0, 0, 0)

	// Claim one — moves it from pending to running.
	job1, err := r.ClaimNextWriteback(ctx)
	if err != nil || job1 == nil {
		t.Fatalf("claim 1: job=%v err=%v", job1, err)
	}
	assertStatus("one running", 2, 1, 0, 0)

	// Finish it successfully — FinishWriteback deletes the row; job_runs
	// records the outcome, exactly as writequeue.Queue.process does.
	if err := r.FinishWriteback(ctx, job1.ID, true, ""); err != nil {
		t.Fatalf("finish 1: %v", err)
	}
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: model.JobKindWriteback, Trigger: model.TriggerManual, Status: model.JobStatusOK,
		StartedAt: time.Now(), FinishedAt: time.Now(), EntityType: model.EnrichEntityVideo,
		EntityID: videoID, BatchID: batchID,
	}); err != nil {
		t.Fatalf("record job run 1: %v", err)
	}
	assertStatus("one done", 2, 0, 1, 0)

	// Claim and fail the second — the queue row is marked 'failed' (not
	// deleted), but the batch-status aggregation must count it via job_runs,
	// not as a still-live running/pending row.
	job2, err := r.ClaimNextWriteback(ctx)
	if err != nil || job2 == nil {
		t.Fatalf("claim 2: job=%v err=%v", job2, err)
	}
	if err := r.FinishWriteback(ctx, job2.ID, false, "exiftool exploded"); err != nil {
		t.Fatalf("finish 2 (fail): %v", err)
	}
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: model.JobKindWriteback, Trigger: model.TriggerManual, Status: model.JobStatusErr,
		StartedAt: time.Now(), FinishedAt: time.Now(), EntityType: model.EnrichEntityVideo,
		EntityID: videoID, BatchID: batchID, Errors: 1, ErrorMessage: "exiftool exploded",
	}); err != nil {
		t.Fatalf("record job run 2: %v", err)
	}
	assertStatus("one done, one failed, one still pending", 1, 0, 1, 1)
}

// TestGetVideoWritebackStatus covers the four states that matter for the
// Metadata header badge (ADR-091, HOLODEX-323, spec R2.1): pending only,
// failed only, both at once (a retry queued while an older failure is
// undismissed), and neither — the last of which is the zero value, deliberately
// indistinguishable from "never had a writeback job" (D3: succeeded, swept, and
// never-existed all correctly render as nothing).
func TestGetVideoWritebackStatus(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	videoID, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "v.mp4"), "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	// Neither: a video with no writeback activity at all reads as the zero value.
	status, err := r.GetVideoWritebackStatus(ctx, videoID)
	if err != nil {
		t.Fatalf("status of a clean video: %v", err)
	}
	if status.Pending || status.Failed || status.Error != "" {
		t.Errorf("clean video = %+v, want the zero value", status)
	}

	// Pending only.
	pendingJob, err := r.EnqueueWriteback(ctx, videoID, `[{"field":"title","values":["X"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	status, err = r.GetVideoWritebackStatus(ctx, videoID)
	if err != nil {
		t.Fatalf("status while pending: %v", err)
	}
	if !status.Pending || status.Failed {
		t.Errorf("pending video = %+v, want Pending=true Failed=false", status)
	}

	// Both at once: a second job for the same video has already failed, while the
	// first is still pending. The aggregation must not let one status hide the
	// other — the header renders both a pending and a failed badge if this occurs.
	failedJob, err := r.EnqueueWriteback(ctx, videoID, `[{"field":"genres","values":["Y"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue second job: %v", err)
	}
	if err := r.FinishWriteback(ctx, failedJob, false, "writeback rename: permission denied"); err != nil {
		t.Fatalf("fail second job: %v", err)
	}
	status, err = r.GetVideoWritebackStatus(ctx, videoID)
	if err != nil {
		t.Fatalf("status with one pending, one failed: %v", err)
	}
	if !status.Pending || !status.Failed || status.Error != "writeback rename: permission denied" {
		t.Errorf("mixed video = %+v, want both true carrying the failure's message", status)
	}

	// Failed only: finish the pending job successfully (deletes its row).
	if err := r.FinishWriteback(ctx, pendingJob, true, ""); err != nil {
		t.Fatalf("finish pending job: %v", err)
	}
	status, err = r.GetVideoWritebackStatus(ctx, videoID)
	if err != nil {
		t.Fatalf("status with only a failure: %v", err)
	}
	if status.Pending || !status.Failed || status.Error != "writeback rename: permission denied" {
		t.Errorf("failed-only video = %+v, want Pending=false Failed=true carrying the message", status)
	}
}

// TestRetryFailedWriteback covers spec R3.3: retry is a reset plus a kick, not a
// distinct mechanism. Nothing else moves a 'failed' row back to 'pending' —
// ClaimNextWriteback only ever claims 'pending', and RecoverRunningWritebacks
// resets only 'running' rows left by a crash — so this is the one path that does.
func TestRetryFailedWriteback(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	videoID, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "v.mp4"), "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	// A safe no-op when nothing is failed for this video — never an error, mirroring
	// GetWritebackJobStatus's "absent row" posture rather than surfacing "nothing to
	// retry" as a failure the caller has to special-case.
	n, err := r.RetryFailedWriteback(ctx, videoID)
	if err != nil || n != 0 {
		t.Fatalf("retry with nothing failed = %d/%v, want 0/nil", n, err)
	}

	jobID, err := r.EnqueueWriteback(ctx, videoID, `[{"field":"title","values":["X"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := r.FinishWriteback(ctx, jobID, false, "exiftool exploded"); err != nil {
		t.Fatalf("fail job: %v", err)
	}

	n, err = r.RetryFailedWriteback(ctx, videoID)
	if err != nil || n != 1 {
		t.Fatalf("retry the failed job = %d/%v, want 1/nil", n, err)
	}
	status, errMsg, err := r.GetWritebackJobStatus(ctx, jobID)
	if err != nil {
		t.Fatalf("status after retry: %v", err)
	}
	if status != repo.WritebackPending || errMsg != "" {
		t.Errorf("retried job = %q/%q, want %q with the error cleared", status, errMsg, repo.WritebackPending)
	}
	// ClaimNextWriteback must be able to pick the retried row straight back up —
	// the whole point of resetting to 'pending' rather than some other state.
	claimed, err := r.ClaimNextWriteback(ctx)
	if err != nil || claimed == nil || claimed.ID != jobID {
		t.Fatalf("claim after retry = %v/%v, want the retried job", claimed, err)
	}
}

// TestDismissFailedWriteback covers spec R3.4/RD2: dismiss deletes the queue row
// without retrying — job_runs (not exercised here; it is a separate write in the
// real queue worker) holds the permanent audit record, so this only has to prove
// the work-queue side clears and nothing gets re-enqueued.
func TestDismissFailedWriteback(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	videoID, err := r.UpsertVideo(ctx, sampleVideo(filepath.Join(t.TempDir(), "v.mp4"), "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	// A safe no-op when nothing is failed.
	n, err := r.DismissFailedWriteback(ctx, videoID)
	if err != nil || n != 0 {
		t.Fatalf("dismiss with nothing failed = %d/%v, want 0/nil", n, err)
	}

	jobID, err := r.EnqueueWriteback(ctx, videoID, `[{"field":"title","values":["X"]}]`, "")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := r.FinishWriteback(ctx, jobID, false, "exiftool exploded"); err != nil {
		t.Fatalf("fail job: %v", err)
	}

	n, err = r.DismissFailedWriteback(ctx, videoID)
	if err != nil || n != 1 {
		t.Fatalf("dismiss the failed job = %d/%v, want 1/nil", n, err)
	}
	status, err := r.GetVideoWritebackStatus(ctx, videoID)
	if err != nil {
		t.Fatalf("status after dismiss: %v", err)
	}
	if status.Pending || status.Failed {
		t.Errorf("dismissed video = %+v, want the zero value — dismissing must not re-enqueue", status)
	}
	// Claiming must find nothing: dismiss removed the row rather than resetting it.
	claimed, err := r.ClaimNextWriteback(ctx)
	if err != nil || claimed != nil {
		t.Fatalf("claim after dismiss = %v/%v, want nil/nil", claimed, err)
	}
}
