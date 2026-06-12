package thumbnail

import (
	"strings"
	"testing"
)

func TestSeekSeconds(t *testing.T) {
	cases := []struct {
		duration, pct, want int
	}{
		{0, 10, 0},     // unknown duration -> first frame
		{100, 10, 10},  // normal case
		{100, 0, 0},    // no seek
		{5, 10, 0},     // very short clip rounds down to 0
		{100, 200, 99}, // clamp to duration-1
		{100, 100, 99}, // never seek past EOF
	}
	for _, c := range cases {
		if got := seekSeconds(c.duration, c.pct); got != c.want {
			t.Errorf("seekSeconds(%d,%d) = %d, want %d", c.duration, c.pct, got, c.want)
		}
	}
}

func TestFrameArgs(t *testing.T) {
	args := frameArgs("/m/clip.mkv", "/d/1.jpg.tmp", 6, 400)
	joined := strings.Join(args, " ")
	for _, want := range []string{"-ss 6", "-i /m/clip.mkv", "-frames:v 1", "scale=400:-1", "-f image2", "-y /d/1.jpg.tmp"} {
		if !strings.Contains(joined, want) {
			t.Errorf("frameArgs missing %q in %q", want, joined)
		}
	}
	// Input path is its own argv element (no shell), so it can't be mis-parsed.
	if indexOf(args, "/m/clip.mkv") < 0 {
		t.Errorf("source path not a discrete arg: %v", args)
	}
}

func TestWrapNiceDisabled(t *testing.T) {
	bin, args := wrapNice(false, "ffmpeg", []string{"-i", "x"})
	if bin != "ffmpeg" || len(args) != 2 {
		t.Errorf("disabled wrapNice altered command: bin=%s args=%v", bin, args)
	}
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}
