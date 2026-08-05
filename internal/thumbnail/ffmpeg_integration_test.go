//go:build integration

// Integration tests that invoke the real ffmpeg binary. Run with:
//
//	go test -tags integration ./internal/thumbnail/...
//
// These guard the ffmpeg argv against breakage that stubbed unit tests cannot
// see — e.g. the output muxer must be set explicitly because the temp file ends
// in ".tmp", which ffmpeg cannot map to a format on its own.
package thumbnail

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"holodex/internal/repo"
)

// TestGenerateFrameRealFfmpeg exercises Tier 2's dual-output requirement
// (P0-4, F53/HOLODEX-253) against a real ffmpeg binary: the source video is
// seeked/decoded once (only the poster-tier ffmpeg call carries -ss/the
// source path; the thumbnail-tier call reads the already-written poster JPEG
// back in with no seek), yielding two correctly-sized JPEGs.
func TestGenerateFrameRealFfmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	dir := t.TempDir()

	// A 1-second synthetic clip.
	src := filepath.Join(dir, "clip.mp4")
	gen := exec.Command("ffmpeg", "-y", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=5", "-loglevel", "error", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("could not synthesize clip: %v: %s", err, out)
	}

	m := New(Config{Enabled: true, Width: 200, PosterWidth: 300, Nice: false, Dir: dir}, nil, nil)
	thumbOut := filepath.Join(dir, "1.jpg")
	posterOut := filepath.Join(dir, "1-poster.jpg")
	if err := m.generateFrame(context.Background(),
		repo.ThumbnailCandidate{ID: 1, FilePath: src, DurationSec: 1}, thumbOut, posterOut); err != nil {
		t.Fatalf("generateFrame: %v", err)
	}
	// assertDecodedWidth is defined in coverart_test.go (same package, no
	// build tag — always compiled in, including under -tags integration).
	assertDecodedWidth(t, thumbOut, 200)
	assertDecodedWidth(t, posterOut, 300)
}
