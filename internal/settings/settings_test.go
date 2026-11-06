package settings_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"holodex/internal/db"
	"holodex/internal/rename"
	"holodex/internal/repo"
	"holodex/internal/settings"
)

// A promoted key round-trips: write, restart (a fresh store over the same DB), effective value.
// An undeclared key is never written, and a refused value leaves the effective one unchanged.
func TestStoreRoundTrip(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	ctx := context.Background()
	keys := rename.SettingKeys(func() []string { return nil })

	s := settings.New(r, keys...)
	if got := s.Get(rename.KeyMode); got != rename.ModeOff {
		t.Fatalf("default mode = %q", got)
	}
	if err := s.Put(ctx, rename.KeyMode, rename.ModeManual); err != nil {
		t.Fatal(err)
	}
	var inv *settings.InvalidError
	if err := s.Put(ctx, rename.KeySettle, "0"); !errors.As(err, &inv) {
		t.Errorf("settle 0: err = %v, want InvalidError", err)
	}
	if err := s.Put(ctx, "rename.other", "x"); !errors.Is(err, settings.ErrUnknownKey) {
		t.Errorf("undeclared key: err = %v", err)
	}
	if _, ok, _ := r.GetSetting(ctx, "rename.other"); ok {
		t.Error("an undeclared key was stored")
	}

	restarted := settings.New(r, keys...)
	if err := restarted.Load(ctx); err != nil {
		t.Fatal(err)
	}
	if got := restarted.Get(rename.KeyMode); got != rename.ModeManual {
		t.Errorf("mode after restart = %q", got)
	}
	if got := restarted.Get(rename.KeySettle); got != "7" {
		t.Errorf("settle after restart = %q", got)
	}
}
