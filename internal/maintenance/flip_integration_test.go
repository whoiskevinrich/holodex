//go:build integration

package maintenance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Real-file proof (testing-strategy §25.5). Fixtures are synthesized here; nothing is
// committed. The picture is testsrc2 with a box drawn in the top-left quadrant only, so the
// mirror oracle is independent of Verify.

func sh(t *testing.T, bin string, args ...string) string {
	t.Helper()
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", bin, args, err, out)
	}
	return string(out)
}

var localTools = Tools{FFmpeg: "ffmpeg", FFprobe: "ffprobe", Exiftool: "exiftool", Mkvmerge: "mkvmerge"}

// buildM1 makes the MP4 fixture: H.264 + AAC + mov_text subtitle, 2 chapters, cover art,
// iTunes title/genre and an XMP edition.
func buildM1(t *testing.T, dir string) string {
	meta := filepath.Join(dir, "meta.txt")
	os.WriteFile(meta, []byte(";FFMETADATA1\ntitle=Example\ngenre=Drama\n"+
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=1000\ntitle=One\n"+
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=1000\nEND=2000\ntitle=Two\n"), 0o644)
	srt := filepath.Join(dir, "s.srt")
	os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nHello\n"), 0o644)
	cover := filepath.Join(dir, "cover.png")
	sh(t, "ffmpeg", "-nostdin", "-y", "-loglevel", "error", "-f", "lavfi", "-i", "color=red:s=64x64", "-frames:v", "1", cover)
	out := filepath.Join(dir, "m1.mp4")
	sh(t, "ffmpeg", "-nostdin", "-y", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=s=320x240:r=25:d=2,drawbox=x=10:y=10:w=60:h=60:color=white:t=fill",
		"-f", "lavfi", "-i", "sine=d=2", "-i", srt, "-i", cover, "-i", meta,
		"-map", "0:v", "-map", "1:a", "-map", "2:s", "-map", "3:v",
		"-map_metadata", "4", "-map_chapters", "4",
		// -t, not -shortest: the one-frame cover would end the output at t=0.
		"-c:v:0", "libx264", "-pix_fmt:v:0", "yuv420p", "-c:a", "aac", "-c:s", "mov_text",
		"-c:v:1", "mjpeg", "-disposition:v:1", "attached_pic", "-t", "2", out)
	sh(t, "exiftool", "-overwrite_original", "-XMP-prism:Edition=Director's Cut", out)
	return out
}

// psnrMirror returns the PSNR between the original's frame 0, mirrored, and the
// candidate's frame 0 — high only if the candidate really is the mirror.
func psnrMirror(t *testing.T, orig, cand string) float64 {
	out, _ := exec.Command("ffmpeg", "-nostdin", "-i", orig, "-i", cand,
		"-lavfi", "[0:v:0]hflip[a];[a][1:v:0]psnr", "-frames:v", "1", "-f", "null", "-").CombinedOutput()
	m := regexp.MustCompile(`average:([0-9.]+|inf)`).FindStringSubmatch(string(out))
	if m == nil {
		t.Fatalf("no PSNR in ffmpeg output:\n%s", out)
	}
	if m[1] == "inf" {
		return 1000
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	return v
}

func TestFlip_RealFiles_MP4(t *testing.T) {
	requireTools(t, "ffmpeg", "ffprobe", "exiftool")
	ctx := context.Background()
	dir := t.TempDir()
	src := buildM1(t, dir)
	dst := src + ".holodex-new"

	p, err := RunProbe(ctx, "ffprobe", "mov", src)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	avail := map[string]bool{"libx264": true}
	if pf, err := CheckFlip(ctx, localTools, avail, src); err != nil || !pf.OK || pf.NeedBytes == 0 {
		t.Fatalf("preflight on a flippable file: %+v %v", pf, err)
	}
	if pf, _ := CheckFlip(ctx, localTools, map[string]bool{}, src); pf.OK || pf.Reason != ReasonEncoderMissing {
		t.Errorf("preflight without libx264: %+v", pf)
	}
	if err := Rebuild(ctx, localTools, FamilyMP4, OpHFlip, avail, src, dst, p); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	after, _ := os.ReadFile(src)
	if string(before) != string(after) {
		t.Fatal("Rebuild touched the original")
	}

	cp, err := RunProbe(ctx, "ffprobe", "mov", dst)
	if err != nil {
		t.Fatal(err)
	}
	origSnap, err := CollectSnapshot(ctx, localTools, FamilyMP4, src, p)
	if err != nil {
		t.Fatal(err)
	}
	candSnap, err := CollectSnapshot(ctx, localTools, FamilyMP4, dst, cp)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(origSnap, candSnap); err != nil {
		t.Fatalf("faithful flip refused: %v", err)
	}
	if !strings.Contains(candSnap.Extra["xmp:XMP-prism:Edition"], "Director") {
		t.Errorf("XMP edition lost: %v", candSnap.Extra)
	}
	if got := psnrMirror(t, src, dst); got < 40 {
		t.Errorf("mirror PSNR %.1f dB, want > 40: the picture is not the mirror of the original", got)
	}

	// Adversarial: a remux that drops metadata must be refused.
	bad := filepath.Join(dir, "bad.mp4")
	sh(t, "ffmpeg", "-nostdin", "-y", "-loglevel", "error", "-i", src, "-map", "0", "-c", "copy",
		"-map_metadata", "-1", bad)
	bp, _ := RunProbe(ctx, "ffprobe", "mov", bad)
	badSnap, err := CollectSnapshot(ctx, localTools, FamilyMP4, bad, bp)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(origSnap, badSnap); err == nil {
		t.Error("a metadata-stripped remux passed verification")
	}
}
