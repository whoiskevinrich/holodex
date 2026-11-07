package maintenance

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Trim-only output-contract failures. RuleCutCheck and RuleScreenCheck carry "<first|last>:<frames>"
// as their detail, so the owner's sentence can name the end and the offset.
const (
	RuleCutCheck    = "cut_check"
	RuleScreenCheck = "screen_check"
)

// TrimExpect is what a trim's output must be, beyond keeping everything else. Edition is the new
// edition value on an edition's file ("" on a replace, where the tag can't change).
type TrimExpect struct {
	Chapters []Chapter
	Edition  string
	// VideoSpan is the output video's first-to-last frame distance, and WantSpan the original's
	// between the kept frames. A file's overall duration also counts an audio packet running past
	// the last frame, so rule 7 is measured on the video itself.
	VideoSpan, WantSpan float64
	// CopiedOK is the packet comparison's result per stream index: "" when that stream's packets
	// inside the range match the original's, otherwise what differs.
	CopiedOK map[int]string
}

const xmpToolkitTag = "xmp:XMP-x:XMPToolkit"

// isEditionTag names a file's edition tag in either container's report of it.
func isEditionTag(k string) bool {
	return strings.EqualFold(k, "xmp:XMP-prism:Edition") || hasTagBase(k, []string{"EDITION"})
}

// VerifyTrim is the F77 gate with a trim's three relaxations: duration, chapters and the copied
// streams' extent. Every other rule stands. A variable-frame-rate source's r_frame_rate is a
// ceiling that depends on which frames were kept, so only a constant-rate file holds to it.
func VerifyTrim(orig, cand Snapshot, want TrimExpect) error {
	ov, ok := orig.Probe.MainVideo()
	if !ok {
		return fail(RuleStreams, "original has no video stream")
	}
	cv, ok := cand.Probe.MainVideo()
	if !ok {
		return fail(RuleStreams, "main video stream moved or missing")
	}
	ostreams, cstreams := withoutChapterTrack(orig.Probe.Streams), withoutChapterTrack(cand.Probe.Streams)
	if len(ostreams) != len(cstreams) {
		return fail(RuleStreams, "%d streams (%s), want %d (%s)", len(cstreams), streamKinds(cand.Probe),
			len(ostreams), streamKinds(orig.Probe))
	}
	for i, os := range ostreams {
		cs := cstreams[i]
		if os.CodecType != cs.CodecType {
			return fail(RuleStreams, "stream %d is %s, want %s", i, cs.CodecType, os.CodecType)
		}
		if os.Index == ov.Index && cs.Index != cv.Index {
			return fail(RuleStreams, "main video stream moved or missing")
		}
		if d := diffInts(os.Disposition, cs.Disposition); d != "" {
			return fail(RuleStreams, "stream %d disposition %s", i, d)
		}
	}

	for i, os := range ostreams {
		if os.Index == ov.Index {
			continue
		}
		cs := cstreams[i]
		switch {
		case os.CodecType == "attachment":
			// compared through Extra below
		case os.Disposition["attached_pic"] != 0:
			// Cover art isn't part of the timeline: it is copied whole.
			if w := orig.StreamHashes[os.Index]; w == "" || w != cand.StreamHashes[cs.Index] {
				return fail(RuleCopied, "stream %d (cover art) changed", os.Index)
			}
		default:
			detail, checked := want.CopiedOK[os.Index]
			if !checked {
				return fail(RuleCopied, "stream %d (%s) has no packet comparison", os.Index, os.CodecType)
			}
			if detail != "" {
				return fail(RuleCopied, "stream %d (%s) %s", os.Index, os.CodecType, detail)
			}
		}
	}

	// An edition's file differs in exactly one tag, and that tag must hold the new value. Writing it
	// into a file with no XMP yet makes exiftool create the packet, stamped with its own toolkit:
	// a record of the write, like a mux stamp, not a tag of the file's.
	editionOK := func(k string) bool { return want.Edition != "" && (isEditionTag(k) || k == xmpToolkitTag) }
	containerIgnore := func(k string) bool { return isFileTypeBrand(k) || editionOK(k) }
	if orig.Family == FamilyMKV {
		containerIgnore = func(k string) bool { return isContainerStamp(k) || editionOK(k) }
	}
	if d := diffTags(orig.Probe.Format.Tags, cand.Probe.Format.Tags, containerIgnore); d != "" {
		return fail(RuleTags, "container %s", d)
	}
	for i, os := range ostreams {
		mp4Video := orig.Family == FamilyMP4 && os.Index == ov.Index
		ignore := func(k string) bool { return isMuxStamp(k) || isEncodedStat(k) || mp4Video && k == "encoder" }
		if d := diffTags(os.Tags, cstreams[i].Tags, ignore); d != "" {
			return fail(RuleTags, "stream %d %s", i, d)
		}
	}
	if d := diffTags(orig.Extra, cand.Extra, editionOK); d != "" {
		return fail(RuleTags, "%s", d)
	}
	if want.Edition != "" && !carriesEdition(cand, want.Edition) {
		return fail(RuleTags, "edition is not %q", want.Edition)
	}

	if err := verifyChapters(want.Chapters, cand.Probe.Chapters); err != nil {
		return err
	}

	frameRateHolds := ov.RFrameRate == ov.AvgFrameRate
	for _, f := range []struct {
		name, want, have string
		skip             bool
	}{
		{"codec", ov.CodecName, cv.CodecName, false},
		{"width", strconv.Itoa(ov.Width), strconv.Itoa(cv.Width), false},
		{"height", strconv.Itoa(ov.Height), strconv.Itoa(cv.Height), false},
		{"pixel format", ov.PixFmt, cv.PixFmt, false},
		{"colour primaries", ov.ColorPrimaries, cv.ColorPrimaries, false},
		{"colour transfer", ov.ColorTransfer, cv.ColorTransfer, false},
		{"colour space", ov.ColorSpace, cv.ColorSpace, false},
		{"colour range", ov.ColorRange, cv.ColorRange, false},
		{"frame rate", ov.RFrameRate, cv.RFrameRate, !frameRateHolds},
	} {
		if !f.skip && f.want != f.have {
			return fail(RulePicture, "%s is %q, want %q", f.name, f.have, f.want)
		}
	}

	frame := ov.FrameDuration()
	if frame == 0 {
		frame = 1.0 / 24
	}
	if math.Abs(want.VideoSpan-want.WantSpan) > frame+1e-6 {
		return fail(RuleDuration, "%.3fs, want %.3fs (±%.3fs)", want.VideoSpan, want.WantSpan, frame)
	}
	return nil
}

func withoutChapterTrack(streams []Stream) []Stream {
	out := make([]Stream, 0, len(streams))
	for _, s := range streams {
		if !isQTChapterTrack(s) {
			out = append(out, s)
		}
	}
	return out
}

// carriesEdition reads the edition value back from wherever this container keeps it.
func carriesEdition(s Snapshot, want string) bool {
	for _, tags := range []map[string]string{s.Probe.Format.Tags, s.Extra} {
		for k, v := range tags {
			// XMP values are kept as JSON text, so a string arrives quoted.
			if isEditionTag(k) && strings.Trim(v, `"`) == want {
				return true
			}
		}
	}
	return false
}

// chapterTolerance absorbs a container's rounding: MP4 chapters store milliseconds.
const chapterTolerance = 0.002

func verifyChapters(want, have []Chapter) error {
	if len(want) != len(have) {
		return fail(RuleChapters, "%d chapters, want %d", len(have), len(want))
	}
	for i := range want {
		ws, _ := strconv.ParseFloat(want[i].StartTime, 64)
		we, _ := strconv.ParseFloat(want[i].EndTime, 64)
		hs, err1 := strconv.ParseFloat(have[i].StartTime, 64)
		he, err2 := strconv.ParseFloat(have[i].EndTime, 64)
		if err1 != nil || err2 != nil || math.Abs(ws-hs) > chapterTolerance || math.Abs(we-he) > chapterTolerance {
			return fail(RuleChapters, "chapter %d moved", i+1)
		}
		if want[i].Tags["title"] != have[i].Tags["title"] {
			return fail(RuleChapters, "chapter %d title is %q, want %q", i+1, have[i].Tags["title"], want[i].Tags["title"])
		}
	}
	return nil
}

// packet is one stream packet: its time from the file's start, and a digest of its bytes.
type packet struct {
	PTS  float64
	Hash string
}

// streamPackets lists a stream's packets. It reads the file's packets but decodes nothing.
func streamPackets(ctx context.Context, t Tools, family, path string, index int) ([]packet, error) {
	out, err := run(ctx, t.FFprobe, "-v", "error", "-protocol_whitelist", "file", "-f", Demuxer(family),
		"-select_streams", strconv.Itoa(index), "-show_data_hash", "sha256",
		"-show_entries", "packet=pts_time,data_hash", "-of", "csv=p=0", toolArg(path))
	if err != nil {
		return nil, fmt.Errorf("list packets: %w", err)
	}
	return parsePackets(out)
}

// parsePackets reads ffprobe's "<pts>,<hash>" lines. A packet with no time keeps its place, at the
// previous packet's time, so the byte comparison still sees it.
func parsePackets(out []byte) ([]packet, error) {
	var pkts []packet
	last := 0.0
	for _, line := range strings.Fields(string(out)) {
		a, b, ok := strings.Cut(line, ",")
		if !ok {
			return nil, fmt.Errorf("list packets: bad line %q", line)
		}
		if v, err := strconv.ParseFloat(a, 64); err == nil {
			last = v
		}
		pkts = append(pkts, packet{PTS: last, Hash: strings.TrimRight(b, ",")})
	}
	return pkts, nil
}

// comparePackets checks that a cut stream's packets are the original's inside the kept range, bit
// for bit and in order. The packet at each end is left out, since which side of a cut it falls on
// is the tool's choice. It returns "" when they match, else what differs.
func comparePackets(orig, cand []packet, in, out float64) string {
	inside := 0
	for _, p := range orig {
		if p.PTS >= in && p.PTS <= out {
			inside++
		}
	}
	if len(cand) < inside-4 {
		return fmt.Sprintf("kept %d packets of %d in the range", len(cand), inside)
	}
	if len(cand) > inside+4 {
		return fmt.Sprintf("kept %d packets, more than the %d in the range", len(cand), inside)
	}
	if len(cand) < 3 {
		have := map[string]bool{}
		for _, p := range orig {
			have[p.Hash] = true
		}
		for _, p := range cand {
			if !have[p.Hash] {
				return "has a packet the original doesn't"
			}
		}
		return ""
	}
	inner := cand[1 : len(cand)-1]
	// Silence repeats a packet's bytes, so the start of the run is found by time as well as digest.
	for i := range orig {
		if orig[i].Hash != inner[0].Hash || math.Abs(orig[i].PTS-in-inner[0].PTS) > 1 || i+len(inner) > len(orig) {
			continue
		}
		match := true
		for j := range inner {
			if orig[i+j].Hash != inner[j].Hash {
				match = false
				break
			}
		}
		if match {
			return ""
		}
	}
	return "changed inside the kept range"
}
