package maintenance

import (
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// U1: a cut's seek and stop are midpoints with the real neighbouring frames, so uneven gaps can't
// reach past a frame. Half a nominal 25 fps interval (20 ms) would pass the 10 ms frames here.
func TestCutPointsUseRealNeighbours(t *testing.T) {
	w := frameWindow{0, 0.04, 0.08, 0.09, 0.10, 0.14}
	if s, ok := w.seekBefore(3); !ok || math.Abs(s-0.085) > 1e-9 {
		t.Errorf("seekBefore(3) = %v, %v; want the midpoint 0.085", s, ok)
	}
	if s, ok := w.stopAfter(3); !ok || math.Abs(s-0.095) > 1e-9 {
		t.Errorf("stopAfter(3) = %v, %v; want the midpoint 0.095", s, ok)
	}
	if _, ok := w.seekBefore(0); ok {
		t.Error("the file's first frame needs no seek")
	}
	if _, ok := w.stopAfter(len(w) - 1); ok {
		t.Error("the file's last frame needs no stop")
	}
	// A correction can push an index past the listed frames; it must clamp, not panic.
	if s, ok := w.seekBefore(len(w) + 2); !ok || math.Abs(s-0.12) > 1e-9 {
		t.Errorf("seekBefore past the end = %v, %v", s, ok)
	}
	if s, ok := w.stopAfter(-2); !ok || math.Abs(s-0.02) > 1e-9 {
		t.Errorf("stopAfter before the start = %v, %v", s, ok)
	}
}

func TestNearestFrame(t *testing.T) {
	w := frameWindow{0, 1.5015, 3.003}
	for _, c := range []struct {
		t    float64
		want int
	}{{-1, 0}, {0, 0}, {1.502, 1}, {1.5, 1}, {2.3, 2}, {99, 2}} {
		if got := w.nearest(c.t); got != c.want {
			t.Errorf("nearest(%v) = %d, want %d", c.t, got, c.want)
		}
	}
}

// An edit list's discarded packets sit before zero. Packets arrive in decode order.
func TestParseFrameTimes(t *testing.T) {
	got, err := parseFrameTimes([]byte("-0.12,\n0.08,\n0.0,\n0.04,\nN/A,\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{0, 0.04, 0.08}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func chapter(start, end, title string) Chapter {
	return Chapter{StartTime: start, EndTime: end, Tags: map[string]string{"title": title}}
}

// U2: chapters follow the cut. in=10, duration=20, so the kept range is 10–30 in the original.
func TestExpectedChapters(t *testing.T) {
	cases := []struct {
		name string
		in   Chapter
		want []string // start, end, or nil when dropped
	}{
		{"inside moves earlier", chapter("12", "15", "a"), []string{"2.000000", "5.000000"}},
		{"before is dropped", chapter("0", "5", "a"), nil},
		{"after is dropped", chapter("40", "50", "a"), nil},
		{"straddling the start is clamped", chapter("5", "15", "a"), []string{"0.000000", "5.000000"}},
		{"straddling the end is clamped", chapter("25", "45", "a"), []string{"15.000000", "20.000000"}},
		{"ending on the cut is dropped", chapter("5", "10", "a"), nil},
		{"starting on the end is dropped", chapter("30", "35", "a"), nil},
		{"spanning the range fills it", chapter("0", "100", "a"), []string{"0.000000", "20.000000"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExpectedChapters([]Chapter{c.in}, 10, 20)
			if c.want == nil {
				if len(got) != 0 {
					t.Fatalf("got %+v, want none", got)
				}
				return
			}
			if len(got) != 1 || got[0].StartTime != c.want[0] || got[0].EndTime != c.want[1] || got[0].Tags["title"] != "a" {
				t.Fatalf("got %+v, want %v", got, c.want)
			}
		})
	}
}

func TestChapterFiles(t *testing.T) {
	cs := []Chapter{chapter("1.5", "2", "A=b;c & <d>")}
	meta := ffmetadataChapters(cs)
	for _, want := range []string{"START=1500000", "END=2000000", `title=A\=b\;c & <d>`} {
		if !strings.Contains(meta, want) {
			t.Errorf("ffmetadata %q missing %q", meta, want)
		}
	}
	xml := matroskaChapters(cs)
	for _, want := range []string{"00:00:01.500000000", "A=b;c &amp; &lt;d&gt;"} {
		if !strings.Contains(xml, want) {
			t.Errorf("chapter XML %q missing %q", xml, want)
		}
	}
}

func TestHMSTime(t *testing.T) {
	if got := hmsTime(3725.5); got != "01:02:05.500000000" {
		t.Errorf("hmsTime = %q", got)
	}
}

// U6: the owner's text becomes a path component only through this.
func TestEditionTarget(t *testing.T) {
	dir := filepath.Join("lib", "films")
	cases := []struct {
		path, name, want string
		ok               bool
	}{
		{"The Long Road (2004).mkv", "Trimmed", "The Long Road (2004) {edition-Trimmed}.mkv", true},
		{"The Long Road (2004) {edition-Theatrical}.mkv", "Trimmed", "The Long Road (2004) {edition-Trimmed}.mkv", true},
		{"a {edition-Old} b.mp4", "New", "a b {edition-New}.mp4", true},
		{"a.mp4", "../../etc/passwd", "a {edition-etcpasswd}.mp4", true},
		{"a.mp4", "...hidden", "a {edition-hidden}.mp4", true},
		{"a.mp4", "x}{y", "a {edition-xy}.mp4", true},
		{"a.mp4", "bad\x00name\n", "a {edition-badname}.mp4", true},
		{"a.mp4", "  ", "", false},
		{"a.mp4", "///", "", false},
		{"a.mp4", "..", "", false},
	}
	for _, c := range cases {
		got, _, ok := EditionTarget(filepath.Join(dir, c.path), c.name)
		if ok != c.ok {
			t.Errorf("EditionTarget(%q, %q) ok = %v", c.path, c.name, ok)
			continue
		}
		if ok && got != filepath.Join(dir, c.want) {
			t.Errorf("EditionTarget(%q, %q) = %q, want %q", c.path, c.name, got, c.want)
		}
		if ok && filepath.Dir(got) != dir {
			t.Errorf("EditionTarget(%q, %q) left the folder: %q", c.path, c.name, got)
		}
	}
}

func TestEditionTargetCapsLengthOnARuneBoundary(t *testing.T) {
	got, name, ok := EditionTarget("a.mkv", strings.Repeat("é", 400))
	if !ok {
		t.Fatal("a long name should be cut, not refused")
	}
	if len(filepath.Base(got)) > maxNameBytes {
		t.Errorf("name is %d bytes, over %d", len(filepath.Base(got)), maxNameBytes)
	}
	if strings.ContainsRune(name, '�') {
		t.Errorf("name was cut inside a rune: %q", name)
	}
}

// flat returns a frame of one grey level, and shaped one with a bright box at x.
func flat(v byte) grey {
	g := make(grey, checkSize)
	for i := range g {
		g[i] = v
	}
	return g
}

func shaped(x int, base, box byte) grey {
	g := flat(base)
	for row := 40; row < 90; row++ {
		for col := x; col < x+40; col++ {
			g[row*checkW+col] = box
		}
	}
	return g
}

func moving() references {
	return references{At: 2, Frames: []grey{
		shaped(20, 50, 200), shaped(60, 50, 200), shaped(100, 50, 200), shaped(140, 50, 200), shaped(180, 50, 200),
	}}
}

// U3: a frame and the same frame 25 levels brighter still correlate; absolute pixels would not.
func TestNCCIgnoresBrightness(t *testing.T) {
	a, b := shaped(60, 50, 200), shaped(60, 75, 225)
	if n := ncc(a, b); n < 0.999 {
		t.Errorf("ncc = %v, want about 1", n)
	}
	if d := mad(a, b); d != 25 {
		t.Errorf("mad = %v, want 25", d)
	}
	if ncc(flat(0), flat(0)) != 1 {
		t.Error("two flat frames have no shape to differ in")
	}
}

func TestCutOffset(t *testing.T) {
	refs := moving()
	for want := -2; want <= 2; want++ {
		if got := cutOffset(refs.Frames[refs.At+want], refs); got != want {
			t.Errorf("output on reference %+d: offset %d", want, got)
		}
	}
	// Near-identical frames tie, because a one-frame slip there can't be seen.
	still := references{At: 2, Frames: []grey{flat(100), flat(100), flat(100), flat(100), flat(100)}}
	if got := cutOffset(flat(100), still); got != 0 {
		t.Errorf("identical frames: offset %d, want 0", got)
	}
}

// U4: the distinct threshold, and which reason the status names.
func TestScreenSkipReason(t *testing.T) {
	if r := screenSkipReason(moving()); r != "" {
		t.Errorf("moving frames skipped: %q", r)
	}
	if r := screenSkipReason(references{At: 1, Frames: []grey{flat(3), flat(3), flat(3)}}); r != SkipBlack {
		t.Errorf("black = %q", r)
	}
	still := shaped(60, 80, 200)
	if r := screenSkipReason(references{At: 1, Frames: []grey{still, still, still}}); r != SkipStill {
		t.Errorf("still = %q", r)
	}
	fade := references{At: 1, Frames: []grey{shaped(60, 80, 200), shaped(60, 90, 210), shaped(60, 100, 220)}}
	if r := screenSkipReason(fade); r != SkipSlowFade {
		t.Errorf("fade = %q", r)
	}
	if r := screenSkipReason(references{At: 0, Frames: []grey{still}}); r == "" {
		t.Error("a frame with no neighbour can't be told from any")
	}
}

func TestScreenOffset(t *testing.T) {
	refs := moving()
	// As a browser shows HDR: same shape, different brightness.
	if got := screenOffset(shaped(100, 70, 220), refs); got != 0 {
		t.Errorf("the chosen frame: offset %d", got)
	}
	if got := screenOffset(shaped(140, 70, 220), refs); got != 1 {
		t.Errorf("the next frame: offset %d", got)
	}
	if got := screenOffset(shaped(20, 70, 220), refs); got != -2 {
		t.Errorf("two back: offset %d", got)
	}
}

// A browser tone-maps an HDR frame, so what it shows differs in contrast as well as level. By pixels
// that capture sits nearer a duller neighbour, as on a fade. By shape the two tie, and a tie is no move.
func TestScreenOffsetIgnoresContrast(t *testing.T) {
	refs := references{At: 1, Frames: []grey{shaped(100, 20, 240), shaped(100, 20, 240), shaped(100, 90, 150)}}
	if got := screenOffset(shaped(100, 88, 152), refs); got != 0 {
		t.Errorf("offset %d: the capture's contrast was taken for a neighbouring frame", got)
	}
}

// U12: a capture of another scene fails instead of matching the least-bad neighbour.
func TestScreenOffsetRefusesAnotherScene(t *testing.T) {
	other := make(grey, checkSize)
	for i := range other {
		other[i] = byte(i * 7 % 251)
	}
	if got := screenOffset(other, moving()); got != screenNoMatch {
		t.Errorf("offset %d, want no match", got)
	}
}

func pkts(start float64, hashes ...string) []packet {
	out := make([]packet, len(hashes))
	for i, h := range hashes {
		out[i] = packet{PTS: start + float64(i)*0.1, Hash: h}
	}
	return out
}

func TestComparePackets(t *testing.T) {
	orig := pkts(0, "a", "b", "c", "d", "e", "f", "g", "h", "i", "j")
	if d := comparePackets(orig, pkts(0, "c", "d", "e", "f", "g"), 0.2, 0.6); d != "" {
		t.Errorf("a clean cut: %s", d)
	}
	// U9: which side of a cut the boundary packet lands on is the tool's choice.
	if d := comparePackets(orig, pkts(0, "X", "d", "e", "f", "Y"), 0.2, 0.6); d != "" {
		t.Errorf("boundary packets differ: %s", d)
	}
	// U8: a changed packet inside the range, and a dropped tail.
	if d := comparePackets(orig, pkts(0, "c", "d", "X", "f", "g"), 0.2, 0.6); d == "" {
		t.Error("a packet changed inside the range passed")
	}
	if d := comparePackets(pkts(0, slices.Repeat([]string{"z"}, 100)...), pkts(0, "z", "z", "z", "z", "z"), 0, 9.9); d == "" {
		t.Error("a dropped tail passed")
	}
	// Silence repeats a digest; the run is still found.
	silent := pkts(0, "s", "s", "s", "s", "t", "u", "v")
	if d := comparePackets(silent, pkts(0, "s", "s", "t", "u", "v"), 0.2, 0.6); d != "" {
		t.Errorf("repeated digests: %s", d)
	}
}

func trimSnapshot(family string) Snapshot {
	return Snapshot{
		Family: family,
		Probe: Probe{
			Streams: []Stream{
				{Index: 0, CodecType: "video", CodecName: "h264", Width: 320, Height: 240, PixFmt: "yuv420p", RFrameRate: "25/1", AvgFrameRate: "25/1",
					Tags: map[string]string{"BPS": "1"}},
				{Index: 1, CodecType: "audio", CodecName: "aac", Tags: map[string]string{"language": "eng", "NUMBER_OF_BYTES": "9"}},
			},
			Chapters: []Chapter{chapter("0.000000", "1.000000", "one")},
			Format:   Format{Tags: map[string]string{"title": "x"}},
		},
		Extra: map[string]string{},
	}
}

func goodExpect() TrimExpect {
	return TrimExpect{Chapters: []Chapter{chapter("0.000000", "1.000000", "one")}, VideoSpan: 4, WantSpan: 4,
		CopiedOK: map[int]string{1: ""}}
}

// U8: one failing candidate per rule a trim relaxes or adds, each expected to be refused.
func TestVerifyTrim(t *testing.T) {
	if err := VerifyTrim(trimSnapshot(FamilyMKV), trimSnapshot(FamilyMKV), goodExpect()); err != nil {
		t.Fatalf("a good candidate: %v", err)
	}
	// Statistics are recomputed on every stream, not only the video.
	stats := trimSnapshot(FamilyMKV)
	stats.Probe.Streams[1].Tags["NUMBER_OF_BYTES"] = "3"
	if err := VerifyTrim(trimSnapshot(FamilyMKV), stats, goodExpect()); err != nil {
		t.Fatalf("recomputed statistics on a copied stream: %v", err)
	}

	cases := []struct {
		name string
		rule string
		mod  func(c *Snapshot, e *TrimExpect)
	}{
		{"one frame and a bit too long", RuleDuration, func(c *Snapshot, e *TrimExpect) { e.VideoSpan = 4.1 }},
		{"one frame and a bit short", RuleDuration, func(c *Snapshot, e *TrimExpect) { e.VideoSpan = 3.9 }},
		{"a chapter not shifted", RuleChapters, func(c *Snapshot, e *TrimExpect) {
			c.Probe.Chapters = []Chapter{chapter("10.000000", "11.000000", "one")}
		}},
		{"a chapter dropped", RuleChapters, func(c *Snapshot, e *TrimExpect) { c.Probe.Chapters = nil }},
		{"a chapter retitled", RuleChapters, func(c *Snapshot, e *TrimExpect) {
			c.Probe.Chapters = []Chapter{chapter("0.000000", "1.000000", "two")}
		}},
		{"audio changed in the range", RuleCopied, func(c *Snapshot, e *TrimExpect) { e.CopiedOK[1] = "changed" }},
		{"audio never compared", RuleCopied, func(c *Snapshot, e *TrimExpect) { delete(e.CopiedOK, 1) }},
		{"a stream dropped", RuleStreams, func(c *Snapshot, e *TrimExpect) { c.Probe.Streams = c.Probe.Streams[:1] }},
		{"a container tag changed", RuleTags, func(c *Snapshot, e *TrimExpect) { c.Probe.Format.Tags["title"] = "y" }},
		{"a stream tag changed", RuleTags, func(c *Snapshot, e *TrimExpect) { c.Probe.Streams[1].Tags["language"] = "jpn" }},
		{"the picture changed shape", RulePicture, func(c *Snapshot, e *TrimExpect) { c.Probe.Streams[0].Width = 300 }},
		{"a constant rate changed", RulePicture, func(c *Snapshot, e *TrimExpect) { c.Probe.Streams[0].RFrameRate = "24/1" }},
		{"a replace changed the edition", RuleTags, func(c *Snapshot, e *TrimExpect) { c.Probe.Format.Tags["EDITION"] = "x" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cand, want := trimSnapshot(FamilyMKV), goodExpect()
			c.mod(&cand, &want)
			err := VerifyTrim(trimSnapshot(FamilyMKV), cand, want)
			re, ok := err.(*RuleError)
			if !ok || re.Rule != c.rule {
				t.Fatalf("got %v, want rule %s", err, c.rule)
			}
		})
	}
}

func TestVerifyTrimVariableRateKeepsItsOwnCeiling(t *testing.T) {
	orig, cand := trimSnapshot(FamilyMKV), trimSnapshot(FamilyMKV)
	orig.Probe.Streams[0].RFrameRate, orig.Probe.Streams[0].AvgFrameRate = "100/1", "30/1"
	cand.Probe.Streams[0].RFrameRate, cand.Probe.Streams[0].AvgFrameRate = "50/1", "31/1"
	if err := VerifyTrim(orig, cand, goodExpect()); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyTrimEdition(t *testing.T) {
	want := goodExpect()
	want.Edition = "Trimmed"
	orig, cand := trimSnapshot(FamilyMKV), trimSnapshot(FamilyMKV)
	orig.Probe.Format.Tags["EDITION"] = "Theatrical"
	cand.Probe.Format.Tags["EDITION"] = "Trimmed"
	if err := VerifyTrim(orig, cand, want); err != nil {
		t.Fatalf("the new edition: %v", err)
	}
	cand.Probe.Format.Tags["EDITION"] = "Theatrical"
	if err := VerifyTrim(orig, cand, want); err == nil {
		t.Error("an edition's file with the old value passed")
	}
	// MP4 keeps it in XMP, as JSON text.
	mp4o, mp4c := trimSnapshot(FamilyMP4), trimSnapshot(FamilyMP4)
	mp4c.Extra["xmp:XMP-prism:Edition"] = `"Trimmed"`
	if err := VerifyTrim(mp4o, mp4c, want); err != nil {
		t.Fatalf("MP4 edition: %v", err)
	}
	// A file with no XMP gains exiftool's toolkit stamp along with the packet it creates.
	mp4c.Extra[xmpToolkitTag] = `"Image::ExifTool 13.25"`
	if err := VerifyTrim(mp4o, mp4c, want); err != nil {
		t.Fatalf("exiftool's stamp on a new XMP packet: %v", err)
	}
	replace := goodExpect()
	if err := VerifyTrim(mp4o, mp4c, replace); err == nil {
		t.Error("a replace must not add XMP at all")
	}
	mp4c.Extra["xmp:XMP-dc:Title"] = `"changed"`
	if err := VerifyTrim(mp4o, mp4c, want); err == nil {
		t.Error("a second tag differing on an edition passed")
	}
}

func TestTrimMKVArgsCutPassLeavesTheVideoOut(t *testing.T) {
	p := Probe{Streams: []Stream{
		{Index: 0, CodecType: "audio"},
		{Index: 1, CodecType: "video", Disposition: map[string]int{"default": 1}},
		{Index: 2, CodecType: "subtitle"},
		{Index: 3, CodecType: "attachment"},
	}}
	r := trimRange{Seek: 1.5, HasSeek: true, Stop: 9, HasStop: true}
	cut := strings.Join(TrimMKVCutArgs(p, "/m/a.mkv", "/m/cut", r), " ")
	for _, want := range []string{"--split parts:00:00:01.500000000-00:00:09.000000000", "-d !1", "--no-chapters"} {
		if !strings.Contains(cut, want) {
			t.Errorf("cut args %q missing %q", cut, want)
		}
	}
	// With the video gone the subtitle is track 1 of the cut file; the video returns between them.
	merge := strings.Join(TrimMKVMergeArgs(p, "/m/cut", true, "/m/enc", []string{""}, "/m/ch.xml", "/m/dst"), " ")
	for _, want := range []string{"--track-order 0:0,1:0,0:1", "--chapters " + toolArg("/m/ch.xml")} {
		if !strings.Contains(merge, want) {
			t.Errorf("merge args %q missing %q", merge, want)
		}
	}
	// A cut from the file's start, or to its end, leaves that side open.
	if got := strings.Join(TrimMKVCutArgs(p, "/m/a.mkv", "/m/cut", trimRange{Stop: 9, HasStop: true}), " "); !strings.Contains(got, "parts:-00:00:09") {
		t.Errorf("no start: %q", got)
	}
}

// mkvmerge can't split FLAC, so step 1 re-encodes each FLAC track beside the video, step 2 leaves
// it out, and step 3 puts it back in place with its own name, language, flags and tags.
func TestTrimMKVArgsReencodeFLACBesideTheVideo(t *testing.T) {
	p := Probe{Streams: []Stream{
		{Index: 0, CodecType: "video", CodecName: "h264", Disposition: map[string]int{"default": 1}},
		{Index: 1, CodecType: "audio", CodecName: "ac3"},
		{Index: 2, CodecType: "audio", CodecName: "flac", Tags: map[string]string{"title": "Commentary", "language": "jpn"}, Disposition: map[string]int{"forced": 1}},
		{Index: 3, CodecType: "attachment"},
		{Index: 4, CodecType: "subtitle"},
		{Index: 5, CodecType: "audio", CodecName: "flac"},
	}}
	r := trimRange{Seek: 1.5, HasSeek: true, Stop: 9, HasStop: true}
	args, err := TrimMKVEncodeArgs(p, map[string]bool{"libx264": true}, "/m/a.mkv", "/m/enc", r)
	if err != nil {
		t.Fatal(err)
	}
	enc := strings.Join(args, " ")
	for _, want := range []string{"-map 0:0 -map 0:2 -map 0:5 ", "-c:a flac -strict:a experimental"} {
		if !strings.Contains(enc, want) {
			t.Errorf("encode args %q missing %q", enc, want)
		}
	}
	if strings.Contains(enc, "-map 0:1 ") {
		t.Errorf("encode args map the AC3 track: %q", enc)
	}
	if otherTracks(p) != 2 {
		t.Errorf("otherTracks = %d, want 2 (AC3 and the subtitle)", otherTracks(p))
	}
	// The attachment isn't a track, so the second FLAC track is mkvmerge track 4.
	cut := strings.Join(TrimMKVCutArgs(p, "/m/a.mkv", "/m/cut", r), " ")
	if !strings.Contains(cut, "-d !0 -a !2,4 ") {
		t.Errorf("cut args %q don't leave the video and both FLAC tracks out", cut)
	}
	merge := strings.Join(TrimMKVMergeArgs(p, "/m/cut", true, "/m/enc", []string{"", "/m/t2.xml", ""}, "", "/m/dst"), " ")
	for _, want := range []string{
		"--track-order 1:0,0:0,1:1,0:1,1:2",
		"--track-name 1:Commentary --language 1:jpn --default-track-flag 1:0 --forced-display-flag 1:1 --tags 1:" + toolArg("/m/t2.xml"),
		"--default-track-flag 2:0 --forced-display-flag 2:0 " + toolArg("/m/enc"),
	} {
		if !strings.Contains(merge, want) {
			t.Errorf("merge args %q missing %q", merge, want)
		}
	}
	// Video and FLAC alone: nothing to cut, so the original contributes no tracks at all.
	only := Probe{Streams: []Stream{p.Streams[0], p.Streams[2]}}
	if otherTracks(only) != 0 {
		t.Fatalf("otherTracks = %d", otherTracks(only))
	}
	base := strings.Join(TrimMKVMergeArgs(only, "/m/a.mkv", false, "/m/enc", []string{"", ""}, "", "/m/dst"), " ")
	if !strings.Contains(base, "-D -A "+toolArg("/m/a.mkv")) || !strings.Contains(base, "--track-order 1:0,1:1") {
		t.Errorf("merge args %q", base)
	}
	// No FLAC: no audio codec option, no -a in the cut.
	plain := Probe{Streams: []Stream{p.Streams[0], p.Streams[1]}}
	if args, _ := TrimMKVEncodeArgs(plain, map[string]bool{"libx264": true}, "/m/a.mkv", "/m/enc", r); strings.Contains(strings.Join(args, " "), "-c:a") {
		t.Errorf("encode args without FLAC: %q", args)
	}
	if got := strings.Join(TrimMKVCutArgs(plain, "/m/a.mkv", "/m/cut", r), " "); strings.Contains(got, " -a ") {
		t.Errorf("cut args without FLAC: %q", got)
	}
}

func TestTrimMP4ArgsSeekEveryStreamAndKeepPriorPacketsOut(t *testing.T) {
	p := Probe{Streams: []Stream{{Index: 0, CodecType: "video", CodecName: "h264"}, {Index: 1, CodecType: "audio"}, {Index: 2, CodecType: "data", CodecTag: "text"}}}
	r := trimRange{Seek: 1, HasSeek: true, Stop: 2, HasStop: true}
	args, err := TrimMP4Args(p, map[string]bool{"libx264": true}, "mp4", "/m/a.mp4", "/m/b", "/tmp/ch", r)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(args, " ")
	for _, want := range []string{"-ss 1.000000 -to 2.000000", "-c copy -copypriorss 0", "-map_chapters 1", "-f ffmetadata -i " + toolArg("/tmp/ch")} {
		if !strings.Contains(got, want) {
			t.Errorf("args %q missing %q", got, want)
		}
	}
	if strings.Contains(got, "-map 0:2") {
		t.Errorf("the chapter track is regenerated, never copied: %q", got)
	}
}
