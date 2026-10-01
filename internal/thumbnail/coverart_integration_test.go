//go:build integration

// Integration test for reading back an MKV cover that mkvpropedit relocated past
// the Clusters (HOLODEX-506). Run with:
//
//	go test -tags integration ./internal/thumbnail/
//
// mkvpropedit replaces a cover in place only when the new one fits; a larger
// provider poster moves the Attachments element to the end of the file. exiftool
// stops reading Matroska at the first Cluster unless -ee is passed, so the
// post-writeback re-extract found no cover and kept the old poster, and the
// scanner's HasCoverArt read the file as coverless.
package thumbnail

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"holodex/internal/metadata"
)

func TestExtractCoverArt_ReadsCoverRelocatedByMkvpropedit(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "exiftool", "ffprobe", "mkvpropedit", "mkvmerge"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	dir := t.TempDir()
	run := func(name string, args ...string) []byte {
		t.Helper()
		out, err := exec.Command(name, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v: %s", name, err, out)
		}
		return out
	}
	oldCover := filepath.Join(dir, "old.jpg")
	newCover := filepath.Join(dir, "new.jpg")
	clip := filepath.Join(dir, "clip.mkv")
	run("ffmpeg", "-y", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:size=32x32", "-frames:v", "1", oldCover)
	// Noise defeats JPEG compression, so the new cover can't fit the old slot.
	run("ffmpeg", "-y", "-loglevel", "error", "-f", "lavfi", "-i", "nullsrc=s=600x900,geq=random(1)*255:128:128",
		"-frames:v", "1", "-q:v", "2", newCover)
	// filename= is explicit: on Windows ffmpeg keeps -attach's full path as the name.
	run("ffmpeg", "-y", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=2:size=64x64:rate=5",
		"-attach", oldCover, "-metadata:s:t:0", "mimetype=image/jpeg", "-metadata:s:t:0", "filename=cover.jpg",
		"-c:v", "mjpeg", clip)

	var doc struct {
		Attachments []struct {
			Properties struct {
				UID uint64 `json:"uid"`
			} `json:"properties"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(run("mkvmerge", "-J", clip), &doc); err != nil || len(doc.Attachments) != 1 {
		t.Fatalf("fixture attachments = %+v, err %v", doc.Attachments, err)
	}
	run("mkvpropedit", clip,
		"--delete-attachment", "="+strconv.FormatUint(doc.Attachments[0].Properties.UID, 10),
		"--attachment-name", "cover.jpg", "--attachment-mime-type", "image/jpeg", "--add-attachment", newCover)

	ex, err := metadata.NewExtractor().Extract(context.Background(), clip)
	if err != nil {
		t.Fatalf("metadata extract: %v", err)
	}
	if !ex.HasCoverArt {
		t.Error("HasCoverArt = false for an MKV whose cover follows the Clusters")
	}

	// PosterWidth above the cover's width writes the poster tier byte-for-byte.
	m := coverArtManager(t, 64, 1000)
	posterDst := filepath.Join(dir, "poster.jpg")
	ok, err := m.extractCoverArt(context.Background(), clip, filepath.Join(dir, "thumb.jpg"), posterDst)
	if err != nil || !ok {
		t.Fatalf("extractCoverArt = %v, %v; want true, nil", ok, err)
	}
	got, err := os.ReadFile(posterDst)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(newCover)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("extracted poster is %d bytes, want the new cover's %d", len(got), len(want))
	}
}
