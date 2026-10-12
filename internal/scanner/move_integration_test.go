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

// A move from before F79 heals in one scan: the older hidden row (no
// fingerprint) absorbs the newer row at the new path by the legacy facts rule,
// keeps its id, and gains the newer copy's later edit. A second scan changes
// nothing (testing-strategy §26.4).
func TestScanHealsPastMoveRealRepo(t *testing.T) {
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
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := database.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	a, b := filepath.Join(dir, "a.mp4"), filepath.Join(dir, "b.mp4")
	writeAged(t, a, seededBytes(31, 1<<20), time.Hour)
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatal(err)
	}
	old, _, _ := r.StatByPath(ctx, a)

	// Recreate what the old scanner left behind: the file moved to b, a new row
	// for b, the old row hidden, neither fingerprinted.
	if err := os.Rename(a, b); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO videos (file_path, file_size, title, duration_sec, width, height, video_codec, audio_codec,
	                          bitrate_kbps, container, recorded_at, indexed_at, file_mtime, active, file_tags)
	      SELECT ?, file_size, title, duration_sec, width, height, video_codec, audio_codec,
	             bitrate_kbps, container, recorded_at, '2026-09-09T09:09:09Z', file_mtime, 1, file_tags
	        FROM videos WHERE id = ?`, b, old.ID)
	exec(`UPDATE videos SET active = 0, content_fp = NULL, indexed_at = '2025-01-01T00:00:00Z' WHERE id = ?`, old.ID)
	newer, _, _ := r.StatByPath(ctx, b)
	exec(`INSERT INTO field_source_decisions (entity_type, entity_id, field_key, source, manual_value, created_at)
	      VALUES ('video', ?, 'title', 'manual', 'Edited after the move', '2026-09-10T00:00:00Z')`, newer.ID)

	if err := s.ScanOnce(ctx); err != nil {
		t.Fatal(err)
	}
	healed, ok, _ := r.StatByPath(ctx, b)
	if !ok || healed.ID != old.ID || !healed.Active {
		t.Fatalf("after heal: row at b = %+v ok=%v, want id %d active", healed, ok, old.ID)
	}
	var n int
	var title, indexed string
	if err := database.QueryRow(`SELECT count(*) FROM videos`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("videos = %d (%v), want 1", n, err)
	}
	if err := database.QueryRow(`SELECT manual_value FROM field_source_decisions
		WHERE entity_type = 'video' AND entity_id = ? AND field_key = 'title'`, old.ID).Scan(&title); err != nil {
		t.Fatalf("carried title decision: %v", err)
	}
	if title != "Edited after the move" {
		t.Errorf("title decision = %q", title)
	}
	if err := database.QueryRow(`SELECT indexed_at FROM videos WHERE id = ?`, old.ID).Scan(&indexed); err != nil {
		t.Fatal(err)
	}
	if indexed != "2025-01-01T00:00:00Z" {
		t.Errorf("indexed_at = %s; a heal is not an update", indexed)
	}

	// Idempotent.
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if again, _, _ := r.StatByPath(ctx, b); again.ID != old.ID {
		t.Fatalf("second scan changed the row: %+v", again)
	}
}

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
