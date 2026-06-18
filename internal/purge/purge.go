// Package purge runs the F24 grace-period sweep (ADR-037): it hard-deletes media
// items whose owner-initiated soft-delete has aged past the grace window, removing
// the file from disk (when enabled) and then the row (whose junctions/FTS clean up
// via ON DELETE CASCADE). It is a dedicated ticker — independent of the scanner's
// clock and able to run even when scanning is disabled — recording each pass in the
// activity history (ADR-028, kind=purge), exactly like a scan.
package purge

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Repo is the slice of the repository the purger needs (kept narrow for testing).
type Repo interface {
	ExpiredSoftDeleted(ctx context.Context, cutoff time.Time) ([]repo.TrashItem, error)
	PurgePath(ctx context.Context, id int64) (string, error)
	SoftDelete(ctx context.Context, id int64) error
	HardDelete(ctx context.Context, id int64) error
	RecordJobRun(ctx context.Context, run model.JobRun) error
}

// Config carries the purge knobs (from the app config, in seconds upstream).
type Config struct {
	Grace       time.Duration // soft-deleted items older than this auto-purge; 0 disables auto-purge
	Interval    time.Duration // how often the sweep runs
	RemoveFiles bool          // whether purge unlinks the file (false = DB-only)
}

// Purger sweeps expired soft-deletes. Construct with New, then Run on a goroutine.
type Purger struct {
	repo Repo
	cfg  Config
	log  *slog.Logger
}

// New builds a Purger. A nil logger is replaced with the default.
func New(r Repo, cfg Config, log *slog.Logger) *Purger {
	if log == nil {
		log = slog.Default()
	}
	return &Purger{repo: r, cfg: cfg, log: log}
}

// Run drives the periodic sweep until ctx is cancelled. The interval clamps to a
// sane minimum so a misconfigured zero doesn't busy-loop.
func (p *Purger) Run(ctx context.Context) {
	interval := p.cfg.Interval
	if interval <= 0 {
		interval = time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.Sweep(ctx)
		}
	}
}

// Sweep hard-deletes every soft-deleted item past the grace window in one pass,
// recording the outcome in the activity history. With Grace == 0 auto-purge is
// disabled (items linger in Trash until the owner purges manually), so the sweep
// no-ops without recording a run.
func (p *Purger) Sweep(ctx context.Context) {
	if p.cfg.Grace <= 0 {
		return
	}
	start := time.Now()
	cutoff := start.Add(-p.cfg.Grace)
	items, err := p.repo.ExpiredSoftDeleted(ctx, cutoff)
	if err != nil {
		p.log.Warn("purge: list expired failed", "err", err)
		return
	}
	if len(items) == 0 {
		return // nothing due — stay quiet (no empty job_runs noise)
	}

	var purged, failed int
	for _, it := range items {
		if err := p.purgeItem(ctx, it.ID, it.FilePath); err != nil {
			failed++
			p.log.Warn("purge: item failed", "id", it.ID, "err", err)
		} else {
			purged++
		}
	}
	p.record(ctx, start, model.TriggerPeriodic, purged, failed)
}

// PurgeNow hard-deletes a single item immediately, bypassing the grace period
// (F24.5 owner override). It first ensures the row is soft-deleted so a failed
// disk removal leaves a consistent Trash state to retry. Returns repo.ErrNotFound
// when the id is already gone; a disk-removal error otherwise (row left intact).
func (p *Purger) PurgeNow(ctx context.Context, id int64) error {
	path, err := p.repo.PurgePath(ctx, id)
	if err != nil {
		return err // ErrNotFound or a query error
	}
	// Mark soft-deleted (idempotent) so that if the unlink fails the item is in
	// Trash and the periodic sweep / a retry can finish it — never a live row with
	// a half-removed file.
	if err := p.repo.SoftDelete(ctx, id); err != nil {
		return err
	}
	if err := p.purgeItem(ctx, id, path); err != nil {
		return err
	}
	p.record(ctx, time.Now(), model.TriggerManual, 1, 0)
	return nil
}

// purgeItem removes one item: the file first (when enabled), then the row. A file
// that is already gone counts as success (the desired end state is "removed"); a
// permission/read-only failure aborts before the row delete, leaving the row
// soft-deleted for retry so disk and DB never desync (F24.8).
func (p *Purger) purgeItem(ctx context.Context, id int64, path string) error {
	if p.cfg.RemoveFiles {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err // permission / read-only — leave the row soft-deleted, retry later
		}
	}
	return p.repo.HardDelete(ctx, id)
}

// record writes a job_runs row (kind=purge) so a purge pass appears in the activity
// history beside scans. Best-effort: a recording failure is logged, not fatal.
func (p *Purger) record(ctx context.Context, start time.Time, trigger string, purged, failed int) {
	status, msg := model.JobStatusOK, ""
	if failed > 0 {
		status = model.JobStatusErr
		msg = "one or more files could not be removed; left in Trash to retry"
	}
	finished := time.Now()
	run := model.JobRun{
		Kind:         model.JobKindPurge,
		Trigger:      trigger,
		Status:       status,
		StartedAt:    start.UTC(),
		FinishedAt:   finished.UTC(),
		DurationMs:   finished.Sub(start).Milliseconds(),
		Removed:      purged, // reuse the "removed" count column for purged items
		Errors:       failed,
		ErrorMessage: msg,
	}
	recCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.repo.RecordJobRun(recCtx, run); err != nil {
		p.log.Warn("purge: record job run failed", "err", err)
	}
}
