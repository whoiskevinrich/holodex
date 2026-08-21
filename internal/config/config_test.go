package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotenv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	content := "# a comment\n\nMEDIA_PATH=E:/Media\nQUOTED=\"a b\"\nexport EXPORTED=yes\nALREADY=fromfile\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// A key already set in the real environment must NOT be overridden by .env.
	t.Setenv("ALREADY", "fromenv")
	// Keys the file should set must start unset.
	for _, k := range []string{"MEDIA_PATH", "QUOTED", "EXPORTED"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}

	loadDotenv(p)

	cases := map[string]string{
		"MEDIA_PATH": "E:/Media",
		"QUOTED":     "a b",     // surrounding quotes stripped
		"EXPORTED":   "yes",     // `export ` prefix handled
		"ALREADY":    "fromenv", // real env wins over .env
	}
	for k, want := range cases {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}

	// A missing file is a silent no-op.
	loadDotenv(filepath.Join(dir, "nope.env"))
}

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
	// ThumbnailPath derives from DATA_PATH alongside the database (ADR-009/014).
	wantThumb := filepath.Join("/srv/holo", "thumbnails")
	if cfg.ThumbnailPath != wantThumb {
		t.Errorf("ThumbnailPath = %q, want %q", cfg.ThumbnailPath, wantThumb)
	}
	// PersonImagePath derives the same way (F25, ADR-038).
	wantPI := filepath.Join("/srv/holo", "person-images")
	if cfg.PersonImagePath != wantPI {
		t.Errorf("PersonImagePath = %q, want %q", cfg.PersonImagePath, wantPI)
	}
}

func TestPersonImageConfig(t *testing.T) {
	// Defaults (F25, ADR-038).
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PersonImageMaxBytes != 10<<20 || cfg.PersonImageMaxDimension != 2000 {
		t.Errorf("unexpected person-image defaults: bytes=%d dim=%d", cfg.PersonImageMaxBytes, cfg.PersonImageMaxDimension)
	}

	// Env overrides.
	t.Setenv("PERSON_IMAGE_MAX_BYTES", "5242880")
	t.Setenv("PERSON_IMAGE_MAX_DIMENSION", "1024")
	cfg, err = Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PersonImageMaxBytes != 5242880 || cfg.PersonImageMaxDimension != 1024 {
		t.Errorf("person-image env overrides not applied: bytes=%d dim=%d", cfg.PersonImageMaxBytes, cfg.PersonImageMaxDimension)
	}

	// DataPath override re-derives the image path (ADR-014).
	cfg.ApplyOverrides(Overrides{DataPath: filepath.FromSlash("/data2")})
	if cfg.PersonImagePath != filepath.Join("/data2", "person-images") {
		t.Errorf("PersonImagePath not re-derived on override: %q", cfg.PersonImagePath)
	}
}

func TestDefaultSourceConfig(t *testing.T) {
	// Default is file-first (F36/RD4).
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultSource != "file" {
		t.Errorf("default_source default = %q, want file", cfg.DefaultSource)
	}

	// A valid override is honored.
	t.Setenv("DEFAULT_SOURCE", "mapping")
	if cfg, err = Load(""); err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultSource != "mapping" {
		t.Errorf("default_source override = %q, want mapping", cfg.DefaultSource)
	}

	// An invalid value falls back to file rather than silently changing precedence.
	t.Setenv("DEFAULT_SOURCE", "bogus")
	if cfg, err = Load(""); err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultSource != "file" {
		t.Errorf("invalid default_source = %q, want file fallback", cfg.DefaultSource)
	}
}

func TestFilmsEnabledConfig(t *testing.T) {
	// Default is off (F56, ADR-085 — opt-in, matching mcp_enabled's precedent).
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FilmsEnabled {
		t.Errorf("films_enabled default = true, want false")
	}

	t.Setenv("FILMS_ENABLED", "true")
	if cfg, err = Load(""); err != nil {
		t.Fatal(err)
	}
	if !cfg.FilmsEnabled {
		t.Errorf("films_enabled override = false, want true")
	}
}

func TestProviderTrustOrderConfig(t *testing.T) {
	// Absent by default (mapping-order fallback among providers).
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ProviderTrustOrder) != 0 {
		t.Errorf("default provider_trust_order = %v, want empty", cfg.ProviderTrustOrder)
	}

	// Comma-separated env is split, trimmed, lower-cased, blank-dropped, and
	// de-duped in order. Lower-casing matters: the resolver only matches lower-cased
	// mapping namespaces, so "TMDB" must fold to "tmdb" (and de-dup against it).
	t.Setenv("PROVIDER_TRUST_ORDER", " TMDB , other ,, tmdb ")
	if cfg, err = Load(""); err != nil {
		t.Fatal(err)
	}
	want := []string{"tmdb", "other"}
	if len(cfg.ProviderTrustOrder) != len(want) {
		t.Fatalf("provider_trust_order = %v, want %v", cfg.ProviderTrustOrder, want)
	}
	for i, w := range want {
		if cfg.ProviderTrustOrder[i] != w {
			t.Errorf("provider_trust_order[%d] = %q, want %q", i, cfg.ProviderTrustOrder[i], w)
		}
	}
}

func TestThumbnailConfig(t *testing.T) {
	// Defaults (ADR-009).
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ThumbnailEnabled || cfg.ThumbnailBackfill != "eager" ||
		cfg.ThumbnailWorkers != 2 || !cfg.ThumbnailNice ||
		cfg.ThumbnailSeekPercent != 10 || cfg.ThumbnailWidth != 400 || cfg.PosterWidth != 1200 {
		t.Errorf("unexpected thumbnail defaults: %+v", cfg)
	}

	// Env overrides, including disabling the bool.
	t.Setenv("THUMBNAIL_ENABLED", "false")
	t.Setenv("THUMBNAIL_BACKFILL", "lazy")
	t.Setenv("THUMBNAIL_WORKERS", "4")
	t.Setenv("THUMBNAIL_WIDTH", "640")
	t.Setenv("POSTER_WIDTH", "1600")
	cfg, err = Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ThumbnailEnabled {
		t.Errorf("THUMBNAIL_ENABLED=false not applied")
	}
	if cfg.ThumbnailBackfill != "lazy" || cfg.ThumbnailWorkers != 4 || cfg.ThumbnailWidth != 640 || cfg.PosterWidth != 1600 {
		t.Errorf("thumbnail env overrides not applied: %+v", cfg)
	}
}
