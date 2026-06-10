package metadata

import "strings"

// ResolutionBucket is the user-facing resolution tier (ADR-012).
type ResolutionBucket string

const (
	ResolutionSD  ResolutionBucket = "SD"
	ResolutionHD  ResolutionBucket = "HD"
	ResolutionFHD ResolutionBucket = "FHD"
	Resolution4K  ResolutionBucket = "4K"
)

// Width thresholds with a 10% downward tolerance applied to each nominal tier
// width, so near-miss encodes round up to their intended tier (ADR-012).
//
//	HD  nominal 1280 -> 1152
//	FHD nominal 1920 -> 1728
//	4K  nominal 3840 -> 3456
const (
	thresholdHD  = 1152 // 1280 * 0.9
	thresholdFHD = 1728 // 1920 * 0.9
	threshold4K  = 3456 // 3840 * 0.9
)

// ClassifyResolution buckets a video by frame width (ADR-012).
//
// Width is used rather than height so cinematic/letterboxed content (e.g. a
// 3840x1606 2.39:1 master) classifies by its true tier instead of dropping one.
func ClassifyResolution(width int) ResolutionBucket {
	switch {
	case width >= threshold4K:
		return Resolution4K
	case width >= thresholdFHD:
		return ResolutionFHD
	case width >= thresholdHD:
		return ResolutionHD
	default:
		return ResolutionSD
	}
}

// ParseResolutionBucket parses a user-facing tier token ("SD", "HD", "FHD",
// "4K", or "4K+") into a bucket. ok is false for unrecognized input (e.g. "all").
func ParseResolutionBucket(s string) (ResolutionBucket, bool) {
	switch strings.ToUpper(strings.TrimRight(strings.TrimSpace(s), "+")) {
	case "SD":
		return ResolutionSD, true
	case "HD":
		return ResolutionHD, true
	case "FHD":
		return ResolutionFHD, true
	case "4K":
		return Resolution4K, true
	default:
		return "", false
	}
}

// ResolutionWidthRange returns the [min, max) width predicate for a bucket,
// used to translate a resolution filter into a SQL range query (ADR-006/012).
// A max of 0 means "no upper bound".
func ResolutionWidthRange(b ResolutionBucket) (min, max int) {
	switch b {
	case ResolutionSD:
		return 0, thresholdHD
	case ResolutionHD:
		return thresholdHD, thresholdFHD
	case ResolutionFHD:
		return thresholdFHD, threshold4K
	case Resolution4K:
		return threshold4K, 0
	default:
		return 0, 0
	}
}
