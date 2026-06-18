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
	mu      sync.Mutex
	nextID  int64
	byPath  map[string]repo.VideoStat
	active  map[int64]bool
	deleted map[int64]bool // soft-deleted ids (F24)
	uparts  int            // upsert call count
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byPath: map[string]repo.VideoStat{}, active: map[int64]bool{}, deleted: map[int64]bool{}}
}

func (f *fakeRepo) StatByPath(_ context.Context, path string) (repo.VideoStat, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st, ok := f.byPath[path]
	if ok {
		st.Active = f.active[st.ID]
		st.Deleted = f.deleted[st.ID]
	}
	return st, ok, nil
}

// markDeleted simulates an owner soft-delete on an already-indexed row (F24).
func (f *fakeRepo) markDeleted(id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted[id] = true
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

func (f *fakeRepo) Reactivate(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active[id] = true
	return nil
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

// fakeRecorder captures job-history records for the status/recording test.
type fakeRecorder struct {
	mu   sync.Mutex
	runs []model.JobRun
}

func (f *fakeRecorder) RecordJobRun(_ context.Context, run model.JobRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs = append(f.runs, run)
	return nil
}

func TestScanStatusAndRecord(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mkv"), 100)
	s := newTestScanner(dir, newFakeRepo())
	rec := &fakeRecorder{}
	s.SetJobRecorder(rec)

	// Idle, no history before the first pass.
	if st := s.Status(); st.State != "idle" || st.LastRun != nil {
		t.Fatalf("pre-scan status = %+v, want idle with no last run", st)
	}

	if err := s.ScanOnce(context.Background()); err != nil {
		t.Fatalf("scan: %v", err)
	}

	st := s.Status()
	if st.State != "idle" {
		t.Errorf("state = %q, want idle after completion", st.State)
	}
	if st.LastRun == nil {
		t.Fatal("last run nil after a completed scan")
	}
	if st.LastRun.Trigger != model.TriggerPeriodic {
		t.Errorf("last-run trigger = %q, want periodic", st.LastRun.Trigger)
	}
	if st.LastRun.Added != 1 {
		t.Errorf("last-run added = %d, want 1", st.LastRun.Added)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.runs) != 1 {
		t.Fatalf("recorded runs = %d, want 1", len(rec.runs))
	}
	if rec.runs[0].Kind != model.JobKindScan || rec.runs[0].Status != model.JobStatusOK {
		t.Errorf("recorded run = %+v, want scan/success", rec.runs[0])
	}
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
	if n := activeCount(fr); n != 1 {
		t.Errorf("active after removal = %d, want 1", n)
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

func activeCount(fr *fakeRepo) int {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	n := 0
	for _, a := range fr.active {
		if a {
			n++
		}
	}
	return n
}

// Issue #26: a row deactivated by a transient empty walk must be reactivated when
// the file reappears unchanged — without re-extraction (no new upsert).
func TestReactivatesUnchangedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.mkv")
	writeFile(t, path, 100)

	fr := newFakeRepo()
	s := newTestScanner(dir, fr)
	ctx := context.Background()

	if err := s.ScanOnce(ctx); err != nil {
		t.Fatalf("scan 1: %v", err)
	}
	if activeCount(fr) != 1 {
		t.Fatalf("active after first scan = %d, want 1", activeCount(fr))
	}

	// Simulate a prior pass having deactivated the row (e.g. a one-off empty walk).
	if _, err := fr.DeactivateExcept(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if activeCount(fr) != 0 {
		t.Fatalf("active after forced deactivation = %d, want 0", activeCount(fr))
	}

	// Next scan sees the file unchanged: it must reactivate without re-extracting.
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatalf("scan 2: %v", err)
	}
	if activeCount(fr) != 1 {
		t.Errorf("active after reactivation scan = %d, want 1", activeCount(fr))
	}
	if fr.uparts != 1 {
		t.Errorf("upserts after reactivation = %d, want still 1 (no re-extract)", fr.uparts)
	}
	if st := s.Status(); st.LastRun == nil || st.LastRun.Updated != 1 {
		t.Errorf("reactivation should count as 1 updated; last run = %+v", st.LastRun)
	}
}

// F24.3 (the cardinal invariant): a soft-deleted row whose file is still on disk
// must survive a re-scan untouched — never reactivated, never re-extracted, never
// deactivated (it's recorded as seen). This is what storing the delete in `active`
// could not guarantee: the #26 reactivation fast-path would otherwise resurrect it.
func TestSoftDeletedRowSurvivesRescan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.mkv")
	writeFile(t, path, 100)

	fr := newFakeRepo()
	s := newTestScanner(dir, fr)
	ctx := context.Background()

	if err := s.ScanOnce(ctx); err != nil {
		t.Fatalf("scan 1: %v", err)
	}
	if fr.uparts != 1 || activeCount(fr) != 1 {
		t.Fatalf("after first scan: upserts=%d active=%d", fr.uparts, activeCount(fr))
	}

	// Owner soft-deletes the row; its file remains on disk, unchanged.
	fr.markDeleted(1)

	if err := s.ScanOnce(ctx); err != nil {
		t.Fatalf("scan 2: %v", err)
	}
	// No re-extract (upserts unchanged) and not deactivated by end-of-scan
	// reconciliation (recorded as seen → still "active" on the disk-presence axis).
	if fr.uparts != 1 {
		t.Errorf("upserts after rescan of soft-deleted file = %d, want still 1 (no re-extract)", fr.uparts)
	}
	if !fr.active[1] {
		t.Errorf("soft-deleted row was deactivated by the rescan; it should be left as-is")
	}
	if !fr.deleted[1] {
		t.Errorf("soft-delete flag was cleared by the rescan")
	}
	// The pass counts it as skipped, not added/updated.
	if st := s.Status(); st.LastRun == nil || st.LastRun.Added != 0 || st.LastRun.Updated != 0 {
		t.Errorf("soft-deleted file should not count as added/updated; last run = %+v", st.LastRun)
	}
}

// Issue #26: a walk that sees zero media files (a transiently empty/unreadable
// media root) must NOT mass-deactivate the existing library.
func TestZeroFileWalkSkipsDeactivation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mkv"), 100)
	writeFile(t, filepath.Join(dir, "b.mp4"), 200)

	fr := newFakeRepo()
	s := newTestScanner(dir, fr)
	ctx := context.Background()

	if err := s.ScanOnce(ctx); err != nil {
		t.Fatalf("scan 1: %v", err)
	}
	if activeCount(fr) != 2 {
		t.Fatalf("active after first scan = %d, want 2", activeCount(fr))
	}

	// Media root goes momentarily empty (mount glitch): remove every file.
	for _, name := range []string{"a.mkv", "b.mp4"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatalf("scan 2: %v", err)
	}

	// The library must remain intact rather than being mass-hidden.
	if activeCount(fr) != 2 {
		t.Errorf("active after zero-file walk = %d, want 2 (deactivation skipped)", activeCount(fr))
	}
	if st := s.Status(); st.LastRun == nil || st.LastRun.Removed != 0 {
		t.Errorf("zero-file walk should remove nothing; last run = %+v", st.LastRun)
	}
}
