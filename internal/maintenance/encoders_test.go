package maintenance

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

var allEncoders = map[string]bool{"libx264": true, "libx265": true, "libvpx-vp9": true, "libsvtav1": true}

func TestEncoderArgs_TableRowsAndColourPins(t *testing.T) {
	for codec, enc := range map[string]string{"h264": "libx264", "hevc": "libx265", "vp9": "libvpx-vp9", "av1": "libsvtav1"} {
		s := Stream{CodecName: codec, PixFmt: "yuv420p10le", ColorPrimaries: "bt2020",
			ColorTransfer: "smpte2084", ColorSpace: "bt2020nc", ColorRange: "tv"}
		args, err := EncoderArgs(s, "v:0", allEncoders)
		if err != nil {
			t.Fatalf("%s: %v", codec, err)
		}
		got := strings.Join(args, " ")
		for _, want := range []string{"-c:v:0 " + enc, "-pix_fmt:v:0 yuv420p10le", "-color_primaries:v:0 bt2020",
			"-color_trc:v:0 smpte2084", "-colorspace:v:0 bt2020nc", "-color_range:v:0 tv", "-fps_mode:v:0 passthrough"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: args %q lack %q", codec, got, want)
			}
		}
		if strings.Contains(got, " -r ") {
			t.Errorf("%s: forces a frame rate: %q", codec, got)
		}
	}
}

func TestEncoderArgs_UnknownColourIsNotPinned(t *testing.T) {
	args, err := EncoderArgs(Stream{CodecName: "h264", ColorPrimaries: "unknown"}, "v:1", allEncoders)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(args, "-color_primaries:v:1") {
		t.Errorf("pinned an unknown colour: %v", args)
	}
}

func TestEncoderArgs_HDR10ParamsFromSideData(t *testing.T) {
	s := Stream{CodecName: "hevc", SideData: []SideData{
		{Type: sideDataMastering, RedX: "34000/50000", RedY: "16000/50000", GreenX: "13250/50000", GreenY: "34500/50000",
			BlueX: "7500/50000", BlueY: "3000/50000", WhitePointX: "15635/50000", WhitePointY: "16450/50000",
			MinLuminance: "50/10000", MaxLuminance: "10000000/10000"},
		{Type: sideDataLightLevel, MaxContent: 1000, MaxAverage: 400},
	}}
	args, err := EncoderArgs(s, "v:0", allEncoders)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.Index(args, "-x265-params:v:0")
	if i < 0 {
		t.Fatalf("no x265 params: %v", args)
	}
	want := "hdr10=1:repeat-headers=1:master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(10000000,50):max-cll=1000,400"
	if args[i+1] != want {
		t.Errorf("x265-params = %q\nwant          %q", args[i+1], want)
	}
}

func TestEncoderArgs_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		s      Stream
		avail  map[string]bool
		reason string
	}{
		{"codec outside the table", Stream{CodecName: "mpeg2video"}, allEncoders, ReasonCodec},
		{"encoder not in this ffmpeg", Stream{CodecName: "av1"}, map[string]bool{"libx264": true}, ReasonEncoderMissing},
		{"dolby vision", Stream{CodecName: "hevc", SideData: []SideData{{Type: sideDataDoVi}}}, allEncoders, ReasonDolbyVision},
	}
	for _, c := range cases {
		_, err := EncoderArgs(c.s, "v:0", c.avail)
		var re *RefusalError
		if !errors.As(err, &re) || re.Reason != c.reason {
			t.Errorf("%s: err = %v, want refusal %q", c.name, err, c.reason)
		}
	}
}
