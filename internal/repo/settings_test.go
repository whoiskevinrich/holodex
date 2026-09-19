package repo_test

import (
	"context"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if v, ok, err := r.GetSetting(ctx, "theme.active"); err != nil || ok || v != "" {
		t.Fatalf("missing key = (%q, %v, %v), want (\"\", false, nil)", v, ok, err)
	}
	if err := r.PutSetting(ctx, "theme.active", "broadcast"); err != nil {
		t.Fatalf("put: %v", err)
	}
	if v, ok, err := r.GetSetting(ctx, "theme.active"); err != nil || !ok || v != "broadcast" {
		t.Fatalf("after put = (%q, %v, %v)", v, ok, err)
	}
	// Upsert replaces, never duplicates.
	if err := r.PutSetting(ctx, "theme.active", "brutalist"); err != nil {
		t.Fatalf("put again: %v", err)
	}
	if v, _, _ := r.GetSetting(ctx, "theme.active"); v != "brutalist" {
		t.Fatalf("after upsert = %q, want brutalist", v)
	}
	if err := r.PutSetting(ctx, "  ", "x"); err == nil {
		t.Fatal("empty key accepted")
	}
}
