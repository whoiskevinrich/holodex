package thumbnail

import "testing"

func TestQueueDedupAndDepth(t *testing.T) {
	q := newQueue()
	q.push(1, false)
	q.push(1, false) // duplicate ignored
	q.push(2, false)
	if d := q.depth(); d != 2 {
		t.Fatalf("depth = %d, want 2", d)
	}
}

func TestQueueHighPriorityFirst(t *testing.T) {
	q := newQueue()
	q.push(1, false)
	q.push(2, false)
	q.push(3, true) // high jumps ahead of the normal items

	got := drain(q)
	want := []int64{3, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("drained %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("drained %v, want %v", got, want)
		}
	}
}

func TestQueueDedupWhileInFlight(t *testing.T) {
	q := newQueue()
	q.push(7, false)
	if _, ok := q.pop(); !ok {
		t.Fatal("expected an item")
	}
	// A popped id stays pending until done(), so a re-enqueue mid-flight is a
	// no-op — two workers must never process the same id (shared temp path).
	q.push(7, false)
	if d := q.depth(); d != 0 {
		t.Fatalf("depth while in-flight = %d, want 0 (deduped)", d)
	}
	// After done() the id can be queued again (regenerate / retry).
	q.done(7)
	q.push(7, false)
	if d := q.depth(); d != 1 {
		t.Fatalf("depth after done+requeue = %d, want 1", d)
	}
}

func drain(q *queue) []int64 {
	var out []int64
	for {
		id, ok := q.pop()
		if !ok {
			return out
		}
		out = append(out, id)
	}
}
