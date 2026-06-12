package scanner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"holodex/internal/metadata"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// fakeRepo is an in-memory Repository keyed by canonical path.
type fakeRepo struct {
	mu     sync.Mutex
	nextID int64
	byPath map[string]repo.VideoStat
	active map[int64]bool
	uparts int // upsert call count
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byPath: map[string]repo.VideoStat{}, active: map[int64]bool{}}
}

func (f *fakeRepo) StatByPath(_ context.Context, path string) (repo.VideoStat, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st, ok := f.byPath[path]
	return st, ok, nil
}

func (f *fakeRepo) UpsertVideo(_ context.Context, v *model.Video, _ []model.ExtraMetadata) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uparts++
	st, ok := f.byPath[v.FilePath]
	if !ok {
		f.nextID++
		st = repo.VideoStat{ID: f.nextID}
	}
	st.Size, st.Mtime = v.FileSize, v.FileMtime
	f.byPath[v.FilePath] = st
	f.active[st.ID] = true
	return st.ID, nil
}

func (f *fakeRepo) DeactivateExcept(_ context.Context, seen []int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	keep := map[int64]bool{}
	for _, id := range seen {
		keep[id] = true
	}
	var n int64
	for id, isActive := range f.active {
		if isActive && !keep[id] {
			f.active[id] = false
			n++
		}
	}
	return n, nil
}

// stubExtractor returns fixed metadata without external binaries.
type stubExtractor struct{}

func (stubExtractor) Extract(_ context.Context, path string) (metadata.Extracted, error) {
	return metadata.Extracted{Title: filepath.Base(path), Width: 1920, Height: 1080, DurationSec: 60}, nil
}

// artExtractor reports a configurable embedded-cover-art flag.
type artExtractor struct{ hasArt bool }

func (a artExtractor) Extract(_ context.Context, path string) (metadata.Extracted, error) {
	return metadata.Extracted{
		Title: filepath.Base(path), Width: 1920, Height: 1080, DurationSec: 60,
		HasCoverArt: a.hasArt,
	}, nil
}

// fakeThumbnailer records pipeline calls.
type fakeThumbnailer struct {
	mu        sync.Mutex
	extracted []int64
	enqueued  []int64
	extractOK bool
}

func (f *fakeThumbnailer) ExtractEmbedded(_ context.Context, id int64, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.extracted = append(f.extracted, id)
	return f.extractOK, nil
}

func (f *fakeThumbnailer) Enqueue(id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enqueued = append(f.enqueued, id)
}

func TestThumbnailHook(t *testing.T) {
	cases := []struct {
		name          string
		hasArt        bool
		extractOK     bool
		wantExtracted bool
		wantEnqueued  bool
	}{
		{"embedded art extracted", true, true, true, false},
		{"art flagged but extract fails -> generate", true, false, true, true},
		{"no art -> generate", false, false, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "a.mkv"), 100)
			fr := newFakeRepo()
			s := New(Config{MediaPath: dir, FollowSymlinks: true, MaxDepth: 64, MinAge: time.Minute, Workers: 1},
				slog.New(slog.NewTextHandler(io.Discard, nil)), fr, artExtractor{hasArt: c.hasArt})
			ft := &fakeThumbnailer{extractOK: c.extractOK}
			s.SetThumbnailer(ft)

			if err := s.ScanOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if got := len(ft.extracted) > 0; got != c.wantExtracted {
				t.Errorf("extracted=%v, want %v", got, c.wantExtracted)
			}
			if got := len(ft.enqueued) > 0; got != c.wantEnqueued {
				t.Errorf("enqueued=%v, want %v", got, c.wantEnqueued)
			}
		})
	}
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	// Backdate mtime so the MinAge guard never skips fixtures.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

func newTestScanner(media string, r Repository) *Scanner {
	return New(Config{MediaPath: media, FollowSymlinks: true, MaxDepth: 64, MinAge: time.Minute, Workers: 2},
		slog.New(slog.NewTextHandler(io.Discard, nil)), r, stubExtractor{})
}

func TestScanAddsSkipsAndRemoves(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mkv"), 100)
	writeFile(t, filepath.Join(dir, "sub", "b.mp4"), 200)
	writeFile(t, filepath.Join(dir, "notes.txt"), 10) // ignored (non-media)

	fr := newFakeRepo()
	s := newTestScanner(dir, fr)
	ctx := context.Background()

	// First scan: both media files indexed.
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatalf("scan 1: %v", err)
	}
	if fr.uparts != 2 {
		t.Errorf("first scan upserts = %d, want 2", fr.uparts)
	}

	// Second scan, unchanged: no new extraction/upsert.
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatalf("scan 2: %v", err)
	}
	if fr.uparts != 2 {
		t.Errorf("unchanged rescan upserts = %d, want still 2", fr.uparts)
	}

	// Remove one file: reconciliation deactivates it.
	if err := os.Remove(filepath.Join(dir, "a.mkv")); err != nil {
		t.Fatal(err)
	}
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatalf("scan 3: %v", err)
	}
	activeCount := 0
	for _, a := range fr.active {
		if a {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Errorf("active after removal = %d, want 1", activeCount)
	}
}

func TestWatcherIndexesNewFile(t *testing.T) {
	dir := t.TempDir()
	fr := newFakeRepo()
	// MinAge 0 so a freshly created file isn't held by the mid-copy guard.
	s := New(Config{MediaPath: dir, FollowSymlinks: true, MaxDepth: 64, MinAge: 0, Workers: 2},
		slog.New(slog.NewTextHandler(io.Discard, nil)), fr, stubExtractor{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx, time.Hour) // long interval → only the fs-watcher can trigger a re-scan

	time.Sleep(300 * time.Millisecond) // let the watcher attach
	writeFile(t, filepath.Join(dir, "new.mkv"), 100)

	// Watcher debounces ~2s before scanning; poll for the resulting upsert.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		fr.mu.Lock()
		n := fr.uparts
		fr.mu.Unlock()
		if n >= 1 {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatal("fs-watcher did not index the new file within 8s")
}

func TestChangedFileIsReindexed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.mkv")
	writeFile(t, path, 100)

	fr := newFakeRepo()
	s := newTestScanner(dir, fr)
	ctx := context.Background()
	_ = s.ScanOnce(ctx)

	// Grow the file and backdate again -> (size,mtime) differ -> re-extract.
	writeFile(t, path, 500)
	_ = s.ScanOnce(ctx)
	if fr.uparts != 2 {
		t.Errorf("upserts after change = %d, want 2", fr.uparts)
	}
}
