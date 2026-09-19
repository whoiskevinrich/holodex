package entityimage_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"holodex/internal/entityimage"
)

func TestPath_ServerAssignedIDsOnly(t *testing.T) {
	got := entityimage.Path("/data/entity-images", 42, 7, ".jpg")
	want := filepath.Join("/data/entity-images", "42", "7.jpg")
	if got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
	// The path is built only from integer ids, so a traversal component can never
	// appear (the ADR-038 rule).
	if strings.Contains(got, "..") {
		t.Fatalf("path contains traversal: %q", got)
	}
}

// TestExt_FollowsBytes: the on-disk extension is derived from the normalized bytes
// (ADR-097) — a PNG signature means .png, anything else (JPEG) .jpg.
func TestExt_FollowsBytes(t *testing.T) {
	if got := entityimage.Ext([]byte("\x89PNG\r\n\x1a\n...")); got != ".png" {
		t.Fatalf("Ext(png) = %q, want .png", got)
	}
	if got := entityimage.Ext([]byte("\xff\xd8\xff\xe0...")); got != ".jpg" {
		t.Fatalf("Ext(jpeg) = %q, want .jpg", got)
	}
	if got := entityimage.ContentType(filepath.Join("x", "7.png")); got != "image/png" {
		t.Fatalf("ContentType(png) = %q", got)
	}
	if got := entityimage.ContentType(filepath.Join("x", "7.jpg")); got != "image/jpeg" {
		t.Fatalf("ContentType(jpg) = %q", got)
	}
}

func TestStoreRemove_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	data := []byte("not-a-real-jpeg-but-bytes")

	if err := entityimage.Store(dir, 3, 9, data); err != nil {
		t.Fatalf("store: %v", err)
	}
	path, err := entityimage.Find(dir, 3, 9)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if path != entityimage.Path(dir, 3, 9, ".jpg") {
		t.Fatalf("opaque bytes stored at %q, want .jpg", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("round-trip mismatch")
	}
	// No temp file left behind after the atomic rename.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: %v", err)
	}

	if err := entityimage.Remove(dir, 3, 9); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := entityimage.Find(dir, 3, 9); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("find after remove = %v, want os.ErrNotExist", err)
	}
	// Removing an absent file is not an error.
	if err := entityimage.Remove(dir, 3, 9); err != nil {
		t.Fatalf("remove absent: %v", err)
	}
}

// TestStoreRemove_PNG: PNG bytes land at {id}.png, Find resolves it, and Remove clears
// it — the extension is never stored, only derived (ADR-097).
func TestStoreRemove_PNG(t *testing.T) {
	dir := t.TempDir()
	data := []byte("\x89PNG\r\n\x1a\nrest-of-a-png")

	if err := entityimage.Store(dir, 3, 10, data); err != nil {
		t.Fatalf("store: %v", err)
	}
	path, err := entityimage.Find(dir, 3, 10)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if path != entityimage.Path(dir, 3, 10, ".png") {
		t.Fatalf("png bytes stored at %q, want .png", path)
	}
	if _, err := os.Stat(entityimage.Path(dir, 3, 10, ".jpg")); !os.IsNotExist(err) {
		t.Fatalf("a .jpg sibling exists for a png image: %v", err)
	}
	if err := entityimage.Remove(dir, 3, 10); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := entityimage.Find(dir, 3, 10); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("find after remove = %v, want os.ErrNotExist", err)
	}
}
