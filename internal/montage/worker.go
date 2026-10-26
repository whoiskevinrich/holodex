package montage

import (
	"context"
	"time"
)

// jobTimeout bounds one montage: several seeks on a slow disk plus an encode.
const jobTimeout = 5 * time.Minute

// backfillLimit caps one startup sweep in automatic mode.
const backfillLimit = 100000

// worker drains the queue until ctx is done. Before each job it waits while any
// still is waiting (spec P0-10), the one read across the boundary.
func (m *Manager) worker(ctx context.Context) {
	for {
		if !m.waitForStills(ctx) {
			return
		}
		id, ok := m.queue.pop()
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-m.queue.ready:
				continue
			}
		}
		if h, n := m.queue.counts(); h+n > 0 {
			m.queue.signal()
		}
		m.setMaking(id, true)
		m.inFlight.Add(1)
		m.process(ctx, id)
		m.inFlight.Add(-1)
		m.setMaking(id, false)
		m.queue.done(id)
	}
}

// waitForStills blocks while the still queue has work. False means ctx is done.
func (m *Manager) waitForStills(ctx context.Context) bool {
	for m.stills != nil && m.stills.QueueDepth() > 0 {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(m.stillPoll):
		}
	}
	return ctx.Err() == nil
}

func (m *Manager) setMaking(id int64, on bool) {
	m.mu.Lock()
	if on {
		m.making[id] = struct{}{}
	} else {
		delete(m.making, id)
	}
	m.mu.Unlock()
}

// process makes one montage and records the outcome. A shutdown-cancelled job
// records nothing, so it is neither failed nor marked attempted.
func (m *Manager) process(ctx context.Context, id int64) {
	c, ok, err := m.repo.MontageCandidateByID(ctx, id)
	if err != nil || !ok {
		if err != nil {
			m.log.Warn("montage candidate", "id", id, "err", err)
		}
		return
	}
	starts := clipStarts(c.DurationSec)
	if starts == nil {
		m.record(ctx, id, StateFailed, ErrTooShort.Error())
		return
	}
	jctx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()
	if err := m.gen(jctx, c.FilePath, starts, Path(m.cfg.Dir, id)); err != nil {
		if ctx.Err() != nil {
			return
		}
		m.log.Warn("montage failed", "id", id, "err", err)
		m.record(ctx, id, StateFailed, err.Error())
		return
	}
	m.record(ctx, id, StateGenerated, "")
}

func (m *Manager) record(ctx context.Context, id int64, state, errText string) {
	if err := m.repo.SetMontageState(ctx, id, state, errText); err != nil {
		m.log.Warn("record montage state", "id", id, "err", err)
	}
}

// backfill queues every never-attempted video once, a few seconds after start
// (automatic mode only).
func (m *Manager) backfill(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
	}
	cands, err := m.repo.MontageBackfillCandidates(ctx, MinDurationSec, backfillLimit)
	if err != nil {
		m.log.Warn("montage backfill", "err", err)
		return
	}
	for _, c := range cands {
		m.queue.push(c.ID, false)
	}
	if len(cands) > 0 {
		m.log.Info("montage backfill queued", "count", len(cands))
	}
}
