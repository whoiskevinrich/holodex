package maintenance

import (
	"errors"
	"testing"
)

// baseSnapshot is a Matroska-shaped file: 24 fps video at index 1 after audio, a subtitle,
// an attachment, global and track tags, two chapters.
func baseSnapshot() Snapshot {
	return Snapshot{
		Family: FamilyMKV,
		Probe: Probe{
			Streams: []Stream{
				{Index: 0, CodecType: "audio", CodecName: "opus", Disposition: map[string]int{"default": 1},
					Tags: map[string]string{"language": "eng", "BPS-eng": "128000", "_STATISTICS_WRITING_DATE_UTC-eng": "2020-01-01 00:00:00"}},
				{Index: 1, CodecType: "video", CodecName: "hevc", Width: 1920, Height: 1080, PixFmt: "yuv420p10le",
					ColorPrimaries: "bt2020", ColorTransfer: "smpte2084", ColorSpace: "bt2020nc", ColorRange: "tv",
					RFrameRate: "24/1", Disposition: map[string]int{"default": 1},
					Tags: map[string]string{"title": "Main", "BPS-eng": "8000000", "NUMBER_OF_BYTES-eng": "1000",
						"_STATISTICS_WRITING_APP-eng": "mkvmerge v80"}},
				{Index: 2, CodecType: "subtitle", CodecName: "subrip", Tags: map[string]string{"language": "jpn"}},
				{Index: 3, CodecType: "attachment", CodecName: "ttf", Tags: map[string]string{"filename": "font.ttf"}},
			},
			Chapters: []Chapter{
				{StartTime: "0.000000", EndTime: "60.000000", Tags: map[string]string{"title": "One"}},
				{StartTime: "60.000000", EndTime: "120.000000", Tags: map[string]string{"title": "Two"}},
			},
			Format: Format{FormatName: "matroska,webm", Duration: "120.000000",
				Tags: map[string]string{"TITLE": "Example", "PUBLISHER": "Acme", "ENCODER": "libebml v1"}},
		},
		StreamHashes: map[int]string{0: "a0", 1: "v-orig", 2: "s0"},
		Extra:        map[string]string{"attachment:font.ttf": "application/x-truetype-font|1234"},
	}
}

// candidate is a faithful re-encode of base: new picture bytes, recomputed statistics on
// the video track, new mux stamps everywhere, a new container writing app.
func candidate() Snapshot {
	c := baseSnapshot()
	c.StreamHashes[1] = "v-new"
	c.Probe.Streams[1].Tags["BPS-eng"] = "9100000"
	c.Probe.Streams[1].Tags["NUMBER_OF_BYTES-eng"] = "1138"
	c.Probe.Streams[1].Tags["_STATISTICS_WRITING_APP-eng"] = "mkvmerge v88"
	c.Probe.Streams[0].Tags["_STATISTICS_WRITING_DATE_UTC-eng"] = "2026-10-11 00:00:00"
	c.Probe.Format.Tags["ENCODER"] = "libebml v2"
	c.Probe.Format.Duration = "120.030000" // under one 24 fps frame
	return c
}

func TestVerify_FaithfulCandidatePasses(t *testing.T) {
	if err := Verify(baseSnapshot(), candidate()); err != nil {
		t.Fatalf("faithful candidate refused: %v", err)
	}
}

func TestVerify_EachRuleRefuses(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Snapshot)
		rule   string
	}{
		{"dropped stream", func(c *Snapshot) { c.Probe.Streams = c.Probe.Streams[:3] }, RuleStreams},
		{"reordered streams", func(c *Snapshot) {
			c.Probe.Streams[0], c.Probe.Streams[2] = c.Probe.Streams[2], c.Probe.Streams[0]
		}, RuleStreams},
		{"default flag changed", func(c *Snapshot) { c.Probe.Streams[0].Disposition["default"] = 0 }, RuleStreams},
		{"copied stream bytes changed", func(c *Snapshot) { c.StreamHashes[0] = "a-different" }, RuleCopied},
		{"copied stream digest missing", func(c *Snapshot) { delete(c.StreamHashes, 2) }, RuleCopied},
		{"container tag missing", func(c *Snapshot) { delete(c.Probe.Format.Tags, "PUBLISHER") }, RuleTags},
		{"container tag changed", func(c *Snapshot) { c.Probe.Format.Tags["TITLE"] = "Other" }, RuleTags},
		{"container tag added", func(c *Snapshot) { c.Probe.Format.Tags["COMMENT"] = "x" }, RuleTags},
		{"stream tag changed", func(c *Snapshot) { c.Probe.Streams[2].Tags["language"] = "eng" }, RuleTags},
		{"video track's own tag lost", func(c *Snapshot) { delete(c.Probe.Streams[1].Tags, "title") }, RuleTags},
		{"encoder tag added to the picture", func(c *Snapshot) { c.Probe.Streams[1].Tags["encoder"] = "Lavc libx265" }, RuleTags},
		{"statistics changed on a COPIED track", func(c *Snapshot) { c.Probe.Streams[0].Tags["BPS-eng"] = "1" }, RuleTags},
		{"attachment changed", func(c *Snapshot) { c.Extra["attachment:font.ttf"] = "application/x-truetype-font|9" }, RuleTags},
		{"chapter missing", func(c *Snapshot) { c.Probe.Chapters = c.Probe.Chapters[:1] }, RuleChapters},
		{"chapter moved", func(c *Snapshot) { c.Probe.Chapters[1].StartTime = "61.000000" }, RuleChapters},
		{"chapter renamed", func(c *Snapshot) { c.Probe.Chapters[0].Tags["title"] = "Uno" }, RuleChapters},
		{"width changed", func(c *Snapshot) { c.Probe.Streams[1].Width = 1918 }, RulePicture},
		{"pixel format changed", func(c *Snapshot) { c.Probe.Streams[1].PixFmt = "yuv420p" }, RulePicture},
		{"colour primaries changed", func(c *Snapshot) { c.Probe.Streams[1].ColorPrimaries = "bt709" }, RulePicture},
		{"frame rate changed", func(c *Snapshot) { c.Probe.Streams[1].RFrameRate = "25/1" }, RulePicture},
		{"two frames longer", func(c *Snapshot) { c.Probe.Format.Duration = "120.084000" }, RuleDuration},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cand := candidate()
			c.mutate(&cand)
			err := Verify(baseSnapshot(), cand)
			var re *RuleError
			if !errors.As(err, &re) || re.Rule != c.rule {
				t.Fatalf("err = %v, want rule %q", err, c.rule)
			}
		})
	}
}

// A variable-frame-rate source reports a timebase-like r_frame_rate; the tolerance must come
// from the average frame, or every VFR flip is refused.
func TestVerify_VFRToleranceUsesAverageFrame(t *testing.T) {
	orig := baseSnapshot()
	orig.Probe.Streams[1].RFrameRate = "90000/1"
	orig.Probe.Streams[1].AvgFrameRate = "30/1"
	cand := candidate()
	cand.Probe.Streams[1].RFrameRate = "90000/1"
	cand.Probe.Streams[1].AvgFrameRate = "30/1"
	cand.Probe.Format.Duration = "120.020000"
	if err := Verify(orig, cand); err != nil {
		t.Fatalf("20 ms on a 30 fps VFR file refused: %v", err)
	}
}

// In MP4 `encoder` is the iTunes ©too tag, not a mux stamp: losing it is a tags failure (the
// exemption is Matroska-only).
func TestVerify_MP4EncoderTagIsARealTag(t *testing.T) {
	orig := baseSnapshot()
	orig.Family = FamilyMP4
	orig.Probe.Format.Tags["encoder"] = "Lavf63.1.102"
	cand := candidate()
	cand.Family = FamilyMP4
	cand.Probe.Format.Tags["ENCODER"] = "libebml v1" // keep the base's own key equal
	var re *RuleError
	if err := Verify(orig, cand); !errors.As(err, &re) || re.Rule != RuleTags {
		t.Fatalf("an MP4 that lost its encoder tag passed: %v", err)
	}
}

// ffmpeg writes its own ftyp, so an MP4's brands may differ (spec rule 4's second exception) —
// but only in MP4, and only those three keys.
func TestVerify_MP4FileTypeBrandsMayDiffer(t *testing.T) {
	orig, cand := baseSnapshot(), candidate()
	orig.Family, cand.Family = FamilyMP4, FamilyMP4
	cand.Probe.Format.Tags["ENCODER"] = orig.Probe.Format.Tags["ENCODER"] // isolate the brands
	orig.Probe.Format.Tags["major_brand"], cand.Probe.Format.Tags["major_brand"] = "mp42", "isom"
	orig.Probe.Format.Tags["minor_version"], cand.Probe.Format.Tags["minor_version"] = "1", "512"
	orig.Probe.Format.Tags["compatible_brands"] = "isomavc1iso2"
	cand.Probe.Format.Tags["compatible_brands"] = "isomiso2avc1mp41"
	if err := Verify(orig, cand); err != nil {
		t.Fatalf("differing ftyp brands failed an MP4: %v", err)
	}
	orig.Family = FamilyMKV
	var re *RuleError
	if err := Verify(orig, cand); !errors.As(err, &re) || re.Rule != RuleTags {
		t.Fatalf("the brand exemption reached Matroska: %v", err)
	}
}

// In MP4 the re-encoded stream's `encoder` is its CompressorName, which ffmpeg always fills
// (spec rule 4's third exception) — but only on that stream, and only in MP4.
func TestVerify_MP4ReencodedCompressorNameMayChange(t *testing.T) {
	orig, cand := baseSnapshot(), candidate()
	orig.Family, cand.Family = FamilyMP4, FamilyMP4
	cand.Probe.Format.Tags["ENCODER"] = orig.Probe.Format.Tags["ENCODER"] // isolate the stream tag
	cand.Probe.Streams[1].Tags["encoder"] = "Lavc libx265"
	if err := Verify(orig, cand); err != nil {
		t.Fatalf("a compressor name on the re-encoded MP4 stream failed: %v", err)
	}
	var re *RuleError
	cand.Probe.Streams[2].Tags["encoder"] = "Lavc mov_text"
	if err := Verify(orig, cand); !errors.As(err, &re) || re.Rule != RuleTags {
		t.Fatalf("the exemption reached a copied stream: %v", err)
	}
	delete(cand.Probe.Streams[2].Tags, "encoder")
	orig.Family = FamilyMKV
	if err := Verify(orig, cand); !errors.As(err, &re) || re.Rule != RuleTags {
		t.Fatalf("the exemption reached Matroska: %v", err)
	}
}

// The MP4 shape: cover art is a video stream before the real one, and must count as copied.
func TestVerify_CoverArtIsACopiedStream(t *testing.T) {
	orig := Snapshot{
		Probe: Probe{Streams: []Stream{
			{Index: 0, CodecType: "video", CodecName: "mjpeg", Disposition: map[string]int{"attached_pic": 1}},
			{Index: 1, CodecType: "video", CodecName: "h264", Width: 320, Height: 240, RFrameRate: "25/1"},
		}, Format: Format{Duration: "2.000000"}},
		StreamHashes: map[int]string{0: "cover", 1: "v"},
	}
	cand := orig
	cand.StreamHashes = map[int]string{0: "cover-flipped", 1: "v2"}
	var re *RuleError
	if err := Verify(orig, cand); !errors.As(err, &re) || re.Rule != RuleCopied {
		t.Fatalf("a flipped cover art passed: %v", err)
	}
}
