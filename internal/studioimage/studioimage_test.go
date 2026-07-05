package studioimage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"holodex/internal/studioimage"
)

func TestImagePath_ServerAssignedIDsOnly(t *testing.T) {
	got := studioimage.ImagePath("/data/studio-logos", 42, 7)
	want := filepath.Join("/data/studio-logos", "42", "7.jpg")
	if got != want {
		t.Fatalf("ImagePath = %q, want %q", got, want)
	}
	// The path is built only from integer ids, so a traversal component can never
	// appear (the ADR-038 rule carried to studios).
	if strings.Contains(got, "..") {
		t.Fatalf("path contains traversal: %q", got)
	}
}

func TestStoreRemove_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	data := []byte("not-a-real-jpeg-but-bytes")

	if err := studioimage.Store(dir, 3, 9, data); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, err := os.ReadFile(studioimage.ImagePath(dir, 3, 9))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("round-trip mismatch")
	}
	// No temp file left behind after the atomic rename.
	if _, err := os.Stat(studioimage.ImagePath(dir, 3, 9) + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: %v", err)
	}

	if err := studioimage.Remove(dir, 3, 9); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(studioimage.ImagePath(dir, 3, 9)); !os.IsNotExist(err) {
		t.Fatalf("file still present after remove")
	}
	// Removing an absent file is not an error.
	if err := studioimage.Remove(dir, 3, 9); err != nil {
		t.Fatalf("remove absent: %v", err)
	}
}
