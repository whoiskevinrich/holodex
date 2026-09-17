package studioimage_test

import (
	"errors"
	"os"
	"testing"

	"holodex/internal/entityimage"
	"holodex/internal/studioimage"
)

// Find/Store/Remove delegate to internal/entityimage (HOLODEX-286), which owns
// the actual disk layout and its round-trip/atomicity/traversal-safety coverage —
// these just confirm the delegation is wired correctly, not that behavior a second
// time.

func TestFind_DelegatesToEntityImage(t *testing.T) {
	dir := t.TempDir()
	if err := studioimage.Store(dir, 42, 7, []byte("opaque-bytes")); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, err := studioimage.Find(dir, 42, 7)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	want, _ := entityimage.Find(dir, 42, 7)
	if got != want {
		t.Fatalf("Find = %q, want %q (entityimage.Find)", got, want)
	}
}

func TestStoreRemove_Delegates(t *testing.T) {
	dir := t.TempDir()
	data := []byte("not-a-real-jpeg-but-bytes")

	if err := studioimage.Store(dir, 3, 9, data); err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, err := entityimage.Find(dir, 3, 9); err != nil {
		t.Fatalf("find after store: %v", err)
	}

	if err := studioimage.Remove(dir, 3, 9); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := entityimage.Find(dir, 3, 9); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file still present after remove")
	}
}
