package api

import (
	"context"
	"fmt"

	"holodex/internal/writequeue"
)

// mergeWriteSource marks a writeback job as merge-triggered (F48.8) in the
// per-field audit trail, alongside "manual"/"filename"/"revert". Defined in
// writequeue (the post-write hook keys off it to skip the entity re-extract for
// merge writes, HOLODEX-196 #4) so the string has one home.
const mergeWriteSource = writequeue.SourceMerge

// mergeBatchID names the shared snapshot batch for one merge's writeback jobs
// (F48.8d, migration 0027) so a single Revert restores every affected video.
// mergedID is only ever merged away once (MergeEntities deletes it), so the
// pair is a stable, collision-free id without a random component.
func mergeBatchID(entityType string, canonicalID, mergedID int64) string {
	return fmt.Sprintf("merge-%s-%d-%d", entityType, canonicalID, mergedID)
}

// propagateMerge (F48.8a/b, ADR-067) syncs a completed Person/Studio merge to
// every affected video's embedded tag: for each video previously linked to
// the loser, the file's field tag is rewritten to the video's full current
// (post-merge) name list for that field — the loser's name is already gone
// from it and the survivor's already present, since the merge repointed the
// association at the DB level; this just carries that same list to disk.
// Uses the merge's own confirm as authorization (F48.8c) — no additional
// preview/confirm gate. All affected videos are enqueued in one transaction
// (EnqueueMany) rather than one call per video, since a merge can affect
// many videos and this runs on the owner-facing request path. Enqueue
// failures are logged, not fatal: the merge itself already committed, so a
// tag-sync hiccup must not surface as a failed merge. No-op when the write
// queue isn't configured (writeback disabled) or no videos were affected.
func (h *Handlers) propagateMerge(ctx context.Context, field, batchID string, videoIDs []int64, namesByVideo map[int64][]string) {
	if h.writeQueue == nil {
		return
	}
	jobs := make([]writequeue.BatchJob, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		values := namesByVideo[videoID]
		if len(values) == 0 {
			continue // the survivor ended up with no links on this video — nothing to write
		}
		jobs = append(jobs, writequeue.BatchJob{
			VideoID: videoID,
			Fields:  []writequeue.JobField{{Field: field, Values: values, Source: mergeWriteSource}},
		})
	}
	if len(jobs) == 0 {
		return
	}
	if _, err := h.writeQueue.EnqueueMany(ctx, jobs, batchID); err != nil {
		h.log.Warn("merge writeback enqueue failed", "field", field, "batch_id", batchID, "videos", len(jobs), "err", err)
	}
}

// namesByVideo flattens a bulk per-video entity lookup (PeopleForVideos,
// StudiosForVideos — same map[int64][]T shape, different T) to the plain
// per-video name lists propagateMerge writes.
func namesByVideo[T any](byVideo map[int64][]T, name func(T) string) map[int64][]string {
	out := make(map[int64][]string, len(byVideo))
	for videoID, items := range byVideo {
		names := make([]string, len(items))
		for i, it := range items {
			names[i] = name(it)
		}
		out[videoID] = names
	}
	return out
}
