package maintenance

import (
	"context"
	"fmt"
	"math"
	"strconv"
)

// Every frame compared is scaled to this size and to grey, so comparing costs the same for a 4K
// file as for a small one (file-maintenance.md, "Frame checks").
const (
	checkW    = 240
	checkH    = 135
	checkSize = checkW * checkH
)

const (
	// distinctNCC is how far below a perfect 1 a frame must correlate with each neighbour before a
	// screen capture can tell it from them. In the spike, frames that could be told apart led by
	// at least 0.04, and still ones by at most 0.002.
	distinctNCC = 0.02
	// screenTieNCC is how close a neighbour may score to the chosen frame and still count as a tie.
	screenTieNCC = 0.002
	// screenMatchNCC is the least a capture must correlate with its best reference. Below this it
	// shows another scene, and the nearest of five references would be a meaningless answer.
	screenMatchNCC = 0.7
	// cutTieMAD is how much nearer than the chosen frame (in grey levels, of 255) a neighbour must
	// be before the cut is off. The output is a lossy re-encode, so a tie this close means the
	// frames barely differ and a one-frame slip can't be seen.
	cutTieMAD = 0.5
	// slowFadeMean tells a slow fade from a still image: its mean brightness moves per frame.
	slowFadeMean = 0.15
	// blackMean is the mean grey level at or below which a frame reads as black.
	blackMean = 18.0
)

// grey is one frame scaled to checkW×checkH, one byte per pixel.
type grey []byte

// mad is the mean absolute difference between two frames, in grey levels.
func mad(a, b grey) float64 {
	var sum int64
	for i := range a {
		d := int64(a[i]) - int64(b[i])
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return float64(sum) / float64(len(a))
}

func (g grey) mean() float64 {
	var sum int64
	for _, v := range g {
		sum += int64(v)
	}
	return float64(sum) / float64(len(g))
}

// ncc is zero-mean normalised correlation: 1 for the same picture at any brightness or contrast,
// which is what a browser's HDR tone mapping leaves intact. Two flat frames have no shape to
// differ in, so they count as identical.
func ncc(a, b grey) float64 {
	ma, mb := a.mean(), b.mean()
	var ab, aa, bb float64
	for i := range a {
		x, y := float64(a[i])-ma, float64(b[i])-mb
		ab += x * y
		aa += x * x
		bb += y * y
	}
	if aa == 0 && bb == 0 {
		return 1
	}
	if aa == 0 || bb == 0 {
		return 0
	}
	return ab / math.Sqrt(aa*bb)
}

// references is the original's frames around one kept frame, decoded by Holodex, with At the
// index in Frames of the kept frame itself. Fewer than five exist at either end of a file.
type references struct {
	Frames []grey
	At     int
}

// cutOffset compares the output's frame with the references and returns how many frames the output
// sits from the chosen frame: +1 means it holds the frame after the chosen one. A neighbour tying
// the chosen frame reads as 0, because the two can't visibly differ.
func cutOffset(frame grey, refs references) int {
	best, bestD := refs.At, mad(frame, refs.Frames[refs.At])
	chosen := bestD
	for i, r := range refs.Frames {
		if d := mad(frame, r); d < bestD {
			best, bestD = i, d
		}
	}
	if chosen-bestD <= cutTieMAD {
		return 0
	}
	return best - refs.At
}

// Why a screen check can't run on a frame. The owner's status copy is keyed on these.
const (
	SkipStill    = "still"
	SkipBlack    = "black"
	SkipSlowFade = "slow_fade"
)

// screenSkipReason says why the chosen frame can't be told from its neighbours, or "" if it can.
func screenSkipReason(refs references) string {
	n := refs.Frames[refs.At]
	distinct := len(refs.Frames) > 1
	fading := false
	for i, r := range refs.Frames {
		if i == refs.At {
			continue
		}
		if ncc(n, r) > 1-distinctNCC {
			distinct = false
		}
		if math.Abs(n.mean()-r.mean()) >= slowFadeMean*math.Abs(float64(i-refs.At)) {
			fading = true
		}
	}
	switch {
	case distinct:
		return ""
	case n.mean() <= blackMean:
		return SkipBlack
	case fading:
		return SkipSlowFade
	}
	return SkipStill
}

// screenNoMatch: a capture resembles none of the frames around the chosen time.
const screenNoMatch = math.MinInt32

// screenOffset returns which reference the capture shows, relative to the chosen frame: +1 means
// the owner saw the frame after the one this time names. screenNoMatch means it shows another
// scene. Compare only frames screenSkipReason passes.
func screenOffset(capture grey, refs references) int {
	best, bestN := refs.At, ncc(capture, refs.Frames[refs.At])
	chosen := bestN
	for i, r := range refs.Frames {
		if n := ncc(capture, r); n > bestN {
			best, bestN = i, n
		}
	}
	if bestN < screenMatchNCC {
		return screenNoMatch
	}
	if bestN-chosen <= screenTieNCC {
		return 0
	}
	return best - refs.At
}

// decodeLeadIn is how far before the wanted frames a decode seeks. The frames themselves are chosen
// by their timestamps afterwards, so this only has to land early, however a file's seek behaves.
const decodeLeadIn = 3.0

// decodeGrey decodes up to count consecutive frames from time from, scaled and in grey. It picks
// frames by their own timestamps, not by where a seek lands. That independence is the point: a
// reference decoded the way the cut seeks would shift with any seek error, and the cut check, which
// compares against it, would pass.
func decodeGrey(ctx context.Context, t Tools, family, path string, p Probe, from float64, hasFrom bool, count int) ([]grey, error) {
	v, ok := p.MainVideo()
	if !ok {
		return nil, refuse(ReasonNoVideo, "")
	}
	seek := from
	args := []string{"-nostdin", "-loglevel", "error"}
	filter := ""
	if hasFrom {
		if lead := from - decodeLeadIn; lead > 0 {
			args = append(args, "-seek_timestamp", "1", "-ss", fmtTime(lead))
		}
		filter = "select='gte(t\\," + fmtTime(from) + ")',"
	}
	args = append(args, "-copyts", "-protocol_whitelist", "file", "-f", Demuxer(family), "-i", toolArg(path),
		"-map", "0:"+strconv.Itoa(v.Index), "-frames:v", strconv.Itoa(count), "-fps_mode", "passthrough",
		"-vf", filter+"scale="+strconv.Itoa(checkW)+":"+strconv.Itoa(checkH)+",format=gray",
		"-f", "rawvideo", "-pix_fmt", "gray", "-")
	out, err := run(ctx, t.FFmpeg, args...)
	if err != nil {
		return nil, fmt.Errorf("decode frames: %w", err)
	}
	var frames []grey
	for len(out) >= checkSize {
		frames = append(frames, grey(out[:checkSize]))
		out = out[checkSize:]
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("decode frames: none at %s", fmtTime(seek))
	}
	return frames, nil
}

// decodeCapture reads an uploaded JPEG the same way. The demuxer is forced: a capture is only ever
// a JPEG, whatever its bytes claim to be.
func decodeCapture(ctx context.Context, t Tools, path string) (grey, error) {
	out, err := run(ctx, t.FFmpeg, "-nostdin", "-loglevel", "error", "-protocol_whitelist", "file",
		"-f", "image2", "-pattern_type", "none", "-c:v", "mjpeg", "-i", toolArg(path), "-frames:v", "1",
		"-vf", "scale="+strconv.Itoa(checkW)+":"+strconv.Itoa(checkH)+",format=gray",
		"-f", "rawvideo", "-pix_fmt", "gray", "-")
	if err != nil {
		return nil, fmt.Errorf("decode capture: %w", err)
	}
	if len(out) < checkSize {
		return nil, fmt.Errorf("decode capture: short frame")
	}
	return grey(out[:checkSize]), nil
}

// loadReferences decodes the original's frames i−2…i+2.
func loadReferences(ctx context.Context, t Tools, family, path string, p Probe, w frameWindow, i int) (references, error) {
	first := i - 2
	if first < 0 {
		first = 0
	}
	seek, hasSeek := w.seekBefore(first)
	frames, err := decodeGrey(ctx, t, family, path, p, seek, hasSeek, i+2-first+1)
	if err != nil {
		return references{}, err
	}
	at := i - first
	if at >= len(frames) {
		return references{}, fmt.Errorf("decode frames: chosen frame missing at %s", fmtTime(w[i]))
	}
	return references{Frames: frames, At: at}, nil
}
