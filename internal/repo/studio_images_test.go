package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/model"
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

func TestStudioImage_ReplaceGetDelete(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	sid := seedStudio(t, r, "Ghibli")

	// Absent → ErrNotFound; GetStudio reports no image URLs.
	if _, err := r.GetStudioImage(ctx, sid, model.StudioImageLogo); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get absent = %v, want ErrNotFound", err)
	}
	if s, _ := r.GetStudio(ctx, sid); len(s.ImageVersions) != 0 {
		t.Fatalf("ImageVersions = %v, want empty", s.ImageVersions)
	}

	// Insert (as if via enrichment).
	id1, err := r.ReplaceStudioImage(ctx, repo.StudioImageInsert{
		StudioID: sid, Role: model.StudioImageLogo, Source: model.StudioImageSourceEnrichment,
		Provider: "tmdb", Width: 100, Height: 40, ByteSize: 1234,
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := r.GetStudioImage(ctx, sid, model.StudioImageLogo)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != id1 || got.Provider != "tmdb" || got.Width != 100 || got.ByteSize != 1234 {
		t.Fatalf("row = %+v", got)
	}
	if s, _ := r.GetStudio(ctx, sid); s.ImageVersions[model.StudioImageLogo] != id1 {
		t.Fatalf("ImageVersions[logo] = %d, want %d", s.ImageVersions[model.StudioImageLogo], id1)
	}

	// A different role is independent — inserting icon doesn't disturb logo.
	iconID, err := r.ReplaceStudioImage(ctx, repo.StudioImageInsert{
		StudioID: sid, Role: model.StudioImageIcon, Source: model.StudioImageSourceUpload,
		Width: 50, Height: 50, ByteSize: 500,
	})
	if err != nil {
		t.Fatalf("replace icon: %v", err)
	}
	if s, _ := r.GetStudio(ctx, sid); s.ImageVersions[model.StudioImageLogo] != id1 || s.ImageVersions[model.StudioImageIcon] != iconID {
		t.Fatalf("ImageVersions after independent roles = %v", s.ImageVersions)
	}

	// Replace advances the id (the ?v= cache-buster) and keeps exactly one row per role.
	id2, err := r.ReplaceStudioImage(ctx, repo.StudioImageInsert{
		StudioID: sid, Role: model.StudioImageLogo, Source: model.StudioImageSourceEnrichment,
		Provider: "tmdb", Width: 200, Height: 80, ByteSize: 5678,
	})
	if err != nil {
		t.Fatalf("replace 2: %v", err)
	}
	if id2 <= id1 {
		t.Fatalf("replace id did not advance: %d then %d", id1, id2)
	}
	if got, _ := r.GetStudioImage(ctx, sid, model.StudioImageLogo); got.ByteSize != 5678 {
		t.Fatalf("byte_size = %d, want 5678", got.ByteSize)
	}

	// Delete → gone; idempotent; other role untouched.
	if err := r.DeleteStudioImage(ctx, sid, model.StudioImageLogo); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.GetStudioImage(ctx, sid, model.StudioImageLogo); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get after delete = %v, want ErrNotFound", err)
	}
	if err := r.DeleteStudioImage(ctx, sid, model.StudioImageLogo); err != nil {
		t.Fatalf("delete idempotent: %v", err)
	}
	if _, err := r.GetStudioImage(ctx, sid, model.StudioImageIcon); err != nil {
		t.Fatalf("icon should be untouched by deleting logo: %v", err)
	}
}

func TestStudioImage_InvalidRole(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	sid := seedStudio(t, r, "Ghibli")

	if _, err := r.GetStudioImage(ctx, sid, "banner"); err == nil {
		t.Fatalf("get with invalid role should error")
	}
	if _, err := r.ReplaceStudioImage(ctx, repo.StudioImageInsert{StudioID: sid, Role: "banner", Source: model.StudioImageSourceUpload}); err == nil {
		t.Fatalf("replace with invalid role should error")
	}
}

// LockedStudioImageRoles reports only 'upload' rows, generalizing the ADR-049
// person provenance-lock to studios (F51, ADR-079).
func TestStudioImage_LockedCoreRoles(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	sid := seedStudio(t, r, "Ghibli")

	locked, err := r.LockedStudioImageRoles(ctx, sid)
	if err != nil || len(locked) != 0 {
		t.Fatalf("locked before any image = %v, %v", locked, err)
	}

	if _, err := r.ReplaceStudioImage(ctx, repo.StudioImageInsert{
		StudioID: sid, Role: model.StudioImageLogo, Source: model.StudioImageSourceEnrichment, Provider: "tmdb",
	}); err != nil {
		t.Fatalf("insert enrichment logo: %v", err)
	}
	if _, err := r.ReplaceStudioImage(ctx, repo.StudioImageInsert{
		StudioID: sid, Role: model.StudioImageIcon, Source: model.StudioImageSourceUpload,
	}); err != nil {
		t.Fatalf("insert uploaded icon: %v", err)
	}

	locked, err = r.LockedStudioImageRoles(ctx, sid)
	if err != nil {
		t.Fatalf("locked: %v", err)
	}
	if _, ok := locked[model.StudioImageIcon]; !ok {
		t.Fatalf("icon (upload) should be locked: %v", locked)
	}
	if _, ok := locked[model.StudioImageLogo]; ok {
		t.Fatalf("logo (enrichment) should NOT be locked: %v", locked)
	}
}
