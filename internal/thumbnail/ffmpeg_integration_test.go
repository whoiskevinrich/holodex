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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"holodex/internal/repo"
)

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

	m := New(Config{Enabled: true, Width: 200, Nice: false, Dir: dir}, nil, nil)
	out := filepath.Join(dir, "1.jpg")
	if err := m.generateFrame(context.Background(),
		repo.ThumbnailCandidate{ID: 1, FilePath: src, DurationSec: 1}, out); err != nil {
		t.Fatalf("generateFrame: %v", err)
	}
	fi, err := os.Stat(out)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("expected a non-empty jpeg at %s (err=%v)", out, err)
	}
}
