package maintenance

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// coverFirstMP4: cover art (attached_pic) precedes the real video, so the filter must target
// v:1, never v:0.
var coverFirstMP4 = Probe{Streams: []Stream{
	{Index: 0, CodecType: "video", CodecName: "mjpeg", Disposition: map[string]int{"attached_pic": 1}},
	{Index: 1, CodecType: "video", CodecName: "h264", Tags: map[string]string{"encoder": "Old Encoder"}},
	{Index: 2, CodecType: "audio", CodecName: "aac"},
}}

// m returns an OS-absolute path, so toolArg passes it through on Windows and Unix alike.
func m(name string) string { return filepath.Join(os.TempDir(), "m", name) }

func TestMP4Args_TargetsTheMainVideoNotCoverArt(t *testing.T) {
	args, err := MP4Args(coverFirstMP4, OpHFlip, allEncoders, "mp4", m("a.mp4"), m("a.mp4.holodex-new"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(args, " ")
	for _, want := range []string{
		"-nostdin", "-protocol_whitelist file -f mov -i " + m("a.mp4"),
		"-map 0:0 -map 0:1 -map 0:2 -c copy -c:v:1 libx264", "-filter:v:1 hflip",
		"-metadata:s:v:1 encoder=Old Encoder",
		"-map_metadata 0 -map_chapters 0 -fflags +bitexact -movflags +faststart -f mp4 " + m("a.mp4.holodex-new"),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("argv lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "-filter:v:0") {
		t.Errorf("filter targets cover art: %s", got)
	}
}

// A stream with no encoder tag gets an empty one set, or ffmpeg stamps "Lavc libx264" on it
// and every flip of such a file fails verification (rule 4).
func TestMP4Args_NoEncoderTagStaysAbsent(t *testing.T) {
	p := Probe{Streams: []Stream{{Index: 0, CodecType: "video", CodecName: "h264"}}}
	args, err := MP4Args(p, OpHFlip, allEncoders, "mp4", m("a.mp4"), m("b"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "-metadata:s:v:0 encoder= ") {
		t.Errorf("argv lets ffmpeg stamp its encoder: %v", args)
	}
}

// A track `name` (udta atom) is written by the muxer only from `title`, addressed by output
// index: the skipped chapter track shifts it.
func TestMP4Args_TrackNameCarriedAsTitle(t *testing.T) {
	p := Probe{Streams: []Stream{
		{Index: 0, CodecType: "data", CodecName: "bin_data", CodecTag: "text"},
		{Index: 1, CodecType: "video", CodecName: "h264"},
		{Index: 2, CodecType: "audio", CodecName: "aac", Tags: map[string]string{"name": "Stereo"}},
	}, Chapters: []Chapter{{StartTime: "0"}}}
	args, err := MP4Args(p, OpHFlip, allEncoders, "mp4", m("a.mp4"), m("b"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "-metadata:s:1 title=Stereo") {
		t.Errorf("track name not carried to output stream 1: %v", args)
	}
}

func TestMP4Args_ChapterTrackIsRegeneratedNotCopied(t *testing.T) {
	p := Probe{Streams: []Stream{
		{Index: 0, CodecType: "video", CodecName: "h264"},
		{Index: 1, CodecType: "subtitle", CodecName: "mov_text", CodecTag: "tx3g"},
		{Index: 2, CodecType: "data", CodecName: "bin_data", CodecTag: "text"},
		{Index: 3, CodecType: "data", CodecName: "bin_data", CodecTag: "tmcd"},
	}, Chapters: []Chapter{{StartTime: "0"}}}
	args, err := MP4Args(p, OpHFlip, allEncoders, "mp4", m("a.mp4"), m("b"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(args, " ")
	if !strings.Contains(got, "-map 0:0 -map 0:1 -map 0:3 -c copy") || !strings.Contains(got, "-map_chapters 0") {
		t.Errorf("chapter track mapped, or another stream dropped: %s", got)
	}
	// Without chapters, a text data track is ordinary data and is copied.
	p.Chapters = nil
	args, _ = MP4Args(p, OpHFlip, allEncoders, "mp4", m("a.mp4"), m("b"))
	if !strings.Contains(strings.Join(args, " "), "-map 0:2") {
		t.Errorf("text data track dropped from a file with no chapters: %v", args)
	}
}

// Video is not track 0, and an attachment sits among the streams: track ids skip it, and the
// new track goes back where the old one was.
var audioFirstMKV = Probe{Streams: []Stream{
	{Index: 0, CodecType: "audio", CodecName: "opus"},
	{Index: 1, CodecType: "video", CodecName: "h264", Tags: map[string]string{"title": "Main", "language": "jpn"},
		Disposition: map[string]int{"default": 1}},
	{Index: 2, CodecType: "subtitle", CodecName: "subrip"},
	{Index: 3, CodecType: "attachment", CodecName: "ttf"},
}}

func TestMKVEncodeArgs_OnlyTheMainVideo(t *testing.T) {
	args, err := MKVEncodeArgs(audioFirstMKV, OpHFlip, allEncoders, m("a.mkv"), m("a.mkv.holodex-enc"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(args, " ")
	for _, want := range []string{"-f matroska -i " + m("a.mkv"), "-map 0:1 -c:v:0 libx264", "-filter:v:0 hflip",
		"-map_metadata -1 -map_chapters -1", "-f matroska " + m("a.mkv.holodex-enc")} {
		if !strings.Contains(got, want) {
			t.Errorf("argv lacks %q:\n%s", want, got)
		}
	}
}

func TestMKVMergeArgs_RestoresPositionAndProperties(t *testing.T) {
	args := MKVMergeArgs(audioFirstMKV, m("a.mkv"), m("a.mkv.holodex-enc"), m("tags.xml"), m("a.mkv.holodex-new"))
	want := []string{"-o", m("a.mkv.holodex-new"), "--no-date", "--disable-track-statistics-tags", "-d", "!1", m("a.mkv"),
		"--no-global-tags", "--no-track-tags", "--no-chapters", "--no-attachments",
		"--track-name", "0:Main", "--language", "0:jpn",
		"--default-track-flag", "0:1", "--forced-display-flag", "0:0",
		"--tags", "0:" + m("tags.xml"),
		m("a.mkv.holodex-enc"), "--track-order", "0:0,1:0,0:2"}
	if !slices.Equal(args, want) {
		t.Errorf("argv\n got %q\nwant %q", args, want)
	}
	// A file mkvmerge wrote keeps its statistics: they are regenerated, not disabled.
	withStats := audioFirstMKV
	withStats.Streams = append([]Stream(nil), audioFirstMKV.Streams...)
	withStats.Streams[0].Tags = map[string]string{"_STATISTICS_TAGS-eng": "BPS DURATION"}
	if slices.Contains(MKVMergeArgs(withStats, m("a.mkv"), m("e"), "", m("n")), "--disable-track-statistics-tags") {
		t.Error("disabled statistics on a file that has them")
	}
	// No track tags of its own: no --tags at all.
	if args := MKVMergeArgs(audioFirstMKV, m("a.mkv"), m("e"), "", m("n")); slices.Contains(args, "--tags") {
		t.Errorf("--tags without a file: %q", args)
	}
}

func TestArgs_NoVideoIsRefused(t *testing.T) {
	audioOnly := Probe{Streams: []Stream{{Index: 0, CodecType: "audio", CodecName: "aac"}}}
	if _, err := MP4Args(audioOnly, OpHFlip, allEncoders, "mp4", m("a.mp4"), m("b")); !isRefusal(err, ReasonNoVideo) {
		t.Errorf("MP4Args err = %v, want no_video refusal", err)
	}
	if _, err := MKVEncodeArgs(audioOnly, OpHFlip, allEncoders, m("a.mkv"), m("b")); !isRefusal(err, ReasonNoVideo) {
		t.Errorf("MKVEncodeArgs err = %v, want no_video refusal", err)
	}
}

func isRefusal(err error, reason string) bool {
	var re *RefusalError
	return errors.As(err, &re) && re.Reason == reason
}

func TestToolArg_RelativePathCannotBeAnOption(t *testing.T) {
	for _, p := range []string{"-y.mkv", "@opts.mkv", "concat:a.mkv"} {
		if got := toolArg(p); strings.HasPrefix(got, "-") || strings.HasPrefix(got, "@") || strings.HasPrefix(got, "concat") {
			t.Errorf("toolArg(%q) = %q", p, got)
		}
	}
}
