package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/repo"
)

// seedStudio creates a studio via the link derivation (the only writer of the
// studios table) and returns its id.
func seedStudio(t *testing.T, r *repo.Repo, name string) int64 {
	t.Helper()
	ctx := context.Background()
	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/"+name+".mkv", name, nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, vid, []string{name}, nil); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil {
		t.Fatalf("list studios: %v", err)
	}
	for _, s := range studios {
		if s.Name == name {
			return s.ID
		}
	}
	t.Fatalf("studio %q not created", name)
	return 0
}

func TestStudioLogo_ReplaceGetDelete(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	sid := seedStudio(t, r, "Ghibli")

	// Absent → ErrNotFound; count 0; GetStudio reports no version.
	if _, err := r.GetStudioLogo(ctx, sid); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get absent = %v, want ErrNotFound", err)
	}
	if n, _ := r.StudioLogoCount(ctx); n != 0 {
		t.Fatalf("count = %d, want 0", n)
	}
	if s, _ := r.GetStudio(ctx, sid); s.LogoVersion != 0 {
		t.Fatalf("LogoVersion = %d, want 0", s.LogoVersion)
	}

	// Insert.
	id1, err := r.ReplaceStudioLogo(ctx, repo.StudioLogoInsert{
		StudioID: sid, SourceURL: "https://cdn.example/a.jpg", Provider: "tmdb",
		Width: 100, Height: 40, ByteSize: 1234,
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := r.GetStudioLogo(ctx, sid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != id1 || got.SourceURL != "https://cdn.example/a.jpg" || got.Provider != "tmdb" || got.Width != 100 || got.ByteSize != 1234 {
		t.Fatalf("row = %+v", got)
	}
	if s, _ := r.GetStudio(ctx, sid); s.LogoVersion != id1 {
		t.Fatalf("LogoVersion = %d, want %d", s.LogoVersion, id1)
	}

	// Replace advances the id (the ?v= cache-buster) and keeps exactly one row.
	id2, err := r.ReplaceStudioLogo(ctx, repo.StudioLogoInsert{
		StudioID: sid, SourceURL: "https://cdn.example/b.jpg", Provider: "tmdb",
		Width: 200, Height: 80, ByteSize: 5678,
	})
	if err != nil {
		t.Fatalf("replace 2: %v", err)
	}
	if id2 <= id1 {
		t.Fatalf("replace id did not advance: %d then %d", id1, id2)
	}
	if n, _ := r.StudioLogoCount(ctx); n != 1 {
		t.Fatalf("count after replace = %d, want 1 (single-slot)", n)
	}
	if got, _ := r.GetStudioLogo(ctx, sid); got.SourceURL != "https://cdn.example/b.jpg" {
		t.Fatalf("source_url = %q, want b.jpg", got.SourceURL)
	}

	// Delete → gone; idempotent.
	if err := r.DeleteStudioLogo(ctx, sid); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.GetStudioLogo(ctx, sid); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get after delete = %v, want ErrNotFound", err)
	}
	if err := r.DeleteStudioLogo(ctx, sid); err != nil {
		t.Fatalf("delete idempotent: %v", err)
	}
}
