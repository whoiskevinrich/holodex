package api

import (
	"context"
	"errors"
	"strings"

	"holodex/internal/fieldsource"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// materializeTags attaches videoID's currently-resolved `genres` values as tags (F50
// P0-9, ADR-075 D4) — the enrichment-apply counterpart to the owner's manual attach
// (S4). Best-effort like relinkStudios: a failure is logged, never failing the
// enrich-apply call that triggered it (the enrichment write already committed;
// materialization self-heals on the next apply/refresh).
func (h *Handlers) materializeTags(ctx context.Context, videoID int64) {
	if err := h.MaterializeVideoTags(ctx, videoID); err != nil {
		h.log.Warn("materialize tags", "video", videoID, "err", err)
	}
}

// MaterializeVideoTags reads videoID's RESOLVED `genres` field — the merge across
// every enrichment source contributing to the video (the same resolver call getMedia
// already makes for display), not the raw per-provider fields the just-applied
// enrich() call returned, since a second provider might already be contributing to
// `genres` (ADR-075 D4) — and attaches each surviving value as a tag in one batch.
// Each value's provenance is the first namespace that contributed it
// (fieldsource.ForNamespace(item.Sources[0]) — every ResolvedValue the resolver
// produces carries at least one source, so this is never empty): "provider:<name>"
// for an enrichment source, "manual" for an owner-curated genres addition. A
// missing/soft-deleted video or a config with no `genres` mapping is a no-op, not an
// error. Exported (like RelinkVideoStudios) so tests can drive it directly without a
// live provider.
func (h *Handlers) MaterializeVideoTags(ctx context.Context, videoID int64) error {
	rf, ok, err := h.resolvedField(ctx, videoID, "genres")
	if err != nil || !ok {
		return err
	}
	tags := make([]repo.MaterializedTag, 0, len(rf.Items))
	for _, item := range rf.Items {
		tags = append(tags, repo.MaterializedTag{
			Name:   item.Value,
			Source: fieldsource.ForNamespace(item.Sources[0]),
		})
	}
	return h.repo.AttachMaterializedTags(ctx, videoID, tags)
}

// resolvedField reads videoID's single resolved canonical field — the shared
// GetVideo + EnrichmentForEntity/CurationForEntity/DecisionsForEntity +
// resolver.Resolve sequence behind both MaterializeVideoTags (S5, above) and
// genreWritebackValues (S6, genre_writeback.go). ok is false with a nil error
// for every no-op condition both callers share alike: no metadata mappings
// configured, canonical unmapped, or a missing/soft-deleted video.
func (h *Handlers) resolvedField(ctx context.Context, videoID int64, canonical string) (resolver.ResolvedField, bool, error) {
	var zero resolver.ResolvedField
	if h.mappings == nil {
		return zero, false, nil
	}
	field, ok := h.mappings.Current().ByCanonical(canonical)
	if !ok {
		return zero, false, nil
	}
	v, extra, err := h.repo.GetVideo(ctx, videoID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return zero, false, nil
		}
		return zero, false, err
	}
	enrRows, err := h.repo.EnrichmentForEntity(ctx, model.EnrichEntityVideo, videoID)
	if err != nil {
		return zero, false, err
	}
	curRows, err := h.repo.CurationForEntity(ctx, model.EnrichEntityVideo, videoID)
	if err != nil {
		return zero, false, err
	}
	decRows, err := h.repo.DecisionsForEntity(ctx, model.EnrichEntityVideo, videoID)
	if err != nil {
		return zero, false, err
	}
	resolved := resolver.Resolve(v, extra, enrichmentFromRows(enrRows), curationFromRows(curRows),
		[]mapping.Field{field}, h.resolveOptions(decisionsFromRows(decRows)))
	for _, rf := range resolved {
		if strings.EqualFold(rf.Canonical, canonical) {
			return rf, true, nil
		}
	}
	return zero, false, nil
}
