package api

import (
	"context"
	"fmt"
	"time"

	"holodex/internal/model"
	"holodex/internal/registry"
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

// propagateMerge (F48.8b, ADR-067) syncs a completed Studio merge to every
// affected video's embedded tag (a Person merge goes through the role-aware
// propagatePersonMerge instead): for each video previously linked to
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

// propagatePersonRename (HOLODEX-551) syncs a completed person rename to every
// video the person is linked to: each person-typed field (registry.PersonTypedFields)
// whose role the person holds on that video is rewritten to the video's linked
// people in that role, under their canonical names — so the new name replaces the
// old one and co-credits are kept. Without it the old spelling stays on file, and
// no other path rewrites a cast tag. All jobs share one snapshot batch, so a
// single Revert undoes the rename's writes. Like propagateMerge, the rename's own
// request is the authorization, and failures are logged, not fatal: the rename
// already committed. No-op when writeback is disabled.
func (h *Handlers) propagatePersonRename(ctx context.Context, personID int64) {
	if h.writeQueue == nil {
		return
	}
	videoIDs, err := h.repo.VideoIDsForPerson(ctx, personID)
	if err != nil {
		h.log.Warn("rename writeback: load linked videos", "person_id", personID, "err", err)
		return
	}
	people, err := h.repo.PeopleForVideos(ctx, videoIDs)
	if err != nil {
		h.log.Warn("rename writeback: load people for videos", "person_id", personID, "err", err)
		return
	}
	h.enqueueRename(ctx, model.EnrichEntityPerson, personID, personRoleJobs(videoIDs, people, personID, writequeue.SourceRename))
}

// propagatePersonMerge (F48.8a, HOLODEX-552) is propagateMerge for people, made
// role-aware: each affected video's person-typed fields are rewritten through
// personRoleJobs, so the survivor's name lands in the fields for the roles it holds
// there and a co-credited director stays out of the cast tag. videoIDs is the
// loser's affected-video list, captured inside the merge's own transaction.
func (h *Handlers) propagatePersonMerge(ctx context.Context, survivorID, mergedID int64, videoIDs []int64) {
	if h.writeQueue == nil || len(videoIDs) == 0 {
		return
	}
	people, err := h.repo.PeopleForVideos(ctx, videoIDs)
	if err != nil {
		h.log.Warn("merge writeback: load people for videos", "err", err)
		return
	}
	h.enqueuePropagation(ctx, mergeBatchID(model.EnrichEntityPerson, survivorID, mergedID),
		personRoleJobs(videoIDs, people, survivorID, mergeWriteSource))
}

// personRoleJobs builds one writeback job per video for the person-typed fields
// (registry.PersonTypedFields) whose role personID holds on that video. Each field
// is rewritten to the video's linked people in that role, under their canonical
// names, so co-credits are kept and one role never leaks into another's field.
func personRoleJobs(videoIDs []int64, people map[int64][]model.Person, personID int64, source string) []writequeue.BatchJob {
	personFields := registry.PersonTypedFields()
	jobs := make([]writequeue.BatchJob, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		var fields []writequeue.JobField
		for _, def := range personFields {
			var names []string
			heldHere := false
			for _, p := range people[videoID] {
				if p.Role != def.Role {
					continue
				}
				names = append(names, p.Name)
				heldHere = heldHere || p.ID == personID
			}
			if heldHere {
				fields = append(fields, writequeue.JobField{Field: def.Canonical, Values: names, Source: source})
			}
		}
		if len(fields) > 0 {
			jobs = append(jobs, writequeue.BatchJob{VideoID: videoID, Fields: fields})
		}
	}
	return jobs
}

// propagateStudioRename (HOLODEX-553) is propagatePersonRename for studios: every
// video linked to the studio gets its field tag rewritten to its linked studios'
// current names, so the new name replaces the old and co-studios are kept.
func (h *Handlers) propagateStudioRename(ctx context.Context, field string, studioID int64) {
	if h.writeQueue == nil {
		return
	}
	videoIDs, err := h.repo.VideoIDsForStudio(ctx, studioID)
	if err != nil {
		h.log.Warn("rename writeback: load linked videos", "studio_id", studioID, "err", err)
		return
	}
	studios, err := h.repo.StudiosForVideos(ctx, videoIDs)
	if err != nil {
		h.log.Warn("rename writeback: load studios for videos", "studio_id", studioID, "err", err)
		return
	}
	names := namesByVideo(studios, func(s model.Studio) string { return s.Name })
	jobs := make([]writequeue.BatchJob, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		jobs = append(jobs, writequeue.BatchJob{
			VideoID: videoID,
			Fields:  []writequeue.JobField{{Field: field, Values: names[videoID], Source: writequeue.SourceRename}},
		})
	}
	h.enqueueRename(ctx, model.EnrichEntityStudio, studioID, jobs)
}

// enqueueRename enqueues one rename's writeback jobs under a shared snapshot batch.
// An entity can be renamed again later, so unlike mergeBatchID the (type, id) pair
// alone is not unique; the timestamp keeps each rename's revert batch its own.
func (h *Handlers) enqueueRename(ctx context.Context, entityType string, id int64, jobs []writequeue.BatchJob) {
	h.enqueuePropagation(ctx, fmt.Sprintf("rename-%s-%d-%d", entityType, id, time.Now().UnixNano()), jobs)
}

// enqueuePropagation enqueues a merge's or rename's jobs under one snapshot batch.
// Failures are logged, not fatal: the identity change already committed.
func (h *Handlers) enqueuePropagation(ctx context.Context, batchID string, jobs []writequeue.BatchJob) {
	if _, err := h.writeQueue.EnqueueMany(ctx, jobs, batchID); err != nil {
		h.log.Warn("writeback propagation enqueue failed", "batch_id", batchID, "videos", len(jobs), "err", err)
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
