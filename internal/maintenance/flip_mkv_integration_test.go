//go:build integration

package maintenance

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"holodex/internal/model"
)

// Matroska real-file proof (testing-strategy §25.2 K1–K4, R1–R2). Needs ffmpeg with the four
// table encoders, plus MKVToolNix: run under `make test-image` (the runtime image has all of
// them). Each fixture carries the top-left drawbox, so psnrMirror proves the flip independently
// of Verify.

const box = "drawbox=x=10:y=10:w=60:h=60:color=white:t=fill"

func mkvFixture(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	full := append([]string{"-nostdin", "-y", "-loglevel", "error"}, args...)
	sh(t, "ffmpeg", append(full, out)...)
	return out
}

// buildK1 is the stress case: 10-bit HEVC with HDR10 mastering metadata, two audio tracks in
// two languages, a subtitle, a font attachment, chapters, a global tag and a tag on the video
// track itself.
func buildK1(t *testing.T, dir string) string {
	meta := filepath.Join(dir, "k1meta.txt")
	os.WriteFile(meta, []byte(";FFMETADATA1\ntitle=K1\nPUBLISHER=Acme\n"+
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=1000\ntitle=One\n"+
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=1000\nEND=2000\ntitle=Two\n"), 0o644)
	srt := filepath.Join(dir, "k1.srt")
	os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nHello\n"), 0o644)
	font := filepath.Join(dir, "font.ttf")
	os.WriteFile(font, []byte("not really a font, but an attachment's bytes all the same"), 0o644)
	return mkvFixture(t, dir, "k1.mkv",
		"-f", "lavfi", "-i", "testsrc2=s=320x240:r=24:d=2,"+box,
		"-f", "lavfi", "-i", "sine=d=2:frequency=440",
		"-f", "lavfi", "-i", "sine=d=2:frequency=880",
		"-i", srt, "-i", meta,
		"-map", "0:v", "-map", "1:a", "-map", "2:a", "-map", "3:s",
		"-map_metadata", "4", "-map_chapters", "4",
		"-c:v", "libx265", "-pix_fmt", "yuv420p10le",
		"-color_primaries", "bt2020", "-color_trc", "smpte2084", "-colorspace", "bt2020nc",
		"-x265-params", "log-level=error:hdr10=1:repeat-headers=1:"+
			"master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(10000000,50):max-cll=1000,400",
		"-c:a", "libopus", "-c:s", "srt",
		"-metadata:s:v:0", "title=Main", "-metadata:s:v:0", "ENCODER_SETTINGS=original",
		"-metadata:s:a:0", "language=eng", "-metadata:s:a:1", "language=jpn",
		"-attach", font, "-metadata:s:t", "mimetype=application/x-truetype-font",
		"-t", "2")
}

// flipMKV runs the whole pipeline the worker runs and returns the candidate.
func flipMKV(t *testing.T, src string) string {
	t.Helper()
	ctx := context.Background()
	enc, err := DetectEncoders(ctx, "ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	if pf, err := CheckFlip(ctx, localTools, enc, src); err != nil || !pf.OK {
		t.Fatalf("preflight: %+v %v", pf, err)
	}
	p, err := RunProbe(ctx, "ffprobe", "matroska", src)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(src)
	dst := NewPath(src)
	if err := Rebuild(ctx, localTools, FamilyMKV, OpHFlip, enc, src, dst, p); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if after, _ := os.ReadFile(src); string(before) != string(after) {
		t.Fatal("Rebuild touched the original")
	}
	cp, err := RunProbe(ctx, "ffprobe", "matroska", dst)
	if err != nil {
		t.Fatal(err)
	}
	orig, err := CollectSnapshot(ctx, localTools, FamilyMKV, src, p)
	if err != nil {
		t.Fatal(err)
	}
	cand, err := CollectSnapshot(ctx, localTools, FamilyMKV, dst, cp)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(orig, cand); err != nil {
		t.Fatalf("faithful flip refused: %v", err)
	}
	if got := psnrMirror(t, src, dst); got < 40 {
		t.Errorf("mirror PSNR %.1f dB, want > 40", got)
	}
	if strings.Contains(strings.Join(dirNames(t, filepath.Dir(src)), " "), ".holodex-tmp") {
		t.Error("the intermediate was left behind")
	}
	return dst
}

func dirNames(t *testing.T, dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func requireMKVTools(t *testing.T) {
	requireTools(t, "ffmpeg", "ffprobe", "mkvmerge", "mkvextract")
}

func TestFlip_RealFiles_MKV_K1(t *testing.T) {
	requireMKVTools(t)
	dir := t.TempDir()
	src := buildK1(t, dir)
	dst := flipMKV(t, src)

	cp, _ := RunProbe(context.Background(), "ffprobe", "matroska", dst)
	v, _ := cp.MainVideo()
	if v.Tags["title"] != "Main" || v.Tags["ENCODER_SETTINGS"] != "original" {
		t.Errorf("video track's own tags not carried: %v", v.Tags)
	}
	var mastering, light bool
	for _, sd := range v.SideData {
		mastering = mastering || sd.Type == sideDataMastering
		light = light || sd.Type == sideDataLightLevel
	}
	if !mastering || !light {
		t.Errorf("HDR10 metadata lost: mastering=%v light=%v (%+v)", mastering, light, v.SideData)
	}
}

func TestFlip_RealFiles_MKV_VP9(t *testing.T) {
	requireMKVTools(t)
	dir := t.TempDir()
	flipMKV(t, mkvFixture(t, dir, "k2.mkv",
		"-f", "lavfi", "-i", "testsrc2=s=320x240:r=25:d=2,"+box, "-f", "lavfi", "-i", "sine=d=2",
		"-c:v", "libvpx-vp9", "-c:a", "libopus", "-metadata", "title=K2", "-t", "2"))
}

func TestFlip_RealFiles_MKV_AV1(t *testing.T) {
	requireMKVTools(t)
	dir := t.TempDir()
	flipMKV(t, mkvFixture(t, dir, "k3.mkv",
		"-f", "lavfi", "-i", "testsrc2=s=320x240:r=25:d=2,"+box, "-f", "lavfi", "-i", "sine=d=2",
		"-c:v", "libsvtav1", "-c:a", "libopus", "-t", "2"))
}

// The video is not the first track: the new track must go back to position 1.
func TestFlip_RealFiles_MKV_VideoNotFirst(t *testing.T) {
	requireMKVTools(t)
	dir := t.TempDir()
	dst := flipMKV(t, mkvFixture(t, dir, "k4.mkv",
		"-f", "lavfi", "-i", "sine=d=2", "-f", "lavfi", "-i", "testsrc2=s=320x240:r=25:d=2,"+box,
		"-map", "0:a", "-map", "1:v", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "libopus", "-t", "2"))
	cp, _ := RunProbe(context.Background(), "ffprobe", "matroska", dst)
	if cp.Streams[0].CodecType != "audio" || cp.Streams[1].CodecType != "video" {
		t.Errorf("track order changed: %s", streamKinds(cp))
	}
}

// Most real MKV libraries were written by mkvmerge: every track carries statistics tags and the
// segment a mux date. The statistics are regenerated (the re-encoded track's change, the copied
// tracks' match), and the original date survives.
func TestFlip_RealFiles_MKV_MkvmergeWritten(t *testing.T) {
	requireMKVTools(t)
	dir := t.TempDir()
	ffmpegMade := buildK1(t, dir)
	src := filepath.Join(dir, "k5.mkv")
	sh(t, "mkvmerge", "-q", "-o", src, ffmpegMade)
	p, err := RunProbe(context.Background(), "ffprobe", "matroska", src)
	if err != nil {
		t.Fatal(err)
	}
	if !hasTrackStatistics(p) || p.Format.Tags["creation_time"] == "" {
		t.Fatalf("fixture isn't mkvmerge-shaped: stats=%v date=%q", hasTrackStatistics(p), p.Format.Tags["creation_time"])
	}
	dst := flipMKV(t, src)
	cp, _ := RunProbe(context.Background(), "ffprobe", "matroska", dst)
	if cp.Format.Tags["creation_time"] != p.Format.Tags["creation_time"] {
		t.Errorf("mux date %q, want the original's %q", cp.Format.Tags["creation_time"], p.Format.Tags["creation_time"])
	}
	v, _ := cp.MainVideo()
	ov, _ := p.MainVideo()
	now, was := statTag(v.Tags, "NUMBER_OF_BYTES"), statTag(ov.Tags, "NUMBER_OF_BYTES")
	if now == "" || now == was {
		t.Errorf("re-encoded track's statistics not recomputed: %q (was %q; tags %v)", now, was, v.Tags)
	}
}

// statTag reads a Matroska tag by base name, with or without its "-<lang>" suffix.
func statTag(tags map[string]string, base string) string {
	for k, v := range tags {
		if hasTagBase(k, []string{base}) {
			return v
		}
	}
	return ""
}

// The worker's own flip (probe → collect → rebuild → collect → verify → swap) on real files,
// MP4 and Matroska, exactly as a queued job runs it.
func TestFlip_RealFiles_ThroughTheWorker(t *testing.T) {
	requireMKVTools(t)
	requireTools(t, "exiftool")
	ctx := context.Background()
	enc, err := DetectEncoders(ctx, "ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	for name, build := range map[string]func(*testing.T, string) string{"mp4": buildM1, "mkv": buildK1} {
		t.Run(name, func(t *testing.T) {
			g := newWorkerRig(t, model.ThumbnailGenerated)
			src := build(t, t.TempDir())
			original, _ := os.ReadFile(src)
			// A library row for the real file; the rig's stand-in video is left alone.
			v, _, _ := g.r.GetVideo(ctx, g.videoID)
			v.ID, v.FilePath = 0, src
			id, err := g.r.UpsertVideo(ctx, v, nil)
			if err != nil {
				t.Fatal(err)
			}
			g.videoID, g.path = id, src
			g.w.tools, g.w.encoders = localTools, enc
			g.w.runFlip = g.w.flip
			g.runOne(t, ctx, string(OpHFlip))
			if s := g.queueStatus(t); s.Failed {
				t.Fatalf("flip failed: %s", s.Error)
			}
			if held, _ := os.ReadFile(HeldPath(src)); string(held) != string(original) {
				t.Error("the held original isn't the original bytes")
			}
			if got := psnrMirror(t, HeldPath(src), src); got < 40 {
				t.Errorf("mirror PSNR %.1f dB", got)
			}
		})
	}
}

func TestFlip_RealFiles_MKV_Refusals(t *testing.T) {
	requireMKVTools(t)
	ctx := context.Background()
	enc, _ := DetectEncoders(ctx, "ffmpeg")
	dir := t.TempDir()
	mpeg2 := mkvFixture(t, dir, "r1.mkv", "-f", "lavfi", "-i", "testsrc2=s=320x240:r=25:d=1",
		"-c:v", "mpeg2video", "-t", "1")
	if pf, err := CheckFlip(ctx, localTools, enc, mpeg2); err != nil || pf.Reason != ReasonCodec {
		t.Errorf("MPEG-2: %+v %v", pf, err)
	}
	audioOnly := mkvFixture(t, dir, "r2.mkv", "-f", "lavfi", "-i", "sine=d=1", "-c:a", "libopus", "-t", "1")
	if pf, err := CheckFlip(ctx, localTools, enc, audioOnly); err != nil || pf.Reason != ReasonNoVideo {
		t.Errorf("audio only: %+v %v", pf, err)
	}
	for _, name := range dirNames(t, dir) {
		if strings.Contains(name, ".holodex") {
			t.Errorf("a refusal left %s behind", name)
		}
	}
}
