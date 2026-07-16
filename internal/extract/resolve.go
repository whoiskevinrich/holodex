package extract

import (
	"fmt"
	"strings"
)

// ReviewAction is the owner's choice when resolving one pending
// metadata_extraction_review row (F48.6c): keep the filename-derived value,
// keep the file's existing tag value, or supply one manually (freeform edit,
// or an entity name picked from search).
type ReviewAction string

const (
	ReviewActionFilename ReviewAction = "filename"
	ReviewActionTag      ReviewAction = "tag"
	ReviewActionManual   ReviewAction = "manual"
)

// ResolvedWrite is what a resolved review row should write, if anything.
// Values is empty for ReviewActionTag — the file's tag already holds that
// value, so there's nothing to write.
type ResolvedWrite struct {
	Values []string
	Source string
}

// ResolveReviewAction decides what (if anything) to write for one pending
// review row's resolution (F48.6c) — the manual-resolution counterpart to
// Process's automatic routing (F48.3/F48.4): both end up deciding a field's
// values+source, just from a different trigger (an explicit owner choice
// instead of a confidence score). Pure — the caller enqueues the write and
// marks the row resolved.
func ResolveReviewAction(action ReviewAction, filenameValue, tagValue, manualValue string) (ResolvedWrite, error) {
	switch action {
	case ReviewActionFilename:
		if filenameValue == "" {
			return ResolvedWrite{}, fmt.Errorf("row has no filename value")
		}
		return ResolvedWrite{Values: splitJoined(filenameValue), Source: Provider}, nil
	case ReviewActionTag:
		if tagValue == "" {
			return ResolvedWrite{}, fmt.Errorf("row has no tag value")
		}
		return ResolvedWrite{}, nil
	case ReviewActionManual:
		v := strings.TrimSpace(manualValue)
		if v == "" {
			return ResolvedWrite{}, fmt.Errorf("value required")
		}
		return ResolvedWrite{Values: []string{v}, Source: "manual"}, nil
	default:
		return ResolvedWrite{}, fmt.Errorf("action must be filename, tag, or manual")
	}
}

// splitJoined reverses joinSorted's ", " formatting for a multi-value
// field's review row — the review table stores only the already-joined
// display string (see store.go/process.go), not the original slice. A value
// containing a literal ", " round-trips as two values; this mirrors the
// tradeoff the review row's own storage already made for display purposes.
func splitJoined(s string) []string {
	parts := strings.Split(s, ", ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
