package thumbnail

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"holodex/internal/repo"
)

// generateFrame extracts a single frame at SeekPercent of the video's duration,
// scaled to Width, as a JPEG. It writes to a temp file and renames into place so
// the serving handler never sees a partial image.
func (m *Manager) generateFrame(ctx context.Context, c repo.ThumbnailCandidate, outPath string) error {
	src := absPath(c.FilePath)
	tmp := outPath + ".tmp"
	args := frameArgs(src, tmp, seekSeconds(c.DurationSec, m.cfg.SeekPercent), m.cfg.Width)
	bin, full := wrapNice(m.cfg.Nice, m.cfg.FfmpegPath, args)

	out, err := exec.CommandContext(ctx, bin, full...).CombinedOutput()
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ffmpeg: %w: %s", err, lastLine(out))
	}
	// Guard against ffmpeg exiting 0 while writing nothing usable.
	if fi, statErr := os.Stat(tmp); statErr != nil || fi.Size() == 0 {
		_ = os.Remove(tmp)
		return fmt.Errorf("ffmpeg produced no output")
	}
	if err := os.Rename(tmp, outPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename thumbnail: %w", err)
	}
	return nil
}

// absPath returns the absolute form of p, falling back to p on error. An
// absolute path always begins with a separator (or drive), so a filename can
// never be mis-parsed as a CLI flag (argv smuggling) by ffmpeg/exiftool.
func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// seekSeconds converts a seek percentage into an absolute offset, clamped to the
// valid range. Short or zero-duration files seek to the first frame.
func seekSeconds(durationSec, pct int) int {
	if durationSec <= 0 || pct <= 0 {
		return 0
	}
	s := durationSec * pct / 100
	if s >= durationSec {
		s = durationSec - 1
	}
	if s < 0 {
		s = 0
	}
	return s
}

// frameArgs builds the ffmpeg argv. Input-seeking (-ss before -i) is fast; each
// element is a separate arg (no shell), so paths can never be mis-parsed. The
// muxer is set explicitly (-f image2) because the destination is a temp file
// whose ".tmp" extension ffmpeg cannot map to an output format on its own.
func frameArgs(src, dst string, seekSec, width int) []string {
	return []string{
		"-nostdin",
		"-loglevel", "error",
		"-ss", strconv.Itoa(seekSec),
		"-i", src,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:-1", width),
		"-q:v", "3",
		"-f", "image2",
		"-y", dst,
	}
}

// wrapNice prepends nice (and ionice, when present) so ffmpeg yields CPU and I/O
// to other host activity. nice/ionice are Unix tools; the deployment target is
// Linux (ADR-007). The Windows guard is load-bearing, not just an optimization:
// a Git-Bash/MSYS `nice` is found by LookPath but cannot exec a native Windows
// ffmpeg (it fails with exit 127), so we must skip wrapping entirely there and
// run ffmpeg unthrottled.
func wrapNice(enabled bool, bin string, args []string) (string, []string) {
	if !enabled || runtime.GOOS == "windows" {
		return bin, args
	}
	nicePath, err := exec.LookPath("nice")
	if err != nil {
		return bin, args
	}
	wrapped := []string{"-n", "19"}
	if ionicePath, ierr := exec.LookPath("ionice"); ierr == nil {
		wrapped = append(wrapped, ionicePath, "-c", "3") // idle I/O class
	}
	wrapped = append(wrapped, bin)
	wrapped = append(wrapped, args...)
	return nicePath, wrapped
}

// lastLine returns the final (most relevant) line of ffmpeg stderr, truncated, so
// error logs stay readable.
func lastLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[i+1:])
	}
	if len(s) > 200 {
		s = s[len(s)-200:]
	}
	return s
}
