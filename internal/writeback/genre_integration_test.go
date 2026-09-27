//go:build integration

// Integration test for the multi-value genre writeback round trip (HOLODEX-464)
// against the real ffmpeg + exiftool binaries. Run with:
//
//	go test -tags integration ./internal/writeback/
//
// A container genre tag is ONE comma-delimited string. exiftool keeps only the
// last of repeated -QuickTime:Genre= assignments, so writing tags as separate
// values cut every MP4 to its last tag — and the next rescan then dropped the
// file-sourced tags from the video. This pins "write 3, read 3" on both
// containers, and that a write replaces an existing genre rather than appending.
package writeback_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"holodex/internal/metadata"
	"holodex/internal/writeback"
)

func TestGenreRoundTrip_MultiValue(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe", "exiftool"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	want := []string{"Drama", "Crime", "Neo-Noir"}
	for _, tc := range []struct{ container, ext string }{
		{"Matroska", "mkv"},
		{"MP4", "mp4"},
	} {
		t.Run(tc.container, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "clip."+tc.ext)
			// Seed a prior genre so the test also proves the write replaces it.
			gen := exec.Command("ffmpeg", "-y", "-f", "lavfi",
				"-i", "testsrc=duration=1:size=64x64:rate=5",
				"-metadata", "genre=Horror, Thriller", "-loglevel", "error", path)
			if out, err := gen.CombinedOutput(); err != nil {
				t.Skipf("could not synthesize clip: %v: %s", err, out)
			}

			tag, ok := writeback.TagForField("genres", tc.container)
			if !ok {
				t.Fatalf("no genres write target for %s", tc.container)
			}
			if err := writeback.Write(context.Background(), path, tag, want); err != nil {
				t.Fatalf("write %s=%q: %v", tag, want, err)
			}

			ex, err := metadata.NewExtractor().Extract(context.Background(), path)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if !slices.Equal(ex.Tags, want) {
				t.Errorf("read back tags %q, want %q", ex.Tags, want)
			}
		})
	}
}
