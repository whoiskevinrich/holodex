//go:build integration

// Integration test for the part writeback round trip (HOLODEX-389 RD5) against
// the real ffmpeg + exiftool binaries, the sibling of edition_integration_test.go.
// Run with:
//
//	go test -tags integration ./internal/writeback/
//
// RD5 chose PART_NUMBER (Matroska/WebM) and the iTunes `disk` atom (MP4). The
// MP4 case is the ADR-093 claim in miniature: exiftool renders a `disk` atom
// with no total as "2 of 0", and it is the extractor's ordinal normalisation
// that turns that back into the bare "2" Holodex wrote — so this reads back
// through internal/metadata, not raw exiftool, and asserts the bare ordinal.
package writeback_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"holodex/internal/metadata"
	"holodex/internal/writeback"
)

func TestPartRoundTrip_BothContainers(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe", "exiftool"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	cases := []struct {
		container string // normalised, as internal/metadata reports it
		ext       string
		readKey   string // the extra_metadata key the mapping's file: source addresses
	}{
		{"Matroska", "mkv", "PartNumber"},
		{"MP4", "mp4", "DiskNumber"},
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

			tag, ok := writeback.TagForField("part", tc.container)
			if !ok {
				t.Fatalf("no part write target for %s", tc.container)
			}
			if err := writeback.Write(context.Background(), path, tag, []string{"2"}); err != nil {
				t.Fatalf("write %s=%q: %v", tag, "2", err)
			}

			ex, err := metadata.NewExtractor().Extract(context.Background(), path)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if ex.Container != tc.container {
				t.Fatalf("container = %q, want %q", ex.Container, tc.container)
			}
			for _, e := range ex.Extra {
				if e.SourceKey == tc.readKey {
					if e.Value != "2" {
						t.Fatalf("%s read back as %q, want bare %q", tc.readKey, e.Value, "2")
					}
					return
				}
			}
			t.Fatalf("%s not read back after writing %s (extra: %+v)", tc.readKey, tag, ex.Extra)
		})
	}
}
