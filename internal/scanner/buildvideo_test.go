package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// BuildVideoFromFile is the read half of a forced per-item refresh (F31,
// ADR-047): it always re-extracts (no (size, mtime) change-detection) and never
// writes to the repo — persistence is the caller's apply phase.
func TestBuildVideoFromFileForcesExtractWithoutPersisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	fr := newFakeRepo()
	s := newTestScanner(dir, fr)

	v, _, err := s.BuildVideoFromFile(context.Background(), path)
	if err != nil {
		t.Fatalf("BuildVideoFromFile: %v", err)
	}
	// Reflects the (stub) extractor output — it re-read the file rather than
	// short-circuiting on an unchanged (size, mtime).
	if v.Title != "clip.mp4" || v.Width != 1920 || v.Height != 1080 {
		t.Fatalf("unexpected built video: %+v", v)
	}
	// Read-only: persistence is the caller's (apply) job.
	if fr.uparts != 0 {
		t.Fatalf("BuildVideoFromFile must not write to the repo: upserts=%d", fr.uparts)
	}
}
