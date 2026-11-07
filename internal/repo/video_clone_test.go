package repo

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"holodex/internal/db"
	"holodex/internal/fieldsource"
	"holodex/internal/model"
)

func cloneRepo(t *testing.T) (*Repo, *sql.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return New(database), database
}

// seedFullVideo fills every copied table for a new video, and one history row.
func seedFullVideo(t *testing.T, r *Repo, d *sql.DB) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, &model.Video{FilePath: "/m/a {edition-Theatrical}.mkv", Title: "A", Duration: 60,
		Width: 640, Height: 360, FileMtime: time.Now().UTC().Truncate(time.Second)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(timeLayout)
	for _, q := range []string{
		`INSERT INTO people (id, name) VALUES (7, 'Ada')`,
		`INSERT INTO tags (id, name) VALUES (8, 'tag')`,
		`INSERT INTO studios (id, name) VALUES (9, 'Studio')`,
		`INSERT INTO films (id, name) VALUES (10, 'Film')`,
		`INSERT INTO playlists (id, name, created_at, updated_at) VALUES (11, 'List', '` + now + `', '` + now + `')`,
		fmt.Sprintf(`INSERT INTO video_people (video_id, person_id, role) VALUES (%d, 7, 'actor')`, id),
		fmt.Sprintf(`INSERT INTO video_tags (video_id, tag_id, source) VALUES (%d, 8, 'manual')`, id),
		fmt.Sprintf(`INSERT INTO video_studios (video_id, studio_id) VALUES (%d, 9)`, id),
		fmt.Sprintf(`INSERT INTO film_videos (film_id, video_id, scene_number, is_full_film, created_at) VALUES (10, %d, 3, 0, '%s')`, id, now),
		fmt.Sprintf(`INSERT INTO playlist_videos (playlist_id, video_id, position) VALUES (11, %d, 4)`, id),
		fmt.Sprintf(`INSERT INTO metadata_curation (entity_type, entity_id, field_key, norm_value, value, action, created_at) VALUES ('video', %d, 'actors', 'x', 'X', 'add', '%s')`, id, now),
		fmt.Sprintf(`INSERT INTO field_source_decisions (entity_type, entity_id, field_key, source, manual_value, created_at) VALUES ('video', %d, 'title', 'manual', 'Mine', '%s')`, id, now),
		fmt.Sprintf(`INSERT INTO field_source_decisions (entity_type, entity_id, field_key, source, manual_value, created_at) VALUES ('video', %d, 'edition', 'manual', 'Theatrical', '%s')`, id, now),
		fmt.Sprintf(`INSERT INTO field_source_decisions (entity_type, entity_id, field_key, source, manual_value, created_at) VALUES ('video', %d, 'part', 'manual', '2', '%s')`, id, now),
		fmt.Sprintf(`INSERT INTO entity_enrichment (entity_type, entity_id, provider, field_key, value, external_id, fetched_at) VALUES ('video', %d, 'provider-1', 'title', 'T', 'e1', '%s')`, id, now),
		fmt.Sprintf(`INSERT INTO facet_not_applicable (entity_type, entity_id, canonical_field, created_at) VALUES ('video', %d, 'director', '%s')`, id, now),
		fmt.Sprintf(`INSERT INTO enrichment_dismissals (entity_type, entity_id, provider, dismissed_at) VALUES ('video', %d, 'provider-2', '%s')`, id, now),
		fmt.Sprintf(`INSERT INTO video_poster_sources (video_id, kind, hash, frame_at, updated_at) VALUES (%d, 'frame', 'abc', 12.5, '%s')`, id, now),
		fmt.Sprintf(`INSERT INTO video_poster_sources (video_id, kind, hash, updated_at) VALUES (%d, 'automatic', 'def', '%s')`, id, now),
		fmt.Sprintf(`INSERT INTO file_renames (video_id, old_name, new_name, trigger, state, created_at) VALUES (%d, 'o', 'n', 'manual', 'done', '%s')`, id, now),
		fmt.Sprintf(`INSERT INTO video_file_names (video_id, locked, lock_source) VALUES (%d, 1, 'owner')`, id),
	} {
		if _, err := d.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	return id
}

func count(t *testing.T, d *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := d.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

// R1: every table holding per-video state is classified, so a new one fails here until someone
// decides whether an edition copies it. The guard for a table added after this was written.
func TestCloneClassifiesEveryTable(t *testing.T) {
	_, d := cloneRepo(t)
	rows, err := d.Query(`SELECT m.name FROM sqlite_master m, pragma_table_info(m.name) c
		WHERE m.type = 'table' AND c.name IN ('video_id', 'entity_id') AND m.name NOT LIKE '%fts%'
		GROUP BY m.name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	known := slices.Concat(CloneTables.Copied, CloneTables.Derived, CloneTables.History, CloneTables.NoVideos)
	seen := 0
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		seen++
		if !slices.Contains(known, name) {
			t.Errorf("table %s holds per-video state but isn't classified in CloneTables", name)
		}
	}
	if seen < 15 {
		t.Fatalf("the listing found only %d tables; the query is wrong", seen)
	}
}

// R2, R4, R6, R8: everything the spec lists copies, and nothing else does.
func TestCloneVideoAsEdition(t *testing.T) {
	r, d := cloneRepo(t)
	ctx := context.Background()
	src := seedFullVideo(t, r, d)

	id, err := r.CloneVideoAsEdition(ctx, src, "/m/a {edition-Trimmed}.mkv", "Trimmed")
	if err != nil {
		t.Fatal(err)
	}
	if id == src {
		t.Fatal("the edition must be a new item")
	}
	for name, q := range map[string]string{
		"people":       `SELECT count(*) FROM video_people WHERE video_id = ? AND person_id = 7 AND role = 'actor'`,
		"tags":         `SELECT count(*) FROM video_tags WHERE video_id = ? AND tag_id = 8 AND source = 'manual'`,
		"studios":      `SELECT count(*) FROM video_studios WHERE video_id = ? AND studio_id = 9`,
		"playlist":     `SELECT count(*) FROM playlist_videos WHERE video_id = ? AND playlist_id = 11`,
		"curation":     `SELECT count(*) FROM metadata_curation WHERE entity_type = 'video' AND entity_id = ? AND value = 'X'`,
		"title edit":   `SELECT count(*) FROM field_source_decisions WHERE entity_type = 'video' AND entity_id = ? AND field_key = 'title' AND manual_value = 'Mine'`,
		"part":         `SELECT count(*) FROM field_source_decisions WHERE entity_type = 'video' AND entity_id = ? AND field_key = 'part' AND manual_value = '2'`,
		"enrichment":   `SELECT count(*) FROM entity_enrichment WHERE entity_type = 'video' AND entity_id = ? AND external_id = 'e1'`,
		"n/a":          `SELECT count(*) FROM facet_not_applicable WHERE entity_type = 'video' AND entity_id = ?`,
		"dismissal":    `SELECT count(*) FROM enrichment_dismissals WHERE entity_type = 'video' AND entity_id = ?`,
		"frame poster": `SELECT count(*) FROM video_poster_sources WHERE video_id = ? AND kind = 'frame' AND frame_at = 12.5`,
		// A film's scene numbers are unique and the source keeps its own.
		"film link": `SELECT count(*) FROM film_videos WHERE video_id = ? AND film_id = 10 AND scene_number IS NULL`,
		"edition":   `SELECT count(*) FROM field_source_decisions WHERE entity_type = 'video' AND entity_id = ? AND field_key = 'edition' AND source = 'manual' AND manual_value = 'Trimmed'`,
	} {
		if count(t, d, q, id) != 1 {
			t.Errorf("%s did not copy", name)
		}
	}
	for name, q := range map[string]string{
		"automatic poster": `SELECT count(*) FROM video_poster_sources WHERE video_id = ? AND kind = 'automatic'`,
		"rename history":   `SELECT count(*) FROM file_renames WHERE video_id = ?`,
		"name lock":        `SELECT count(*) FROM video_file_names WHERE video_id = ?`,
	} {
		if count(t, d, q, id) != 0 {
			t.Errorf("%s copied, and must not", name)
		}
	}
	if count(t, d, `SELECT count(*) FROM field_source_decisions WHERE entity_type = 'video' AND entity_id = ? AND field_key = 'edition' AND manual_value = 'Theatrical'`, src) != 1 {
		t.Error("the source's own edition changed")
	}
	if count(t, d, `SELECT count(*) FROM film_videos WHERE video_id = ? AND scene_number = 3`, src) != 1 {
		t.Error("the source's scene number changed")
	}
	lo, hi := orderPair(src, id)
	if count(t, d, `SELECT count(*) FROM entity_keep_separate WHERE entity_type = 'video' AND id_lo = ? AND id_hi = ?`, lo, hi) != 1 {
		t.Error("the pair isn't labelled as editions")
	}
	from, err := r.GetTrimmedFrom(ctx, id)
	if err != nil || from == nil || from.VideoID != src || from.Title != "A" {
		t.Errorf("trimmed from = %+v, %v", from, err)
	}
	if from, _ := r.GetTrimmedFrom(ctx, src); from != nil {
		t.Errorf("the source wasn't trimmed from anything: %+v", from)
	}
}

// R9: after creation an edit to either side leaves the other unchanged.
func TestCloneIsIndependent(t *testing.T) {
	r, d := cloneRepo(t)
	ctx := context.Background()
	src := seedFullVideo(t, r, d)
	id, err := r.CloneVideoAsEdition(ctx, src, "/m/b.mkv", "B")
	if err != nil {
		t.Fatal(err)
	}
	if err := upsertDecision(ctx, d, model.EnrichEntityVideo, id, "title", fieldsource.Manual, "Edition's"); err != nil {
		t.Fatal(err)
	}
	if count(t, d, `SELECT count(*) FROM field_source_decisions WHERE entity_id = ? AND field_key = 'title' AND manual_value = 'Mine'`, src) != 1 {
		t.Error("an edit to the edition changed the source")
	}
	if err := upsertDecision(ctx, d, model.EnrichEntityVideo, src, "title", fieldsource.Manual, "Source's"); err != nil {
		t.Fatal(err)
	}
	if count(t, d, `SELECT count(*) FROM field_source_decisions WHERE entity_id = ? AND field_key = 'title' AND manual_value = 'Edition''s'`, id) != 1 {
		t.Error("an edit to the source changed the edition")
	}
}

// R10: a scanner's row is filled in, and a second run after a crash changes nothing.
func TestCloneFillsAScannerRowAndIsRepeatable(t *testing.T) {
	r, d := cloneRepo(t)
	ctx := context.Background()
	src := seedFullVideo(t, r, d)
	scanned, err := r.UpsertVideo(ctx, &model.Video{FilePath: "/m/b.mkv", Title: "scanned",
		FileMtime: time.Now().UTC().Truncate(time.Second)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	id, err := r.CloneVideoAsEdition(ctx, src, "/m/b.mkv", "B")
	if err != nil {
		t.Fatal(err)
	}
	if id != scanned {
		t.Fatalf("clone made id %d beside the scanner's %d", id, scanned)
	}
	again, err := r.CloneVideoAsEdition(ctx, src, "/m/b.mkv", "B")
	if err != nil || again != id {
		t.Fatalf("second run: %d, %v", again, err)
	}
	if n := count(t, d, `SELECT count(*) FROM video_people WHERE video_id = ?`, id); n != 1 {
		t.Errorf("%d people rows after two runs", n)
	}
	if n := count(t, d, `SELECT count(*) FROM videos WHERE file_path = '/m/b.mkv'`); n != 1 {
		t.Errorf("%d video rows", n)
	}
}

func TestCloneMissingSource(t *testing.T) {
	r, _ := cloneRepo(t)
	if _, err := r.CloneVideoAsEdition(context.Background(), 999, "/m/b.mkv", "B"); err == nil {
		t.Fatal("cloning a missing video must fail")
	}
}

func TestTrimResultRoundTrip(t *testing.T) {
	r, d := cloneRepo(t)
	ctx := context.Background()
	src := seedFullVideo(t, r, d)
	if got, err := r.GetTrimResult(ctx, src); err != nil || got != nil {
		t.Fatalf("none yet: %+v, %v", got, err)
	}
	if err := r.SetTrimResult(ctx, TrimResult{VideoID: src, Mode: "edition", EditionName: "T", Skipped: `[{"end":"last","reason":"still"}]`}); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetTrimResult(ctx, src)
	if err != nil || got == nil || got.EditionName != "T" || got.Skipped == "[]" || got.CreatedAt.IsZero() {
		t.Fatalf("got %+v, %v", got, err)
	}
	if err := r.ClearReplaceTrimResult(ctx, src); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetTrimResult(ctx, src); got == nil {
		t.Fatal("clearing a replace's outcome took an edition's")
	}
	if ok, err := r.DismissTrimResult(ctx, src); err != nil || !ok {
		t.Fatalf("dismiss: %v, %v", ok, err)
	}
	if got, _ := r.GetTrimResult(ctx, src); got != nil {
		t.Fatalf("still there: %+v", got)
	}
}
