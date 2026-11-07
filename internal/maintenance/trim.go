package maintenance

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// OpTrim keeps one continuous range of a file (F88, file-maintenance.md "A trim is a maintenance
// operation"). It has no video filter: the cut comes from the seek, not a filter chain.
const OpTrim Op = "trim"

// OpTrimEdition labels the job_runs row attributed to the new edition.
const OpTrimEdition = "trim-edition"

// Trim save modes.
const (
	TrimReplace = "replace"
	TrimEdition = "edition"
)

// Trim stages recorded in the queue row, so a crash after an edition's file landed finishes the
// clone instead of cutting the file a second time.
const StageLanded = "landed"

// TrimParams is a trim's queue payload. InTime and OutTime are the media times the picker read
// for the first and last frames to keep. Captures is a server-generated folder name under
// DATA_PATH/maintenance holding in.jpg and out.jpg; it never comes from a request.
type TrimParams struct {
	InTime   float64 `json:"in_time"`
	OutTime  float64 `json:"out_time"`
	Mode     string  `json:"mode"`
	Edition  string  `json:"edition,omitempty"`
	Captures string  `json:"captures,omitempty"`
	Stage    string  `json:"stage,omitempty"`
	Target   string  `json:"target,omitempty"`
}

// frameWindow is the video frames' presentation times around one cut point, in seconds from the
// file's start, ascending. Only frames near the cut are listed, so a long file costs nothing.
type frameWindow []float64

// listFrames lists the main video's frame times within window seconds either side of t. It reads
// packets, not decoded frames, so it costs a demux and never a decode. Times are the file's own
// presentation times, what a browser reports. They are never shifted by the container's start time:
// audio priming makes that negative, while the video still starts at zero. A packet before zero is
// one an MP4 edit list discards, so it is no frame at all.
func listFrames(ctx context.Context, t Tools, family, path string, p Probe, around, window float64) (frameWindow, error) {
	v, ok := p.MainVideo()
	if !ok {
		return nil, refuse(ReasonNoVideo, "")
	}
	// From the file's start there is nothing to seek to. A seek to zero in a file with an edit list
	// lands on the next keyframe, past the frames wanted.
	interval := "%+" + fmtTime(around+window)
	if from := around - window; from > 0 {
		interval = fmtTime(from) + "%+" + fmtTime(2*window)
	}
	out, err := run(ctx, t.FFprobe, "-v", "error", "-protocol_whitelist", "file", "-f", Demuxer(family),
		"-select_streams", strconv.Itoa(v.Index),
		"-read_intervals", interval,
		"-show_entries", "packet=pts_time", "-of", "csv=p=0", toolArg(path))
	if err != nil {
		return nil, fmt.Errorf("list frames: %w", err)
	}
	return parseFrameTimes(out)
}

// parseFrameTimes reads ffprobe's one-time-per-line packet listing into sorted, unique times.
func parseFrameTimes(out []byte) (frameWindow, error) {
	var times frameWindow
	for _, line := range strings.Fields(string(out)) {
		field, _, _ := strings.Cut(line, ",")
		if field == "" || field == "N/A" {
			continue
		}
		v, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return nil, fmt.Errorf("list frames: bad time %q", line)
		}
		if v >= -1e-6 {
			times = append(times, v)
		}
	}
	sort.Float64s(times)
	return times, nil
}

// nearest returns the index of the frame closest to t. A browser rounds a Matroska time to the
// millisecond (1.502 for 1.5015), so the owner's time names a frame without equalling its time.
func (w frameWindow) nearest(t float64) int {
	i := sort.SearchFloat64s(w, t)
	switch {
	case i == 0:
		return 0
	case i == len(w):
		return len(w) - 1
	case t-w[i-1] <= w[i]-t:
		return i - 1
	}
	return i
}

// seekBefore is where a cut must start so that frame i is the first frame kept: the midpoint with
// the frame before it, and no seek at all for the file's first frame. Half of the nominal frame
// interval is not enough: on a variable-frame-rate file real frames sit closer than that.
func (w frameWindow) seekBefore(i int) (float64, bool) {
	if i <= 0 {
		return 0, false
	}
	if i >= len(w) { // a correction can reach past the listed frames
		i = len(w) - 1
	}
	return (w[i-1] + w[i]) / 2, true
}

// stopAfter is where a cut must stop so that frame i is the last frame kept.
func (w frameWindow) stopAfter(i int) (float64, bool) {
	if i >= len(w)-1 {
		return 0, false
	}
	if i < 0 { // a correction can reach past the listed frames
		i = 0
	}
	return (w[i] + w[i+1]) / 2, true
}

// trimRange is one attempt's cut: the frames to keep (their exact times) and the seek and stop
// that select them. A correction moves the seek without moving the frames it is meant to reach.
type trimRange struct {
	InTime, OutTime  float64 // the kept frames' exact times
	Seek, Stop       float64
	HasSeek, HasStop bool
}

// keptDuration is the kept range's expected length: from the first kept frame up to the frame
// after the last one. A cut with no stop runs to the file's end, which only the file knows.
func (r trimRange) keptDuration(fileDuration float64) float64 {
	end := fileDuration
	if r.HasStop {
		end = r.Stop + (r.Stop - r.OutTime)
	}
	return end - r.InTime
}

// fmtTime is a time as ffmpeg, ffprobe and mkvmerge all read it, to the microsecond. A shortest
// decimal form could round a midpoint across the frame it was chosen to clear.
func fmtTime(t float64) string { return strconv.FormatFloat(t, 'f', 6, 64) }

// ExpectedChapters is the chapter list a trim must produce from the original's (spec rule 5): a
// chapter inside the range moves earlier by the length cut from the start, one outside is dropped,
// and one straddling a cut is clamped. in is the kept range's start; dur its length.
func ExpectedChapters(orig []Chapter, in, dur float64) []Chapter {
	out := []Chapter{}
	for _, c := range orig {
		s, err1 := strconv.ParseFloat(c.StartTime, 64)
		e, err2 := strconv.ParseFloat(c.EndTime, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		s, e = math.Max(s-in, 0), math.Min(e-in, dur)
		if e <= s {
			continue
		}
		out = append(out, Chapter{StartTime: fmtTime(s), EndTime: fmtTime(e), Tags: c.Tags})
	}
	return out
}

// ffmetadataChapters writes chapters in ffmpeg's FFMETADATA format, for an MP4's -map_chapters.
func ffmetadataChapters(chapters []Chapter) string {
	var b strings.Builder
	b.WriteString(";FFMETADATA1\n")
	for _, c := range chapters {
		s, _ := strconv.ParseFloat(c.StartTime, 64)
		e, _ := strconv.ParseFloat(c.EndTime, 64)
		fmt.Fprintf(&b, "[CHAPTER]\nTIMEBASE=1/1000000\nSTART=%d\nEND=%d\n", int64(math.Round(s*1e6)), int64(math.Round(e*1e6)))
		if t := c.Tags["title"]; t != "" {
			b.WriteString("title=" + ffmetaEscape.Replace(t) + "\n")
		}
	}
	return b.String()
}

var ffmetaEscape = strings.NewReplacer(`\`, `\\`, `=`, `\=`, `;`, `\;`, `#`, `\#`, "\n", "\\\n")

// matroskaChapters writes chapters as the XML mkvmerge's --chapters reads.
func matroskaChapters(chapters []Chapter) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\"?>\n<Chapters><EditionEntry>\n")
	for _, c := range chapters {
		s, _ := strconv.ParseFloat(c.StartTime, 64)
		e, _ := strconv.ParseFloat(c.EndTime, 64)
		b.WriteString("<ChapterAtom><ChapterTimeStart>" + hmsTime(s) + "</ChapterTimeStart><ChapterTimeEnd>" + hmsTime(e) + "</ChapterTimeEnd>")
		if t := c.Tags["title"]; t != "" {
			b.WriteString("<ChapterDisplay><ChapterString>" + xmlEscape.Replace(t) + "</ChapterString></ChapterDisplay>")
		}
		b.WriteString("</ChapterAtom>\n")
	}
	b.WriteString("</EditionEntry></Chapters>\n")
	return b.String()
}

var xmlEscape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// hmsTime is a time as HH:MM:SS.nnnnnnnnn, the only form Matroska chapter XML and mkvmerge's
// --split parts share.
func hmsTime(t float64) string {
	ns := int64(math.Round(t * 1e9))
	return fmt.Sprintf("%02d:%02d:%02d.%09d", ns/3600e9, ns/60e9%60, ns/1e9%60, ns%1e9)
}

// writeTempFile writes content to a new temp file and returns its name.
func writeTempFile(pattern, content string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// editionMarker is the filename grammar internal/extract/pattern.go lifts the edition from.
var editionMarker = regexp.MustCompile(`\s*\{edition-[^{}]*\}\s*`)

// maxNameBytes is the longest file name the common volumes allow; the working suffix counts.
const maxNameBytes = 255 - len(".holodex-new")

// SanitizeEditionName returns name as it can sit inside a filename marker: path separators,
// braces and control characters removed, no leading dots, trimmed. An empty result means the name
// is refused. This is the one place an owner's text becomes a path component.
func SanitizeEditionName(name string) string {
	clean := strings.Map(func(r rune) rune {
		switch {
		case r == '/', r == '\\', r == '{', r == '}', r == ':', r == '*', r == '?', r == '"', r == '<', r == '>', r == '|':
			return -1
		case unicode.IsControl(r), r == utf8.RuneError:
			return -1
		}
		return r
	}, name)
	return strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(clean), "."))
}

// EditionTarget returns the file an edition lands in: the original's folder, stem and extension,
// with the edition marker in place of any it already carries. ok is false when name sanitizes to
// nothing. A long name is cut at a rune boundary so the whole file name fits the volume.
func EditionTarget(path, name string) (target, cleanName string, ok bool) {
	cleanName = SanitizeEditionName(name)
	if cleanName == "" {
		return "", "", false
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSpace(editionMarker.ReplaceAllString(strings.TrimSuffix(base, ext), " "))
	room := maxNameBytes - len(stem) - len(ext) - len(" {edition-}")
	for len(cleanName) > room && room > 0 {
		_, size := utf8.DecodeLastRuneInString(cleanName)
		cleanName = cleanName[:len(cleanName)-size]
	}
	cleanName = strings.TrimSpace(cleanName)
	if cleanName == "" {
		return "", "", false
	}
	// The bound on the result, whatever the sanitizer above allowed: one name, in the original's
	// folder. Same check as rename's name builder.
	fileName := stem + " {edition-" + cleanName + "}" + ext
	if !filepath.IsLocal(fileName) || filepath.Base(fileName) != fileName {
		return "", "", false
	}
	return filepath.Join(filepath.Dir(path), fileName), cleanName, true
}
