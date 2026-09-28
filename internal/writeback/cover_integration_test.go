//go:build integration

// Integration test for the ffmpeg cover writeback (HOLODEX-484) against the
// real ffmpeg + ffprobe binaries. Run with:
//
//	go test -tags integration ./internal/writeback/
//
// A Matroska file whose cover is cover.webp, beside a subtitle font, used to
// fail the remux with "Attachment stream N has no mimetype tag": the new
// cover's labels were addressed at t:0, which was the copied cover.webp. This
// pins that the write succeeds, leaves exactly one cover (the new cover.jpg,
// labelled image/jpeg) and keeps the font untouched.
package writeback

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWriteMKVWithFFmpeg_ReplacesOtherFormatCoverKeepsFont(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("ffmpeg", append([]string{"-y", "-loglevel", "error"}, args...)...).CombinedOutput(); err != nil {
			t.Skipf("could not synthesize fixture: %v: %s", err, out)
		}
	}
	oldCover := filepath.Join(dir, "cover.webp")
	newCover := filepath.Join(dir, "new.jpg")
	font := filepath.Join(dir, "font.ttf")
	clip := filepath.Join(dir, "clip.mkv")
	run("-f", "lavfi", "-i", "color=c=red:size=32x32", "-frames:v", "1", oldCover)
	run("-f", "lavfi", "-i", "color=c=blue:size=32x32", "-frames:v", "1", newCover)
	if err := os.WriteFile(font, []byte("not really a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=5",
		"-attach", font, "-metadata:s:t:0", "mimetype=application/x-truetype-font", "-metadata:s:t:0", "filename=font.ttf",
		"-attach", oldCover, "-metadata:s:t:1", "mimetype=image/webp", "-metadata:s:t:1", "filename=cover.webp",
		"-c:v", "mjpeg", clip)

	jpeg, err := os.ReadFile(newCover)
	if err != nil {
		t.Fatal(err)
	}
	withImageFetcher(t, func(context.Context, string) ([]byte, error) { return jpeg, nil })

	err = writeMKVWithFFmpeg(context.Background(), clip, []FieldWrite{
		{TagName: "cover.jpg", Values: []string{"https://cdn.example.com/poster.jpg"}, IsImage: true},
	})
	if err != nil {
		t.Fatalf("cover writeback: %v", err)
	}

	out, err := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "stream_tags=filename,mimetype", "-of", "json", clip).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	var doc struct {
		Streams []struct {
			Tags struct{ Filename, Mimetype string } `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range doc.Streams {
		if s.Tags.Filename != "" {
			got[s.Tags.Filename] = s.Tags.Mimetype
		}
	}
	want := map[string]string{"cover.jpg": "image/jpeg", "font.ttf": "application/x-truetype-font"}
	if len(got) != len(want) || got["cover.jpg"] != want["cover.jpg"] || got["font.ttf"] != want["font.ttf"] {
		t.Errorf("attachments after write = %v, want %v", got, want)
	}
}
