package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrecedence(t *testing.T) {
	// YAML overrides defaults.
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "holodex.yaml")
	if err := os.WriteFile(yamlPath, []byte("port: 9000\nlog_level: debug\nscan_workers: 8\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9000 {
		t.Errorf("YAML should override default port: got %d, want 9000", cfg.Port)
	}
	if cfg.ScanWorkers != 8 {
		t.Errorf("YAML should override default scan_workers: got %d, want 8", cfg.ScanWorkers)
	}
	// Untouched key keeps its default.
	if cfg.ScanIntervalSeconds != 300 {
		t.Errorf("default scan_interval should remain 300, got %d", cfg.ScanIntervalSeconds)
	}

	// Env overrides YAML.
	t.Setenv("PORT", "7000")
	cfg, err = Load(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 7000 {
		t.Errorf("env should override YAML port: got %d, want 7000", cfg.Port)
	}
}

func TestApplyOverridesPrecedence(t *testing.T) {
	// Env sets PORT; a CLI override must win (CLI > env, F9.5).
	t.Setenv("PORT", "7000")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ApplyOverrides(Overrides{Port: 6000, DataPath: filepath.FromSlash("/srv/cli")})
	if cfg.Port != 6000 {
		t.Errorf("CLI should override env port: got %d, want 6000", cfg.Port)
	}
	// DataPath override re-derives DatabasePath.
	want := filepath.Join("/srv/cli", "holodex.db")
	if cfg.DatabasePath != want {
		t.Errorf("DatabasePath after data-path override = %q, want %q", cfg.DatabasePath, want)
	}
	// Empty override fields leave existing values untouched.
	cfg.ApplyOverrides(Overrides{})
	if cfg.Port != 6000 {
		t.Errorf("empty overrides must not reset port: got %d", cfg.Port)
	}
}

func TestDatabasePathDerivation(t *testing.T) {
	t.Setenv("DATA_PATH", filepath.FromSlash("/srv/holo"))
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/srv/holo", "holodex.db")
	if cfg.DatabasePath != want {
		t.Errorf("DatabasePath = %q, want %q", cfg.DatabasePath, want)
	}
}
