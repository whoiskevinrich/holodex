package montage

import (
	"slices"
	"sync"
)

// queue is an in-memory, deduplicated, two-tier priority queue of video IDs:
// owner requests (high) drain before the automatic backfill (normal). It is the
// same shape as the still thumbnail's queue, but a separate copy and a separate
// instance, so the two pipelines share no state (F83 RD5). Durable state lives in
// video_montages; a restart drops whatever was queued.
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

// push enqueues id unless it is already pending. A high push of an id waiting in
// the normal tier promotes it, so an owner request never waits behind the
// automatic backfill (spec P0-15).
func (q *queue) push(id int64, high bool) {
	q.mu.Lock()
	if _, dup := q.pending[id]; dup {
		if high {
			if i := slices.Index(q.normal, id); i >= 0 {
				q.normal = slices.Delete(q.normal, i, i+1)
				q.high = append(q.high, id)
			}
		}
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

func (q *queue) signal() {
	select {
	case q.ready <- struct{}{}:
	default:
	}
}

// pop returns the next id (high tier first). The id stays pending until done().
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

func (q *queue) done(id int64) {
	q.mu.Lock()
	delete(q.pending, id)
	q.mu.Unlock()
}

func (q *queue) isPending(id int64) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	_, ok := q.pending[id]
	return ok
}

func (q *queue) counts() (high, normal int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.high), len(q.normal)
}
