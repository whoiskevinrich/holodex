package maintenance

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Output-contract rules (spec "Output contract"). A verification failure names one, and the
// failure reason the owner sees is built from it.
const (
	RuleStreams  = "streams"  // 2: same streams, same order, type and disposition
	RuleCopied   = "copied"   // 3: every non-picture stream byte-identical
	RuleTags     = "tags"     // 4: same container and stream tags
	RuleChapters = "chapters" // 5: same chapters
	RulePicture  = "picture"  // 6: same codec, size, pixel format, colour, frame rate
	RuleDuration = "duration" // 7: same length within one frame
)

// RuleError is a verification failure: the candidate breaks Rule, so it must not replace
// the original.
type RuleError struct {
	Rule   string
	Detail string
}

func (e *RuleError) Error() string { return "verification failed (" + e.Rule + "): " + e.Detail }

func fail(rule, format string, a ...any) error {
	return &RuleError{Rule: rule, Detail: fmt.Sprintf(format, a...)}
}

// Snapshot is everything the verifier compares about one file. StreamHashes holds a content
// digest per stream index (ffmpeg's streamhash muxer); Extra holds container-specific facts
// ffprobe can't see (XMP for MP4, attachment name/type/size for Matroska), keyed uniquely.
type Snapshot struct {
	Family       string // FamilyMP4 or FamilyMKV
	Probe        Probe
	StreamHashes map[int]string
	Extra        map[string]string
}

// Writing-application statistics tags mkvmerge stamps on every track it writes; they record
// the mux, not the content, so they may differ on any track.
var muxStampTags = []string{"_STATISTICS_WRITING_APP", "_STATISTICS_WRITING_DATE_UTC"}

// Statistics tags describing a track's encoded bytes; recomputed on the re-encoded track only
// (spec rule 4's first exception, Q1). In MP4 the re-encoded track's `encoder` is exempt too:
// it is the sample description's CompressorName, which ffmpeg 7.1 always fills on a re-encode
// (an empty -metadata value counts as unset), so it describes the new bytes (third exception).
var encodedStatsTags = []string{"BPS", "DURATION", "NUMBER_OF_FRAMES", "NUMBER_OF_BYTES", "_STATISTICS_TAGS"}

// Verify checks that cand differs from orig only in the main video stream's pixels. A flip
// relaxes no rule; an operation that changes the picture's shape (aspect ratio, P2) will
// need its relaxations threaded in here.
func Verify(orig, cand Snapshot) error {
	ov, ok := orig.Probe.MainVideo()
	if !ok {
		return fail(RuleStreams, "original has no video stream")
	}
	cv, ok := cand.Probe.MainVideo()
	if !ok || cv.Index != ov.Index {
		return fail(RuleStreams, "main video stream moved or missing")
	}

	if len(orig.Probe.Streams) != len(cand.Probe.Streams) {
		return fail(RuleStreams, "%d streams (%s), want %d (%s)", len(cand.Probe.Streams), streamKinds(cand.Probe),
			len(orig.Probe.Streams), streamKinds(orig.Probe))
	}
	for i, os := range orig.Probe.Streams {
		cs := cand.Probe.Streams[i]
		if os.CodecType != cs.CodecType {
			return fail(RuleStreams, "stream %d is %s, want %s", i, cs.CodecType, os.CodecType)
		}
		if d := diffInts(os.Disposition, cs.Disposition); d != "" {
			return fail(RuleStreams, "stream %d disposition %s", i, d)
		}
	}

	for _, os := range orig.Probe.Streams {
		if os.Index == ov.Index {
			continue
		}
		want, have := orig.StreamHashes[os.Index], cand.StreamHashes[os.Index]
		if want != "" && want != have {
			return fail(RuleCopied, "stream %d (%s) changed", os.Index, os.CodecType)
		}
		if os.CodecType != "attachment" && want == "" {
			return fail(RuleCopied, "stream %d (%s) has no digest to compare", os.Index, os.CodecType)
		}
	}

	containerIgnore := isFileTypeBrand
	if orig.Family == FamilyMKV {
		containerIgnore = isContainerStamp
	}
	if d := diffTags(orig.Probe.Format.Tags, cand.Probe.Format.Tags, containerIgnore); d != "" {
		return fail(RuleTags, "container %s", d)
	}
	for i, os := range orig.Probe.Streams {
		ignore := isMuxStamp
		if os.Index == ov.Index {
			mp4 := orig.Family == FamilyMP4
			ignore = func(k string) bool { return isMuxStamp(k) || isEncodedStat(k) || mp4 && k == "encoder" }
		}
		if d := diffTags(os.Tags, cand.Probe.Streams[i].Tags, ignore); d != "" {
			return fail(RuleTags, "stream %d %s", i, d)
		}
	}
	if d := diffTags(orig.Extra, cand.Extra, func(string) bool { return false }); d != "" {
		return fail(RuleTags, "%s", d)
	}

	oc, cc := orig.Probe.Chapters, cand.Probe.Chapters
	if len(oc) != len(cc) {
		return fail(RuleChapters, "%d chapters, want %d", len(cc), len(oc))
	}
	for i := range oc {
		if oc[i].StartTime != cc[i].StartTime || oc[i].EndTime != cc[i].EndTime {
			return fail(RuleChapters, "chapter %d moved", i+1)
		}
		if d := diffTags(oc[i].Tags, cc[i].Tags, func(string) bool { return false }); d != "" {
			return fail(RuleChapters, "chapter %d %s", i+1, d)
		}
	}

	for _, f := range []struct{ name, want, have string }{
		{"codec", ov.CodecName, cv.CodecName},
		{"width", strconv.Itoa(ov.Width), strconv.Itoa(cv.Width)},
		{"height", strconv.Itoa(ov.Height), strconv.Itoa(cv.Height)},
		{"pixel format", ov.PixFmt, cv.PixFmt},
		{"colour primaries", ov.ColorPrimaries, cv.ColorPrimaries},
		{"colour transfer", ov.ColorTransfer, cv.ColorTransfer},
		{"colour space", ov.ColorSpace, cv.ColorSpace},
		{"colour range", ov.ColorRange, cv.ColorRange},
		{"frame rate", ov.RFrameRate, cv.RFrameRate},
	} {
		if f.want != f.have {
			return fail(RulePicture, "%s is %q, want %q", f.name, f.have, f.want)
		}
	}

	od, err1 := strconv.ParseFloat(orig.Probe.Format.Duration, 64)
	cd, err2 := strconv.ParseFloat(cand.Probe.Format.Duration, 64)
	if err1 != nil || err2 != nil {
		return fail(RuleDuration, "unreadable duration %q / %q", orig.Probe.Format.Duration, cand.Probe.Format.Duration)
	}
	frame := ov.FrameDuration()
	if frame == 0 {
		frame = 1.0 / 24
	}
	if math.Abs(od-cd) > frame+1e-6 {
		return fail(RuleDuration, "%.3fs, want %.3fs (±%.3fs)", cd, od, frame)
	}
	return nil
}

// streamKinds summarises a file's streams as "video/h264,audio/aac,…" for failure messages.
func streamKinds(p Probe) string {
	kinds := make([]string, len(p.Streams))
	for i, s := range p.Streams {
		kinds[i] = s.CodecType + "/" + s.CodecName
	}
	return strings.Join(kinds, ",")
}

// isContainerStamp: Matroska's writing-application field records the mux, not content. Only
// Matroska: in MP4 `encoder` is the iTunes ©too tag, a real tag the restore must keep (F77 QA
// caught a flip dropping it while this exemption hid the loss).
func isContainerStamp(k string) bool { return strings.EqualFold(k, "encoder") }

// isFileTypeBrand: an MP4's ftyp box declares the brands its muxer wrote to, and ffmpeg writes
// its own; neither -brand (major only) nor exiftool can put an arbitrary original's back
// (spec rule 4's second exception).
func isFileTypeBrand(k string) bool {
	return k == "major_brand" || k == "minor_version" || k == "compatible_brands"
}

func isMuxStamp(k string) bool { return hasTagBase(k, muxStampTags) }

func isEncodedStat(k string) bool { return hasTagBase(k, encodedStatsTags) }

// hasTagBase matches a Matroska tag name with or without its "-<lang>" suffix (ffprobe
// reports "BPS-eng" for a BPS tag in English).
func hasTagBase(k string, bases []string) bool {
	base, _, _ := strings.Cut(strings.ToUpper(k), "-")
	for _, b := range bases {
		if base == b {
			return true
		}
	}
	return false
}

// diffTags returns "" when want and have agree on every key ignore doesn't exempt, else the
// first difference in key order, so a failure is reproducible.
func diffTags(want, have map[string]string, ignore func(string) bool) string {
	keys := map[string]bool{}
	for k := range want {
		keys[k] = true
	}
	for k := range have {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		if !ignore(k) {
			sorted = append(sorted, k)
		}
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		w, inW := want[k]
		h, inH := have[k]
		switch {
		case !inH:
			return fmt.Sprintf("tag %q missing", k)
		case !inW:
			return fmt.Sprintf("tag %q added", k)
		case w != h:
			return fmt.Sprintf("tag %q is %q, want %q", k, h, w)
		}
	}
	return ""
}

func diffInts(want, have map[string]int) string {
	keys := map[string]bool{}
	for k := range want {
		keys[k] = true
	}
	for k := range have {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		if want[k] != have[k] {
			return fmt.Sprintf("%s is %d, want %d", k, have[k], want[k])
		}
	}
	return ""
}
