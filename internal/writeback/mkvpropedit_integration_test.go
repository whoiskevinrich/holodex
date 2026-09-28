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

func probeTitle(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format_tags=title",
		"-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	return strings.TrimSpace(string(out))
}
