package api

import (
	"context"
	"strings"
)

// GenreWritebackValues computes P0-10 (F50, ADR-075 RD9)'s value list for a
// "genres" writeback: the video's attached tags (ancestor-expanded, canonical
// names — repo.TagNamesForVideo) unioned with the raw resolved `genres` field
// (the same union MaterializeVideoTags, S5, reads), with the raw-union side
// deny-list-filtered first — so a term that could never become a Tag can't
// reach the file's Genre tag either (closing the gap the spec's Non-Goals
// section calls out: without this, a denied term could still reach the file
// via the unfiltered raw union). Deduplicated case-insensitively, tag names
// first. Uses the existing TagForField/ResolveForContainer mapping unmodified
// — this only decides which values feed that mapping's "genres" field, not
// how they're written. Exported (like MaterializeVideoTags) so tests can
// drive it directly without a live provider.
func (h *Handlers) GenreWritebackValues(ctx context.Context, videoID int64) ([]string, error) {
	tagNames, err := h.repo.TagNamesForVideo(ctx, videoID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(tagNames))
	out := make([]string, 0, len(tagNames))
	for _, name := range tagNames {
		k := genreDedupKey(name)
		if !seen[k] {
			seen[k] = true
			out = append(out, name)
		}
	}

	rf, ok, err := h.resolvedField(ctx, videoID, "genres")
	if err != nil {
		return nil, err
	}
	if !ok {
		return out, nil
	}
	for _, item := range rf.Items {
		denied, err := h.repo.IsTagDenied(ctx, item.Value)
		if err != nil {
			return nil, err
		}
		if denied {
			continue
		}
		k := genreDedupKey(item.Value)
		if !seen[k] {
			seen[k] = true
			out = append(out, item.Value)
		}
	}
	return out, nil
}

// genreDedupKey is the case/whitespace-insensitive dedup key for the tag ∪
// raw-genres union above — trim + lower, matching resolver.normKey and
// repo.curationNorm's identical convention (this package can't import either
// unexported symbol, so it mirrors the same two operations).
func genreDedupKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
