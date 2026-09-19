package enrich

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"holodex/internal/model"
)

// Entity refresh sweep (F66, ADR-103 D6/D8/D9): the per-entity RefreshPair step run
// over every person or studio in the background, one entity at a time (the providers
// of one entity fan out concurrently, exactly as one click does), with live progress
// on the activity poll and one batch_id tying every run it records together.

// Breaker thresholds (F66 RD9): a provider that fails this many times in a row, or
// pauses this many times in a row, is skipped for the rest of the sweep.
const (
	breakerFailures = 5
	breakerPauses   = 3
)

// Breaker reasons, as the done line names them.
const (
	BreakerStoppedResponding = "stopped responding"
	BreakerRateLimited       = "rate-limited"
)

// SweepLister enumerates the entities a sweep covers. *repo.Repo satisfies it; the
// sets are the ones the /people and /studios pages list, so the confirm's count and
// the sweep's total agree.
type SweepLister interface {
	ListPeople(ctx context.Context, sortByCount bool) ([]model.Person, error)
	ListStudios(ctx context.Context, sortByCount bool) ([]model.Studio, error)
}

// SweepCounts are the per-pair tallies (F66 RD11). Linked counts every pair that
// ended the sweep linked with fresh data — an unlinked pair auto-applied and a linked
// pair refreshed alike; both produced an enrich run under the batch.
type SweepCounts struct {
	Linked       int `json:"linked"`
	NeedsReview  int `json:"needs_review"`
	NoCandidates int `json:"no_candidates"`
	Failed       int `json:"failed"`
	Skipped      int `json:"skipped"`
	StaleSkipped int `json:"stale_skipped"`
}

// SkippedProvider names a provider the breaker tripped and why.
type SkippedProvider struct {
	Provider string `json:"provider"`
	Reason   string `json:"reason"`
}

// SweepSummary is a finished sweep, kept in memory until the next one starts or the
// process restarts (ADR-103 D8); the durable record is the enrich-sweep job run.
type SweepSummary struct {
	Kind       string    `json:"kind"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMs int64     `json:"duration_ms"`
	BatchID    string    `json:"batch_id"`
	Total      int       `json:"total"`
	SweepCounts
	SkippedProviders []SkippedProvider `json:"skipped_providers"`
	Error            string            `json:"error,omitempty"`
}

// SweepStatus is the `sweep` block on GET /admin/activity (spec P0-3). Kind is the
// entity type (person/studio), never the route plural. LastRun is nil until a sweep
// has finished in this process.
type SweepStatus struct {
	State     string     `json:"state"` // "idle" | "running"
	Kind      string     `json:"kind,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	BatchID   string     `json:"batch_id,omitempty"`
	Total     int        `json:"total"`
	Done      int        `json:"done"`
	SweepCounts
	LastRun *SweepSummary `json:"last_run"`
}

// SweepRunner owns the one sweep that may run at a time — across both kinds (F66
// RD4). Mirrors extract.BatchRunner (TryLock + goroutine on the server-lifetime
// context) with the live Status() the scanner has and the extractor lacks.
type SweepRunner struct {
	svc    *Service
	lister SweepLister
	log    *slog.Logger
	// sleep waits out a provider's 429 pause before the one retry (F66 RD8);
	// injectable so tests never wait.
	sleep func(context.Context, time.Duration) error

	mu      sync.Mutex // the single-flight lock (TryLock)
	baseCtx context.Context

	stMu sync.Mutex
	st   SweepStatus
}

// NewSweepRunner wires a runner over the service and the entity lister.
func NewSweepRunner(svc *Service, lister SweepLister, log *slog.Logger) *SweepRunner {
	return &SweepRunner{svc: svc, lister: lister, log: log, sleep: sleepCtx, st: SweepStatus{State: "idle"}}
}

// SetBaseContext supplies the server-lifetime context a sweep runs on, so it is
// cancelled on shutdown rather than tied to the triggering request.
func (r *SweepRunner) SetBaseContext(ctx context.Context) { r.baseCtx = ctx }

// Status is the live read-model for the activity poll.
func (r *SweepRunner) Status() SweepStatus {
	r.stMu.Lock()
	defer r.stMu.Unlock()
	st := r.st
	if st.LastRun != nil {
		lr := *st.LastRun
		// Copy, keeping an empty (non-nil) slice empty so it serialises as [] (P0-3).
		lr.SkippedProviders = append(make([]SkippedProvider, 0, len(lr.SkippedProviders)), lr.SkippedProviders...)
		st.LastRun = &lr
	}
	return st
}

// Trigger starts a sweep of kind (person or studio) in the background and reports
// whether it started one — false when a sweep of either kind is already running,
// which is informational, never an error (RD4). force ignores the staleness window
// (RD3). An unsupported kind is an error.
func (r *SweepRunner) Trigger(kind string, force bool) (bool, error) {
	if kind != model.EnrichEntityPerson && kind != model.EnrichEntityStudio {
		return false, fmt.Errorf("unsupported sweep kind %q", kind)
	}
	if !r.mu.TryLock() {
		return false, nil
	}
	ctx := r.baseCtx
	if ctx == nil {
		ctx = context.Background()
	}
	batchID := newBatchID()
	now := time.Now()
	r.stMu.Lock()
	r.st = SweepStatus{State: "running", Kind: kind, StartedAt: &now, BatchID: batchID, LastRun: r.st.LastRun}
	r.stMu.Unlock()
	go func() {
		defer r.mu.Unlock()
		r.run(ctx, kind, force, batchID, now)
	}()
	return true, nil
}

func newBatchID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "sweep-" + hex.EncodeToString(b[:])
}

// sweepEntity is one entity to refresh: its id and the name the resolve hint carries
// (the same hint the per-entity click builds for a person/studio).
type sweepEntity struct {
	id   int64
	name string
}

func (r *SweepRunner) entities(ctx context.Context, kind string) ([]sweepEntity, error) {
	var out []sweepEntity
	switch kind {
	case model.EnrichEntityPerson:
		people, err := r.lister.ListPeople(ctx, false)
		if err != nil {
			return nil, err
		}
		for _, p := range people {
			out = append(out, sweepEntity{id: p.ID, name: p.Name})
		}
	case model.EnrichEntityStudio:
		studios, err := r.lister.ListStudios(ctx, false)
		if err != nil {
			return nil, err
		}
		for _, s := range studios {
			out = append(out, sweepEntity{id: s.ID, name: s.Name})
		}
	}
	return out, nil
}

// providerState is the per-sweep breaker for one provider (ADR-103 D6): two
// consecutive-streak counters, reset by any successful call; tripped is sticky for
// the rest of the sweep. Only this provider's goroutine touches it within an entity,
// and entities run sequentially, so it needs no lock.
type providerState struct {
	failures int
	pauses   int
	tripped  string // reason, "" until tripped
}

func (r *SweepRunner) run(ctx context.Context, kind string, force bool, batchID string, started time.Time) {
	var supported []string
	for _, src := range r.svc.Sources() {
		if src.Supports(kind) {
			supported = append(supported, src.Name)
		}
	}
	breakers := make(map[string]*providerState, len(supported))
	for _, p := range supported {
		breakers[p] = &providerState{}
	}

	ents, err := r.entities(ctx, kind)
	if err != nil {
		r.finish(kind, batchID, started, 0, SweepCounts{}, nil, err)
		return
	}
	r.stMu.Lock()
	r.st.Total = len(ents)
	r.stMu.Unlock()

	var counts SweepCounts
	for _, e := range ents {
		if ctx.Err() != nil {
			err = ctx.Err()
			break
		}
		matches, merr := r.svc.ProviderMatches(ctx, kind, e.id)
		if merr != nil {
			r.log.Warn("sweep match lookup failed", "kind", kind, "id", e.id, "err", merr)
			counts.Failed += len(supported)
			r.advance(counts)
			continue
		}
		linked := make(map[string]*Match, len(matches))
		for i := range matches {
			linked[matches[i].Provider] = &matches[i]
		}
		hint := Hint{Query: e.name}
		results := make([]PairStatus, len(supported))
		var wg sync.WaitGroup
		for i, provider := range supported {
			wg.Add(1)
			go func(i int, provider string) {
				defer wg.Done()
				results[i] = r.pair(ctx, breakers[provider], kind, e.id, provider, hint, linked[provider], RefreshOpts{
					Force: force, BatchID: batchID, BypassGalleryCap: true,
				})
			}(i, provider)
		}
		wg.Wait()
		for _, status := range results {
			switch status {
			case PairRefreshed, PairAutoApplied:
				counts.Linked++
			case PairNeedsReview:
				counts.NeedsReview++
			case PairNoCandidates:
				counts.NoCandidates++
			case PairFailed:
				counts.Failed++
			case PairStaleSkipped:
				counts.StaleSkipped++
			case PairDismissed, pairBreakerSkipped:
				counts.Skipped++
			}
		}
		r.advance(counts)
	}

	var skipped []SkippedProvider
	for _, p := range supported {
		if reason := breakers[p].tripped; reason != "" {
			skipped = append(skipped, SkippedProvider{Provider: p, Reason: reason})
		}
	}
	r.finish(kind, batchID, started, len(ents), counts, skipped, err)
}

// pairBreakerSkipped is the sweep-only status for a pair not attempted because its
// provider's breaker had tripped; it is counted as skipped, like a dismissed pair.
const pairBreakerSkipped PairStatus = "breaker_skipped"

// pair runs RefreshPair for one provider under the breaker and the retry-once rule
// (F66 RD8/RD9, ADR-103 D4/D6): a tripped provider is not called; a paused provider
// is waited out once and retried; a second pause counts the pair failed and the
// provider's pause streak up; any other failure counts the failure streak up; a
// successful call resets both.
func (r *SweepRunner) pair(ctx context.Context, ps *providerState, kind string, id int64, provider string, hint Hint, link *Match, opts RefreshOpts) PairStatus {
	if ps.tripped != "" {
		return pairBreakerSkipped
	}
	out := r.svc.RefreshPair(ctx, kind, id, provider, hint, link, opts)
	var paused *ErrProviderPaused
	if errors.As(out.Err, &paused) {
		if err := r.sleep(ctx, paused.RetryAfter); err != nil {
			return PairFailed
		}
		out = r.svc.RefreshPair(ctx, kind, id, provider, hint, link, opts)
		if errors.As(out.Err, &paused) {
			ps.pauses++
			if ps.pauses >= breakerPauses {
				ps.tripped = BreakerRateLimited
			}
			return PairFailed
		}
	}
	switch out.Status {
	case PairFailed:
		if ctx.Err() != nil {
			return PairFailed // shutdown, not the provider's fault — no streak
		}
		r.log.Warn("sweep pair failed", "kind", kind, "id", id, "provider", provider, "err", out.Err)
		ps.failures++
		if ps.failures >= breakerFailures {
			ps.tripped = BreakerStoppedResponding
		}
	case PairRefreshed, PairAutoApplied, PairNeedsReview, PairNoCandidates:
		ps.failures, ps.pauses = 0, 0
	}
	return out.Status
}

// advance publishes progress after one entity (done is monotonic, ≤ total).
func (r *SweepRunner) advance(counts SweepCounts) {
	r.stMu.Lock()
	r.st.Done++
	r.st.SweepCounts = counts
	r.stMu.Unlock()
}

// finish publishes the summary, records the enrich-sweep job run and returns the
// status to idle.
func (r *SweepRunner) finish(kind, batchID string, started time.Time, total int, counts SweepCounts, skipped []SkippedProvider, runErr error) {
	finished := time.Now()
	sum := &SweepSummary{
		Kind: kind, FinishedAt: finished, DurationMs: finished.Sub(started).Milliseconds(), BatchID: batchID,
		Total: total, SweepCounts: counts, SkippedProviders: skipped,
	}
	if skipped == nil {
		sum.SkippedProviders = []SkippedProvider{}
	}
	status := model.JobStatusOK
	if runErr != nil {
		status = model.JobStatusErr
		sum.Error = runErr.Error()
	}
	r.stMu.Lock()
	r.st = SweepStatus{State: "idle", LastRun: sum}
	r.stMu.Unlock()

	r.svc.recordRun(model.JobRun{
		Kind: model.JobKindEnrichSweep, Trigger: model.TriggerManual, Status: status,
		StartedAt: started.UTC(), FinishedAt: finished.UTC(), DurationMs: sum.DurationMs,
		Seen: total, Updated: counts.Linked, Skipped: counts.Skipped + counts.StaleSkipped, Errors: counts.Failed,
		ErrorMessage: sum.Error, Detail: sweepDetail(kind, total, counts, skipped), BatchID: batchID,
	})
}

// sweepDetail is the history row's done line (spec P0-4):
// "people · 212 · linked 23 · 31 need review · 2 failed · 44 skipped (tmdb) · 64 recently refreshed".
func sweepDetail(kind string, total int, c SweepCounts, skipped []SkippedProvider) string {
	plural := map[string]string{model.EnrichEntityPerson: "people", model.EnrichEntityStudio: "studios"}[kind]
	parts := []string{plural, fmt.Sprint(total), fmt.Sprintf("linked %d", c.Linked)}
	if c.NeedsReview > 0 {
		parts = append(parts, fmt.Sprintf("%d need review", c.NeedsReview))
	}
	if c.NoCandidates > 0 {
		parts = append(parts, fmt.Sprintf("%d no candidates", c.NoCandidates))
	}
	if c.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", c.Failed))
	}
	if c.Skipped > 0 {
		s := fmt.Sprintf("%d skipped", c.Skipped)
		if len(skipped) > 0 {
			names := make([]string, len(skipped))
			for i, sp := range skipped {
				names[i] = sp.Provider
			}
			s += " (" + strings.Join(names, ", ") + ")"
		}
		parts = append(parts, s)
	}
	if c.StaleSkipped > 0 {
		parts = append(parts, fmt.Sprintf("%d recently refreshed", c.StaleSkipped))
	}
	return strings.Join(parts, " · ")
}
