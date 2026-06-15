package thumbnail

import "sync"

// queue is an in-memory, deduplicated, two-tier priority queue of video IDs.
// High-priority ids (visible/regenerated items, Tier 3) are always drained before
// normal-priority backfill ids (Tier 2). The pending set guarantees an id is
// queued at most once. Durable state lives in the DB (thumbnail_state); this is
// only the in-flight buffer, so a restart re-seeds it from the backfill sweep.
type queue struct {
	mu      sync.Mutex
	high    []int64
	normal  []int64
	pending map[int64]struct{}
	ready   chan struct{} // buffered(1) wakeup signal for blocked workers
}

func newQueue() *queue {
	return &queue{
		pending: make(map[int64]struct{}),
		ready:   make(chan struct{}, 1),
	}
}

// push enqueues id at the given priority unless it is already pending. An id
// already queued at normal priority is not promoted — a minor, bounded miss that
// keeps the structure simple (personal-scale libraries drain quickly anyway).
func (q *queue) push(id int64, high bool) {
	q.mu.Lock()
	if _, dup := q.pending[id]; dup {
		q.mu.Unlock()
		return
	}
	q.pending[id] = struct{}{}
	if high {
		q.high = append(q.high, id)
	} else {
		q.normal = append(q.normal, id)
	}
	q.mu.Unlock()
	q.signal()
}

// signal wakes one blocked worker (non-blocking; a buffered slot coalesces bursts).
func (q *queue) signal() {
	select {
	case q.ready <- struct{}{}:
	default:
	}
}

// pop returns the next id (high tier first), or ok=false when empty. The id
// stays in the pending set until done() is called, so an id being processed
// cannot be re-queued — that would mean two workers generating the same file to
// the same temp path concurrently.
func (q *queue) pop() (int64, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	switch {
	case len(q.high) > 0:
		id := q.high[0]
		q.high = q.high[1:]
		return id, true
	case len(q.normal) > 0:
		id := q.normal[0]
		q.normal = q.normal[1:]
		return id, true
	default:
		return 0, false
	}
}

// done releases an id from the pending set after its job completes, allowing it
// to be enqueued again (e.g. a regenerate, or a retry of a failed item).
func (q *queue) done(id int64) {
	q.mu.Lock()
	delete(q.pending, id)
	q.mu.Unlock()
}

func (q *queue) depth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.high) + len(q.normal)
}

// counts returns the per-tier pending depths (F21.1 activity surface).
func (q *queue) counts() (high, normal int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.high), len(q.normal)
}
