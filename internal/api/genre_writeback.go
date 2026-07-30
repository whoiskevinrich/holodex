package api

import (
	"context"
	"errors"

	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/resolver"
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
	v, extra, err := h.repo.GetVideo(ctx, videoID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return h.genreWritebackValuesForVideo(ctx, v, extra)
}

// genreWritebackValuesForVideo is GenreWritebackValues' video-already-loaded
// half, split out so writebackMedia (which has already fetched the video for
// the write itself) doesn't pay for a second GetVideo just to compute the
// genres override.
func (h *Handlers) genreWritebackValuesForVideo(ctx context.Context, v *model.Video, extra []model.ExtraMetadata) ([]string, error) {
	tagNames, err := h.repo.TagNamesForVideo(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(tagNames))
	out := make([]string, 0, len(tagNames))
	for _, name := range tagNames {
		k := resolver.NormKey(name)
		if !seen[k] {
			seen[k] = true
			out = append(out, name)
		}
	}

	rf, ok, err := h.resolvedFieldForVideo(ctx, v, extra, "genres")
	if err != nil {
		return nil, err
	}
	if !ok {
		return out, nil
	}
	values := make([]string, len(rf.Items))
	for i, item := range rf.Items {
		values[i] = item.Value
	}
	denied, err := h.repo.DeniedTagSet(ctx, values)
	if err != nil {
		return nil, err
	}
	for _, item := range rf.Items {
		if denied[item.Value] {
			continue
		}
		k := resolver.NormKey(item.Value)
		if !seen[k] {
			seen[k] = true
			out = append(out, item.Value)
		}
	}
	return out, nil
}
