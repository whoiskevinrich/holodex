// Package db opens the SQLite database (modernc, pure Go — ADR-003) and applies
// embedded migrations on startup (ADR-016).
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	migsqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"

	"holodex/internal/db/migrations"
)

// Open opens (creating parent dirs if needed) the SQLite database at path with
// the WAL + performance pragmas from ADR-003, then runs all pending migrations.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}

	// Register holo_shuffle() (ADR-045) before opening — it must exist for the
	// connection the pool creates so the "Random" media sort can order by it.
	if err := registerShuffle(); err != nil {
		return nil, fmt.Errorf("register holo_shuffle: %w", err)
	}

	dsn := "file:" + path + "?" + url.Values{
		"_pragma": []string{
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			"busy_timeout(5000)",
			"foreign_keys(ON)",
			"cache_size(-64000)", // 64 MB
			"temp_store(MEMORY)",
		},
	}.Encode()

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := Migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}
	if err := optimize(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return sqlDB, nil
}

// optimize refreshes the query planner's statistics (sqlite_stat1) for every table that
// lacks them or has grown since they were taken. Without statistics the planner drives
// the review queue's per-side video counts from the near-total videos.active index: on a
// 20k-video library the Duplicates read took ~23 s without statistics and ~0.1 s with
// them (F70 OQ5). analysis_limit samples each index instead of scanning it, so this stays
// cheap at boot; both pragmas are per-connection, hence the pinned conn.
func optimize(sqlDB *sql.DB) error {
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("optimize: %w", err)
	}
	defer conn.Close()
	for _, pragma := range []string{"PRAGMA analysis_limit=1000", "PRAGMA optimize=0x10002"} {
		if _, err := conn.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("optimize (%s): %w", pragma, err)
		}
	}
	return nil
}

// Migrate applies all pending up-migrations. A failure aborts rather than
// leaving a half-migrated schema (ADR-016).
func Migrate(sqlDB *sql.DB) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	driver, err := migsqlite.WithInstance(sqlDB, &migsqlite.Config{})
	if err != nil {
		return fmt.Errorf("migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
