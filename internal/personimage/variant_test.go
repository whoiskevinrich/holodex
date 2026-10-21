package personimage

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"holodex/internal/entityimage"
)

func TestSnapWidth(t *testing.T) {
	cases := map[string]int{
		"":     0,
		"abc":  0,
		"1.5":  0,
		"0":    0,
		"-160": 0,
		"1":    160,
		"159":  160,
		"160":  160,
		"161":  320,
		"320":  320,
		"500":  640,
		"640":  640,
		"641":  0,
		"2000": 0,
	}
	for raw, want := range cases {
		if got := SnapWidth(raw); got != want {
			t.Errorf("SnapWidth(%q) = %d, want %d", raw, got, want)
		}
	}
}

// storeOriginal writes bytes as person 1's image 7 and returns the path Find resolves.
func storeOriginal(t *testing.T, data []byte) (dir, original string) {
	t.Helper()
	dir = t.TempDir()
	if err := Store(dir, 1, 7, data); err != nil {
		t.Fatalf("store: %v", err)
	}
	original, err := Find(dir, 1, 7)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	return dir, original
}

func decodeFile(t *testing.T, path string) (image.Image, string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return img, format
}

// variantFiles lists the -w files (and any temp) in person 1's directory.
func variantFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, "1"))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var out []string
	for _, e := range entries {
		if strings.Contains(e.Name(), "-w") || strings.HasSuffix(e.Name(), ".tmp") {
			out = append(out, e.Name())
		}
	}
	return out
}

// TestVariantScalesToTierWidth: a tier is the output width; the height follows the
// aspect ratio, and an opaque original stays JPEG.
func TestVariantScalesToTierWidth(t *testing.T) {
	_, original := storeOriginal(t, jpegBytes(t, 1333, 2000))
	path, err := Variant(original, 320)
	if err != nil {
		t.Fatalf("variant: %v", err)
	}
	if want := entityimage.VariantPath(original, 320); path != want || filepath.Base(path) != "7-w320.jpg" {
		t.Fatalf("variant path = %q, want %q", path, want)
	}
	img, format := decodeFile(t, path)
	if format != "jpeg" {
		t.Errorf("format = %q, want jpeg", format)
	}
	if b := img.Bounds(); b.Dx() != 320 || b.Dy() != 480 {
		t.Errorf("variant = %dx%d, want 320x480", b.Dx(), b.Dy())
	}
}

// TestVariantKeepsAlphaAsPNG: a PNG original (the alpha path) yields a PNG variant
// whose transparent background is still transparent.
func TestVariantKeepsAlphaAsPNG(t *testing.T) {
	norm, _, _, err := Normalize(transparentPNGBytes(t, 800, 600), 0)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	_, original := storeOriginal(t, norm)
	path, err := Variant(original, 160)
	if err != nil {
		t.Fatalf("variant: %v", err)
	}
	if filepath.Base(path) != "7-w160.png" {
		t.Fatalf("variant path = %q, want 7-w160.png", path)
	}
	img, format := decodeFile(t, path)
	if format != "png" {
		t.Fatalf("format = %q, want png", format)
	}
	if b := img.Bounds(); b.Dx() != 160 || b.Dy() != 120 {
		t.Errorf("variant = %dx%d, want 160x120", b.Dx(), b.Dy())
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("corner alpha = %d, want 0 (transparent)", a)
	}
}

// TestVariantNoUpscale: an original at or below the tier is served as is and no
// variant file is written.
func TestVariantNoUpscale(t *testing.T) {
	for _, w := range []int{300, 320} {
		dir, original := storeOriginal(t, jpegBytes(t, w, 400))
		path, err := Variant(original, 320)
		if err != nil {
			t.Fatalf("variant: %v", err)
		}
		if path != original {
			t.Errorf("%dpx original at w=320: served %q, want the original", w, path)
		}
		if got := variantFiles(t, dir); len(got) != 0 {
			t.Errorf("%dpx original at w=320 wrote %v, want no variant file", w, got)
		}
	}
}

// TestVariantCachedNotRegenerated: a second request serves the cached file without
// rewriting it or decoding the original again — the original is corrupted in between,
// so a re-decode would fail.
func TestVariantCachedNotRegenerated(t *testing.T) {
	_, original := storeOriginal(t, jpegBytes(t, 800, 800))
	path, err := Variant(original, 160)
	if err != nil {
		t.Fatalf("first variant: %v", err)
	}
	old := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if err := os.WriteFile(original, []byte("not an image"), 0o644); err != nil {
		t.Fatalf("corrupt original: %v", err)
	}
	again, err := Variant(original, 160)
	if err != nil || again != path {
		t.Fatalf("second variant = %q, %v; want %q", again, err, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.ModTime().Equal(old) {
		t.Errorf("variant mtime = %v, want %v (regenerated)", info.ModTime(), old)
	}
}

// TestVariantConcurrentFirstRequests: racing first requests for one tier all succeed,
// each can decode the whole file it is handed the moment it returns (as the handler
// would serve it), and the race leaves exactly one variant and no temp file.
func TestVariantConcurrentFirstRequests(t *testing.T) {
	dir, original := storeOriginal(t, jpegBytes(t, 900, 1200))
	const n = 32
	var wg sync.WaitGroup
	paths := make([]string, n)
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if paths[i], errs[i] = Variant(original, 640); errs[i] != nil {
				return
			}
			raw, err := os.ReadFile(paths[i])
			if err == nil {
				_, _, err = image.Decode(bytes.NewReader(raw))
			}
			errs[i] = err
		}()
	}
	wg.Wait()
	want := entityimage.VariantPath(original, 640)
	for i := range n {
		if errs[i] != nil || paths[i] != want {
			t.Errorf("request %d = %q, %v; want %q", i, paths[i], errs[i], want)
		}
	}
	if got := variantFiles(t, dir); len(got) != 1 || got[0] != "7-w640.jpg" {
		t.Errorf("files after race = %v, want [7-w640.jpg]", got)
	}
	if img, _ := decodeFile(t, want); img.Bounds().Dx() != 640 {
		t.Errorf("variant width = %d, want 640", img.Bounds().Dx())
	}
}
