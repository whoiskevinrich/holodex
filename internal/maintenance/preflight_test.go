package maintenance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// requireTools skips a test when a media tool isn't installed.
func requireTools(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			t.Skipf("%s not installed", n)
		}
	}
}

// realTools resolves to whatever is installed; the refusals below fire before any tool runs
// unless a test says otherwise.
var realTools = Tools{FFmpeg: "ffmpeg", FFprobe: "ffprobe", Exiftool: "exiftool", Mkvmerge: "mkvmerge"}

func TestCheckFlip_CheapRefusals(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	pf, err := CheckFlip(ctx, realTools, allEncoders, filepath.Join(dir, "gone.mkv"))
	if err != nil || pf.OK || pf.Reason != ReasonFileMissing {
		t.Errorf("missing file: %+v %v", pf, err)
	}

	avi := filepath.Join(dir, "a.avi")
	write(t, avi, "x")
	if pf, err := CheckFlip(ctx, realTools, allEncoders, avi); err != nil || pf.Reason != ReasonUnsupportedFile || pf.Detail != ".avi" {
		t.Errorf("avi: %+v %v", pf, err)
	}

	mp4 := filepath.Join(dir, "a.mp4")
	write(t, mp4, "x")
	noTool := Tools{FFmpeg: "holodex-no-such-ffmpeg", FFprobe: "ffprobe", Exiftool: "exiftool"}
	if pf, err := CheckFlip(ctx, noTool, allEncoders, mp4); err != nil || pf.Reason != ReasonToolMissing || pf.Detail != "holodex-no-such-ffmpeg" {
		t.Errorf("missing tool: %+v %v", pf, err)
	}
}

func TestCheckFlip_ReadOnlyDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions don't make a read-only mount on Windows")
	}
	requireTools(t, "ffmpeg", "ffprobe", "exiftool")
	dir := t.TempDir()
	mp4 := filepath.Join(dir, "a.mp4")
	write(t, mp4, "x")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	pf, err := CheckFlip(context.Background(), realTools, allEncoders, mp4)
	if err != nil || pf.OK || pf.Reason != ReasonReadOnly {
		t.Errorf("read-only dir: %+v %v", pf, err)
	}
}

func TestCheckFlip_NotEnoughSpaceReportsBothNumbers(t *testing.T) {
	requireTools(t, "ffmpeg", "ffprobe", "exiftool")
	orig := diskFree
	diskFree = func(string) (uint64, error) { return 10, nil }
	t.Cleanup(func() { diskFree = orig })

	mp4 := filepath.Join(t.TempDir(), "a.mp4")
	write(t, mp4, strings.Repeat("x", 1000))
	pf, err := CheckFlip(context.Background(), realTools, allEncoders, mp4)
	if err != nil || pf.OK || pf.Reason != ReasonNoSpace || pf.NeedBytes != 1500 || pf.FreeBytes != 10 {
		t.Errorf("no space: %+v %v", pf, err)
	}
}

// The writability probe leaves nothing behind and never touches an existing file.
func TestProbeWritable_LeavesNoTrace(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".holodex-probe"), "stale probe from an old build")
	if err := probeWritable(dir); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || read(filepath.Join(dir, ".holodex-probe")) != "stale probe from an old build" {
		t.Errorf("directory after probe: %v", entries)
	}
}
