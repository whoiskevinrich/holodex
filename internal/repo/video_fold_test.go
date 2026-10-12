package repo_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"holodex/internal/repo"
	"strconv"
)

func scalar[T any](t *testing.T, database *sql.DB, q string, args ...any) T {
	t.Helper()
	var v T
	if err := database.QueryRow(q, args...).Scan(&v); err != nil {
		t.Fatalf("query %q: %v", q, err)
	}
	return v
}

// seedMovePair inserts a hidden older row and a visible newer row for the same
// content (fp set on both, or neither for the legacy rule) and returns their ids.
func seedMovePair(t *testing.T, r *repo.Repo, database *sql.DB, oldPath, newPath, fp string) (older, newer int64) {
	t.Helper()
	ctx := context.Background()
	o := legacyVideo(oldPath)
	o.ContentFP = fp
	older, err := r.UpsertVideo(ctx, o, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, database, `UPDATE videos SET active = 0, indexed_at = '2025-01-01T00:00:00Z' WHERE id = ?`, older)
	n := legacyVideo(newPath)
	n.ContentFP = fp
	newer, err = r.UpsertVideo(ctx, n, nil)
	if err != nil {
		t.Fatal(err)
	}
	return older, newer
}

func foldPairs(t *testing.T, r *repo.Repo) ([]repo.FoldPair, int) {
	t.Helper()
	pairs, amb, err := r.FoldPairs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return pairs, len(amb)
}

func TestFoldPairsListsOnlyUnambiguousLivePairs(t *testing.T) {
	t.Run("fingerprint", func(t *testing.T) {
		r, database := newRepoDB(t)
		o, n := seedMovePair(t, r, database, "/m/a.mp4", "/m/b.mp4", "fp")
		if p, amb := foldPairs(t, r); len(p) != 1 || p[0] != (repo.FoldPair{Older: o, Newer: n}) || amb != 0 {
			t.Fatalf("pairs = %v amb = %d", p, amb)
		}
	})
	t.Run("legacy exact facts", func(t *testing.T) {
		r, database := newRepoDB(t)
		seedMovePair(t, r, database, "/m/a.mp4", "/m/b.mp4", "")
		if p, _ := foldPairs(t, r); len(p) != 1 {
			t.Fatalf("pairs = %v, want the legacy pair", p)
		}
	})
	t.Run("two hidden rows, one newer", func(t *testing.T) {
		r, database := newRepoDB(t)
		ctx := context.Background()
		for _, p := range []string{"/m/x.mp4", "/m/y.mp4", "/m/new.mp4"} {
			v := legacyVideo(p)
			v.ContentFP = "fp"
			if _, err := r.UpsertVideo(ctx, v, nil); err != nil {
				t.Fatal(err)
			}
		}
		mustExec(t, database, `UPDATE videos SET active = 0 WHERE file_path IN ('/m/x.mp4', '/m/y.mp4')`)
		p, amb := foldPairs(t, r)
		if len(p) != 0 || amb != 1 {
			t.Fatalf("pairs = %v amb = %d, want none and 1 ambiguous", p, amb)
		}
	})
	t.Run("fingerprinted rows never match on facts alone", func(t *testing.T) {
		r, database := newRepoDB(t)
		_, n := seedMovePair(t, r, database, "/m/a.mp4", "/m/b.mp4", "fp-a")
		mustExec(t, database, `UPDATE videos SET content_fp = 'fp-b' WHERE id = ?`, n)
		if p, _ := foldPairs(t, r); len(p) != 0 {
			t.Fatalf("identical facts, different fingerprints folded: %v", p)
		}
	})
	t.Run("keep-separate", func(t *testing.T) {
		r, database := newRepoDB(t)
		o, n := seedMovePair(t, r, database, "/m/a.mp4", "/m/b.mp4", "fp")
		mustExec(t, database, `INSERT INTO entity_keep_separate (entity_type, id_lo, id_hi) VALUES ('video', ?, ?)`, o, n)
		if p, _ := foldPairs(t, r); len(p) != 0 {
			t.Fatalf("kept-separate pair listed: %v", p)
		}
		if err := r.FoldMovedVideo(context.Background(), o, n); !errors.Is(err, repo.ErrFoldNotLive) {
			t.Fatalf("fold of a kept-separate pair: %v, want ErrFoldNotLive", err)
		}
	})
	t.Run("trashed older", func(t *testing.T) {
		r, database := newRepoDB(t)
		o, _ := seedMovePair(t, r, database, "/m/a.mp4", "/m/b.mp4", "fp")
		mustExec(t, database, `UPDATE videos SET deleted_at = '2026-01-01T00:00:00Z' WHERE id = ?`, o)
		if p, _ := foldPairs(t, r); len(p) != 0 {
			t.Fatalf("trashed row listed: %v", p)
		}
	})
	t.Run("writeback in flight", func(t *testing.T) {
		r, database := newRepoDB(t)
		_, n := seedMovePair(t, r, database, "/m/a.mp4", "/m/b.mp4", "fp")
		mustExec(t, database, `INSERT INTO writeback_queue (video_id, payload, status, enqueued_at, updated_at)
			VALUES (?, '{}', 'pending', 'x', 'x')`, n)
		if p, _ := foldPairs(t, r); len(p) != 0 {
			t.Fatalf("pair with pending writeback listed: %v", p)
		}
	})
	t.Run("hidden row newer than the visible one", func(t *testing.T) {
		r, database := newRepoDB(t)
		o, n := seedMovePair(t, r, database, "/m/a.mp4", "/m/b.mp4", "fp")
		mustExec(t, database, `UPDATE videos SET active = 1 WHERE id = ?`, o)
		mustExec(t, database, `UPDATE videos SET active = 0 WHERE id = ?`, n)
		if p, _ := foldPairs(t, r); len(p) != 0 {
			t.Fatalf("a newer hidden row folded into an older visible one: %v", p)
		}
	})
}

// testing-strategy §26.3 "Fold carry" and "Fold survivor".
func TestFoldMovedVideoCarriesWorkAndKeepsOlderIdentity(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()
	o, n := seedMovePair(t, r, database, "/m/old.mp4", "/m/new.mp4", "fp")

	// Owner edits. title on both: newer wins. studio on older only: kept. date on
	// newer only: carried. tags curation on both: newer's set wins.
	for _, d := range []struct {
		id         int64
		field, val string
		at         string
	}{
		{o, "title", "Old title", "2025-01-01T00:00:00Z"},
		{n, "title", "New title", "2026-01-01T00:00:00Z"},
		{o, "studio", "Old studio", "2025-01-01T00:00:00Z"},
		{n, "date", "2020-02-02", "2026-01-01T00:00:00Z"},
	} {
		mustExec(t, database, `INSERT INTO field_source_decisions (entity_type, entity_id, field_key, source, manual_value, created_at)
			VALUES ('video', ?, ?, 'manual', ?, ?)`, d.id, d.field, d.val, d.at)
	}
	mustExec(t, database, `INSERT INTO metadata_curation (entity_type, entity_id, field_key, norm_value, value, action, created_at)
		VALUES ('video', ?, 'tags', 'old', 'old', 'add', '2025-01-01T00:00:00Z'),
		       ('video', ?, 'tags', 'new', 'new', 'add', '2026-01-01T00:00:00Z')`, o, n)
	mustExec(t, database, `INSERT INTO facet_not_applicable (entity_type, entity_id, canonical_field, created_at)
		VALUES ('video', ?, 'director', '2026-01-01T00:00:00Z')`, n)

	// Playlists: P1 holds both (older keeps its place); P2 holds only newer.
	mustExec(t, database, `INSERT INTO playlists (id, name) VALUES (1, 'p1'), (2, 'p2')`)
	mustExec(t, database, `INSERT INTO playlist_videos (playlist_id, video_id, position) VALUES (1, ?, 1), (1, ?, 5), (2, ?, 7)`, o, n, n)
	// Films: F1 links only newer (scene 3); F2 links both (older's link stands).
	mustExec(t, database, `INSERT INTO films (id, name) VALUES (1, 'f1'), (2, 'f2')`)
	mustExec(t, database, `INSERT INTO film_videos (film_id, video_id, scene_number, created_at)
		VALUES (1, ?, 3, 'x'), (2, ?, 1, 'x'), (2, ?, 2, 'x')`, n, o, n)

	// Provider data. A on both, newer fresher: newer's wins. B older only: kept.
	// C newer only: moved. D on both, older fresher: older's kept.
	for _, e := range []struct {
		id              int64
		provider, value string
		at              string
	}{
		{o, "A", "old-A", "2025-01-01"}, {n, "A", "new-A", "2026-01-01"},
		{o, "B", "old-B", "2025-01-01"},
		{n, "C", "new-C", "2026-01-01"},
		{o, "D", "old-D", "2026-06-01"}, {n, "D", "new-D", "2026-01-01"},
	} {
		mustExec(t, database, `INSERT INTO entity_enrichment (entity_type, entity_id, provider, field_key, value, fetched_at)
			VALUES ('video', ?, ?, 'title', ?, ?)`, e.id, e.provider, e.value, e.at)
	}
	mustExec(t, database, `INSERT INTO enrichment_dismissals (entity_type, entity_id, provider, dismissed_at) VALUES ('video', ?, 'Z', 'x')`, n)
	mustExec(t, database, `INSERT INTO writeback_queue (video_id, payload, status, enqueued_at, updated_at) VALUES (?, '{}', 'done', 'x', 'x')`, n)
	mustExec(t, database, `INSERT INTO job_runs (kind, trigger, status, started_at, finished_at, entity_type, entity_id)
		VALUES ('refresh', 'manual', 'ok', 'x', 'x', 'video', ?)`, n)
	mustExec(t, database, `INSERT INTO tags (name) VALUES ('manualtag')`)
	mustExec(t, database, `INSERT INTO video_tags (video_id, tag_id, source) SELECT ?, id, 'manual' FROM tags WHERE name = 'manualtag'`, n)

	if err := r.FoldMovedVideo(ctx, o, n); err != nil {
		t.Fatal(err)
	}

	// Survivor.
	if got := scalar[string](t, database, `SELECT file_path FROM videos WHERE id = ?`, o); got != "/m/new.mp4" {
		t.Errorf("survivor path = %s", got)
	}
	if !scalar[bool](t, database, `SELECT active FROM videos WHERE id = ?`, o) {
		t.Error("survivor hidden")
	}
	if got := scalar[string](t, database, `SELECT indexed_at FROM videos WHERE id = ?`, o); got != "2025-01-01T00:00:00Z" {
		t.Errorf("survivor indexed_at = %s; a heal is not an update", got)
	}
	if got := scalar[int](t, database, `SELECT count(*) FROM videos WHERE id = ?`, n); got != 0 {
		t.Error("newer row still exists")
	}

	// Edits.
	decision := func(field string) string {
		return scalar[string](t, database, `SELECT coalesce((SELECT manual_value FROM field_source_decisions
			WHERE entity_type = 'video' AND entity_id = ? AND field_key = ?), '')`, o, field)
	}
	if got := decision("title"); got != "New title" {
		t.Errorf("title = %q, want the newer edit", got)
	}
	if got := decision("studio"); got != "Old studio" {
		t.Errorf("studio = %q, want older's edit kept", got)
	}
	if got := decision("date"); got != "2020-02-02" {
		t.Errorf("date = %q, want newer's edit carried", got)
	}
	if got := scalar[string](t, database, `SELECT group_concat(norm_value) FROM metadata_curation
		WHERE entity_type = 'video' AND entity_id = ? AND field_key = 'tags'`, o); got != "new" {
		t.Errorf("tags curation = %q, want newer's set only", got)
	}
	if got := scalar[int](t, database, `SELECT count(*) FROM facet_not_applicable
		WHERE entity_type = 'video' AND entity_id = ? AND canonical_field = 'director'`, o); got != 1 {
		t.Error("not-applicable mark not carried")
	}

	// Memberships.
	if got := scalar[int](t, database, `SELECT position FROM playlist_videos WHERE playlist_id = 1 AND video_id = ?`, o); got != 1 {
		t.Errorf("P1 position = %d, want older's 1", got)
	}
	if got := scalar[int](t, database, `SELECT count(*) FROM playlist_videos WHERE playlist_id = 1`); got != 1 {
		t.Errorf("P1 entries = %d, want 1", got)
	}
	if got := scalar[int](t, database, `SELECT position FROM playlist_videos WHERE playlist_id = 2 AND video_id = ?`, o); got != 7 {
		t.Errorf("P2 position = %d, want newer's 7", got)
	}
	if got := scalar[int](t, database, `SELECT scene_number FROM film_videos WHERE film_id = 1 AND video_id = ?`, o); got != 3 {
		t.Errorf("F1 scene = %d, want 3", got)
	}
	if got := scalar[int](t, database, `SELECT scene_number FROM film_videos WHERE film_id = 2 AND video_id = ?`, o); got != 1 {
		t.Errorf("F2 scene = %d, want older's 1", got)
	}
	if got := scalar[int](t, database, `SELECT count(*) FROM video_tags vt JOIN tags t ON t.id = vt.tag_id
		WHERE vt.video_id = ? AND t.name = 'manualtag' AND vt.source = 'manual'`, o); got != 1 {
		t.Error("manual tag not carried")
	}

	// Provider data and dismissals.
	for provider, want := range map[string]string{"A": "new-A", "B": "old-B", "C": "new-C", "D": "old-D"} {
		got := scalar[string](t, database, `SELECT coalesce((SELECT value FROM entity_enrichment
			WHERE entity_type = 'video' AND entity_id = ? AND provider = ?), '')`, o, provider)
		if got != want {
			t.Errorf("provider %s = %q, want %q", provider, got, want)
		}
	}
	if got := scalar[int](t, database, `SELECT count(*) FROM enrichment_dismissals WHERE entity_type = 'video' AND entity_id = ?`, o); got != 1 {
		t.Error("dismissal not carried")
	}
	if got := scalar[int](t, database, `SELECT count(*) FROM writeback_queue WHERE video_id = ?`, o); got != 1 {
		t.Error("writeback queue entry not moved")
	}
	if got := scalar[int](t, database, `SELECT count(*) FROM job_runs WHERE entity_type = 'video' AND entity_id = ?`, o); got != 1 {
		t.Error("job attribution not moved")
	}

	assertNothingReferences(t, database, n)
}

// assertNothingReferences checks every table with a video_id column, or an
// (entity_type, entity_id) pair, for a row still pointing at id. Enumerating the
// schema means a table added later can't be forgotten silently.
func assertNothingReferences(t *testing.T, database *sql.DB, id int64) {
	t.Helper()
	tables, err := database.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'videos_fts%'`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for tables.Next() {
		var n string
		if err := tables.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	tables.Close()
	for _, table := range names {
		cols := map[string]bool{}
		rows, err := database.Query(`SELECT name FROM pragma_table_info(?)`, table)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				t.Fatal(err)
			}
			cols[c] = true
		}
		rows.Close()
		var q string
		switch {
		case cols["video_id"]:
			q = `SELECT count(*) FROM "` + table + `" WHERE video_id = ?`
		case cols["entity_type"] && cols["entity_id"]:
			q = `SELECT count(*) FROM "` + table + `" WHERE entity_type = 'video' AND entity_id = ?`
		case table == "videos":
			q = `SELECT count(*) FROM videos WHERE id = ?`
		default:
			continue
		}
		if got := scalar[int](t, database, q, id); got != 0 {
			t.Errorf("%s still has %d row(s) for folded video %d", table, got, id)
		}
	}
}

// A failure anywhere rolls the whole fold back (testing-strategy §26.3).
func TestFoldMovedVideoIsAtomic(t *testing.T) {
	r, database := newRepoDB(t)
	o, n := seedMovePair(t, r, database, "/m/old.mp4", "/m/new.mp4", "fp")
	mustExec(t, database, `INSERT INTO field_source_decisions (entity_type, entity_id, field_key, source, manual_value, created_at)
		VALUES ('video', ?, 'title', 'manual', 'Old', 'a'), ('video', ?, 'title', 'manual', 'New', 'b')`, o, n)
	mustExec(t, database, `CREATE TRIGGER abort_fold BEFORE DELETE ON videos WHEN old.id = `+itoa(n)+`
		BEGIN SELECT RAISE(ABORT, 'boom'); END`)

	if err := r.FoldMovedVideo(context.Background(), o, n); err == nil {
		t.Fatal("fold succeeded despite the aborting trigger")
	}
	if got := scalar[string](t, database, `SELECT manual_value FROM field_source_decisions WHERE entity_id = ? AND field_key = 'title'`, o); got != "Old" {
		t.Errorf("older's title = %q after a failed fold", got)
	}
	if got := scalar[string](t, database, `SELECT manual_value FROM field_source_decisions WHERE entity_id = ? AND field_key = 'title'`, n); got != "New" {
		t.Errorf("newer's title = %q after a failed fold", got)
	}
	if scalar[bool](t, database, `SELECT active FROM videos WHERE id = ?`, o) {
		t.Error("older shown after a failed fold")
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
