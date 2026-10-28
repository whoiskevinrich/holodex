package db

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Open leaves the planner statistics in place for a table that has rows but was never
// analyzed (F70 OQ5): without sqlite_stat1 the Duplicates studio and tag reads pick the
// near-total videos.active index and slow down by ~50x.
func TestOpenAnalyzesTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// Stand in for a library that grew under a build that never analyzed: drop whatever
	// the first Open recorded, then add rows.
	if _, err := database.Exec(`DELETE FROM sqlite_stat1`); err != nil {
		t.Fatal(err)
	}
	for i := range 500 {
		if _, err := database.Exec(`INSERT INTO videos (file_path, indexed_at, file_mtime) VALUES (?, '', '')`,
			fmt.Sprintf("/media/v%d.mp4", i)); err != nil {
			t.Fatal(err)
		}
	}
	database.Close()

	database, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var n int
	if err := database.QueryRow(`SELECT count(*) FROM sqlite_stat1 WHERE tbl = 'videos'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("Open left videos without planner statistics")
	}
}
