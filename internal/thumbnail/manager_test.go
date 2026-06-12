package thumbnail

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// fakeRepo is an in-memory Repository for pipeline tests.
type fakeRepo struct {
	mu     sync.Mutex
	cands  map[int64]repo.ThumbnailCandidate
	states map[int64]string // "" == NULL
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{cands: map[int64]repo.ThumbnailCandidate{}, states: map[int64]string{}}
}

func (f *fakeRepo) add(c repo.ThumbnailCandidate) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cands[c.ID] = c
}

func (f *fakeRepo) state(id int64) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.states[id]
}

func (f *fakeRepo) SetThumbnailState(_ context.Context, id int64, s string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states[id] = s
	return nil
}

func (f *fakeRepo) ResetThumbnailState(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states[id] = ""
	return nil
}

func (f *fakeRepo) ThumbnailBackfillCandidates(_ context.Context, limit int) ([]repo.ThumbnailCandidate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []repo.ThumbnailCandidate
	for id, c := range f.cands {
		if s := f.states[id]; s == "" || s == model.ThumbnailFailed {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeRepo) ThumbnailCandidateByID(_ context.Context, id int64) (repo.ThumbnailCandidate, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.cands[id]
	return c, ok, nil
}

func testManager(t *testing.T, fr *fakeRepo) *Manager {
	t.Helper()
	m := New(Config{Enabled: true, Workers: 2, Width: 400, Dir: t.TempDir()},
		slog.New(slog.NewTextHandler(io.Discard, nil)), fr)
	return m
}

// TestProcessGeneratesAndMarks drives the worker loop end-to-end with a stub
// frame generator and asserts the file and state transition.
func TestProcessGeneratesAndMarks(t *testing.T) {
	fr := newFakeRepo()
	fr.add(repo.ThumbnailCandidate{ID: 1, FilePath: "/m/a.mkv", DurationSec: 100})
	m := testManager(t, fr)
	m.gen = func(_ context.Context, _ repo.ThumbnailCandidate, out string) error {
		return os.WriteFile(out, []byte("JPEG"), 0o644)
	}

	m.process(context.Background(), 1)

	if got := fr.state(1); got != model.ThumbnailGenerated {
		t.Fatalf("state = %q, want %q", got, model.ThumbnailGenerated)
	}
	if _, err := os.Stat(m.thumbPath(1)); err != nil {
		t.Fatalf("thumbnail file missing: %v", err)
	}
}

func TestProcessMarksFailed(t *testing.T) {
	fr := newFakeRepo()
	fr.add(repo.ThumbnailCandidate{ID: 2, FilePath: "/m/bad.mkv", DurationSec: 0})
	m := testManager(t, fr)
	m.gen = func(_ context.Context, _ repo.ThumbnailCandidate, _ string) error {
		return io.ErrUnexpectedEOF
	}

	m.process(context.Background(), 2)

	if got := fr.state(2); got != model.ThumbnailFailed {
		t.Fatalf("state = %q, want %q", got, model.ThumbnailFailed)
	}
}

func TestRunDrainsBackfill(t *testing.T) {
	fr := newFakeRepo()
	for id := int64(1); id <= 5; id++ {
		fr.add(repo.ThumbnailCandidate{ID: id, FilePath: "/m/x.mkv", DurationSec: 10})
	}
	m := New(Config{Enabled: true, Workers: 3, Width: 400, Backfill: "eager", Dir: t.TempDir()},
		slog.New(slog.NewTextHandler(io.Discard, nil)), fr)
	var generated sync.Map
	m.gen = func(_ context.Context, c repo.ThumbnailCandidate, out string) error {
		generated.Store(c.ID, true)
		return os.WriteFile(out, []byte("JPEG"), 0o644)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Enqueue directly (don't wait the 5s backfill delay) and run the pool.
	m.EnqueueHigh([]int64{1, 2, 3, 4, 5})
	go m.Run(ctx)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		all := true
		for id := int64(1); id <= 5; id++ {
			if fr.state(id) != model.ThumbnailGenerated {
				all = false
			}
		}
		if all {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("not all thumbnails generated within 3s")
}

func TestExtractEmbedded(t *testing.T) {
	fr := newFakeRepo()
	fr.add(repo.ThumbnailCandidate{ID: 9, FilePath: "/m/art.mkv"})
	m := testManager(t, fr)
	m.art = func(_ context.Context, _, dst string) (bool, error) {
		return true, os.WriteFile(dst, []byte("COVER"), 0o644)
	}

	ok, err := m.ExtractEmbedded(context.Background(), 9, "/m/art.mkv")
	if err != nil || !ok {
		t.Fatalf("ExtractEmbedded = (%v, %v)", ok, err)
	}
	if got := fr.state(9); got != model.ThumbnailEmbedded {
		t.Fatalf("state = %q, want %q", got, model.ThumbnailEmbedded)
	}
	if !strings.HasSuffix(m.thumbPath(9), "9.jpg") {
		t.Fatalf("unexpected thumb path %q", m.thumbPath(9))
	}
}

func TestDisabledManagerNoops(t *testing.T) {
	fr := newFakeRepo()
	m := New(Config{Enabled: false, Dir: t.TempDir()},
		slog.New(slog.NewTextHandler(io.Discard, nil)), fr)
	m.EnqueueHigh([]int64{1})
	m.Enqueue(2)
	if d := m.QueueDepth(); d != 0 {
		t.Fatalf("disabled manager queued work: depth=%d", d)
	}
	ok, err := m.ExtractEmbedded(context.Background(), 1, "/m/a.mkv")
	if ok || err != nil {
		t.Fatalf("disabled ExtractEmbedded = (%v,%v)", ok, err)
	}
}
