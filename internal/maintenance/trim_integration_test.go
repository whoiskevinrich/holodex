//go:build integration

package maintenance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Real-file proof for a trim (testing-strategy §41.4). Fixtures are synthesized here; nothing is
// committed. Every frame carries its number as ten vertical bars, so which frame an output holds is
// read from its pixels, sharing no code with the cut check.

const (
	bcW, bcH, bcBars = 320, 240, 10
)

var barcode = fmt.Sprintf(`geq=lum='255*mod(floor(N/pow(2,floor(X/%d))),2)':cb=128:cr=128`, bcW/bcBars)

func ff(t *testing.T, args ...string) {
	t.Helper()
	sh(t, "ffmpeg", append([]string{"-nostdin", "-y", "-loglevel", "error"}, args...)...)
}

// readBars decodes a file's frames at the display size and reads each one's number.
func readBars(t *testing.T, path string) []int {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-i", path, "-map", "0:V:0", "-an",
		"-fps_mode", "passthrough", "-f", "rawvideo", "-pix_fmt", "gray", "-").Output()
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	var nums []int
	for off := 0; off+bcW*bcH <= len(out); off += bcW * bcH {
		row := out[off+bcH/2*bcW : off+(bcH/2+1)*bcW]
		n := 0
		for b := 0; b < bcBars; b++ {
			if row[b*(bcW/bcBars)+bcW/(bcBars*2)] > 128 {
				n |= 1 << b
			}
		}
		nums = append(nums, n)
	}
	return nums
}

// frameTimesOf lists a file's video frame times from its start.
func frameTimesOf(t *testing.T, path string) []float64 {
	t.Helper()
	out := sh(t, "ffprobe", "-v", "error", "-select_streams", "V:0", "-show_entries", "frame=pts_time", "-of", "csv=p=0", path)
	var ts []float64
	for _, f := range strings.Fields(out) {
		v, err := strconv.ParseFloat(strings.TrimRight(f, ","), 64)
		if err != nil {
			t.Fatalf("time %q: %v", f, err)
		}
		ts = append(ts, v)
	}
	return ts
}

// ffEnc runs ffmpeg with the fixtures' x264 settings between the inputs and the rest of the output.
func ffEnc(t *testing.T, pre []string, post ...string) {
	t.Helper()
	args := append(pre, "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p")
	ff(t, append(args, post...)...)
}

func chapterMeta(t *testing.T, dir string) string {
	t.Helper()
	meta := filepath.Join(dir, "meta.txt")
	content := ";FFMETADATA1\ntitle=Example\n" +
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=2000\ntitle=One\n" +
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=2000\nEND=4000\ntitle=Two\n" +
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=4000\nEND=6000\ntitle=Three\n"
	if err := os.WriteFile(meta, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return meta
}

type trimFixture struct {
	name  string
	build func(t *testing.T, dir string) string
}

func vfrExpr() string {
	return "if(lt(mod(N,20),10),mod(N,20)*0.04,0.4+(mod(N,20)-10)*0.01)+floor(N/20)*0.5"
}

var trimFixtures = []trimFixture{
	{"T1 mp4 25fps, audio, subtitle, chapters, cover, XMP", func(t *testing.T, dir string) string {
		srt := filepath.Join(dir, "s.srt")
		var subs strings.Builder
		for i := 0; i < 6; i++ {
			fmt.Fprintf(&subs, "%d\n00:00:0%d,100 --> 00:00:0%d,900\nLine %d\n\n", i+1, i, i, i)
		}
		os.WriteFile(srt, []byte(subs.String()), 0o644)
		cover := filepath.Join(dir, "cover.png")
		ff(t, "-f", "lavfi", "-i", "color=red:s=64x64", "-frames:v", "1", cover)
		out := filepath.Join(dir, "t1.mp4")
		ff(t, "-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=25:d=6,%s", bcW, bcH, barcode),
			"-f", "lavfi", "-i", "sine=d=6", "-i", srt, "-i", cover, "-i", chapterMeta(t, dir),
			"-map", "0:v", "-map", "1:a", "-map", "2:s", "-map", "3:v", "-map_metadata", "4", "-map_chapters", "4",
			"-c:v:0", "libx264", "-preset", "veryfast", "-pix_fmt:v:0", "yuv420p", "-g:v:0", "25", "-bf:v:0", "2",
			"-c:a", "aac", "-c:s", "mov_text", "-c:v:1", "mjpeg", "-disposition:v:1", "attached_pic", "-t", "6", out)
		sh(t, "exiftool", "-overwrite_original", "-XMP-prism:Edition=Director's Cut", out)
		return out
	}},
	{"T2 mp4 23.976", func(t *testing.T, dir string) string {
		out := filepath.Join(dir, "t2.mp4")
		ffEnc(t, []string{"-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=24000/1001:d=6,%s", bcW, bcH, barcode),
			"-f", "lavfi", "-i", "sine=d=6"}, "-c:a", "aac", "-shortest", out)
		return out
	}},
	{"T3 mp4 edit list", func(t *testing.T, dir string) string {
		base := filepath.Join(dir, "base.mp4")
		ffEnc(t, []string{"-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=25:d=8,%s", bcW, bcH, barcode),
			"-f", "lavfi", "-i", "sine=d=8"}, "-bf", "2", "-g", "25", "-c:a", "aac", "-shortest", base)
		out := filepath.Join(dir, "t3.mp4")
		ff(t, "-ss", "1.4", "-i", base, "-c", "copy", "-use_editlist", "1", out)
		return out
	}},
	{"T4 mp4 variable rate", func(t *testing.T, dir string) string {
		out := filepath.Join(dir, "t4.mp4")
		ffEnc(t, []string{"-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=25:d=6", bcW, bcH),
			"-vf", barcode + ",settb=1/1000,setpts='(" + vfrExpr() + ")/TB'", "-fps_mode", "passthrough", "-enc_time_base", "1:1000"}, out)
		return out
	}},
	{"T4 mkv variable rate", func(t *testing.T, dir string) string {
		out := filepath.Join(dir, "t4.mkv")
		ffEnc(t, []string{"-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=25:d=6", bcW, bcH),
			"-vf", barcode + ",settb=1/1000,setpts='(" + vfrExpr() + ")/TB'", "-fps_mode", "passthrough", "-enc_time_base", "1:1000"}, out)
		return out
	}},
	{"T5 mp4 one keyframe, audio first", func(t *testing.T, dir string) string {
		out := filepath.Join(dir, "t5.mp4")
		ff(t, "-f", "lavfi", "-i", "sine=d=6", "-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=25:d=6,%s", bcW, bcH, barcode),
			"-map", "0:a", "-map", "1:v", "-c:a", "aac", "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-g", "9999", "-shortest", out)
		return out
	}},
	{"K5 mkv 23.976, audio, subtitle, tags", func(t *testing.T, dir string) string {
		srt := filepath.Join(dir, "s.srt")
		os.WriteFile(srt, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n\n2\n00:00:03,000 --> 00:00:04,000\nThere\n"), 0o644)
		out := filepath.Join(dir, "k5.mkv")
		ff(t, "-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=24000/1001:d=6,%s", bcW, bcH, barcode),
			"-f", "lavfi", "-i", "sine=d=6", "-i", srt, "-i", chapterMeta(t, dir),
			"-map", "0:v", "-map", "1:a", "-map", "2:s", "-map_metadata", "3", "-map_chapters", "3",
			"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-c:s", "srt", "-t", "6", out)
		// Through mkvmerge, as a real library's files are, so statistics tags exist.
		remuxed := filepath.Join(dir, "k5m.mkv")
		sh(t, "mkvmerge", "-q", "-o", remuxed, out)
		return remuxed
	}},
	{"K6 mkv HEVC 10-bit HDR tags", func(t *testing.T, dir string) string {
		out := filepath.Join(dir, "k6.mkv")
		ff(t, "-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=25:d=6,%s", bcW, bcH, barcode),
			"-f", "lavfi", "-i", "sine=d=6", "-f", "lavfi", "-i", "sine=f=880:d=6",
			"-map", "0:v", "-map", "1:a", "-map", "2:a",
			"-c:v", "libx265", "-preset", "ultrafast", "-pix_fmt", "yuv420p10le",
			"-color_primaries", "bt2020", "-color_trc", "smpte2084", "-colorspace", "bt2020nc",
			"-x265-params", "log-level=none:hdr10=1:max-cll=1000,400:master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(10000000,1)",
			"-c:a", "aac", "-metadata:s:a:0", "language=eng", "-metadata:s:a:1", "language=jpn", "-shortest", out)
		return out
	}},
	{"K7 mkv AC3 and AAC", func(t *testing.T, dir string) string {
		out := filepath.Join(dir, "k7.mkv")
		ff(t, "-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=25:d=6,%s", bcW, bcH, barcode),
			"-f", "lavfi", "-i", "sine=d=6", "-f", "lavfi", "-i", "sine=f=660:d=6",
			"-map", "0:v", "-map", "1:a", "-map", "2:a", "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
			"-c:a:0", "ac3", "-c:a:1", "aac", "-shortest", out)
		return out
	}},
	{"K8 mkv video only", func(t *testing.T, dir string) string {
		out := filepath.Join(dir, "k8.mkv")
		ffEnc(t, []string{"-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=25:d=6,%s", bcW, bcH, barcode)}, out)
		return out
	}},
	{"K9 mkv FLAC beside AC3 and a subtitle", buildFLACMKV(true, 24)},
	{"K9 mkv video and FLAC only", buildFLACMKV(false, 24)},
}

// buildFLACMKV makes a Matroska file with a FLAC track of pink noise (never periodic, so its samples
// can be found in the original by content alone) at bits per sample (24 or 32). With others, an AC3
// track comes before it and a subtitle after, so the cut pass runs; without, nothing is left to cut.
// Through mkvmerge, as a real library's files are, so the FLAC track has tags of its own.
func buildFLACMKV(others bool, bits int) func(t *testing.T, dir string) string {
	return func(t *testing.T, dir string) string {
		raw := filepath.Join(dir, "k9raw.mkv")
		args := []string{"-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=25:d=6,%s", bcW, bcH, barcode),
			"-f", "lavfi", "-i", "anoisesrc=d=6:c=pink:r=48000:a=0.5:seed=7"}
		maps := []string{"-map", "0:v"}
		codecs := []string{"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p"}
		flac := "0"
		if others {
			srt := filepath.Join(dir, "s.srt")
			os.WriteFile(srt, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n\n2\n00:00:03,000 --> 00:00:04,000\nThere\n"), 0o644)
			args = append(args, "-f", "lavfi", "-i", "sine=f=660:d=6", "-i", srt)
			maps = append(maps, "-map", "2:a", "-map", "1:a", "-map", "3:s")
			codecs = append(codecs, "-c:a:0", "ac3", "-c:s", "srt")
			flac = "1"
		} else {
			maps = append(maps, "-map", "1:a")
		}
		codecs = append(codecs, "-c:a:"+flac, "flac", "-sample_fmt:a:"+flac, "s32",
			"-bits_per_raw_sample:a:"+flac, strconv.Itoa(bits), "-strict:a:"+flac, "experimental",
			"-metadata:s:a:"+flac, "title=Commentary", "-metadata:s:a:"+flac, "language=jpn", "-t", "6", raw)
		ff(t, append(append(args, maps...), codecs...)...)
		out := filepath.Join(dir, "k9.mkv")
		sh(t, "mkvmerge", "-q", "-o", out, raw)
		return out
	}
}

func detectedEncoders(t *testing.T) map[string]bool {
	t.Helper()
	enc, err := DetectEncoders(context.Background(), "ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

func cutWorker(t *testing.T) *Worker {
	t.Helper()
	return &Worker{tools: localTools, encoders: detectedEncoders(t), log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func runCut(t *testing.T, w *Worker, src, name string, params TrimParams, edition string) (string, []SkippedCheck, error) {
	t.Helper()
	family, err := familyOf(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(filepath.Dir(src), name+filepath.Ext(src))
	skipped, err := w.trim(context.Background(), family, src, dst, &params, edition)
	return dst, skipped, err
}

// TestTrim_RealFiles: for every fixture, a cut at both ends, and at each end alone, starts and ends
// on exactly the chosen frames, with the gate's metadata rules holding.
func TestTrim_RealFiles(t *testing.T) {
	w := cutWorker(t)
	for _, fx := range trimFixtures {
		t.Run(fx.name, func(t *testing.T) {
			dir := t.TempDir()
			src := fx.build(t, dir)
			ts := frameTimesOf(t, src)
			bars := readBars(t, src)
			if len(ts) != len(bars) || len(ts) < 60 {
				t.Fatalf("%d times, %d frames", len(ts), len(bars))
			}
			if strings.Contains(fx.name, "variable rate") {
				tight := false
				for i := 1; i < len(ts); i++ {
					if ts[i]-ts[i-1] < 0.015 {
						tight = true
					}
				}
				if !tight {
					t.Fatal("the fixture's 10 ms gaps rounded away")
				}
			}
			last := len(ts) - 1
			for _, c := range []struct {
				name    string
				in, out int
			}{{"both", 15, last - 16}, {"start", 15, last}, {"end", 0, last - 16}, {"on a tight frame", 16, 56}} {
				t.Run(c.name, func(t *testing.T) {
					dst, _, err := runCut(t, w, src, "cut", TrimParams{InTime: ts[c.in], OutTime: ts[c.out], Mode: TrimReplace}, "")
					if err != nil {
						t.Fatalf("trim %d-%d: %v", c.in, c.out, err)
					}
					defer os.Remove(dst)
					got := readBars(t, dst)
					if len(got) == 0 || got[0] != bars[c.in] || got[len(got)-1] != bars[c.out] {
						t.Fatalf("frames %d..%d (%d frames), want %d..%d", got[0], got[len(got)-1], len(got), bars[c.in], bars[c.out])
					}
					if len(got) != c.out-c.in+1 {
						t.Errorf("%d frames, want %d", len(got), c.out-c.in+1)
					}
					// The barcode and the cut check can't see this: frames rounded onto one
					// timestamp hold the right pictures and play with a judder.
					times := frameTimesOf(t, dst)
					for i := 1; i < len(times); i++ {
						if times[i] <= times[i-1] {
							t.Fatalf("frame %d's time %v doesn't follow %v", i, times[i], times[i-1])
						}
					}
				})
			}
		})
	}
}

func chapterTitles(t *testing.T, path string) []string {
	t.Helper()
	p, err := RunProbe(context.Background(), "ffprobe", Demuxer(mustFamily(t, path)), path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range p.Chapters {
		s, _ := strconv.ParseFloat(c.StartTime, 64)
		out = append(out, fmt.Sprintf("%s@%.1f", c.Tags["title"], s))
	}
	return out
}

func mustFamily(t *testing.T, path string) string {
	t.Helper()
	f, err := familyOf(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// Chapters oracle: computed by hand here. A cut 1.0–5.0 s keeps "One" clamped, "Two" shifted, and
// "Three" clamped at the end.
func TestTrim_RealFiles_Chapters(t *testing.T) {
	w := cutWorker(t)
	for _, name := range []string{"T1", "K5"} {
		for _, fx := range trimFixtures {
			if !strings.HasPrefix(fx.name, name) {
				continue
			}
			t.Run(name, func(t *testing.T) {
				src := fx.build(t, t.TempDir())
				ts := frameTimesOf(t, src)
				in := FrameAt(ts, 1.0)
				out := FrameAt(ts, 5.0)
				dst, _, err := runCut(t, w, src, "cut", TrimParams{InTime: ts[in], OutTime: ts[out], Mode: TrimReplace}, "")
				if err != nil {
					t.Fatal(err)
				}
				got := strings.Join(chapterTitles(t, dst), ",")
				if got != "One@0.0,Two@1.0,Three@3.0" {
					t.Errorf("chapters = %s", got)
				}
			})
		}
	}
}

// FrameAt returns the index of the frame nearest t.
func FrameAt(ts []float64, t float64) int { return frameWindow(ts).nearest(t) }

// Stream oracle, independent of Verify: the cover art and the XMP survive, and the audio is copied.
func TestTrim_RealFiles_KeepsWhatItMust(t *testing.T) {
	w := cutWorker(t)
	src := trimFixtures[0].build(t, t.TempDir())
	ts := frameTimesOf(t, src)
	dst, _, err := runCut(t, w, src, "cut", TrimParams{InTime: ts[25], OutTime: ts[100], Mode: TrimReplace}, "")
	if err != nil {
		t.Fatal(err)
	}
	// The chapter list is meant to change, and the chapters oracle checks how.
	for _, line := range strings.Split(exifTagDiff(t, src, dst), "\n") {
		if line != "" && !strings.Contains(line, "ChapterList") {
			t.Errorf("tag changed: %s", line)
		}
	}
	ctx := context.Background()
	op, _ := RunProbe(ctx, "ffprobe", "mov", src)
	cp, _ := RunProbe(ctx, "ffprobe", "mov", dst)
	osnap, _ := CollectSnapshot(ctx, localTools, FamilyMP4, src, op)
	csnap, _ := CollectSnapshot(ctx, localTools, FamilyMP4, dst, cp)
	cover := func(p Probe) int {
		for _, s := range p.Streams {
			if s.Disposition["attached_pic"] != 0 {
				return s.Index
			}
		}
		t.Fatal("no cover art")
		return -1
	}
	if osnap.StreamHashes[cover(op)] != csnap.StreamHashes[cover(cp)] {
		t.Error("cover art changed")
	}
}

// Save as edition's file half: the new edition is written, and nothing else differs.
func TestTrim_RealFiles_EditionTag(t *testing.T) {
	w := cutWorker(t)
	// T1 already has XMP, T2 has none, and K5 is Matroska. A file with no XMP once failed every edition.
	for _, idx := range []int{0, 1, 6} {
		fx := trimFixtures[idx]
		t.Run(fx.name, func(t *testing.T) {
			src := fx.build(t, t.TempDir())
			ts := frameTimesOf(t, src)
			dst, _, err := runCut(t, w, src, "cut", TrimParams{InTime: ts[25], OutTime: ts[100], Mode: TrimEdition}, "Trimmed")
			if err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("exiftool", "-j", "-Edition", dst).Output()
			if err != nil {
				t.Fatal(err)
			}
			var rows []map[string]any
			if err := json.Unmarshal(out, &rows); err != nil || len(rows) != 1 || rows[0]["Edition"] != "Trimmed" {
				t.Errorf("edition read back: %s (%v)", out, err)
			}
		})
	}
}

// grabJPEG saves one source frame as a JPEG, as a capture would be. shift brightens it, which is
// what a browser's HDR tone mapping does to the pixels but not the shape.
func grabJPEG(t *testing.T, src string, seek float64, dir, name string, shift float64) string {
	t.Helper()
	out := filepath.Join(dir, name)
	ff(t, "-ss", fmtTime(seek), "-i", src, "-map", "0:V:0", "-frames:v", "1", "-vf", fmt.Sprintf("eq=brightness=%.3f", shift), "-q:v", "3", out)
	return out
}

// midBefore is the seek that lands on frame i.
func midBefore(ts []float64, i int) float64 {
	if i == 0 {
		return 0
	}
	return (ts[i-1] + ts[i]) / 2
}

func withCaptures(t *testing.T, w *Worker, src string, ts []float64, in, out int, shift float64) TrimParams {
	t.Helper()
	root := t.TempDir()
	w.trimHooks.CaptureRoot = root
	dir := filepath.Join(root, "job")
	os.MkdirAll(dir, 0o700)
	grabJPEG(t, src, midBefore(ts, in), dir, "in.jpg", shift)
	grabJPEG(t, src, midBefore(ts, out), dir, "out.jpg", shift)
	return TrimParams{Mode: TrimReplace, Captures: "job"}
}

// The screen check: a brightness-shifted capture of the right frame passes, one frame off corrects
// the cut to what the owner saw, and another scene fails with nothing landing.
func TestTrim_RealFiles_ScreenCheck(t *testing.T) {
	w := cutWorker(t)
	dir := t.TempDir()
	src := trimFixtures[1].build(t, dir)
	ts := frameTimesOf(t, src)
	bars := readBars(t, src)

	t.Run("the right frame at another brightness", func(t *testing.T) {
		p := withCaptures(t, w, src, ts, 20, 90, 0.08)
		p.InTime, p.OutTime = ts[20], ts[90]
		dst, skipped, err := runCut(t, w, src, "ok", p, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(skipped) != 0 {
			t.Errorf("skipped: %+v", skipped)
		}
		if got := readBars(t, dst); got[0] != bars[20] || got[len(got)-1] != bars[90] {
			t.Errorf("frames %d..%d", got[0], got[len(got)-1])
		}
	})
	t.Run("the owner saw the next frame", func(t *testing.T) {
		p := withCaptures(t, w, src, ts, 21, 90, 0)
		p.InTime, p.OutTime = ts[20], ts[90]
		dst, _, err := runCut(t, w, src, "next", p, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := readBars(t, dst); got[0] != bars[21] {
			t.Errorf("first frame %d, want the one on screen, %d", got[0], bars[21])
		}
	})
	t.Run("another scene", func(t *testing.T) {
		other := filepath.Join(dir, "other.mp4")
		ff(t, "-f", "lavfi", "-i", "testsrc2=s=320x240:r=25:d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", other)
		root := t.TempDir()
		w.trimHooks.CaptureRoot = root
		jd := filepath.Join(root, "job")
		os.MkdirAll(jd, 0o700)
		grabJPEG(t, other, 0.5, jd, "in.jpg", 0)
		grabJPEG(t, other, 0.5, jd, "out.jpg", 0)
		dst, _, err := runCut(t, w, src, "scene", TrimParams{InTime: ts[20], OutTime: ts[90], Mode: TrimReplace, Captures: "job"}, "")
		re, ok := err.(*RuleError)
		if !ok || re.Rule != RuleScreenCheck {
			t.Fatalf("err = %v", err)
		}
		if _, statErr := os.Stat(dst); statErr == nil {
			t.Error("a failed trim left its output")
		}
	})
}

// C1–C3: frames that can't be told apart skip the screen check and say why; C4 runs it.
func TestTrim_RealFiles_SkipReasons(t *testing.T) {
	w := cutWorker(t)
	cases := []struct {
		name, source, want string
	}{
		{"still", "testsrc2=s=320x240:r=25:d=1,loop=loop=-1:size=1:start=0,trim=duration=4,setpts=N/25/TB", SkipStill},
		{"black", "color=black:s=320x240:r=25:d=4", SkipBlack},
		{"motion", fmt.Sprintf("color=black:s=%dx%d:r=25:d=4,%s", bcW, bcH, barcode), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, c.name+".mp4")
			ff(t, "-f", "lavfi", "-i", c.source, "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", src)
			ts := frameTimesOf(t, src)
			p := withCaptures(t, w, src, ts, 20, 60, 0)
			p.InTime, p.OutTime = ts[20], ts[60]
			_, skipped, err := runCut(t, w, src, "out", p, "")
			if err != nil {
				t.Fatal(err)
			}
			if c.want == "" {
				if len(skipped) != 0 {
					t.Errorf("skipped: %+v", skipped)
				}
				return
			}
			if len(skipped) != 2 || skipped[0].Reason != c.want || skipped[1].Reason != c.want {
				t.Errorf("skipped: %+v, want %s at both ends", skipped, c.want)
			}
		})
	}
}

// K6: HDR metadata survives, and a capture as a browser would tone-map it still passes.
func TestTrim_RealFiles_HDR(t *testing.T) {
	var fx trimFixture
	for _, f := range trimFixtures {
		if strings.HasPrefix(f.name, "K6") {
			fx = f
		}
	}
	w := cutWorker(t)
	src := fx.build(t, t.TempDir())
	ts := frameTimesOf(t, src)
	p := withCaptures(t, w, src, ts, 25, 100, 0.08)
	p.InTime, p.OutTime = ts[25], ts[100]
	dst, skipped, err := runCut(t, w, src, "cut", p, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped: %+v", skipped)
	}
	out := sh(t, "ffprobe", "-v", "error", "-select_streams", "V:0", "-read_intervals", "%+#1", "-show_frames", dst)
	for _, want := range []string{"Mastering display metadata", "Content light level metadata"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lost %s", want)
		}
	}
}

// Sync oracle: a click at a known frame sits at that frame's new time, within an audio packet.
func TestTrim_RealFiles_AudioStaysInSync(t *testing.T) {
	w := cutWorker(t)
	const clickAt = 3.0
	click := fmt.Sprintf("aevalsrc='if(between(t\\,%v\\,%v)\\,0.9\\,0)':d=6", clickAt, clickAt+0.02)
	for _, c := range []struct{ ext, codec string }{{"mp4", "aac"}, {"mkv", "ac3"}, {"mkv", "flac"}} {
		t.Run(c.codec, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "sync."+c.ext)
			ff(t, "-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=25:d=6,%s", bcW, bcH, barcode), "-f", "lavfi", "-i", click,
				"-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-c:a", c.codec, "-b:a", "192k", src)
			ts := frameTimesOf(t, src)
			in := FrameAt(ts, 1.0)
			dst, _, err := runCut(t, w, src, "cut", TrimParams{InTime: ts[in], OutTime: ts[FrameAt(ts, 5.0)], Mode: TrimReplace}, "")
			if err != nil {
				t.Fatal(err)
			}
			// When did the click start, relative to the first video frame?
			first := frameTimesOf(t, dst)[0]
			out, _ := exec.Command("ffmpeg", "-nostdin", "-i", dst, "-map", "0:a", "-af", "silencedetect=n=-30dB:d=0.1", "-f", "null", "-").CombinedOutput()
			var end float64
			found := false
			for _, line := range strings.Split(string(out), "\n") {
				if _, after, ok := strings.Cut(line, "silence_end: "); ok && !found {
					v, _, _ := strings.Cut(after, " ")
					end, _ = strconv.ParseFloat(v, 64)
					found = true
				}
			}
			if !found {
				t.Fatalf("no click found:\n%s", out)
			}
			got, want := end-first, clickAt-ts[in]
			if d := got - want; d > 0.05 || d < -0.05 {
				t.Errorf("click %.3fs after the first frame, want %.3fs", got, want)
			}
		})
	}
}

// A range that keeps every frame cuts nothing, so nothing is encoded or held for it.
func TestTrim_RealFiles_RefusesTheWholeFile(t *testing.T) {
	w := cutWorker(t)
	src := trimFixtures[1].build(t, t.TempDir())
	ts := frameTimesOf(t, src)
	dst, _, err := runCut(t, w, src, "all", TrimParams{InTime: ts[0], OutTime: ts[len(ts)-1] + 100, Mode: TrimReplace}, "")
	if err == nil || !strings.Contains(err.Error(), "whole file") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(dst); statErr == nil {
		t.Error("left an output behind")
	}
}

// decodePCM decodes path's audio track a as raw 32-bit samples.
func decodePCM(t *testing.T, path string, a int) []byte {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-i", path, "-map", "0:a:"+strconv.Itoa(a), "-f", "s32le", "-").Output()
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return out
}

// FLAC is re-encoded rather than cut on packets (mkvmerge can't split it). The kept samples must be
// the original's bit for bit, starting where the first kept frame does, with the track's bit depth,
// name, language and position kept, and lasting as long as the kept video.
func TestTrim_RealFiles_FLAC(t *testing.T) {
	w := cutWorker(t)
	for _, c := range []struct {
		others bool
		bits   int
	}{{true, 24}, {false, 24}, {false, 32}} {
		others := c.others
		t.Run(fmt.Sprintf("others=%v bits=%d", others, c.bits), func(t *testing.T) {
			dir := t.TempDir()
			src := buildFLACMKV(others, c.bits)(t, dir)
			ts := frameTimesOf(t, src)
			in, out := 15, len(ts)-17
			dst, _, err := runCut(t, w, src, "cut", TrimParams{InTime: ts[in], OutTime: ts[out], Mode: TrimReplace}, "")
			if err != nil {
				t.Fatal(err)
			}
			p, err := RunProbe(context.Background(), "ffprobe", "matroska", dst)
			if err != nil {
				t.Fatal(err)
			}
			var types []string
			for _, s := range p.Streams {
				types = append(types, s.CodecType+":"+s.CodecName)
			}
			want, flacAt := "video:h264 audio:flac", 0
			if others {
				want, flacAt = "video:h264 audio:ac3 audio:flac subtitle:subrip", 1
			}
			if got := strings.Join(types, " "); got != want {
				t.Fatalf("tracks %q, want %q", got, want)
			}
			var f Stream
			for _, s := range p.Streams {
				if isFLAC(s) {
					f = s
				}
			}
			if f.Tags["title"] != "Commentary" || f.Tags["language"] != "jpn" {
				t.Errorf("FLAC track tags %v", f.Tags)
			}
			if bits, _ := exec.Command("ffprobe", "-v", "error", "-select_streams", "a:"+strconv.Itoa(flacAt),
				"-show_entries", "stream=bits_per_raw_sample", "-of", "csv=p=0", dst).Output(); strings.TrimSpace(string(bits)) != strconv.Itoa(c.bits) {
				t.Errorf("bits per sample %q, want %d", bits, c.bits)
			}
			// Bit for bit: the kept samples are a run of the original's, starting at the cut.
			orig, kept := decodePCM(t, src, flacAt), decodePCM(t, dst, flacAt)
			const rate, frameBytes = 48000.0, 4 // mono, 32-bit
			at := bytes.Index(orig, kept)
			if at < 0 {
				t.Fatalf("the kept %d bytes of audio aren't a run of the original's", len(kept))
			}
			first := frameTimesOf(t, dst)[0]
			if start, wantStart := float64(at/frameBytes)/rate, ts[in]-first; math.Abs(start-wantStart) > 0.001 {
				t.Errorf("audio starts at the original's %.4fs, want %.4fs", start, wantStart)
			}
			kd, vd := float64(len(kept)/frameBytes)/rate, ts[out]-ts[in]+0.04
			if math.Abs(kd-vd) > 0.04 {
				t.Errorf("audio lasts %.3fs, the kept video %.3fs", kd, vd)
			}
			// The gate's sample comparison sees a track cut one frame off.
			sp, err := RunProbe(context.Background(), "ffprobe", "matroska", src)
			if err != nil {
				t.Fatal(err)
			}
			var origIdx int
			for _, s := range sp.Streams {
				if isFLAC(s) {
					origIdx = s.Index
				}
			}
			shifted := trimRange{Seek: ts[in+1] - 0.02, HasSeek: true, Stop: ts[out] + 0.02, HasStop: true}
			if detail, err := compareSamples(context.Background(), localTools, src, dst, origIdx, f.Index, shifted); err != nil || detail == "" {
				t.Errorf("a shifted range passed the sample comparison: %q, %v", detail, err)
			}
		})
	}
}

// What a cut can't do is refused before anything runs.
func TestCheckTrimRefusals(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	flac := filepath.Join(dir, "flac.mkv")
	ff(t, "-f", "lavfi", "-i", "testsrc2=s=64x64:r=25:d=1", "-f", "lavfi", "-i", "sine=d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "flac", flac)
	if pf, err := checkOp(ctx, localTools, detectedEncoders(t), flac, 1, true); err != nil || !pf.OK {
		t.Errorf("FLAC in Matroska is trimmed, not refused: %+v, %v", pf, err)
	}
	two := filepath.Join(dir, "two.mkv")
	ff(t, "-f", "lavfi", "-i", "testsrc2=s=64x64:r=25:d=1", "-f", "lavfi", "-i", "testsrc=s=64x64:r=25:d=1", "-map", "0", "-map", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", two)
	if pf, err := checkOp(ctx, localTools, detectedEncoders(t), two, 1, true); err != nil || pf.OK || pf.Reason != ReasonMultiVideo {
		t.Errorf("two video tracks: %+v, %v", pf, err)
	}
}

// The correction path on real files, in the direction that matters. A cut placed two frames late is
// seen by the cut check as +2 at both ends, its output goes, and the original is untouched. Moving the
// cut back by what was measured lands exactly on the chosen frames.
func TestTrim_RealFiles_CutCheckCorrects(t *testing.T) {
	w := cutWorker(t)
	ctx := context.Background()
	dir := t.TempDir()
	src := trimFixtures[1].build(t, dir)
	before, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	ts := frameTimesOf(t, src)
	bars := readBars(t, src)
	p, err := RunProbe(ctx, "ffprobe", "mov", src)
	if err != nil {
		t.Fatal(err)
	}
	orig, err := CollectSnapshot(ctx, localTools, FamilyMP4, src, p)
	if err != nil {
		t.Fatal(err)
	}
	inW, _ := listFrames(ctx, localTools, FamilyMP4, src, p, ts[30], 10)
	outW, _ := listFrames(ctx, localTools, FamilyMP4, src, p, ts[100], 10)
	s := trimState{p: p, orig: orig, inW: inW, outW: outW, inAt: inW.nearest(ts[30]), outAt: outW.nearest(ts[100]), inCut: 2, outCut: 2}
	dst := filepath.Join(dir, "cut.mp4")

	off, err := w.cutOnce(ctx, FamilyMP4, src, dst, &s, "")
	if err != nil {
		t.Fatal(err)
	}
	if off != [2]int{2, 2} {
		t.Errorf("offsets = %v, want the output two frames late at both ends", off)
	}
	if _, err := os.Stat(dst); err == nil {
		t.Error("a cut that missed left its output")
	}
	after, _ := os.ReadFile(src)
	if !bytes.Equal(before, after) {
		t.Error("the original changed")
	}

	s.moveCut(off)
	off, err = w.cutOnce(ctx, FamilyMP4, src, dst, &s, "")
	if err != nil {
		t.Fatal(err)
	}
	if off != [2]int{} {
		t.Fatalf("offsets after the correction = %v", off)
	}
	got := readBars(t, dst)
	if got[0] != bars[30] || got[len(got)-1] != bars[100] {
		t.Errorf("frames %d..%d, want %d..%d", got[0], got[len(got)-1], bars[30], bars[100])
	}
}

// Statistics describe the kept range on every stream, not the original's whole length. The gate
// exempts these tags, so this is the only thing checking them.
func TestTrim_RealFiles_StatisticsRecomputed(t *testing.T) {
	w := cutWorker(t)
	ctx := context.Background()
	var fx trimFixture
	for _, f := range trimFixtures {
		if strings.HasPrefix(f.name, "K5") {
			fx = f
		}
	}
	src := fx.build(t, t.TempDir())
	ts := frameTimesOf(t, src)
	dst, _, err := runCut(t, w, src, "cut", TrimParams{InTime: ts[25], OutTime: ts[100], Mode: TrimReplace}, "")
	if err != nil {
		t.Fatal(err)
	}
	op, _ := RunProbe(ctx, "ffprobe", "matroska", src)
	cp, err := RunProbe(ctx, "ffprobe", "matroska", dst)
	if err != nil {
		t.Fatal(err)
	}
	stat := func(s Stream, base string) int {
		for k, v := range s.Tags {
			if hasTagBase(k, []string{base}) {
				n, _ := strconv.Atoi(v)
				return n
			}
		}
		t.Fatalf("stream %d (%s) has no %s tag: %v", s.Index, s.CodecType, base, s.Tags)
		return 0
	}
	if v, _ := cp.MainVideo(); stat(v, "NUMBER_OF_FRAMES") != 76 {
		t.Errorf("video NUMBER_OF_FRAMES = %d, want the 76 kept", stat(v, "NUMBER_OF_FRAMES"))
	}
	for i, s := range op.Streams {
		if s.CodecType == "audio" && stat(cp.Streams[i], "NUMBER_OF_BYTES") >= stat(s, "NUMBER_OF_BYTES") {
			t.Errorf("audio NUMBER_OF_BYTES %d wasn't recomputed from %d", stat(cp.Streams[i], "NUMBER_OF_BYTES"), stat(s, "NUMBER_OF_BYTES"))
		}
	}
}

// A damaged candidate is refused on real files: a remux dropping the audio can't pass as a trim.
func TestTrim_RealFiles_GateRefusesDamage(t *testing.T) {
	dir := t.TempDir()
	src := trimFixtures[1].build(t, dir)
	ctx := context.Background()
	op, err := RunProbe(ctx, "ffprobe", "mov", src)
	if err != nil {
		t.Fatal(err)
	}
	orig, err := CollectSnapshot(ctx, localTools, FamilyMP4, src, op)
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.mp4")
	ff(t, "-i", src, "-map", "0:v", "-c", "copy", bad)
	cp, err := RunProbe(ctx, "ffprobe", "mov", bad)
	if err != nil {
		t.Fatal(err)
	}
	cand, err := CollectSnapshot(ctx, localTools, FamilyMP4, bad, cp)
	if err != nil {
		t.Fatal(err)
	}
	err = VerifyTrim(orig, cand, TrimExpect{})
	if re, ok := err.(*RuleError); !ok || re.Rule != RuleStreams {
		t.Errorf("err = %v", err)
	}
}
