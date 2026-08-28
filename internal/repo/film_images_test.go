package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// TestFilmImage_ProviderSurfacesWhenNoUpload locks in the ADR-086 §2 read-path
// priority: a provider-sourced poster (film_images' UNIQUE(film_id, role, source)
// lets it coexist with an upload row) must still surface through both the batch
// list/detail path (GetFilm/ImageVersions) and the single-image serve path
// (GetFilmImageDisplayed) when no upload exists for that role.
func TestFilmImage_ProviderSurfacesWhenNoUpload(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	fid, err := r.CreateFilm(ctx, "Spirited Away", 2001)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}

	if _, err := r.GetFilmImageDisplayed(ctx, fid, model.FilmImagePoster); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("displayed (absent) = %v, want ErrNotFound", err)
	}

	providerID, err := r.ReplaceFilmImage(ctx, repo.FilmImageInsert{
		FilmID: fid, Role: model.FilmImagePoster, Source: "provider:tmdb", Provider: "tmdb", ExternalID: "129",
	})
	if err != nil {
		t.Fatalf("insert provider image: %v", err)
	}

	// Batch path (feeds Film.ImageVersions / PosterURL).
	f, err := r.GetFilm(ctx, fid)
	if err != nil {
		t.Fatalf("get film: %v", err)
	}
	if f.ImageVersions[model.FilmImagePoster] != providerID {
		t.Fatalf("ImageVersions[poster] = %v, want provider row id %d", f.ImageVersions, providerID)
	}

	// Single-image serve path.
	displayed, err := r.GetFilmImageDisplayed(ctx, fid, model.FilmImagePoster)
	if err != nil {
		t.Fatalf("displayed (provider only): %v", err)
	}
	if displayed.ID != providerID || displayed.Source != "provider:tmdb" {
		t.Fatalf("displayed = %+v, want provider row %d", displayed, providerID)
	}

	// An owner upload for the same role must win over the existing provider row,
	// on both paths — the coexistence ADR-086 §2 describes, with upload priority.
	uploadID, err := r.ReplaceFilmImage(ctx, repo.FilmImageInsert{
		FilmID: fid, Role: model.FilmImagePoster, Source: model.FilmImageSourceUpload,
	})
	if err != nil {
		t.Fatalf("insert upload image: %v", err)
	}
	if uploadID == providerID {
		t.Fatalf("upload row id collided with provider row id %d", providerID)
	}

	f, err = r.GetFilm(ctx, fid)
	if err != nil {
		t.Fatalf("get film after upload: %v", err)
	}
	if f.ImageVersions[model.FilmImagePoster] != uploadID {
		t.Fatalf("ImageVersions[poster] = %v, want upload row id %d (upload beats provider)", f.ImageVersions, uploadID)
	}

	displayed, err = r.GetFilmImageDisplayed(ctx, fid, model.FilmImagePoster)
	if err != nil {
		t.Fatalf("displayed (upload + provider): %v", err)
	}
	if displayed.ID != uploadID || displayed.Source != model.FilmImageSourceUpload {
		t.Fatalf("displayed = %+v, want upload row %d", displayed, uploadID)
	}

	// The provider row still exists untouched underneath the upload (distinct
	// UNIQUE(film_id, role, source) slots) — deleting the upload falls back to it.
	if err := r.DeleteFilmImage(ctx, fid, model.FilmImagePoster, model.FilmImageSourceUpload); err != nil {
		t.Fatalf("delete upload: %v", err)
	}
	displayed, err = r.GetFilmImageDisplayed(ctx, fid, model.FilmImagePoster)
	if err != nil {
		t.Fatalf("displayed (after upload deleted): %v", err)
	}
	if displayed.ID != providerID {
		t.Fatalf("displayed after upload delete = %+v, want fallback to provider row %d", displayed, providerID)
	}
}
