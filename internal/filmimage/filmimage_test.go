package filmimage_test

import (
	"errors"
	"os"
	"testing"

	"holodex/internal/entityimage"
	"holodex/internal/filmimage"
	"holodex/internal/model"
)

// ImagePath/Store/Remove delegate to internal/entityimage (HOLODEX-286), which owns
// the actual disk layout and its round-trip/atomicity/traversal-safety coverage —
// these just confirm the delegation is wired correctly, not that behavior a second
// time.

func TestImagePath_DelegatesToEntityImage(t *testing.T) {
	got := filmimage.ImagePath("/data/film-images", 42, 7)
	want := entityimage.Path("/data/film-images", 42, 7)
	if got != want {
		t.Fatalf("ImagePath = %q, want %q (entityimage.Path)", got, want)
	}
}

func TestStoreRemove_Delegates(t *testing.T) {
	dir := t.TempDir()
	data := []byte("not-a-real-jpeg-but-bytes")

	if err := filmimage.Store(dir, 3, 9, data); err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, err := os.Stat(entityimage.Path(dir, 3, 9)); err != nil {
		t.Fatalf("stat after store: %v", err)
	}

	if err := filmimage.Remove(dir, 3, 9); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(entityimage.Path(dir, 3, 9)); !os.IsNotExist(err) {
		t.Fatalf("file still present after remove")
	}
}

// The banner role must be strictly wider than tall; every other role takes any
// aspect (HOLODEX-386). The typed error carries the dimensions both ingest paths
// report.
func TestCheckRoleAspect(t *testing.T) {
	for _, tc := range []struct {
		role   string
		w, h   int
		refuse bool
	}{
		{model.FilmImageBanner, 1000, 1500, true},
		{model.FilmImageBanner, 500, 500, true},
		{model.FilmImageBanner, 1280, 720, false},
		{model.FilmImageBanner, 501, 500, false},
		{model.FilmImagePoster, 1000, 1500, false},
		{model.FilmImagePoster, 500, 500, false},
	} {
		err := filmimage.CheckRoleAspect(tc.role, tc.w, tc.h)
		var portrait *filmimage.PortraitBannerError
		if got := errors.As(err, &portrait); got != tc.refuse {
			t.Errorf("%s %dx%d: refused=%v, want %v (err=%v)", tc.role, tc.w, tc.h, got, tc.refuse, err)
		}
		if tc.refuse && (portrait.Width != tc.w || portrait.Height != tc.h) {
			t.Errorf("%s %dx%d: refusal carries %dx%d", tc.role, tc.w, tc.h, portrait.Width, portrait.Height)
		}
	}
	if got := (&filmimage.PortraitBannerError{Width: 1000, Height: 1500}).Error(); got != "1000×1500 is portrait, banner role requires landscape" {
		t.Errorf("message = %q", got)
	}
}
