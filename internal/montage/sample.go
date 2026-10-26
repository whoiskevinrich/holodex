package montage

import "math"

// Sampling defaults (spec F83 P0-1, P0-2). Parameters, not hardcoded in the
// ffmpeg argv, so tuning them (spec Q2) is a one-line change here.
const (
	maxClips       = 6
	clipSeconds    = 1.0
	spanStart      = 0.10 // skip most intros
	spanEnd        = 0.90 // and most credits
	fullClipsFrom  = 30   // seconds: at or above this, a video gets maxClips
	minClips       = 2
	MinDurationSec = 6 // shorter (or unknown) videos get no montage
)

// clipStarts returns the start time, in seconds, of each clip for a video of
// durationSec: evenly spaced between 10% and 90% of the duration, one clip
// centred in each slice. Nil means the video is too short for a montage.
func clipStarts(durationSec int) []float64 {
	if durationSec < MinDurationSec {
		return nil
	}
	d := float64(durationSec)
	n := maxClips
	if durationSec < fullClipsFrom {
		n = int(math.Round(maxClips * d / fullClipsFrom))
		n = max(minClips, min(maxClips, n))
	}
	span := (spanEnd - spanStart) * d
	slice := span / float64(n)
	starts := make([]float64, n)
	for i := range starts {
		s := spanStart*d + (float64(i)+0.5)*slice - clipSeconds/2
		starts[i] = math.Max(0, math.Min(s, d-clipSeconds))
	}
	return starts
}
