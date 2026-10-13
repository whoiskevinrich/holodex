package scanner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/db"
	"holodex/internal/repo"
)

func TestMoveDetail(t *testing.T) {
	for _, c := range []struct {
		repointed, folded, ambiguous int
		want                         string
	}{
		{0, 0, 0, ""},
		{1, 0, 0, "1 moved file recognised"},
		{2, 3, 0, "5 moved files recognised"},
		{0, 0, 1, "1 ambiguous match skipped"},
		{1, 1, 2, "2 moved files recognised; 2 ambiguous matches skipped"},
	} {
		st := &stats{repointed: c.repointed, folded: c.folded, ambiguous: c.ambiguous}
		if got := moveDetail(st); got != c.want {
			t.Errorf("moveDetail(%d,%d,%d) = %q, want %q", c.repointed, c.folded, c.ambiguous, got, c.want)
		}
	}
}

func (r *fakeRecorder) last(t *testing.T) string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.runs) == 0 {
		t.Fatal("no job run recorded")
	}
	return r.runs[len(r.runs)-1].Detail
}

// The scan row reports recognised moves, and says nothing when there were none
// (P0-7).
func TestScanRecordsMoveCounts(t *testing.T) {
	m := newMoveFixture(t)
	rec := &fakeRecorder{}
	m.s.SetJobRecorder(rec)
	m.write("a.mp4", seededBytes(40, moveSize))
	m.scan()
	if d := rec.last(t); d != "" {
		t.Fatalf("first scan detail = %q, want none", d)
	}
	m.move("a.mp4", "b.mp4")
	m.scan()
	if d := rec.last(t); d != "1 moved file recognised" {
		t.Fatalf("detail = %q", d)
	}
}

// An in-pass ambiguous file is declined by the scan-time match and then shows
// up again as an ambiguous fold pair; it must count once.
func TestScanCountsInPassAmbiguityOnceRealRepo(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	dir := t.TempDir()
	s := New(Config{MediaPath: dir, FollowSymlinks: true, MaxDepth: 64, MinAge: time.Minute, Workers: 2},
		slog.New(slog.NewTextHandler(io.Discard, nil)), repo.New(database), stubExtractor{})
	rec := &fakeRecorder{}
	s.SetJobRecorder(rec)
	ctx := context.Background()

	b := seededBytes(41, moveSize)
	for _, name := range []string{"x.mp4", "y.mp4"} {
		writeAged(t, filepath.Join(dir, name), b, time.Hour)
	}
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"x.mp4", "y.mp4"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	writeAged(t, filepath.Join(dir, "z.mp4"), b, time.Hour)
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if d := rec.last(t); d != "1 ambiguous match skipped" {
		t.Fatalf("detail = %q, want one ambiguous match", d)
	}
}
