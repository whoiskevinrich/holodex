package maintenance

import (
	"fmt"
	"math"
	"strconv"
)

// Refusal reasons a preflight reports. Each maps to one line of copy in the handoff (§3), so
// they are stable codes, not prose.
const (
	ReasonNoVideo        = "no_video"
	ReasonCodec          = "unsupported_codec"
	ReasonEncoderMissing = "encoder_missing"
	ReasonDolbyVision    = "dolby_vision"
)

// RefusalError is a preflight refusal: the operation cannot succeed on this file, so nothing
// is queued. Detail is for logs and the owner's dialog (e.g. the codec name).
type RefusalError struct {
	Reason string
	Detail string
}

func (e *RefusalError) Error() string {
	if e.Detail == "" {
		return "maintenance refused: " + e.Reason
	}
	return "maintenance refused: " + e.Reason + " (" + e.Detail + ")"
}

func refuse(reason, detail string) error { return &RefusalError{Reason: reason, Detail: detail} }

// encoderRow is one row of the fixed encoder table (file-maintenance.md, "A fixed encoder
// table"). The table is code, not config: each row is proven against the verifier.
type encoderRow struct {
	encoder string
	args    []string // rate control and speed, appended after -c:<spec>
}

var encoderTable = map[string]encoderRow{
	"h264": {"libx264", []string{"-preset", "slow", "-crf", "16"}},
	"hevc": {"libx265", []string{"-preset", "slow", "-crf", "18"}},
	"vp9":  {"libvpx-vp9", []string{"-row-mt", "1", "-crf", "20", "-b:v", "0"}},
	"av1":  {"libsvtav1", []string{"-preset", "6", "-crf", "22"}},
}

const (
	sideDataDoVi       = "DOVI configuration record"
	sideDataMastering  = "Mastering display metadata"
	sideDataLightLevel = "Content light level metadata"
)

// EncoderArgs returns the ffmpeg output options that re-encode s in its own codec, for the
// output stream spec (e.g. "v:0"). available is the set of encoders the installed ffmpeg
// reports; a codec outside the table, a missing encoder, or a Dolby Vision stream is refused.
// Pixel format and colour tags are pinned to the source; frame timing passes through.
func EncoderArgs(s Stream, spec string, available map[string]bool) ([]string, error) {
	for _, sd := range s.SideData {
		if sd.Type == sideDataDoVi {
			return nil, refuse(ReasonDolbyVision, s.CodecName)
		}
	}
	row, ok := encoderTable[s.CodecName]
	if !ok {
		return nil, refuse(ReasonCodec, s.CodecName)
	}
	if !available[row.encoder] {
		return nil, refuse(ReasonEncoderMissing, row.encoder)
	}
	args := []string{"-c:" + spec, row.encoder}
	args = append(args, row.args...)
	if s.PixFmt != "" {
		args = append(args, "-pix_fmt:"+spec, s.PixFmt)
	}
	for _, c := range []struct{ flag, val string }{
		{"-color_primaries", s.ColorPrimaries},
		{"-color_trc", s.ColorTransfer},
		{"-colorspace", s.ColorSpace},
		{"-color_range", s.ColorRange},
	} {
		if c.val != "" && c.val != "unknown" {
			args = append(args, c.flag+":"+spec, c.val)
		}
	}
	args = append(args, "-fps_mode:"+spec, "passthrough")
	if s.CodecName == "hevc" {
		if p := x265HDRParams(s.SideData); p != "" {
			args = append(args, "-x265-params:"+spec, p)
		}
	}
	return args, nil
}

// x265HDRParams carries HDR10 static metadata through a libx265 re-encode: mastering display
// primaries in units of 0.00002, luminance in units of 0.0001, and MaxCLL/MaxFALL.
func x265HDRParams(sides []SideData) string {
	var md, cl *SideData
	for i := range sides {
		switch sides[i].Type {
		case sideDataMastering:
			md = &sides[i]
		case sideDataLightLevel:
			cl = &sides[i]
		}
	}
	out := ""
	if md != nil {
		c := func(v string) string { return scaled(v, 50000) }
		l := func(v string) string { return scaled(v, 10000) }
		out = fmt.Sprintf("hdr10=1:repeat-headers=1:master-display=G(%s,%s)B(%s,%s)R(%s,%s)WP(%s,%s)L(%s,%s)",
			c(md.GreenX), c(md.GreenY), c(md.BlueX), c(md.BlueY), c(md.RedX), c(md.RedY),
			c(md.WhitePointX), c(md.WhitePointY), l(md.MaxLuminance), l(md.MinLuminance))
	}
	if cl != nil {
		if out != "" {
			out += ":"
		}
		out += "max-cll=" + strconv.Itoa(cl.MaxContent) + "," + strconv.Itoa(cl.MaxAverage)
	}
	return out
}

// scaled turns an ffprobe "num/den" rational into an integer count of 1/unit steps.
func scaled(v string, unit float64) string {
	num, den, ok := parseRatio(v)
	if !ok {
		return "0"
	}
	return strconv.FormatInt(int64(math.Round(num/den*unit)), 10)
}
