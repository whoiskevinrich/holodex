package thumbnail

import (
	"context"
	"time"

	"holodex/internal/model"
)

// jobTimeout bounds a single ffmpeg generation. Generous: pathological inputs on
// slow disks can take seconds to seek even though CPU cost is sub-second.
const jobTimeout = 2 * time.Minute

// worker drains the queue until ctx is cancelled. On waking it pops one job,
// nudges a sibling if work remains (so the pool fans out under a backlog), then
// processes the job.
func (m *Manager) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
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
		if m.queue.depth() > 0 {
			m.queue.signal()
		}
		m.inFlight.Add(1)
		m.process(ctx, id)
		m.inFlight.Add(-1)
		m.queue.done(id)
	}
}

// process generates one thumbnail and records the outcome. A failure that is not
// caused by shutdown is marked "failed" (retried by the next startup sweep); a
// shutdown-cancelled job is left untouched (NULL) so it is retried next run.
func (m *Manager) process(ctx context.Context, id int64) {
	cand, ok, err := m.repo.ThumbnailCandidateByID(ctx, id)
	if err != nil {
		m.log.Warn("thumbnail: resolve candidate", "id", id, "err", err)
		return
	}
	if !ok {
		return // video removed between enqueue and now
	}

	jobCtx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	if err := m.gen(jobCtx, cand, m.thumbPath(id), m.posterPath(id)); err != nil {
		if ctx.Err() != nil {
			return // shutting down; leave state NULL for next run
		}
		m.log.Warn("thumbnail: generate", "id", id, "path", cand.FilePath, "err", err)
		if serr := m.repo.SetThumbnailState(ctx, id, model.ThumbnailFailed); serr != nil {
			m.log.Warn("thumbnail: mark failed", "id", id, "err", serr)
		}
		return
	}
	if err := m.repo.SetThumbnailState(ctx, id, model.ThumbnailGenerated); err != nil {
		m.log.Warn("thumbnail: mark generated", "id", id, "err", err)
		return
	}
	m.log.Debug("thumbnail generated", "id", id)
}

// backfill is a one-shot sweep run at startup (eager mode): it enqueues every
// active video still needing an image so the whole library is filled in the
// background. New files indexed after startup are enqueued by the scanner; the
// queue's dedup absorbs any overlap.
func (m *Manager) backfill(ctx context.Context) {
	// Let the initial scan settle so freshly-indexed rows are present.
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
	}

	const maxSeed = 100000
	cands, err := m.repo.ThumbnailBackfillCandidates(ctx, maxSeed)
	if err != nil {
		m.log.Warn("thumbnail backfill query failed", "err", err)
		return
	}
	if len(cands) == 0 {
		return
	}
	m.log.Info("thumbnail backfill seeding", "count", len(cands))
	for _, c := range cands {
		select {
		case <-ctx.Done():
			return
		default:
		}
		m.queue.push(c.ID, false)
	}
}
