package montage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// frameRate keeps a hover preview small; motion still reads as motion at 15 fps.
const frameRate = 15

// generate cuts the montage for src into dst in one ffmpeg run: one input-side
// seek per clip, joined by the concat filter and scaled once (media-ingest.md,
// "Hover montages"). It writes dst+".tmp" and renames it into place.
func (m *Manager) generate(ctx context.Context, src string, starts []float64, dst string) error {
	tmp := dst + ".tmp"
	args := buildArgs(absPath(src), starts, m.cfg.Width, tmp)
	bin, full := wrapNice(m.cfg.Nice, m.cfg.FfmpegPath, args)
	out, err := exec.CommandContext(ctx, bin, full...).CombinedOutput()
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ffmpeg: %w: %s", err, lastLine(out))
	}
	if fi, statErr := os.Stat(tmp); statErr != nil || fi.Size() == 0 {
		_ = os.Remove(tmp)
		return fmt.Errorf("ffmpeg produced no output")
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename montage: %w", err)
	}
	return nil
}

// buildArgs is the whole ffmpeg argv for one montage. Each clip is its own input
// with the seek before -i (a fast keyframe seek), then every input is scaled to
// at most width, set to a common frame rate and SAR, and concatenated video-only.
// -an drops audio, +faststart puts the moov atom first so the first frame plays
// before the file has fully loaded.
func buildArgs(src string, starts []float64, width int, out string) []string {
	args := []string{"-nostdin", "-loglevel", "error"}
	for _, s := range starts {
		args = append(args, "-ss", strconv.FormatFloat(s, 'f', 3, 64),
			"-t", strconv.FormatFloat(clipSeconds, 'f', 3, 64), "-i", src)
	}
	var g strings.Builder
	for i := range starts {
		fmt.Fprintf(&g, "[%d:v:0]scale=w='min(%d,iw)':h=-2,setsar=1,fps=%d[v%d];", i, width, frameRate, i)
	}
	for i := range starts {
		fmt.Fprintf(&g, "[v%d]", i)
	}
	fmt.Fprintf(&g, "concat=n=%d:v=1:a=0[out]", len(starts))
	return append(args,
		"-filter_complex", g.String(),
		"-map", "[out]",
		"-an",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "30",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		"-f", "mp4", // the .tmp extension hides the container from ffmpeg
		"-y", out)
}

// absPath returns the absolute form of p, so a filename can never be parsed as
// an ffmpeg flag.
func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// wrapNice prepends nice (and ionice, when present), the same throttle stills
// use. Skipped on Windows, where an MSYS nice cannot exec a native ffmpeg.
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
		wrapped = append(wrapped, ionicePath, "-c", "3")
	}
	wrapped = append(wrapped, bin)
	return nicePath, append(wrapped, args...)
}

// lastLine returns the final line of ffmpeg stderr, truncated for logs and for
// the failure reason the owner sees.
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
