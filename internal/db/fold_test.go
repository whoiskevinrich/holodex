package db_test

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migsqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"

	"holodex/internal/db/migrations"
)

// openAt opens a fresh DB (FK enforcement on, like db.Open) and returns a migrate
// handle so a test can step to a specific schema version.
func openAt(t *testing.T) (*sql.DB, *migrate.Migrate) {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "fold.db") + "?" +
		url.Values{"_pragma": []string{"busy_timeout(5000)", "foreign_keys(ON)"}}.Encode()
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB.SetMaxOpenConns(1) // single connection so the FK pragma always applies
	t.Cleanup(func() { sqlDB.Close() })

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	driver, err := migsqlite.WithInstance(sqlDB, &migsqlite.Config{})
	if err != nil {
		t.Fatalf("migration driver: %v", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		t.Fatalf("migrate init: %v", err)
	}
	return sqlDB, m
}

func mustExec(t *testing.T, db *sql.DB, q string) {
	t.Helper()
	if _, err := db.Exec(q); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func count(t *testing.T, db *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", q, err)
	}
	return n
}

// TestMigration0022FoldsCaseDuplicates exercises the load-bearing in-SQL fold that
// migration 0022 runs before building the nameKey unique indexes (F43 RD10): it must
// collapse exact-key canonical duplicates that predate the index, moving associations
// onto the survivor and registering the loser's spelling as an alias — so the unique
// index build (which follows in the same migration) sees a clean set. This is the only
// path that runs the fold; a clean DB skips it.
func TestMigration0022FoldsCaseDuplicates(t *testing.T) {
	db, m := openAt(t)
	if err := m.Migrate(21); err != nil { // one before the F43 migration
		t.Fatalf("migrate to 21: %v", err)
	}

	// Two case-variant people that the pre-F43 binary UNIQUE(name) allowed to coexist,
	// each crediting a different video; the same shape for tags (internal-whitespace).
	mustExec(t, db, `INSERT INTO videos (id, file_path, indexed_at, file_mtime) VALUES
		(1,'/m/a.mkv','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z'),
		(2,'/m/b.mkv','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO people (id, name) VALUES (1,'fox'),(2,'Fox')`)
	mustExec(t, db, `INSERT INTO video_people (video_id, person_id) VALUES (1,1),(2,2)`)
	// The loser carries its own F23 alias — the fold must move it onto the survivor.
	mustExec(t, db, `INSERT INTO person_aliases (person_id, alias) VALUES (2,'Foxy')`)
	mustExec(t, db, `INSERT INTO tags (id, name) VALUES (1,'sci fi'),(2,'SciFi')`)
	mustExec(t, db, `INSERT INTO video_tags (video_id, tag_id) VALUES (1,1),(2,2)`)

	if err := m.Migrate(22); err != nil {
		t.Fatalf("migrate to 22 (fold): %v", err)
	}

	// People folded to the lowest id; the loser spelling survives as an alias.
	if n := count(t, db, `SELECT COUNT(*) FROM people`); n != 1 {
		t.Fatalf("people not folded: got %d, want 1", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM people WHERE id = 1`); n != 1 {
		t.Fatalf("survivor should be the lowest id (1)")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases WHERE entity_type='person' AND entity_id=1 AND alias='Fox'`); n != 1 {
		t.Fatalf("loser name 'Fox' not registered as an alias of the survivor")
	}
	// The loser's own F23 alias moved onto the survivor; nothing dangles on the loser id.
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases WHERE entity_type='person' AND entity_id=1 AND alias='Foxy'`); n != 1 {
		t.Fatalf("loser's own alias 'Foxy' not moved to the survivor")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM entity_aliases WHERE entity_type='person' AND entity_id=2`); n != 0 {
		t.Fatalf("aliases orphaned on the deleted loser id")
	}
	// Both videos now credit the survivor; nothing dangles on the deleted loser.
	if n := count(t, db, `SELECT COUNT(*) FROM video_people WHERE person_id = 1`); n != 2 {
		t.Fatalf("associations not moved to survivor: got %d, want 2", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM video_people WHERE person_id = 2`); n != 0 {
		t.Fatalf("loser still has associations")
	}
	// Tags folded on the whitespace-stripped key ('sci fi' / 'SciFi' → 'scifi').
	if n := count(t, db, `SELECT COUNT(*) FROM tags`); n != 1 {
		t.Fatalf("tags not folded: got %d, want 1", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM video_tags WHERE tag_id = 1`); n != 2 {
		t.Fatalf("tag associations not moved: got %d, want 2", n)
	}
	// The unique nameKey index now makes a fresh case variant unrepresentable.
	if _, err := db.Exec(`INSERT INTO people (name) VALUES ('FOX')`); err == nil {
		t.Fatalf("ux_people_namekey should reject a case-variant of an existing name")
	}
}
