package scanner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"holodex/internal/metadata"
	"holodex/internal/repo"
)

// countingExtractor wraps stubExtractor and counts Extract calls.
type countingExtractor struct{ n atomic.Int64 }

func (c *countingExtractor) Extract(ctx context.Context, path string) (metadata.Extracted, error) {
	c.n.Add(1)
	return stubExtractor{}.Extract(ctx, path)
}

// writeAged writes b to path and backdates its mtime past the MinAge guard.
func writeAged(t *testing.T, path string, b []byte, age time.Duration) time.Time {
	t.Helper()
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-age).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	return old.UTC()
}

func TestScanStoresFingerprintOnInsert(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.mp4")
	writeAged(t, p, seededBytes(10, 1<<20), time.Hour)
	fr := newFakeRepo()
	if err := newTestScanner(dir, fr).ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, ok, _ := fr.StatByPath(context.Background(), p)
	if !ok {
		t.Fatal("file not indexed")
	}
	if got, want := fr.fp(st.ID), mustFP(t, p); got != want {
		t.Fatalf("stored fingerprint = %q, want %q", got, want)
	}
}

// An unchanged file whose row predates migration 0064 is fingerprinted on the
// next pass without re-extraction or an upsert (testing-strategy §26.3).
func TestScanBackfillsFingerprintWithoutReextract(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.mp4")
	b := seededBytes(11, 1<<20)
	mtime := writeAged(t, p, b, time.Hour)

	fr := newFakeRepo()
	fr.nextID = 7
	fr.byPath[p] = repo.VideoStat{ID: 7, Size: int64(len(b)), Mtime: mtime, FileTagsKnown: true}
	fr.active[7] = true

	ext := &countingExtractor{}
	s := New(Config{MediaPath: dir, FollowSymlinks: true, MaxDepth: 64, MinAge: time.Minute, Workers: 2},
		slog.New(slog.NewTextHandler(io.Discard, nil)), fr, ext)
	if err := s.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := ext.n.Load(); n != 0 {
		t.Errorf("extract calls = %d, want 0", n)
	}
	if fr.uparts != 0 {
		t.Errorf("upserts = %d, want 0", fr.uparts)
	}
	if got, want := fr.fp(7), mustFP(t, p); got != want {
		t.Fatalf("backfilled fingerprint = %q, want %q", got, want)
	}
	if !fr.active[7] {
		t.Error("backfilled row was deactivated")
	}

	// A second pass has nothing left to backfill.
	fr.fps[7] = "sentinel"
	if err := s.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fr.fp(7) != "sentinel" {
		t.Error("fingerprint recomputed for a row that already had one")
	}
}

// A writeback-style in-place change refreshes the fingerprint in the same pass
// that sees the size or mtime change (P0-1).
func TestScanRefreshesFingerprintOnInPlaceChange(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.mp4")
	writeAged(t, p, seededBytes(12, 1<<20), 2*time.Hour)
	fr := newFakeRepo()
	s := newTestScanner(dir, fr)
	ctx := context.Background()
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatal(err)
	}
	st, _, _ := fr.StatByPath(ctx, p)
	before := fr.fp(st.ID)

	writeAged(t, p, seededBytes(13, 1<<20), time.Hour)
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatal(err)
	}
	after := fr.fp(st.ID)
	if after == before {
		t.Fatal("fingerprint not refreshed after an in-place change")
	}
	if after != mustFP(t, p) {
		t.Fatal("refreshed fingerprint doesn't match the file")
	}
}

func TestBuildVideoFromFileCarriesFingerprint(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.mp4")
	writeAged(t, p, seededBytes(14, 1<<20), time.Hour)
	v, _, err := newTestScanner(dir, newFakeRepo()).BuildVideoFromFile(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if v.ContentFP != mustFP(t, p) {
		t.Fatalf("ContentFP = %q, want the file's fingerprint", v.ContentFP)
	}
}
