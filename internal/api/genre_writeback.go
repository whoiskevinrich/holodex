package api

import (
	"context"
	"errors"
	"strings"

	"holodex/internal/mapping"
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
	items, err := h.genreWritebackItems(ctx, v, extra)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Value
	}
	return out, nil
}

// genreWritebackItems computes the same union as genreWritebackValuesForVideo but
// keeps each value's provenance, so a display caller (getMedia, below) can show the
// video's attached tags in the "genres" row without recomputing the union itself.
// Resolves the raw "genres" field itself — writebackMedia and GenreWritebackValues
// have no other reason to have resolved it. getMedia already has (it runs its own
// full field resolve for the detail response), so it calls genreWritebackItemsFrom
// directly with that result instead of paying for a second resolve pass here.
func (h *Handlers) genreWritebackItems(ctx context.Context, v *model.Video, extra []model.ExtraMetadata) ([]resolver.ResolvedValue, error) {
	rf, ok, err := h.resolvedFieldForVideo(ctx, v, extra, "genres")
	if err != nil {
		return nil, err
	}
	return h.genreWritebackItemsFrom(ctx, v.ID, rf, ok)
}

// genreWritebackItemsFrom is genreWritebackItems' core: given the raw resolved
// "genres" field (rf, ok — exactly resolvedFieldForVideo's return shape), unions in
// the video's attached tags. Tag-sourced values carry Sources ["tag"]; a raw value
// keeps its original resolver-assigned Sources. This is the write-value union ONLY
// — never reused as resolvedFieldForVideo's replacement, since MaterializeVideoTags
// (the opposite direction: resolved genres -> Tag rows) must keep reading the
// plain, tag-blind merge to avoid re-materializing a tag it just read back out of
// "genres".
func (h *Handlers) genreWritebackItemsFrom(ctx context.Context, videoID int64, rf resolver.ResolvedField, ok bool) ([]resolver.ResolvedValue, error) {
	tagNames, err := h.repo.TagNamesForVideo(ctx, videoID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(tagNames))
	items := make([]resolver.ResolvedValue, 0, len(tagNames))
	for _, name := range tagNames {
		k := resolver.NormKey(name)
		if !seen[k] {
			seen[k] = true
			items = append(items, resolver.ResolvedValue{Value: name, Sources: []string{"tag"}})
		}
	}
	if !ok {
		return items, nil
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
			items = append(items, item)
		}
	}
	return items, nil
}

// resolvedByCanonical finds a field by canonical name in an already-resolved
// slice — getMedia's read-only lookup half of what applyGenreWriteback does
// (which additionally needs the index, to replace or delete in place).
func resolvedByCanonical(fields []resolver.ResolvedField, canonical string) (resolver.ResolvedField, bool) {
	for _, rf := range fields {
		if strings.EqualFold(rf.Canonical, canonical) {
			return rf, true
		}
	}
	return resolver.ResolvedField{}, false
}

// applyGenreWriteback overwrites (or, if absent, appends) the "genres" entry in a
// getMedia response's resolved[] with the actual writeback union (P0-10, ADR-075
// RD9) — the video's attached tags plus the deny-filtered raw resolved genres —
// instead of the plain provider/file merge ResolveFields produced. Without this,
// a video whose "genres" only comes from manually-attached tags (no provider/file
// genre value) never gets a "genres" row at all — ResolveFields drops empty merge
// fields — so the Writeback dialog has no way to write those tags to the file, and
// a video that *does* have a provider genre shows a value that doesn't reflect the
// tags the owner just added/removed. Local to the API response only — see
// genreWritebackItemsFrom for why the shared resolver core must stay untouched.
func applyGenreWriteback(resolved []resolver.ResolvedField, field mapping.Field, items []resolver.ResolvedValue) []resolver.ResolvedField {
	values, winner := genreWritebackFieldValues(items)
	for i, rf := range resolved {
		if !strings.EqualFold(rf.Canonical, "genres") {
			continue
		}
		if len(items) == 0 {
			// Mirrors writebackMedia's own "drop rather than leave an empty
			// field" posture: nothing to decide, so no row to show.
			return append(resolved[:i], resolved[i+1:]...)
		}
		resolved[i].Values, resolved[i].Items, resolved[i].WinningSource = values, items, winner
		return resolved
	}
	if len(items) == 0 {
		return resolved
	}
	label, display := resolver.LabelAndDisplay(field)
	return append(resolved, resolver.ResolvedField{
		Canonical:     "genres",
		Label:         label,
		Display:       display,
		Values:        values,
		Items:         items,
		Multi:         true,
		WinningSource: winner,
	})
}

// genreWritebackFieldValues derives a ResolvedField's Values/WinningSource from the
// genre writeback union, shared by both applyGenreWriteback branches.
func genreWritebackFieldValues(items []resolver.ResolvedValue) (values []string, winner string) {
	values = make([]string, len(items))
	for i, it := range items {
		values[i] = it.Value
	}
	winner = "tag:genres"
	if len(items) > 0 && len(items[0].Sources) > 0 && items[0].Sources[0] != "tag" {
		winner = items[0].Sources[0] + ":genres"
	}
	return values, winner
}
