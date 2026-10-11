package maintenance

import (
	"strconv"
	"strings"
)

// Op is one maintenance operation. Each names the ffmpeg video filter it applies and the
// verifier rules it relaxes (none for a flip: rule 6 holds in full).
type Op string

// OpHFlip mirrors the picture left to right (F77 v1).
const OpHFlip Op = "hflip"

var opFilters = map[Op]string{OpHFlip: "hflip"}

// Filter returns op's ffmpeg filter, or "" for an unknown op.
func (o Op) Filter() string { return opFilters[o] }

// Container families maintenance supports. The scanner admits only .mp4 and .mkv
// (scanner.mediaExts), so these are the whole surface.
const (
	FamilyMP4 = "mp4"
	FamilyMKV = "mkv"
)

// inputArgs opens src as the file it claims to be: no protocol but `file`, the demuxer
// forced, stdin closed. A playlist named .mkv can't make ffmpeg fetch anything.
func inputArgs(demuxer, src string) []string {
	return []string{"-nostdin", "-y", "-loglevel", "error",
		"-protocol_whitelist", "file", "-f", demuxer, "-i", toolArg(src)}
}

// MP4Args builds the single ffmpeg pass for an MP4/MOV: every stream copied, the main video
// stream re-encoded and filtered, container and chapter metadata mapped. format is "mp4" or
// "mov", matching the source extension. exiftool restores the remaining tags afterwards.
// The encoder args are built here, for the same stream spec the filter targets, so the two
// can never disagree (cover art is a video stream too).
func MP4Args(p Probe, op Op, available map[string]bool, format, src, dst string) ([]string, error) {
	v, ok := p.MainVideo()
	if !ok {
		return nil, refuse(ReasonNoVideo, "")
	}
	spec := "v:" + strconv.Itoa(p.VideoOrdinal(v))
	enc, err := EncoderArgs(v, spec, available)
	if err != nil {
		return nil, err
	}
	args := inputArgs("mov", src)
	for _, s := range p.Streams {
		// The QuickTime chapter track is regenerated, in place, by -map_chapters; mapping the
		// source's copy too writes it twice (seen with two or more chapters).
		if len(p.Chapters) > 0 && isQTChapterTrack(s) {
			continue
		}
		args = append(args, "-map", "0:"+strconv.Itoa(s.Index))
	}
	args = append(args, "-c", "copy")
	args = append(args, enc...)
	args = append(args, "-filter:"+spec, op.Filter())
	if e := v.Tags["encoder"]; e != "" {
		// ffmpeg only stamps its own encoder name when the stream carries none.
		args = append(args, "-metadata:s:"+spec, "encoder="+e)
	}
	return append(args, "-map_metadata", "0", "-map_chapters", "0", "-fflags", "+bitexact",
		"-movflags", "+faststart", "-f", format, toolArg(dst)), nil
}

// MKVEncodeArgs builds step 1 for a Matroska file: only the main video track, re-encoded and
// filtered, into an intermediate carrying no tags or chapters of its own.
func MKVEncodeArgs(p Probe, op Op, available map[string]bool, src, intermediate string) ([]string, error) {
	v, ok := p.MainVideo()
	if !ok {
		return nil, refuse(ReasonNoVideo, "")
	}
	enc, err := EncoderArgs(v, "v:0", available)
	if err != nil {
		return nil, err
	}
	args := inputArgs("matroska", src)
	args = append(args, "-map", "0:"+strconv.Itoa(v.Index))
	args = append(args, enc...)
	return append(args, "-filter:v:0", op.Filter(), "-map_metadata", "-1", "-map_chapters", "-1",
		"-fflags", "+bitexact", "-f", "matroska", toolArg(intermediate)), nil
}

// MKVMergeArgs builds step 2: dst is the original with its main video track removed plus the
// intermediate's track, put back in the original's position with the original's name,
// language and flags. Everything else (tracks, attachments, chapters, global and track tags)
// comes from the original byte for byte; mkvmerge regenerates the statistics tags. The
// original video track's own tags vanish with the removed track, so trackTags (from
// PrepareTrackTags; "" when there are none) carries them onto the new one. p must have a
// main video stream (MKVEncodeArgs refuses one without, and runs first).
func MKVMergeArgs(p Probe, src, intermediate, trackTags, dst string) []string {
	v, _ := p.MainVideo()
	vid := mkvTrackID(p, v)
	var order []string
	for _, s := range p.Streams {
		if s.CodecType == "attachment" {
			continue
		}
		if tid := mkvTrackID(p, s); tid == vid {
			order = append(order, "1:0")
		} else {
			order = append(order, "0:"+strconv.Itoa(tid))
		}
	}
	args := []string{"-o", toolArg(dst), "-d", "!" + strconv.Itoa(vid), toolArg(src),
		"--no-global-tags", "--no-track-tags", "--no-chapters", "--no-attachments"}
	if name := v.Tags["title"]; name != "" {
		args = append(args, "--track-name", "0:"+name)
	}
	if lang := v.Tags["language"]; lang != "" {
		args = append(args, "--language", "0:"+lang)
	}
	args = append(args,
		"--default-track-flag", "0:"+flag(v.Disposition["default"]),
		"--forced-display-flag", "0:"+flag(v.Disposition["forced"]))
	if trackTags != "" {
		args = append(args, "--tags", "0:"+toolArg(trackTags))
	}
	return append(args, toolArg(intermediate), "--track-order", strings.Join(order, ","))
}

// isQTChapterTrack reports whether s is an MP4/MOV chapter text track: a data stream whose
// sample entry is 'text' (a subtitle track is 'tx3g' and typed subtitle).
func isQTChapterTrack(s Stream) bool { return s.CodecType == "data" && s.CodecTag == "text" }

// mkvTrackID is s's mkvmerge track id: its position among the file's tracks. ffprobe lists
// Matroska attachments as streams too, but they are not tracks.
func mkvTrackID(p Probe, s Stream) int {
	n := 0
	for _, st := range p.Streams {
		if st.Index == s.Index {
			return n
		}
		if st.CodecType != "attachment" {
			n++
		}
	}
	return -1
}

func flag(v int) string {
	if v != 0 {
		return "1"
	}
	return "0"
}
