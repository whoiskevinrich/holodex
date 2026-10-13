package repo_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// HOLODEX-567 (spec phase-2 F12.1, F12.7): "Date added" is arrival order and never
// moves when a file is re-upserted; "Updated" is when the file itself last changed,
// and only a size or mtime change moves it.

func newRepoWithDB(t *testing.T) (*repo.Repo, *sql.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return repo.New(database), database
}

var sortMtime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func sortVideo(path string, size int64) *model.Video {
	return &model.Video{FilePath: path, Title: path, FileSize: size, FileMtime: sortMtime}
}

func upsert(t *testing.T, r *repo.Repo, v *model.Video) int64 {
	t.Helper()
	id, err := r.UpsertVideo(context.Background(), v, nil)
	if err != nil {
		t.Fatalf("upsert %s: %v", v.FilePath, err)
	}
	return id
}

// backdate pins indexed_at so the test doesn't depend on wall-clock seconds.
func backdate(t *testing.T, database *sql.DB, id int64, at string) {
	t.Helper()
	if _, err := database.Exec(`UPDATE videos SET indexed_at = ? WHERE id = ?`, at, id); err != nil {
		t.Fatalf("backdate %d: %v", id, err)
	}
}

func indexedAt(t *testing.T, database *sql.DB, id int64) string {
	t.Helper()
	var s string
	if err := database.QueryRow(`SELECT indexed_at FROM videos WHERE id = ?`, id).Scan(&s); err != nil {
		t.Fatalf("read indexed_at %d: %v", id, err)
	}
	return s
}

func sortedIDs(t *testing.T, r *repo.Repo, sort string) []int64 {
	t.Helper()
	items, _, err := r.ListVideos(context.Background(), repo.VideoFilter{Sort: sort, Limit: 50})
	if err != nil {
		t.Fatalf("list %s: %v", sort, err)
	}
	ids := make([]int64, len(items))
	for i, v := range items {
		ids[i] = v.ID
	}
	return ids
}

func TestDateAddedIsArrivalOrderAndUpdatedFollowsFileChanges(t *testing.T) {
	r, database := newRepoWithDB(t)
	a := upsert(t, r, sortVideo("/m/a.mkv", 100))
	b := upsert(t, r, sortVideo("/m/b.mkv", 100))
	c := upsert(t, r, sortVideo("/m/c.mkv", 100))
	backdate(t, database, a, "2024-01-01T00:00:00Z")
	backdate(t, database, b, "2024-01-02T00:00:00Z")
	backdate(t, database, c, "2024-01-03T00:00:00Z")

	// The oldest arrival's file changes on disk (a writeback, a replaced file).
	changed := sortVideo("/m/a.mkv", 200)
	if got := upsert(t, r, changed); got != a {
		t.Fatalf("re-upsert changed the id: %d, want %d", got, a)
	}

	if got, want := sortedIDs(t, r, "added_desc"), []int64{c, b, a}; !slices.Equal(got, want) {
		t.Errorf("added_desc = %v, want %v (a changed file must keep its arrival place)", got, want)
	}
	if got, want := sortedIDs(t, r, "added_asc"), []int64{a, b, c}; !slices.Equal(got, want) {
		t.Errorf("added_asc = %v, want %v", got, want)
	}
	if got, want := sortedIDs(t, r, ""), []int64{c, b, a}; !slices.Equal(got, want) {
		t.Errorf("default sort = %v, want added_desc order %v", got, want)
	}
	if got, want := sortedIDs(t, r, "updated_desc"), []int64{a, c, b}; !slices.Equal(got, want) {
		t.Errorf("updated_desc = %v, want %v (the changed file tops it)", got, want)
	}
	if got, want := sortedIDs(t, r, "updated_asc"), []int64{b, c, a}; !slices.Equal(got, want) {
		t.Errorf("updated_asc = %v, want %v", got, want)
	}
}

func TestReupsertOfUnchangedFileKeepsIndexedAt(t *testing.T) {
	r, database := newRepoWithDB(t)
	id := upsert(t, r, sortVideo("/m/a.mkv", 100))
	const old = "2024-01-01T00:00:00Z"
	backdate(t, database, id, old)

	// A manual refresh, the file_tags backfill or a re-extract: same size, same mtime.
	upsert(t, r, sortVideo("/m/a.mkv", 100))
	if got := indexedAt(t, database, id); got != old {
		t.Fatalf("unchanged re-upsert moved indexed_at to %s, want %s", got, old)
	}

	// Only the mtime changes (a writeback that kept the size): it counts.
	touched := sortVideo("/m/a.mkv", 100)
	touched.FileMtime = sortMtime.Add(time.Hour)
	upsert(t, r, touched)
	if got := indexedAt(t, database, id); got == old {
		t.Fatal("an mtime change left indexed_at unchanged")
	}

	backdate(t, database, id, old)
	upsert(t, r, sortVideo("/m/a.mkv", 300))
	if got := indexedAt(t, database, id); got == old {
		t.Fatal("a size change left indexed_at unchanged")
	}
}

func TestUpdatedTiesBreakOnID(t *testing.T) {
	r, database := newRepoWithDB(t)
	var ids []int64
	for _, p := range []string{"/m/a.mkv", "/m/b.mkv", "/m/c.mkv"} {
		id := upsert(t, r, sortVideo(p, 100))
		backdate(t, database, id, "2024-01-01T00:00:00Z")
		ids = append(ids, id)
	}
	if got, want := sortedIDs(t, r, "updated_desc"), []int64{ids[2], ids[1], ids[0]}; !slices.Equal(got, want) {
		t.Errorf("updated_desc tie = %v, want id DESC %v", got, want)
	}
	if got, want := sortedIDs(t, r, "updated_asc"), ids; !slices.Equal(got, want) {
		t.Errorf("updated_asc tie = %v, want id ASC %v", got, want)
	}
}
