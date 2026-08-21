package repo_test

import (
	"context"
	"path/filepath"
	"testing"

	"holodex/internal/db"
	"holodex/internal/repo"
)

func TestFilmsForVideo(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	r := repo.New(sqlDB)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	other, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed other video: %v", err)
	}

	// No attachments yet.
	got, err := r.FilmsForVideo(ctx, id)
	if err != nil {
		t.Fatalf("films for video (empty): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("unattached video: got %v, want empty", got)
	}

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := sqlDB.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
	exec(`INSERT INTO films (id, name, year) VALUES (1, 'Zoo Film', 2020)`)
	exec(`INSERT INTO films (id, name, year) VALUES (2, 'Alpha Film', 2019)`)
	exec(`INSERT INTO film_videos (film_id, video_id, scene_number, is_full_film, created_at) VALUES (1, ?, 6, 0, '2026-01-01T00:00:00Z')`, id)
	exec(`INSERT INTO film_videos (film_id, video_id, scene_number, is_full_film, created_at) VALUES (2, ?, NULL, 1, '2026-01-01T00:00:00Z')`, id)
	exec(`INSERT INTO film_videos (film_id, video_id, scene_number, is_full_film, created_at) VALUES (1, ?, NULL, 0, '2026-01-01T00:00:00Z')`, other)

	got, err = r.FilmsForVideo(ctx, id)
	if err != nil {
		t.Fatalf("films for video: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d attachments, want 2: %+v", len(got), got)
	}
	// Ordered by film name COLLATE NOCASE: "Alpha Film" before "Zoo Film".
	if got[0].FilmID != 2 || got[0].FilmName != "Alpha Film" || !got[0].IsFullFilm {
		t.Errorf("attachment[0] = %+v, want {2 Alpha Film true}", got[0])
	}
	if got[1].FilmID != 1 || got[1].FilmName != "Zoo Film" || got[1].IsFullFilm {
		t.Errorf("attachment[1] = %+v, want {1 Zoo Film false}", got[1])
	}

	// The other video's attachment must not leak in.
	gotOther, err := r.FilmsForVideo(ctx, other)
	if err != nil {
		t.Fatalf("films for other video: %v", err)
	}
	if len(gotOther) != 1 || gotOther[0].FilmID != 1 {
		t.Fatalf("other video attachments = %+v, want [film 1]", gotOther)
	}
}
