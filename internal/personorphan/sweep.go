// Package personorphan runs the F40 orphan-grace sweep (ADR-072 §4/P0-9): it
// deletes people whose last video link was removed more than gracePeriod ago and
// who carry no authored identity (alias, curated image, manual field decision/
// curation). A dedicated daily ticker, independent of the scanner's clock,
// recording each pass in the activity history (ADR-028, kind=person-orphan-sweep)
// — mirrors internal/purge's grace-period sweep shape exactly.
package personorphan

import (
	"context"
	"log/slog"
	"time"

	"holodex/internal/model"
)

// Repo is the slice of the repository the sweeper needs (kept narrow for testing).
type Repo interface {
	SweepOrphanedPeople(ctx context.Context, graceDays int) (deleted, skipped int, err error)
	RecordJobRun(ctx context.Context, run model.JobRun) error
}

// Config carries the sweep knobs.
type Config struct {
	GraceDays int           // people orphaned longer than this are eligible for deletion
	Interval  time.Duration // how often the sweep runs
}

// Sweeper periodically prunes unauthored orphaned people. Construct with New,
// then Run on a goroutine.
type Sweeper struct {
	repo Repo
	cfg  Config
	log  *slog.Logger
}

// New builds a Sweeper. A nil logger is replaced with the default; a non-positive
// GraceDays defaults to 30 (ADR-072 §4).
func New(r Repo, cfg Config, log *slog.Logger) *Sweeper {
	if log == nil {
		log = slog.Default()
	}
	if cfg.GraceDays <= 0 {
		cfg.GraceDays = 30
	}
	return &Sweeper{repo: r, cfg: cfg, log: log}
}

// Run drives the periodic sweep until ctx is cancelled. The interval clamps to a
// sane minimum so a misconfigured zero doesn't busy-loop.
func (s *Sweeper) Run(ctx context.Context) {
	interval := s.cfg.Interval
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Sweep(ctx)
		}
	}
}

// Sweep runs one pass, recording the outcome in the activity history. Quiet when
// nothing was due (deleted == 0 && skipped == 0) — no empty job_runs noise.
func (s *Sweeper) Sweep(ctx context.Context) {
	start := time.Now()
	deleted, skipped, err := s.repo.SweepOrphanedPeople(ctx, s.cfg.GraceDays)
	if err != nil {
		s.log.Warn("person orphan sweep failed", "err", err)
		s.record(ctx, start, model.JobStatusErr, 0, 0, err.Error())
		return
	}
	if deleted == 0 && skipped == 0 {
		return
	}
	s.record(ctx, start, model.JobStatusOK, deleted, skipped, "")
}

func (s *Sweeper) record(ctx context.Context, start time.Time, status string, deleted, skipped int, errMsg string) {
	finished := time.Now()
	run := model.JobRun{
		Kind:         model.JobKindPersonOrphanSweep,
		Trigger:      model.TriggerPeriodic,
		Status:       status,
		StartedAt:    start.UTC(),
		FinishedAt:   finished.UTC(),
		DurationMs:   finished.Sub(start).Milliseconds(),
		Removed:      deleted,
		Skipped:      skipped,
		ErrorMessage: errMsg,
	}
	recCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.repo.RecordJobRun(recCtx, run); err != nil {
		s.log.Warn("person orphan sweep: record job run failed", "err", err)
	}
}
