//go:build integration

// F85 integration (testing-strategy §37.3 I3): with real exiftool, the post-write
// cover-art re-extract refreshes the file slot and the served poster keeps the
// owner's chosen image. Run with:
//
//	go test -tags integration ./internal/thumbnail/
package thumbnail

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"holodex/internal/fieldsource"
)

func TestChosenPosterSurvivesRealReExtract(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "exiftool"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("ffmpeg", append([]string{"-y", "-loglevel", "error"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg: %v: %s", err, out)
		}
	}
	cover := filepath.Join(dir, "cover.jpg")
	clip := filepath.Join(dir, "clip.mkv")
	run("-f", "lavfi", "-i", "color=c=red:size=32x48", "-frames:v", "1", cover)
	run("-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=5",
		"-attach", cover, "-metadata:s:t:0", "mimetype=image/jpeg", "-metadata:s:t:0", "filename=cover.jpg",
		"-c:v", "mjpeg", clip)

	fr := newFakeRepo()
	m := New(Config{Enabled: true, Width: 400, Dir: filepath.Join(dir, "thumbs")},
		slog.New(slog.NewTextHandler(io.Discard, nil)), fr)
	ctx := context.Background()
	if ok, err := m.ExtractEmbedded(ctx, 1, clip); err != nil || !ok {
		t.Fatalf("index-time extract = (%v, %v)", ok, err)
	}
	upload := []byte("\xff\xd8\xff owner upload")
	if err := m.SetOwnerImage(ctx, 1, fieldsource.LocalUpload, upload, nil); err != nil {
		t.Fatal(err)
	}
	// The post-write hook, after any writeback of the file.
	if ok, err := m.ExtractEmbedded(ctx, 1, clip); err != nil || !ok {
		t.Fatalf("post-write extract = (%v, %v)", ok, err)
	}
	if got := readString(t, m.posterPath(1)); got != string(upload) {
		t.Fatal("the re-extract displaced the chosen upload")
	}
	art, _ := os.ReadFile(cover)
	if got := readString(t, m.slotPath(1, SlotFile)); got != string(art) {
		t.Fatalf("file slot holds %d bytes, not the file's cover art (%d)", len(got), len(art))
	}
}
