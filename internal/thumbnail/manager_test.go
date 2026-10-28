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

	"holodex/internal/fieldsource"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// fakeRepo is an in-memory Repository for pipeline tests.
type fakeRepo struct {
	mu     sync.Mutex
	cands  map[int64]repo.ThumbnailCandidate
	states map[int64]string // "" == NULL

	decisions map[int64]string // poster_url decision source
	sources   map[int64]map[string]repo.PosterSource
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{cands: map[int64]repo.ThumbnailCandidate{}, states: map[int64]string{},
		decisions: map[int64]string{}, sources: map[int64]map[string]repo.PosterSource{}}
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

func (f *fakeRepo) PosterDecision(_ context.Context, id int64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.decisions[id], nil
}

func (f *fakeRepo) SetPosterSource(_ context.Context, id int64, kind, hash string, frameAt *float64, decide bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sources[id] == nil {
		f.sources[id] = map[string]repo.PosterSource{}
	}
	f.sources[id][kind] = repo.PosterSource{Kind: kind, Hash: hash, FrameAt: frameAt}
	if decide {
		f.decisions[id] = fieldsource.ForLocal(kind)
	}
	return nil
}

func (f *fakeRepo) DeletePosterSource(_ context.Context, id int64, kind string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sources[id], kind)
	if f.decisions[id] == fieldsource.ForLocal(kind) {
		delete(f.decisions, id)
		return true, nil
	}
	return false, nil
}

func (f *fakeRepo) VideosWithThumbnailImage(_ context.Context) ([]repo.ThumbnailStateRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []repo.ThumbnailStateRow
	for id, s := range f.states {
		if model.HasThumbnailImage(s) {
			out = append(out, repo.ThumbnailStateRow{ID: id, State: s})
		}
	}
	return out, nil
}

func (f *fakeRepo) source(id int64, kind string) (repo.PosterSource, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ps, ok := f.sources[id][kind]
	return ps, ok
}

func (f *fakeRepo) decide(id int64, source string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.decisions[id] = source
}

// writeIf writes a stub tier, skipping the thumbnail tier the pipeline no longer
// asks for (F85: a source slot is one file, and only Present derives the pair).
func writeIf(path, data string) error {
	if path == "" {
		return nil
	}
	return os.WriteFile(path, []byte(data), 0o644)
}

func readString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
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
	m.gen = func(_ context.Context, _ repo.ThumbnailCandidate, thumb, poster string) error {
		if err := writeIf(thumb, "JPEG"); err != nil {
			return err
		}
		return os.WriteFile(poster, []byte("JPEGPOSTER"), 0o644)
	}

	m.process(context.Background(), 1)

	if got := fr.state(1); got != model.ThumbnailGenerated {
		t.Fatalf("state = %q, want %q", got, model.ThumbnailGenerated)
	}
	if got := readString(t, m.slotPath(1, SlotAuto)); got != "JPEGPOSTER" {
		t.Fatalf("auto slot = %q", got)
	}
	if got := readString(t, m.posterPath(1)); got != "JPEGPOSTER" {
		t.Fatalf("served poster = %q", got)
	}
	if _, err := os.Stat(m.thumbPath(1)); err != nil {
		t.Fatalf("thumbnail file missing: %v", err)
	}
	// G7: the automatic source exists once a frame was generated, never decided.
	if ps, ok := fr.source(1, fieldsource.LocalAutomatic); !ok || ps.Hash != ContentHash([]byte("JPEGPOSTER")) {
		t.Fatalf("automatic source = %+v, %v", ps, ok)
	}
	if d, _ := fr.PosterDecision(context.Background(), 1); d != "" {
		t.Fatalf("generation decided the poster: %q", d)
	}
}

func TestProcessMarksFailed(t *testing.T) {
	fr := newFakeRepo()
	fr.add(repo.ThumbnailCandidate{ID: 2, FilePath: "/m/bad.mkv", DurationSec: 0})
	m := testManager(t, fr)
	m.gen = func(_ context.Context, _ repo.ThumbnailCandidate, _, _ string) error {
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
	m.gen = func(_ context.Context, c repo.ThumbnailCandidate, thumb, poster string) error {
		generated.Store(c.ID, true)
		if err := writeIf(thumb, "JPEG"); err != nil {
			return err
		}
		return os.WriteFile(poster, []byte("JPEGPOSTER"), 0o644)
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
	m.art = func(_ context.Context, _, thumbDst, posterDst string) (bool, error) {
		if err := writeIf(thumbDst, "COVER"); err != nil {
			return false, err
		}
		return true, os.WriteFile(posterDst, []byte("COVERPOSTER"), 0o644)
	}

	ok, err := m.ExtractEmbedded(context.Background(), 9, "/m/art.mkv")
	if err != nil || !ok {
		t.Fatalf("ExtractEmbedded = (%v, %v)", ok, err)
	}
	if got := fr.state(9); got != model.ThumbnailEmbedded {
		t.Fatalf("state = %q, want %q", got, model.ThumbnailEmbedded)
	}
	// The art lands in the file slot; Present projects it onto the served pair.
	if got := readString(t, m.slotPath(9, SlotFile)); got != "COVERPOSTER" {
		t.Fatalf("file slot = %q", got)
	}
	if got := readString(t, m.posterPath(9)); got != "COVERPOSTER" {
		t.Fatalf("served poster = %q", got)
	}
	if !strings.HasSuffix(m.thumbPath(9), "9.jpg") {
		t.Fatalf("unexpected thumb path %q", m.thumbPath(9))
	}
	if !strings.HasSuffix(m.posterPath(9), "9-poster.jpg") {
		t.Fatalf("unexpected poster path %q", m.posterPath(9))
	}
}

// TestUploadedPosterSurvivesPostWriteExtract is F85's regression R1 (P0-6): the
// post-write hook re-extracts embedded cover art after every writeback, and that
// must never displace the owner's uploaded poster on the page.
func TestUploadedPosterSurvivesPostWriteExtract(t *testing.T) {
	fr := newFakeRepo()
	fr.add(repo.ThumbnailCandidate{ID: 9, FilePath: "/m/art.mkv"})
	m := testManager(t, fr)
	m.art = func(_ context.Context, _, thumbDst, posterDst string) (bool, error) {
		if err := writeIf(thumbDst, "COVER"); err != nil {
			return false, err
		}
		return true, os.WriteFile(posterDst, []byte("COVERPOSTER"), 0o644)
	}
	// The file's cover art is extracted at index time; then the owner uploads.
	if _, err := m.ExtractEmbedded(context.Background(), 9, "/m/art.mkv"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetOwnerImage(context.Background(), 9, fieldsource.LocalUpload, []byte("UPLOAD"), nil); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(m.posterPath(9))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := m.ExtractEmbedded(context.Background(), 9, "/m/art.mkv"); err != nil {
		t.Fatalf("ExtractEmbedded: %v", err)
	}

	for _, p := range []string{m.thumbPath(9), m.posterPath(9)} {
		if got, _ := os.ReadFile(p); string(got) != "UPLOAD" {
			t.Fatalf("%s = %q after the post-write extract, want the upload", p, got)
		}
	}
	if after, _ := os.Stat(m.posterPath(9)); !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("served poster was rewritten, so its ?v= token moved")
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
