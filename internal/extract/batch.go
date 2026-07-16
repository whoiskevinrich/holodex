package extract

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"holodex/internal/model"
)

// VideoLister enumerates the videos a library-wide extraction pass covers
// (F48.5b). *repo.Repo satisfies this directly.
type VideoLister interface {
	AllActiveVideoIDs(ctx context.Context) ([]int64, error)
}

// JobRecorder durably records a completed extraction batch for the activity
// history (F48.5b, ADR-028) — the extraction analogue of scanner.JobRecorder.
type JobRecorder interface {
	RecordJobRun(ctx context.Context, run model.JobRun) error
}

// BatchRunner drives a library-wide extraction pass ("Extract all", F48.5b),
// deduplicating concurrent triggers the same way scanner.TriggerRescan does
// (a pass already running satisfies a new request) and reporting progress via
// the System Activity surface (kind=extraction, ADR-028) once it completes.
type BatchRunner struct {
	Orchestrator *Orchestrator
	Videos       VideoLister
	Recorder     JobRecorder
	Log          *slog.Logger

	mu      sync.Mutex
	baseCtx context.Context
}

// SetBaseContext supplies the server-lifetime context a triggered batch run
// uses, so it is cancelled on shutdown rather than tied to the triggering
// HTTP request. Call once at startup; without it TriggerAll falls back to
// context.Background().
func (b *BatchRunner) SetBaseContext(ctx context.Context) { b.baseCtx = ctx }

// TriggerAll starts a library-wide extraction pass in the background,
// returning true if it started one and false if a pass was already running
// (which already satisfies the request) — mirrors scanner.TriggerRescan.
func (b *BatchRunner) TriggerAll() bool {
	if !b.mu.TryLock() {
		return false
	}
	ctx := b.baseCtx
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		defer b.mu.Unlock()
		b.runLocked(ctx)
	}()
	return true
}

func (b *BatchRunner) runLocked(ctx context.Context) {
	start := time.Now()
	ids, err := b.Videos.AllActiveVideoIDs(ctx)
	if err != nil {
		b.recordRun(start, 0, 0, 0, err)
		return
	}

	// A per-run entity-name cache (internal/extract/process.go's
	// resolveEntityMatch doc comment predicted exactly this caller): without
	// it, every people/studio field on every video in the library would
	// re-read the full, unchanging people/studio table from scratch.
	orch := b.Orchestrator.withCachedResolver()

	var matched, errored int
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		res, err := orch.ExtractVideo(ctx, id)
		if err != nil {
			errored++
			if b.Log != nil {
				b.Log.Warn("batch extraction failed", "video_id", id, "err", err)
			}
			continue
		}
		if res.Matched {
			matched++
		}
	}
	b.recordRun(start, len(ids), matched, errored, nil)
}

// recordRun persists a completed pass to the activity history (F48.5b,
// ADR-028), mirroring scanner.recordRun: a detached, bounded context so a
// pass that finishes during shutdown still records; best-effort, never
// propagated.
func (b *BatchRunner) recordRun(start time.Time, seen, matched, errored int, runErr error) {
	if b.Recorder == nil {
		return
	}
	status, msg := model.JobStatusOK, ""
	if runErr != nil {
		status, msg = model.JobStatusErr, runErr.Error()
	}
	finished := time.Now()
	run := model.JobRun{
		Kind: model.JobKindExtraction, Trigger: model.TriggerManual, Status: status,
		StartedAt: start.UTC(), FinishedAt: finished.UTC(), DurationMs: finished.Sub(start).Milliseconds(),
		Seen: seen, Updated: matched, Errors: errored, ErrorMessage: msg,
		Detail: fmt.Sprintf("filename extraction: %d matched of %d scanned", matched, seen),
	}
	recCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := b.Recorder.RecordJobRun(recCtx, run); err != nil && b.Log != nil {
		b.Log.Warn("record extraction job run failed", "err", err)
	}
}
