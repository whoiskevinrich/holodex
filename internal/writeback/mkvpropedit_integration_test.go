//go:build integration

// ADR-117 D5: once the runtime image bundles MKVToolNix, mkvpropedit is the
// production MKV path, so an ADR-110 Title delete (no Values) must remove the
// Segment Info title rather than index an empty slice. Run with:
//
//	go test -tags integration ./internal/writeback/   (or `make test-image`)
package writeback

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteMKVWithMkvpropedit_DeletesTitle(t *testing.T) {
	requireCoverTools(t, "mkvpropedit", "mkvextract", "mkvmerge")
	clip := filepath.Join(t.TempDir(), "clip.mkv")
	if out, err := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=64x64:rate=5", "-c:v", "mjpeg",
		"-metadata", "title=Old Title", clip).CombinedOutput(); err != nil {
		t.Skipf("could not synthesize clip: %v: %s", err, out)
	}
	if got := probeTitle(t, clip); got != "Old Title" {
		t.Fatalf("fixture title = %q, want %q", got, "Old Title")
	}

	if err := writeMKVWithMkvpropedit(context.Background(), clip, []FieldWrite{{TagName: "Title", Delete: true}}); err != nil {
		t.Fatalf("title delete: %v", err)
	}
	if got := probeTitle(t, clip); got != "" {
		t.Errorf("title after delete = %q, want none", got)
	}
}

// HOLODEX-488: every ffmpeg-muxed MKV carries per-track tags (DURATION,
// ENCODER under a TrackUID target). mkvpropedit's --tags global: refused the
// merged document wholesale ("No changes were made.", exit 0), so tag fields
// were silently dropped while the write reported success. The trigger needs a
// file with track Tags but no untargeted Tag (the merge then appends one after
// them); +bitexact drops ffmpeg's global ENCODER Tag to get exactly that.
func TestWriteMKVWithMkvpropedit_WritesTagsBesidePerTrackTags(t *testing.T) {
	requireCoverTools(t, "mkvpropedit", "mkvextract", "mkvmerge")
	clip := filepath.Join(t.TempDir(), "clip.mkv")
	if out, err := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=64x64:rate=5", "-c:v", "mjpeg",
		"-fflags", "+bitexact", clip).CombinedOutput(); err != nil {
		t.Skipf("could not synthesize clip: %v: %s", err, out)
	}
	if before := extractTags(t, clip); !strings.Contains(before, "<TrackUID>") || strings.Contains(before, "<Targets />") {
		t.Fatalf("fixture needs track tags and no untargeted Tag to reproduce the bug:\n%s", before)
	}

	err := writeMKVWithMkvpropedit(context.Background(), clip, []FieldWrite{
		{TagName: "PUBLISHER", Values: []string{"Example Studio"}},
		{TagName: "Genre", Values: []string{"Drama"}},
	})
	if err != nil {
		t.Fatalf("tag write: %v", err)
	}

	after := extractTags(t, clip)
	for _, want := range []string{
		"<String>Example Studio</String>",
		"<String>Drama</String>",
		"<TrackUID>", // the per-track Tags survive the all: replace
	} {
		if !strings.Contains(after, want) {
			t.Errorf("tags after write missing %q:\n%s", want, after)
		}
	}
}

// HOLODEX-518: written to stdout, mkvextract converts the tags document to the
// locale's charset, and under the runtime image's non-UTF-8 locale it silently
// stops at the first non-ASCII character (exit 0). The truncated document then
// failed to parse ("unexpected EOF"), blocking every writeback to the file.
// The non-ASCII file name covers the same locale on the argument side: under
// the C locale MKVToolNix could not open such a path at all.
func TestWriteMKVWithMkvpropedit_KeepsNonASCIITags(t *testing.T) {
	requireCoverTools(t, "mkvpropedit", "mkvextract", "mkvmerge")
	clip := filepath.Join(t.TempDir(), "café.mkv")
	if out, err := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=64x64:rate=5", "-c:v", "mjpeg",
		"-metadata", "comment=Café crème", clip).CombinedOutput(); err != nil {
		t.Skipf("could not synthesize clip: %v: %s", err, out)
	}

	if err := writeMKVWithMkvpropedit(context.Background(), clip, []FieldWrite{
		{TagName: "PUBLISHER", Values: []string{"Example Studio"}},
	}); err != nil {
		t.Fatalf("tag write: %v", err)
	}

	after := extractTags(t, clip)
	for _, want := range []string{"<String>Café crème</String>", "<String>Example Studio</String>"} {
		if !strings.Contains(after, want) {
			t.Errorf("tags after write missing %q:\n%s", want, after)
		}
	}
}

func extractTags(t *testing.T, path string) string {
	t.Helper()
	out, err := existingTagsXML(context.Background(), path)
	if err != nil {
		t.Fatalf("mkvextract tags: %v", err)
	}
	return out
}

func probeTitle(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format_tags=title",
		"-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	return strings.TrimSpace(string(out))
}
