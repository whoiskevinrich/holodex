//go:build integration

// Integration test that runs the real ffmpeg. Run with:
//
//	go test -tags integration ./internal/montage/
//
// It guards what a stubbed argv test can't see: a silent MP4 with the moov atom
// first, the right length, and no wider than the still.
package montage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestMontage_RealFFmpeg(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")
	// A 40 s 1280x720 clip with an audio track, so -an has something to drop.
	if out, err := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=30:duration=40",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=40",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-shortest", src).CombinedOutput(); err != nil {
		t.Fatalf("make source: %v: %s", err, out)
	}

	m := New(Config{Mode: ModeManual, Width: 400, Dir: dir}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	dst := Path(dir, 1)
	if err := m.generate(context.Background(), src, clipStarts(40), dst); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := os.Stat(dst + ".tmp"); !os.IsNotExist(err) {
		t.Error("the .tmp file was left behind")
	}

	out, err := exec.Command("ffprobe", "-v", "error", "-show_streams", "-show_format", "-of", "json", dst).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	var probe struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
		} `json:"streams"`
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatal(err)
	}
	if len(probe.Streams) != 1 || probe.Streams[0].CodecType != "video" || probe.Streams[0].CodecName != "h264" {
		t.Fatalf("streams = %+v, want exactly one h264 video stream and no audio", probe.Streams)
	}
	if w := probe.Streams[0].Width; w > 400 || w <= 0 {
		t.Errorf("width = %d, want at most 400", w)
	}
	if d, _ := strconv.ParseFloat(probe.Format.Duration, 64); d < 5.8 || d > 6.2 {
		t.Errorf("duration = %v, want 6 s ± 0.2", d)
	}

	// faststart: the moov atom comes before mdat.
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	moov, mdat := bytes.Index(b, []byte("moov")), bytes.Index(b, []byte("mdat"))
	if moov < 0 || mdat < 0 || moov > mdat {
		t.Errorf("moov at %d, mdat at %d; want moov first (+faststart)", moov, mdat)
	}
}
