package api

import (
	"context"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// partsFor resolves `part` (HOLODEX-389) for a batch of videos through the same pure
// resolver GET /media/{id} uses — container-tag baseline, filename candidate,
// standing decision, curation — so every list surface reads exactly what the media
// page shows. Unlike applyBrowseTitles it loads the file layer, because part's
// baseline is a container tag (PartNumber/DiskNumber) that lives only in
// video_metadata; the two wide loads are scoped to the field's own keys so a
// 500-video entity page pays for two rows per video, not its whole tag set. Videos
// with no resolved part are absent from the map. A mapping that declares no `part`
// yields nil, nil.
func (h *Handlers) partsFor(ctx context.Context, ids []int64) (map[int64]string, error) {
	if h.mappings == nil || len(ids) == 0 {
		return nil, nil
	}
	field, ok := h.mappings.Current().ByCanonical("part")
	if !ok {
		return nil, nil
	}
	// The enrich queue hands over the whole library, and every id becomes one bound
	// parameter in each batch's IN (...) — so chunk well under SQLite's variable
	// ceiling rather than let a large library turn every pill off with one failed query.
	const chunk = 500
	if len(ids) > chunk {
		out := make(map[int64]string, len(ids))
		for start := 0; start < len(ids); start += chunk {
			part, err := h.partsFor(ctx, ids[start:min(start+chunk, len(ids))])
			if err != nil {
				return nil, err
			}
			for id, v := range part {
				out[id] = v
			}
		}
		return out, nil
	}
	var fileKeys []string
	for _, s := range field.ParsedSources {
		if s.Namespace == "file" {
			fileKeys = append(fileKeys, s.Key)
		}
	}
	extra, err := h.repo.ExtraMetadataForVideosByKey(ctx, ids, fileKeys)
	if err != nil {
		return nil, err
	}
	enrich, err := h.repo.EnrichmentForVideosField(ctx, ids, field.Canonical)
	if err != nil {
		return nil, err
	}
	curation, err := h.repo.CurationForVideos(ctx, ids)
	if err != nil {
		return nil, err
	}
	decisions, err := h.repo.DecisionsForVideos(ctx, ids)
	if err != nil {
		return nil, err
	}
	fields := []mapping.Field{field}
	out := make(map[int64]string, len(ids))
	for _, id := range ids {
		// part's sources never read a video column (they are file tags, the filename
		// namespace and a decision), so an id-only baseline is a complete one.
		v := model.Video{ID: id}
		resolved := resolver.Resolve(&v, extra[id], enrichmentFromRows(enrich[id]),
			curationFromRows(curation[id]), fields, h.resolveOptions(decisionsFromRows(decisions[id])))
		if len(resolved) > 0 && len(resolved[0].Values) > 0 {
			out[id] = resolved[0].Values[0]
		}
	}
	return out, nil
}

// applyParts stamps Video.Part on each item (RD9: the pill on every surface that
// shows the triplet). Best-effort, like applyBrowseTitles: a load failure logs and
// leaves every part empty rather than failing the list.
func (h *Handlers) applyParts(ctx context.Context, items []*model.Video) {
	ids := make([]int64, len(items))
	for i, v := range items {
		ids[i] = v.ID
	}
	parts, err := h.partsFor(ctx, ids)
	if err != nil {
		h.log.Warn("resolve parts for list", "err", err)
		return
	}
	for _, v := range items {
		v.Part = parts[v.ID]
	}
}

// applyPartsTo is applyParts over a value slice — the shape every list handler holds.
func (h *Handlers) applyPartsTo(ctx context.Context, items []model.Video) {
	ptrs := make([]*model.Video, len(items))
	for i := range items {
		ptrs[i] = &items[i]
	}
	h.applyParts(ctx, ptrs)
}

// applyPartsToEnrichQueue stamps part on the queue's video rows — the surface the
// design handoff calls most likely to be forgotten (§4): the rows name a video by
// title alone, so three parts read as one duplicated entry without it.
func (h *Handlers) applyPartsToEnrichQueue(ctx context.Context, rows []repo.EnrichQueueRow) {
	var ids []int64
	for _, row := range rows {
		if row.EntityType == model.EnrichEntityVideo {
			ids = append(ids, row.EntityID)
		}
	}
	parts, err := h.partsFor(ctx, ids)
	if err != nil {
		h.log.Warn("resolve parts for enrich queue", "err", err)
		return
	}
	for i := range rows {
		if rows[i].EntityType == model.EnrichEntityVideo {
			rows[i].Part = parts[rows[i].EntityID]
		}
	}
}

// applyPartsToExtractionQueue is the same stamp for the F48 review-queue rows.
func (h *Handlers) applyPartsToExtractionQueue(ctx context.Context, rows []repo.ExtractionQueueRow) {
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.VideoID
	}
	parts, err := h.partsFor(ctx, ids)
	if err != nil {
		h.log.Warn("resolve parts for extraction queue", "err", err)
		return
	}
	for i := range rows {
		rows[i].Part = parts[rows[i].VideoID]
	}
}

// applyPartsToFilmVideos stamps part on the videos embedded in film rows — scenes
// and full-film files alike, because the scenes grid renders the same VideoCard the
// browse grid does (design handoff §1), unlike edition, which is a full-film fact.
func (h *Handlers) applyPartsToFilmVideos(ctx context.Context, rows ...[]repo.FilmVideo) {
	var ptrs []*model.Video
	for _, list := range rows {
		for i := range list {
			ptrs = append(ptrs, &list[i].Video)
		}
	}
	h.applyParts(ctx, ptrs)
}
