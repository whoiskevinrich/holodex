package writequeue

import (
	"context"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// processRename runs one kind='rename' job (F86) on the tag-write lane, which keeps it from
// overlapping any other job on the same file. A job waiting for another rename in its batch goes
// back to pending without a run record. Otherwise it records a job_runs row (kind=rename) whose
// detail never names a file: System activity holds counts, never names (security-perimeter.md).
// The queue row keeps the error for the owner's Retry/Dismiss.
func (q *Queue) processRename(ctx context.Context, job *repo.WritebackJob) {
	started := time.Now()
	if q.rename == nil {
		_ = q.repo.FinishWriteback(ctx, job.ID, false, "renaming is not configured")
		return
	}
	err := q.rename(ctx, job)
	if ctx.Err() != nil {
		return // shutting down: the row stays running, and boot recovery requeues it
	}
	if err != nil && q.renameRequeue(err) {
		if rqErr := q.repo.RequeueWriteback(ctx, job.ID); rqErr != nil {
			q.log.Warn("requeue rename", "id", job.ID, "err", rqErr)
		}
		return
	}
	ok := err == nil || q.renameNoop(err)
	run := model.JobRun{
		Kind: model.JobKindRename, Trigger: model.TriggerManual, Status: model.JobStatusOK,
		StartedAt: started, FinishedAt: time.Now(), DurationMs: time.Since(started).Milliseconds(),
		EntityType: model.EnrichEntityVideo, EntityID: job.VideoID, BatchID: job.BatchID,
		Detail: "renamed",
	}
	switch {
	case err == nil:
		run.Updated = 1
	case ok:
		run.Detail = "already named"
	default:
		run.Status, run.Errors, run.Detail = model.JobStatusErr, 1, "rename failed"
		q.log.Warn("rename failed", "video", job.VideoID, "err", err)
	}
	if rErr := q.repo.RecordJobRun(ctx, run); rErr != nil {
		q.log.Warn("record rename job run", "id", job.ID, "err", rErr)
	}
	msg := ""
	if !ok {
		msg = err.Error()
	}
	if fErr := q.repo.FinishWriteback(ctx, job.ID, ok, msg); fErr != nil {
		q.log.Warn("finish rename", "id", job.ID, "err", fErr)
	}
	// The batch's last job completes the apply's summary row (P0-15).
	if bErr := q.repo.FinishRenameBatch(ctx, job.BatchID); bErr != nil {
		q.log.Warn("finish rename batch", "batch", job.BatchID, "err", bErr)
	}
}
