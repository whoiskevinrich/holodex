//go:build integration

// Integration test for the edition writeback round trip (F60 RD8) against the
// real ffmpeg + exiftool binaries. Run with:
//
//	go test -tags integration ./internal/writeback/
//
// RD8 chose the tag keys from a manual write+read on generated samples; this
// pins that evidence so a future exiftool/ffmpeg bump that stops reading the
// key back — which would leave in_sync false forever (ADR-093) — fails here
// instead of on a real library.
package writeback_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"holodex/internal/metadata"
	"holodex/internal/writeback"
)

func TestEditionRoundTrip_BothContainers(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe", "exiftool"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	cases := []struct {
		container string // normalised, as internal/metadata reports it
		ext       string
	}{
		{"Matroska", "mkv"},
		{"MP4", "mp4"},
	}
	for _, tc := range cases {
		t.Run(tc.container, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "clip."+tc.ext)
			gen := exec.Command("ffmpeg", "-y", "-f", "lavfi",
				"-i", "testsrc=duration=1:size=64x64:rate=5", "-loglevel", "error", path)
			if out, err := gen.CombinedOutput(); err != nil {
				t.Skipf("could not synthesize clip: %v: %s", err, out)
			}

			tag, ok := writeback.TagForField("edition", tc.container)
			if !ok {
				t.Fatalf("no edition write target for %s", tc.container)
			}
			if err := writeback.Write(context.Background(), path, tag, []string{"Final Cut"}); err != nil {
				t.Fatalf("write %s=%q: %v", tag, "Final Cut", err)
			}

			// Read back through the scanner's own extractor: the mapping example's
			// bare `Edition` source must find the written value in the file layer.
			ex, err := metadata.NewExtractor().Extract(context.Background(), path)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if ex.Container != tc.container {
				t.Fatalf("container = %q, want %q", ex.Container, tc.container)
			}
			for _, e := range ex.Extra {
				if e.SourceKey == "Edition" {
					if e.Value != "Final Cut" {
						t.Fatalf("Edition read back as %q, want %q", e.Value, "Final Cut")
					}
					return
				}
			}
			t.Fatalf("Edition tag not read back after writing %s (extra: %+v)", tag, ex.Extra)
		})
	}
}
