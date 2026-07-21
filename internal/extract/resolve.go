package extract

import (
	"fmt"
	"strings"

	"holodex/internal/model"
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
//
// field is the review row's field key; for a multi-value field (People) the
// manual value is the owner's edited cast, ", "-joined the same way the
// filename value is, so it is split into several values rather than written as
// one string — editing one person in the queue's chip list can't collapse the
// whole cast to a single name (HOLODEX-196 #1).
func ResolveReviewAction(action ReviewAction, field, filenameValue, tagValue, manualValue string) (ResolvedWrite, error) {
	switch action {
	case ReviewActionFilename:
		if filenameValue == "" {
			return ResolvedWrite{}, fmt.Errorf("row has no filename value")
		}
		return ResolvedWrite{Values: model.SplitJoined(filenameValue), Source: Provider}, nil
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
		values := []string{v}
		if IsMultiValueField(field) {
			if values = model.SplitJoined(v); len(values) == 0 {
				return ResolvedWrite{}, fmt.Errorf("value required")
			}
		}
		return ResolvedWrite{Values: values, Source: "manual"}, nil
	default:
		return ResolvedWrite{}, fmt.Errorf("action must be filename, tag, or manual")
	}
}
