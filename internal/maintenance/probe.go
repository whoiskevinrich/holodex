// Package maintenance rewrites a media file's picture and nothing else (F77, HOLODEX-556):
// the per-container re-encode pipeline, the encoder table, the verification gate, and the
// journaled swap. See docs/architecture/file-maintenance.md.
package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Probe is the subset of `ffprobe -show_format -show_streams -show_chapters -of json` the
// encoder table and the verifier read. metadata.Extractor has its own, narrower struct; this
// one carries the colour, disposition, tag and side-data fields a re-encode must preserve.
type Probe struct {
	Streams  []Stream  `json:"streams"`
	Chapters []Chapter `json:"chapters"`
	Format   Format    `json:"format"`
}

// Stream is one ffprobe stream.
type Stream struct {
	Index          int               `json:"index"`
	CodecType      string            `json:"codec_type"`
	CodecName      string            `json:"codec_name"`
	CodecTag       string            `json:"codec_tag_string"`
	Profile        string            `json:"profile"`
	Width          int               `json:"width"`
	Height         int               `json:"height"`
	PixFmt         string            `json:"pix_fmt"`
	ColorRange     string            `json:"color_range"`
	ColorSpace     string            `json:"color_space"`
	ColorTransfer  string            `json:"color_transfer"`
	ColorPrimaries string            `json:"color_primaries"`
	RFrameRate     string            `json:"r_frame_rate"`
	AvgFrameRate   string            `json:"avg_frame_rate"`
	Disposition    map[string]int    `json:"disposition"`
	Tags           map[string]string `json:"tags"`
	SideData       []SideData        `json:"side_data_list"`
}

// SideData is one entry of a stream's side_data_list. Only the fields HDR10 and Dolby
// Vision detection read are named; ffprobe reports rationals as "num/den" strings.
type SideData struct {
	Type         string `json:"side_data_type"`
	RedX         string `json:"red_x"`
	RedY         string `json:"red_y"`
	GreenX       string `json:"green_x"`
	GreenY       string `json:"green_y"`
	BlueX        string `json:"blue_x"`
	BlueY        string `json:"blue_y"`
	WhitePointX  string `json:"white_point_x"`
	WhitePointY  string `json:"white_point_y"`
	MinLuminance string `json:"min_luminance"`
	MaxLuminance string `json:"max_luminance"`
	MaxContent   int    `json:"max_content"`
	MaxAverage   int    `json:"max_average"`
}

// Chapter is one ffprobe chapter.
type Chapter struct {
	StartTime string            `json:"start_time"`
	EndTime   string            `json:"end_time"`
	Tags      map[string]string `json:"tags"`
}

// Format is ffprobe's container section.
type Format struct {
	FormatName string            `json:"format_name"`
	Duration   string            `json:"duration"`
	Tags       map[string]string `json:"tags"`
}

// RunProbe runs ffprobe on path with the demuxer forced ("mov" or "matroska") and only the
// file protocol allowed: a playlist named .mkv must not be able to make ffprobe fetch URLs or
// read the local files it lists.
func RunProbe(ctx context.Context, ffprobe, demuxer, path string) (Probe, error) {
	out, err := run(ctx, ffprobe, "-v", "error", "-protocol_whitelist", "file", "-f", demuxer,
		"-of", "json", "-show_format", "-show_streams", "-show_chapters", toolArg(path))
	if err != nil {
		return Probe{}, err
	}
	var p Probe
	if err := json.Unmarshal(out, &p); err != nil {
		return Probe{}, fmt.Errorf("ffprobe output: %w", err)
	}
	return p, nil
}

// MainVideo returns the stream a picture operation acts on: the first video stream that is
// not embedded cover art. ok is false when the file has none.
func (p Probe) MainVideo() (s Stream, ok bool) {
	for _, st := range p.Streams {
		if st.CodecType == "video" && st.Disposition["attached_pic"] == 0 {
			return st, true
		}
	}
	return Stream{}, false
}

// VideoOrdinal is s's position among the file's video streams (cover art included), the
// number ffmpeg's `v:<n>` stream specifier takes.
func (p Probe) VideoOrdinal(s Stream) int {
	n := 0
	for _, st := range p.Streams {
		if st.Index == s.Index {
			return n
		}
		if st.CodecType == "video" {
			n++
		}
	}
	return -1
}

// FrameDuration is one average frame of s in seconds; 0 when unknown. avg_frame_rate, not
// r_frame_rate: on a variable-frame-rate source r_frame_rate is a timebase-like ceiling
// (e.g. 90000/1) that would shrink the duration tolerance to microseconds.
func (s Stream) FrameDuration() float64 {
	for _, rate := range []string{s.AvgFrameRate, s.RFrameRate} {
		if num, den, ok := parseRatio(rate); ok && num > 0 {
			return den / num
		}
	}
	return 0
}

func parseRatio(v string) (num, den float64, ok bool) {
	a, b, found := strings.Cut(v, "/")
	n, err1 := strconv.ParseFloat(a, 64)
	if !found {
		return n, 1, err1 == nil
	}
	d, err2 := strconv.ParseFloat(b, 64)
	if err1 != nil || err2 != nil || d == 0 {
		return 0, 0, false
	}
	return n, d, true
}

// toolArg keeps a relative path from being read as an option, an @option-file or a
// protocol. Paths here come from the videos row and are absolute, so this is a guard, not a
// transform. Same rule as writeback.ffmpegArg.
func toolArg(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return "." + string(filepath.Separator) + p
}
