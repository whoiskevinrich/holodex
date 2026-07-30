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
	if h.mappings == nil {
		return nil
	}
	genresField, ok := h.mappings.Current().ByCanonical("genres")
	if !ok {
		return nil
	}
	v, extra, err := h.repo.GetVideo(ctx, videoID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil
		}
		return err
	}
	enrRows, err := h.repo.EnrichmentForEntity(ctx, model.EnrichEntityVideo, videoID)
	if err != nil {
		return err
	}
	curRows, err := h.repo.CurationForEntity(ctx, model.EnrichEntityVideo, videoID)
	if err != nil {
		return err
	}
	decRows, err := h.repo.DecisionsForEntity(ctx, model.EnrichEntityVideo, videoID)
	if err != nil {
		return err
	}
	resolved := resolver.Resolve(v, extra, enrichmentFromRows(enrRows), curationFromRows(curRows),
		[]mapping.Field{genresField}, h.resolveOptions(decisionsFromRows(decRows)))

	var tags []repo.MaterializedTag
	for _, rf := range resolved {
		if !strings.EqualFold(rf.Canonical, "genres") {
			continue
		}
		for _, item := range rf.Items {
			tags = append(tags, repo.MaterializedTag{
				Name:   item.Value,
				Source: fieldsource.ForNamespace(item.Sources[0]),
			})
		}
	}
	return h.repo.AttachMaterializedTags(ctx, videoID, tags)
}
