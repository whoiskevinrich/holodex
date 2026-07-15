package scanner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

// TestExtractionHook proves the F48.5c import-time trigger: scanning a file
// runs the wired extraction hook once per indexed video, right alongside the
// existing studio-relink hook (same best-effort, post-upsert shape).
func TestExtractionHook(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mkv"), 100)
	fr := newFakeRepo()
	s := New(Config{MediaPath: dir, FollowSymlinks: true, MaxDepth: 64, MinAge: time.Minute, Workers: 1},
		slog.New(slog.NewTextHandler(io.Discard, nil)), fr, stubExtractor{})

	var extracted []int64
	s.SetExtractionRunner(func(_ context.Context, id int64) error {
		extracted = append(extracted, id)
		return nil
	})

	if err := s.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(extracted) != 1 {
		t.Fatalf("extraction hook called %d times, want 1", len(extracted))
	}
}

// TestExtractionHook_ErrorDoesNotFailScan proves the hook is best-effort: a
// failing extraction is logged, never aborting the scan (mirrors the relink
// hook's own never-abort contract).
func TestExtractionHook_ErrorDoesNotFailScan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mkv"), 100)
	fr := newFakeRepo()
	s := New(Config{MediaPath: dir, FollowSymlinks: true, MaxDepth: 64, MinAge: time.Minute, Workers: 1},
		slog.New(slog.NewTextHandler(io.Discard, nil)), fr, stubExtractor{})
	s.SetExtractionRunner(func(context.Context, int64) error { return errExtractionFailed })

	if err := s.ScanOnce(context.Background()); err != nil {
		t.Fatalf("ScanOnce returned an error despite the extraction hook failing: %v", err)
	}
	if fr.uparts != 1 {
		t.Fatalf("upserts = %d, want 1 (the extraction failure must not block indexing)", fr.uparts)
	}
}

var errExtractionFailed = errors.New("extraction failed")
