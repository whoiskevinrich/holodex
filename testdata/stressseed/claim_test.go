package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"holodex/internal/db"
)

// Go ignores directories named testdata when matching ./..., so these tests do
// not run under `make test`. Run them explicitly after touching the guard:
//
//	go test ./testdata/stressseed
//
// They are worth keeping despite that: the guard is the only thing standing
// between a mistyped -data and someone's real library.

func TestInspect_MissingDatabaseIsClaimable(t *testing.T) {
	c, err := inspect(filepath.Join(t.TempDir(), "holodex.db"))
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if c.claimed || len(c.foreign) > 0 {
		t.Fatalf("a database that does not exist yet should be free to claim, got %+v", c)
	}
}

// The trap this covers: starting the backend-stress server before the first seed
// creates an empty, migrated database. Refusing that would make the tool useless
// in the order people actually do things.
func TestInspect_EmptyDatabaseIsClaimable(t *testing.T) {
	path := newDatabase(t)

	c, err := inspect(path)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if c.claimed || len(c.foreign) > 0 {
		t.Fatalf("an empty migrated database should be free to claim, got %+v", c)
	}
}

func TestInspect_ContentRowsAreForeign(t *testing.T) {
	path := newDatabase(t)
	withDatabase(t, path, func(database *sql.DB) {
		if _, err := database.Exec(
			`INSERT INTO videos (file_path, indexed_at, file_mtime) VALUES ('/media/real.mp4', '', '')`); err != nil {
			t.Fatalf("insert video: %v", err)
		}
	})

	c, err := inspect(path)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if len(c.foreign) != 1 || c.foreign[0] != "videos: 1" {
		t.Fatalf("expected the video row to be reported as foreign, got %+v", c.foreign)
	}
}

func TestInspect_MarkerClaimsTheDatabase(t *testing.T) {
	path := newDatabase(t)
	withDatabase(t, path, func(database *sql.DB) {
		if err := markOwned(context.Background(), database, 7, 100); err != nil {
			t.Fatalf("markOwned: %v", err)
		}
	})

	c, err := inspect(path)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !c.claimed || c.seed != 7 {
		t.Fatalf("expected the marker to claim the database with seed 7, got %+v", c)
	}
}

// Rows the fixture itself creates must not read as foreign on the next run —
// otherwise the tool refuses to re-run against its own output.
func TestInspect_OwnRowsAreNotForeign(t *testing.T) {
	path := newDatabase(t)
	withDatabase(t, path, func(database *sql.DB) {
		if err := markOwned(context.Background(), database, 1, 100); err != nil {
			t.Fatalf("markOwned: %v", err)
		}
		if _, err := database.Exec(
			`INSERT INTO videos (file_path, indexed_at, file_mtime) VALUES ('/stress/1.mp4', '', '')`); err != nil {
			t.Fatalf("insert video: %v", err)
		}
	})

	c, err := inspect(path)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !c.claimed || len(c.foreign) > 0 {
		t.Fatalf("the fixture's own rows should not read as foreign, got %+v", c)
	}
}

func TestRun_RefusesAForeignDatabase(t *testing.T) {
	dir := t.TempDir()
	withDatabase(t, filepath.Join(dir, "holodex.db"), func(database *sql.DB) {
		if _, err := database.Exec(
			`INSERT INTO videos (file_path, indexed_at, file_mtime) VALUES ('/media/real.mp4', '', '')`); err != nil {
			t.Fatalf("insert video: %v", err)
		}
	})

	err := run(dir, 100, 1)
	if err == nil {
		t.Fatal("expected run to refuse a database holding rows it did not create")
	}
	if !strings.Contains(err.Error(), "videos: 1") {
		t.Fatalf("the refusal should name what it found, got %v", err)
	}
}

func TestRun_RefusesADifferentSeed(t *testing.T) {
	dir := t.TempDir()
	if err := run(dir, 100, 1); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := run(dir, 100, 2); err == nil {
		t.Fatal("expected run to refuse re-seeding an existing fixture with a different seed")
	}
}

func TestRun_IsRepeatable(t *testing.T) {
	dir := t.TempDir()
	for i := range 2 {
		if err := run(dir, 100, 1); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
}

// newDatabase returns the path to a migrated, empty database — the same thing
// the server creates on first start.
func newDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "holodex.db")
	withDatabase(t, path, func(*sql.DB) {})
	return path
}

// withDatabase opens the database, runs fn, and closes it again. Closing matters:
// inspect opens its own connection, and Windows will not remove the temp dir
// while a handle is open.
func withDatabase(t *testing.T, path string, fn func(*sql.DB)) {
	t.Helper()
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer database.Close()
	fn(database)
}
