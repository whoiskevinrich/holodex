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

func TestScaleArgs(t *testing.T) {
	// Tier 2: seek + source file ahead of the shared scale/quality/muxer tail.
	frame := scaleArgs([]string{"-ss", "6", "-i", "/m/clip.mkv", "-frames:v", "1"}, 400, "/d/1.jpg.tmp")
	joined := strings.Join(frame, " ")
	for _, want := range []string{"-ss 6", "-i /m/clip.mkv", "-frames:v 1", "scale=400:-1", "-f image2", "-y /d/1.jpg.tmp"} {
		if !strings.Contains(joined, want) {
			t.Errorf("scaleArgs (frame) missing %q in %q", want, joined)
		}
	}

	// Tier 1: a bare stdin pipe ahead of the same tail — embedded cover art.
	art := scaleArgs([]string{"-i", "pipe:0"}, 400, "/d/2.jpg.tmp")
	joined = strings.Join(art, " ")
	for _, want := range []string{"-i pipe:0", "scale=400:-1", "-f image2", "-y /d/2.jpg.tmp"} {
		if !strings.Contains(joined, want) {
			t.Errorf("scaleArgs (cover art) missing %q in %q", want, joined)
		}
	}
}

func TestWrapNiceDisabled(t *testing.T) {
	bin, args := wrapNice(false, "ffmpeg", []string{"-i", "x"})
	if bin != "ffmpeg" || len(args) != 2 {
		t.Errorf("disabled wrapNice altered command: bin=%s args=%v", bin, args)
	}
}
