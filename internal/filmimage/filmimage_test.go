package filmimage_test

import (
	"os"
	"testing"

	"holodex/internal/entityimage"
	"holodex/internal/filmimage"
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
