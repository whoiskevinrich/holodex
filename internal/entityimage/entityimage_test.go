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

// TestVariantPath: a width variant is a sibling of the original carrying its
// extension (HOLODEX-332).
func TestVariantPath(t *testing.T) {
	got := entityimage.VariantPath(entityimage.Path("/d", 4, 7, ".png"), 320)
	if want := filepath.Join("/d", "4", "7-w320.png"); got != want {
		t.Fatalf("VariantPath = %q, want %q", got, want)
	}
}

// TestStoreVariant_NoTempLeft: the per-writer temp is renamed away.
func TestStoreVariant_NoTempLeft(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "7-w160.jpg")
	if err := entityimage.StoreVariant(p, []byte("variant")); err != nil {
		t.Fatalf("store variant: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "7-w160.jpg" {
		t.Fatalf("dir = %v, want only 7-w160.jpg", entries)
	}
}

// writeFiles creates empty files named in entity 3's directory.
func writeFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	sub := filepath.Join(dir, "3")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(sub, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRemove_ClearsVariants: Remove takes an image's -w siblings (and a crashed
// variant write's temp) with it, and leaves other ids' files alone — 19 and 90 share
// digits with 9 but not the "9-w" prefix.
func TestRemove_ClearsVariants(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "9.jpg", "9-w160.jpg", "9-w640.jpg", "9-w320.jpg.123.tmp",
		"19.jpg", "19-w160.jpg", "90-w160.jpg")
	if err := entityimage.Remove(dir, 3, 9); err != nil {
		t.Fatalf("remove: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "3"))
	var left []string
	for _, e := range entries {
		left = append(left, e.Name())
	}
	if strings.Join(left, ",") != "19-w160.jpg,19.jpg,90-w160.jpg" {
		t.Fatalf("left after remove = %v", left)
	}
}

// TestRemoveAll_ClearsVariants: the entity purge takes variants too.
func TestRemoveAll_ClearsVariants(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "9.jpg", "9-w160.jpg")
	if err := entityimage.RemoveAll(dir, 3); err != nil {
		t.Fatalf("remove all: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "3")); !os.IsNotExist(err) {
		t.Fatalf("entity dir survived: %v", err)
	}
}

// TestFind_IgnoresVariants: Find resolves originals only — a lone -w file is not an
// original, and with both present Find returns the original.
func TestFind_IgnoresVariants(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "5-w320.jpg", "5-w160.png")
	if _, err := entityimage.Find(dir, 3, 5); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("find with only variants = %v, want os.ErrNotExist", err)
	}
	writeFiles(t, dir, "5.png")
	if p, err := entityimage.Find(dir, 3, 5); err != nil || p != entityimage.Path(dir, 3, 5, ".png") {
		t.Fatalf("find = %q, %v; want the original", p, err)
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
