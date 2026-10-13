//go:build integration

// Integration test for the tagline and original_language writeback round trip
// (HOLODEX-336) against the real ffmpeg + exiftool binaries. Run with:
//
//	go test -tags integration ./internal/writeback/
//
// Both fields once wrote an MP4 tag that never came back: original_language went to
// QuickTime:MediaLanguage, which exiftool does not define (the write was dropped with a
// warning), and tagline went to Keywords, which the scanner reads as content tags.
// This pins that each lands under the key the mapping example's file: source names.
package writeback_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"holodex/internal/metadata"
	"holodex/internal/writeback"
)

func TestLanguageAndTaglineRoundTrip_BothContainers(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe", "exiftool"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	cases := []struct {
		container, ext, field, value, readKey string
	}{
		{"Matroska", "mkv", "original_language", "ja", "Language"},
		{"MP4", "mp4", "original_language", "ja", "Language"},
		{"Matroska", "mkv", "tagline", "Some doors stay shut.", "Subtitle"},
		{"MP4", "mp4", "tagline", "Some doors stay shut.", "Description"},
	}
	for _, tc := range cases {
		t.Run(tc.container+"/"+tc.field, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "clip."+tc.ext)
			gen := exec.Command("ffmpeg", "-y", "-f", "lavfi",
				"-i", "testsrc=duration=1:size=64x64:rate=5", "-loglevel", "error", path)
			if out, err := gen.CombinedOutput(); err != nil {
				t.Skipf("could not synthesize clip: %v: %s", err, out)
			}

			tag, ok := writeback.TagForField(tc.field, tc.container)
			if !ok {
				t.Fatalf("no %s write target for %s", tc.field, tc.container)
			}
			if err := writeback.Write(context.Background(), path, tag, []string{tc.value}); err != nil {
				t.Fatalf("write %s=%q: %v", tag, tc.value, err)
			}

			ex, err := metadata.NewExtractor().Extract(context.Background(), path)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if len(ex.Tags) != 0 {
				t.Errorf("%s read back as content tags: %v", tc.field, ex.Tags)
			}
			for _, e := range ex.Extra {
				if e.SourceKey == tc.readKey {
					if e.Value != tc.value {
						t.Fatalf("%s read back as %q, want %q", tc.readKey, e.Value, tc.value)
					}
					return
				}
			}
			t.Fatalf("%s not read back under %s after writing %s (extra: %+v)", tc.field, tc.readKey, tag, ex.Extra)
		})
	}
}
