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

// A moved file keeps its row end to end: the real scanner over real SQLite
// repoints it, so the id, indexed_at and a manual tag attached to it survive
// (testing-strategy §26.4).
func TestScanMoveKeepsRowRealRepo(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)

	dir := t.TempDir()
	s := New(Config{MediaPath: dir, FollowSymlinks: true, MaxDepth: 64, MinAge: time.Minute, Workers: 2},
		slog.New(slog.NewTextHandler(io.Discard, nil)), r, stubExtractor{})
	ctx := context.Background()

	a := filepath.Join(dir, "a.mp4")
	writeAged(t, a, seededBytes(30, 1<<20), time.Hour)
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatal(err)
	}
	st, ok, err := r.StatByPath(ctx, a)
	if err != nil || !ok {
		t.Fatalf("not indexed: %v", err)
	}
	var indexed string
	if err := database.QueryRow(`SELECT indexed_at FROM videos WHERE id = ?`, st.ID).Scan(&indexed); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO tags (name) VALUES ('keepme')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO video_tags (video_id, tag_id, source)
		SELECT ?, id, 'manual' FROM tags WHERE name = 'keepme'`, st.ID); err != nil {
		t.Fatal(err)
	}

	b := filepath.Join(dir, "sub", "b.mp4")
	if err := os.MkdirAll(filepath.Dir(b), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(a, b); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond) // a bumped indexed_at would now differ
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatal(err)
	}

	moved, ok, err := r.StatByPath(ctx, b)
	if err != nil || !ok {
		t.Fatalf("moved file has no row: %v", err)
	}
	if moved.ID != st.ID || !moved.Active {
		t.Fatalf("moved row id=%d active=%v, want id %d active", moved.ID, moved.Active, st.ID)
	}
	var (
		n        int
		indexed2 string
		tagged   int
	)
	if err := database.QueryRow(`SELECT COUNT(*) FROM videos`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("videos = %d (%v), want 1", n, err)
	}
	if err := database.QueryRow(`SELECT indexed_at FROM videos WHERE id = ?`, st.ID).Scan(&indexed2); err != nil {
		t.Fatal(err)
	}
	if indexed2 != indexed {
		t.Errorf("indexed_at %s → %s; a move is not an update", indexed, indexed2)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM video_tags WHERE video_id = ? AND source = 'manual'`, st.ID).
		Scan(&tagged); err != nil || tagged != 1 {
		t.Errorf("manual tag on moved video = %d (%v), want 1", tagged, err)
	}
}
