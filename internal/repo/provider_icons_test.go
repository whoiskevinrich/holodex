package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/repo"
)

// TestProviderIcon_ReplaceGetListDelete exercises the provider_icons cache CRUD
// (ADR-059): absent → ErrNotFound, insert, replace advances the id (the ?v=
// cache-buster) while keeping the single row, List reflects the state, delete is
// idempotent. Keyed by provider name — no entity seeding needed.
func TestProviderIcon_ReplaceGetListDelete(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if _, err := r.GetProviderIcon(ctx, "tmdb"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get absent = %v, want ErrNotFound", err)
	}
	if rows, err := r.ListProviderIcons(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("list empty = %v (%d rows)", err, len(rows))
	}

	id1, err := r.ReplaceProviderIcon(ctx, repo.ProviderIconInsert{
		Provider: "tmdb", SourceURL: "https://cdn.example/a.png", Width: 64, Height: 64, ByteSize: 1234,
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := r.GetProviderIcon(ctx, "tmdb")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != id1 || got.SourceURL != "https://cdn.example/a.png" || got.Width != 64 || got.ByteSize != 1234 {
		t.Fatalf("row = %+v", got)
	}

	// A second provider is independent (UNIQUE is per provider, not global).
	if _, err := r.ReplaceProviderIcon(ctx, repo.ProviderIconInsert{
		Provider: "other", SourceURL: "https://cdn.example/o.png", Width: 32, Height: 32, ByteSize: 99,
	}); err != nil {
		t.Fatalf("replace other: %v", err)
	}
	if rows, _ := r.ListProviderIcons(ctx); len(rows) != 2 {
		t.Fatalf("list = %d rows, want 2", len(rows))
	}

	// Replace advances the id and keeps exactly one row for that provider.
	id2, err := r.ReplaceProviderIcon(ctx, repo.ProviderIconInsert{
		Provider: "tmdb", SourceURL: "https://cdn.example/b.png", Width: 128, Height: 128, ByteSize: 5678,
	})
	if err != nil {
		t.Fatalf("replace 2: %v", err)
	}
	if id2 <= id1 {
		t.Fatalf("replace id did not advance: %d then %d", id1, id2)
	}
	if got, _ := r.GetProviderIcon(ctx, "tmdb"); got.SourceURL != "https://cdn.example/b.png" || got.ID != id2 {
		t.Fatalf("after replace = %+v", got)
	}
	if rows, _ := r.ListProviderIcons(ctx); len(rows) != 2 {
		t.Fatalf("list after replace = %d rows, want 2 (single-slot per provider)", len(rows))
	}

	// Delete → gone; idempotent.
	if err := r.DeleteProviderIcon(ctx, "tmdb"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.GetProviderIcon(ctx, "tmdb"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get after delete = %v, want ErrNotFound", err)
	}
	if err := r.DeleteProviderIcon(ctx, "tmdb"); err != nil {
		t.Fatalf("delete idempotent: %v", err)
	}
	if rows, _ := r.ListProviderIcons(ctx); len(rows) != 1 {
		t.Fatalf("list after delete = %d rows, want 1 (other survives)", len(rows))
	}
}
