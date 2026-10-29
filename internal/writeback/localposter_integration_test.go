//go:build integration

// F85 integration (testing-strategy §37.3 I1/I2): a local poster is embedded from
// its slot, byte for byte, with real ffmpeg/mkvpropedit/exiftool, and the guarded
// image fetcher is never called. Run with:
//
//	go test -tags integration ./internal/writeback/
package writeback

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"holodex/internal/thumbnail"
)

func ffmpegFixture(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("ffmpeg", append([]string{"-y", "-loglevel", "error"}, args...)...).CombinedOutput(); err != nil {
		t.Skipf("could not synthesize fixture: %v: %s", err, out)
	}
}

// writeLocalPoster stages a local token for video 1 and writes it through
// WriteBatch, the production entry point, returning the slot's bytes.
func writeLocalPoster(t *testing.T, clip, tag, kind, slot string) []byte {
	t.Helper()
	dir := withLocalPosterDir(t)
	noFetch(t)
	img := filepath.Join(t.TempDir(), "slot.jpg")
	ffmpegFixture(t, "-f", "lavfi", "-i", "color=c=green:size=48x72", "-frames:v", "1", img)
	data, err := os.ReadFile(img)
	if err != nil {
		t.Fatal(err)
	}
	writeSlotFile(t, dir, 1, slot, data)
	batch := []FieldWrite{{TagName: tag, Values: []string{"local:" + kind + "#" + thumbnail.ContentHash(data)}, IsImage: true}}
	written, cleanup, err := StageLocalPosters(1, batch)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if written[0] != "local:"+kind+"#"+thumbnail.ContentHash(data) {
		t.Fatalf("ledger token = %q", written[0])
	}
	if err := WriteBatch(context.Background(), clip, batch); err != nil {
		t.Fatalf("writeback: %v", err)
	}
	return data
}

func exiftoolBinary(t *testing.T, tag, path string) []byte {
	t.Helper()
	out, err := exec.Command("exiftool", "-b", tag, path).Output()
	if err != nil {
		t.Fatalf("exiftool %s: %v", tag, err)
	}
	return out
}

// I1: MKV with cover art already embedded; local:frame replaces it with the slot.
func TestLocalPosterEmbedsInMKV(t *testing.T) {
	requireCoverTools(t, "exiftool")
	dir := t.TempDir()
	old := filepath.Join(dir, "old.jpg")
	clip := filepath.Join(dir, "clip.mkv")
	ffmpegFixture(t, "-f", "lavfi", "-i", "color=c=red:size=32x32", "-frames:v", "1", old)
	ffmpegFixture(t, "-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=5",
		"-attach", old, "-metadata:s:t:0", "mimetype=image/jpeg", "-metadata:s:t:0", "filename=cover.jpg",
		"-c:v", "mjpeg", clip)

	want := writeLocalPoster(t, clip, "cover.jpg", "frame", thumbnail.SlotFrame)
	if got := exiftoolBinary(t, "-AttachedFileData", clip); !bytes.Equal(got, want) {
		t.Fatalf("MKV cover is %d bytes, not the frame slot's %d", len(got), len(want))
	}
}

// I2: MP4, local:upload, read back through QuickTime:CoverArt.
func TestLocalPosterEmbedsInMP4(t *testing.T) {
	requireCoverTools(t, "exiftool")
	clip := filepath.Join(t.TempDir(), "clip.mp4")
	ffmpegFixture(t, "-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=5", "-pix_fmt", "yuv420p", clip)

	want := writeLocalPoster(t, clip, "QuickTime:CoverArt", "upload", thumbnail.SlotUpload)
	if got := exiftoolBinary(t, "-CoverArt", clip); !bytes.Equal(got, want) {
		t.Fatalf("MP4 cover is %d bytes, not the upload slot's %d", len(got), len(want))
	}
}
