package thumbnail

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// pngOfWidth encodes a solid-color square PNG of the given width so
// image.DecodeConfig reports a known, deterministic size without needing a
// real cover-art extraction.
func pngOfWidth(t *testing.T, w int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, w))
	for y := 0; y < w; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func coverArtManager(t *testing.T, width, posterWidth int) *Manager {
	t.Helper()
	return New(Config{Enabled: true, Width: width, PosterWidth: posterWidth, Dir: t.TempDir()},
		slog.New(slog.NewTextHandler(os.Stderr, nil)), newFakeRepo())
}

// TestWriteCoverArtTiersWithinBothCaps covers the P0-3 band that needs no
// ffmpeg at all: a source image already <= min(Width, PosterWidth) is written
// byte-identical to both destinations (docs/testing-strategy.md F53 GWT §10).
func TestWriteCoverArtTiersWithinBothCaps(t *testing.T) {
	m := coverArtManager(t, 400, 1200)
	data := pngOfWidth(t, 100)
	thumbDst := filepath.Join(t.TempDir(), "1.jpg")
	posterDst := filepath.Join(t.TempDir(), "1-poster.jpg")

	if err := m.writeCoverArtTiers(context.Background(), data, thumbDst, posterDst); err != nil {
		t.Fatalf("writeCoverArtTiers: %v", err)
	}

	thumbBytes, err := os.ReadFile(thumbDst)
	if err != nil {
		t.Fatalf("read thumb: %v", err)
	}
	posterBytes, err := os.ReadFile(posterDst)
	if err != nil {
		t.Fatalf("read poster: %v", err)
	}
	if !bytes.Equal(thumbBytes, data) || !bytes.Equal(posterBytes, data) {
		t.Errorf("expected both tiers byte-identical to the source when within both caps")
	}
}

// TestWriteCoverArtTiersScaling covers the two ffmpeg-backed bands: a source
// between ThumbnailWidth and PosterWidth (poster raw copy, thumbnail scaled),
// and a source over PosterWidth (both scaled independently). Skips if ffmpeg
// isn't on PATH, mirroring the existing integration-test convention.
func TestWriteCoverArtTiersScaling(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}

	t.Run("between thumbnail and poster width", func(t *testing.T) {
		m := coverArtManager(t, 400, 1200)
		data := pngOfWidth(t, 800)
		thumbDst := filepath.Join(t.TempDir(), "1.jpg")
		posterDst := filepath.Join(t.TempDir(), "1-poster.jpg")

		if err := m.writeCoverArtTiers(context.Background(), data, thumbDst, posterDst); err != nil {
			t.Fatalf("writeCoverArtTiers: %v", err)
		}
		posterBytes, err := os.ReadFile(posterDst)
		if err != nil {
			t.Fatalf("read poster: %v", err)
		}
		if !bytes.Equal(posterBytes, data) {
			t.Errorf("poster tier should be an untouched copy of the source bytes")
		}
		assertDecodedWidth(t, thumbDst, 400)
	})

	t.Run("over poster width", func(t *testing.T) {
		m := coverArtManager(t, 400, 1200)
		data := pngOfWidth(t, 2000)
		thumbDst := filepath.Join(t.TempDir(), "1.jpg")
		posterDst := filepath.Join(t.TempDir(), "1-poster.jpg")

		if err := m.writeCoverArtTiers(context.Background(), data, thumbDst, posterDst); err != nil {
			t.Fatalf("writeCoverArtTiers: %v", err)
		}
		assertDecodedWidth(t, thumbDst, 400)
		assertDecodedWidth(t, posterDst, 1200)
	})
}

func assertDecodedWidth(t *testing.T, path string, want int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if cfg.Width != want {
		t.Errorf("%s width = %d, want %d", path, cfg.Width, want)
	}
}
