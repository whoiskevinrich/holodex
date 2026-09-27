// Package writequeue is the durable, bounded-concurrency batch-writeback worker
// (F30, ADR-048). Owner "write to file" actions are enqueued (one job per file) and
// drained by a small worker pool — default 1 (WRITEBACK_CONCURRENCY) so bulk writes
// don't thrash the filesystem. The queue is durable: it survives restart, recovers
// crash-interrupted jobs (the original file is intact per ADR-041's
// copy→write→rename), sweeps orphan temp files on boot, and records each write in
// job_runs (kind=writeback) plus per-field file_writebacks audit rows.
package writequeue

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writeback"
)

// WriteFunc embeds a batch of tags into one file in a single tool pass (ADR-041).
// Production wires internal/writeback.WriteBatch.
type WriteFunc func(ctx context.Context, path string, fields []writeback.FieldWrite) error

// PostWriteFunc is an optional best-effort hook run after a successful write —
// used to re-extract embedded cover art (a poster just written) and to re-read
// the file so the DB's copy of its tags matches what was written: that copy is
// the baseline `in_sync` is computed against (HOLODEX-214), and reading it back
// is also what materializes a newly-written Person/Studio (HOLODEX-196 #4).
// Nil disables it. Deliberately not passed the job's fields — the hook runs for
// every write, and reintroducing a per-field gate is what ADR-073 undid.
type PostWriteFunc func(ctx context.Context, videoID int64, path string)

// JobField is one curated, write-enabled canonical field in a queued job's payload.
// Tag names are intentionally NOT stored — they are re-resolved from the file's
// container at processing time so a stale/forged tag can't be replayed (security C2).
type JobField struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
	Source string   `json:"source"`
}

// Write-job sources whose values are already reconciled in the DB before the
// file is touched: SourceMerge (the merge already repointed associations) and
// SourceRevert (restoring a prior on-disk value). SourceMerge is also
// referenced by internal/api's merge propagation so the string has one
// definition.
const (
	SourceMerge  = "merge"
	SourceRevert = "revert"
)

// genresField is the canonical field tags reach the file through; a job that
// writes it also filters the file's other tag keys (ADR-110).
const genresField = "genres"

// Queue is the durable writeback worker.
type Queue struct {
	repo        *repo.Repo
	write       WriteFunc
	postWrite   PostWriteFunc
	readTagKeys func(ctx context.Context, path, container string) (map[string][]string, error)
	readCurrent func(ctx context.Context, path string, mapped []writeback.Mapped) (map[string]string, error)
	log         *slog.Logger
	mediaRoot   string
	concurrency int
	pollEvery   time.Duration
	notify      chan struct{}
}

// New builds a queue. concurrency < 1 is clamped to 1. mediaRoot (may be "") bounds
// the boot-time orphan-temp sweep.
func New(r *repo.Repo, write WriteFunc, log *slog.Logger, concurrency int, mediaRoot string) *Queue {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Queue{
		repo:        r,
		write:       write,
		readTagKeys: writeback.ReadTagKeys,
		readCurrent: writeback.ReadCurrentValues,
		log:         log,
		mediaRoot:   mediaRoot,
		concurrency: concurrency,
		pollEvery:   3 * time.Second,
		notify:      make(chan struct{}, 1),
	}
}

// SetPostWrite wires the optional post-write hook (e.g. cover-art re-extract).
func (q *Queue) SetPostWrite(fn PostWriteFunc) { q.postWrite = fn }

// Enqueue persists a batch-write job for one video and wakes a worker. fields are
// the curated, write-enabled canonical fields (sanitized by the caller). Equivalent
// to EnqueueBatch with no shared batch id.
func (q *Queue) Enqueue(ctx context.Context, videoID int64, fields []JobField) (int64, error) {
	return q.EnqueueBatch(ctx, videoID, fields, "")
}

// EnqueueBatch is Enqueue with a caller-supplied batch id: jobs that share a
// batchID also share one snapshot batch, so a single Revert restores every
// one of them (F48.8, ADR-067, migration 0027). batchID == "" behaves
// exactly like Enqueue (the job's own id becomes its snapshot batch at write
// time). For enqueuing several videos' jobs under one shared batch at once,
// see EnqueueMany — it does the same thing in a single transaction instead
// of one call per video.
func (q *Queue) EnqueueBatch(ctx context.Context, videoID int64, fields []JobField, batchID string) (int64, error) {
	payload, err := json.Marshal(fields)
	if err != nil {
		return 0, fmt.Errorf("marshal writeback payload: %w", err)
	}
	id, err := q.repo.EnqueueWriteback(ctx, videoID, string(payload), batchID)
	if err != nil {
		return 0, err
	}
	q.kick()
	return id, nil
}

// BatchJob is one video's fields for EnqueueMany.
type BatchJob struct {
	VideoID int64
	Fields  []JobField
}

// EnqueueMany persists several videos' jobs under one shared batchID in a
// single transaction and wakes a worker once (F48.8): the multi-video
// counterpart to EnqueueBatch, for a caller — merge propagation — that
// already has the full list in hand and would otherwise pay one writeMu
// acquisition and one commit per video on the owner-facing request path.
// Jobs with no fields are skipped; a nil/empty result (no jobs to enqueue)
// is a no-op success. Returns the new jobs' ids.
func (q *Queue) EnqueueMany(ctx context.Context, jobs []BatchJob, batchID string) ([]int64, error) {
	inserts := make([]repo.WritebackJobInsert, 0, len(jobs))
	for _, j := range jobs {
		if len(j.Fields) == 0 {
			continue
		}
		payload, err := json.Marshal(j.Fields)
		if err != nil {
			return nil, fmt.Errorf("marshal writeback payload: %w", err)
		}
		inserts = append(inserts, repo.WritebackJobInsert{VideoID: j.VideoID, Payload: string(payload), BatchID: batchID})
	}
	if len(inserts) == 0 {
		return nil, nil
	}
	ids, err := q.repo.EnqueueWritebackBatch(ctx, inserts)
	if err != nil {
		return nil, err
	}
	q.kick()
	return ids, nil
}

// Depth returns the number of queued + in-flight jobs (for the SPA's position hint).
func (q *Queue) Depth(ctx context.Context) (int, error) { return q.repo.PendingWritebackCount(ctx) }

// RetryFailed resets a video's failed writeback job(s) back to pending and wakes a
// worker (spec R3.3) — the reset plus the same kick() Enqueue uses. A safe no-op
// (0, nil) when the video has nothing failed.
func (q *Queue) RetryFailed(ctx context.Context, videoID int64) (int64, error) {
	n, err := q.repo.RetryFailedWriteback(ctx, videoID)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		q.kick()
	}
	return n, nil
}

// Start performs boot recovery, then runs the worker pool until ctx is cancelled.
// Non-blocking: spawns goroutines and returns.
func (q *Queue) Start(ctx context.Context) {
	if n, err := q.repo.RecoverRunningWritebacks(ctx); err != nil {
		q.log.Warn("recover running writebacks", "err", err)
	} else if n > 0 {
		q.log.Info("recovered interrupted writebacks", "count", n)
	}
	q.sweepOrphans()
	for i := 0; i < q.concurrency; i++ {
		go q.worker(ctx)
	}
}

func (q *Queue) kick() {
	select {
	case q.notify <- struct{}{}:
	default: // a wake-up is already pending
	}
}

func (q *Queue) worker(ctx context.Context) {
	for {
		job, err := q.repo.ClaimNextWriteback(ctx)
		if err != nil {
			q.log.Warn("claim writeback", "err", err)
		}
		if job != nil {
			q.process(ctx, job)
			continue // drain greedily while work remains
		}
		// Queue empty — wait for a kick or the poll fallback (covers missed wakeups).
		select {
		case <-ctx.Done():
			return
		case <-q.notify:
		case <-time.After(q.pollEvery):
		}
	}
}

// process executes one job: re-resolve tag names from the current container, write
// in one pass, record audit + job_run, and finish (delete on success, mark failed
// otherwise). The original file is never left half-written (ADR-041).
func (q *Queue) process(ctx context.Context, job *repo.WritebackJob) {
	started := time.Now()
	// batchID starts as the job's own caller-supplied batch id (e.g. a tag-sync
	// trigger's batch, HOLODEX-239/ADR-077 D3) so every job_runs row this job
	// produces -- including an early-return no-op/failure that never reaches a
	// snapshot -- is attributable to its batch (ADR-071); GetWritebackBatchStatus
	// aggregates job_runs by this column. snapshotBeforeWrite below still
	// reassigns it once a write is actually attempted, to the same value (or an
	// id-derived fallback when job.BatchID is "").
	batchID := job.BatchID
	finish := func(ok bool, errMsg string, written int, detail string) {
		run := model.JobRun{
			Kind: model.JobKindWriteback, Trigger: model.TriggerManual,
			Status: model.JobStatusOK, StartedAt: started, FinishedAt: time.Now(),
			DurationMs: time.Since(started).Milliseconds(), Updated: written, Detail: detail,
			EntityType: model.EnrichEntityVideo, EntityID: job.VideoID, BatchID: batchID,
		}
		if !ok {
			run.Status = model.JobStatusErr
			run.Errors = 1
			run.ErrorMessage = errMsg
		}
		if err := q.repo.RecordJobRun(ctx, run); err != nil {
			q.log.Warn("record writeback job run", "id", job.ID, "err", err)
		}
		if err := q.repo.FinishWriteback(ctx, job.ID, ok, errMsg); err != nil {
			q.log.Warn("finish writeback", "id", job.ID, "err", err)
		}
	}

	var fields []JobField
	if err := json.Unmarshal([]byte(job.Payload), &fields); err != nil {
		finish(false, "corrupt payload", 0, "")
		return
	}

	v, _, err := q.repo.GetVideo(ctx, job.VideoID)
	if err != nil {
		finish(false, "video not found", 0, "")
		return
	}

	mapped, unmapped := buildBatch(fields, v.Container)
	writesGenres := slices.ContainsFunc(mapped, func(m writeback.Mapped) bool { return m.Field == genresField })
	// A revert restores the tag keys it snapshotted as fields of its own; filtering
	// them against the restored Genre would undo the undo (ADR-110 D3).
	if writesGenres && !slices.ContainsFunc(mapped, func(m writeback.Mapped) bool { return m.Source == SourceRevert }) {
		mapped = q.withTagKeyFilter(ctx, job, v, mapped)
	}
	if len(mapped) == 0 {
		// Nothing writable for this container — a success no-op, recorded so the
		// operator sees why (e.g. .avi with no tag mapping).
		finish(true, "", 0, detailLine(v.FilePath, 0, unmapped, ""))
		return
	}

	batch := make([]writeback.FieldWrite, len(mapped))
	for i, m := range mapped {
		batch[i] = writeback.FieldWrite{TagName: m.TagName, Values: m.Values, IsImage: m.IsImage, Delete: m.Delete}
	}

	// F48.9a (ADR-067): snapshot each field's current on-disk value before it
	// is overwritten, so a bad batch can be reverted. batchID is "" when
	// nothing was recorded — logged, not fatal (a snapshot hiccup must not
	// block a working write).
	batchID = q.snapshotBeforeWrite(ctx, job, v.FilePath, mapped)

	if err := q.write(ctx, v.FilePath, batch); err != nil {
		q.log.Warn("writeback batch failed", "id", job.ID, "video", job.VideoID, "err", err)
		finish(false, err.Error(), 0, detailLine(v.FilePath, 0, unmapped, ""))
		return
	}

	// Audit rows — one per field, only on success (ADR-041 invariant).
	for _, m := range mapped {
		if auditErr := q.repo.InsertWriteback(ctx, job.VideoID, m.Field, m.TagName, strings.Join(m.Values, "\n"), m.Source); auditErr != nil {
			q.log.Warn("insert writeback audit", "id", job.ID, "field", m.Field, "err", auditErr)
		}
	}
	// Before the post-write re-read: that re-read rescans the file's tags, and
	// an ignored tag this write just removed must already be off 'file' by then.
	if writesGenres {
		if _, err := q.repo.PromoteIgnoredFileTags(ctx, job.VideoID); err != nil {
			q.log.Warn("promote ignored file tags", "id", job.ID, "video", job.VideoID, "err", err)
		}
	}
	if q.postWrite != nil {
		q.postWrite(ctx, job.VideoID, v.FilePath)
	}
	finish(true, "", len(mapped), detailLine(v.FilePath, len(mapped), unmapped, batchID))
}

// withTagKeyFilter adds a genres job's derived tag-key writes (ADR-110 D2): every
// ExtraTagKeys instance on the file is filtered down to the values that resolve
// to the written set, so the file's tags match the UI after the write. Computed
// here, at run time, because the file can change while the job waits. It also
// drops a Genre delete when the file carries no Genre — on Matroska that write
// would be a full remux for nothing. A read failure is logged and the job writes
// Genre alone, the same non-fatal posture as the pre-write snapshot.
func (q *Queue) withTagKeyFilter(ctx context.Context, job *repo.WritebackJob, v *model.Video, mapped []writeback.Mapped) []writeback.Mapped {
	i := slices.IndexFunc(mapped, func(m writeback.Mapped) bool { return m.Field == genresField })
	genres := mapped[i]

	present, err := q.readTagKeys(ctx, v.FilePath, v.Container)
	// The names come from the media file's own metadata, via exiftool: hold them
	// to the same allowlist as a payload's tag-key fields before any is written.
	for name := range present {
		if !writeback.ValidTagKeyName(v.Container, name) {
			delete(present, name)
		}
	}
	if err == nil {
		var keep func(string) bool
		if keep, err = q.tagKeeper(ctx, genres.Values, present); err == nil {
			mapped = append(mapped, writeback.FilterTagKeys(present, keep, genres.Source)...)
		}
	}
	if err != nil {
		q.log.Warn("writeback tag-key filter skipped", "id", job.ID, "video", job.VideoID, "err", err)
	}

	if genres.Delete {
		cur, err := q.readCurrent(ctx, v.FilePath, []writeback.Mapped{genres})
		if err == nil && cur[genresField] == "" {
			mapped = slices.Delete(mapped, i, i+1)
		}
	}
	return mapped
}

// tagKeeper decides which file tag-key values survive a genres write: those that
// resolve — the way the scanner resolves a tag name (the name key, then aliases
// and merged-away names) — to a tag in the written set. Resolution happens up
// front so the returned func is a plain set lookup. The rule is
// repo.TagIdentityKeys — shared with the on-file read-back (ADR-111 D2).
func (q *Queue) tagKeeper(ctx context.Context, written []string, present map[string][]string) (func(string) bool, error) {
	names := slices.Clone(written)
	for _, values := range present {
		names = append(names, values...)
	}
	ident, err := q.repo.TagIdentityKeys(ctx, names)
	if err != nil {
		return nil, err
	}
	want := make(map[string]bool, len(written))
	for _, w := range written {
		want[ident[repo.TagNameKey(w)]] = true
	}
	return func(v string) bool {
		id, ok := ident[repo.TagNameKey(v)]
		return ok && want[id]
	}, nil
}

// snapshotBeforeWrite records each mapped field's current on-disk value under
// a batch id — job.BatchID when the caller supplied one (F48.8, ADR-067,
// migration 0027: several jobs, one per video, sharing a batch so a single
// Revert restores all of them), otherwise one derived from the job's own id
// (F48.9a) — not a fresh random id per attempt. That matters for crash
// recovery: RecoverRunningWritebacks resets a crash-interrupted 'running' job
// back to 'pending' and reprocesses it from scratch, and by then the file may
// already carry the first attempt's write (a crash between a successful
// WriteBatch and FinishWriteback). A fresh id per attempt would re-read that
// already-written value as "prior", corrupting the snapshot. Reusing a stable
// batch id instead makes this idempotent: a retry finds its own earlier
// snapshot and reuses it rather than re-deriving a wrong one — the existence
// check is scoped to (batch id, this job's video) via SnapshotExistsForVideo
// rather than the whole batch, so a shared batch id doesn't let one video's
// already-taken snapshot make a sibling video's job skip taking its own.
// Returns "" when nothing was recorded — the read or insert failed, logged
// and treated as non-fatal so a snapshot problem never blocks an otherwise-
// working write.
func (q *Queue) snapshotBeforeWrite(ctx context.Context, job *repo.WritebackJob, path string, mapped []writeback.Mapped) string {
	batchID := job.BatchID
	if batchID == "" {
		batchID = strconv.FormatInt(job.ID, 10)
	}
	if exists, err := q.repo.SnapshotExistsForVideo(ctx, batchID, job.VideoID); err != nil {
		q.log.Warn("writeback snapshot lookup failed", "id", job.ID, "video", job.VideoID, "err", err)
	} else if exists {
		return batchID // already captured on an earlier attempt for this job
	}

	prior, err := q.readCurrent(ctx, path, mapped)
	if err != nil {
		q.log.Warn("writeback snapshot read failed", "id", job.ID, "video", job.VideoID, "err", err)
		return ""
	}
	if err := q.repo.InsertWritebackSnapshots(ctx, job.VideoID, batchID, prior); err != nil {
		q.log.Warn("writeback snapshot insert failed", "id", job.ID, "video", job.VideoID, "err", err)
		return ""
	}
	return batchID
}

// Revert restores every field in a completed write batch to its pre-write
// value, one inverse writeback job per affected video (F48.9b). The revert
// enqueues through the same path as any other write, so it is itself
// snapshotted and can be re-reverted (undo-of-undo, F48.9c).
//
// A field whose prior_value is "" (absent before the original write) is
// skipped: the write path has no "clear this tag" primitive today — a
// job's values are sanitized and empty values dropped (enrich.SanitizeValues)
// — so there is nothing to revert-write for that field. This only affects a
// field the original write *added* where none existed before; the common
// "wrong value overwrote an existing one" case reverts fully.
func (q *Queue) Revert(ctx context.Context, batchID string) ([]int64, error) {
	snaps, err := q.repo.SnapshotsForBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if len(snaps) == 0 {
		return nil, fmt.Errorf("revert batch %q: %w", batchID, repo.ErrNotFound)
	}

	byVideo := make(map[int64][]JobField)
	for _, s := range snaps {
		if s.PriorValue == "" {
			continue
		}
		byVideo[s.VideoID] = append(byVideo[s.VideoID], JobField{
			Field:  s.FieldKey,
			Values: strings.Split(s.PriorValue, "\n"),
			Source: SourceRevert,
		})
	}

	jobIDs := make([]int64, 0, len(byVideo))
	for videoID, fields := range byVideo {
		id, err := q.Enqueue(ctx, videoID, fields)
		if err != nil {
			return jobIDs, fmt.Errorf("revert enqueue video %d: %w", videoID, err)
		}
		jobIDs = append(jobIDs, id)
	}
	return jobIDs, nil
}

// buildBatch sanitizes values defensively and re-resolves canonical→tag for the
// container via the shared mapper the HTTP handler also uses
// (writeback.ResolveForContainer) — never trusting a stored tag (security C2).
// Unmappable fields are returned so the job_run detail can name them (per-field,
// not whole-batch — F30.4g).
//
// Two shapes bypass the canonical mapping (ADR-110): a TagKeyFieldPrefix field
// (a derived tag-key write's revert) is written under its own tag name, but only
// when writeback.ValidTagKeyName allows it; and a genres field with no values
// deletes the Genre tag — the written set is empty, so the file holds no tags.
func buildBatch(fields []JobField, container string) (mapped []writeback.Mapped, unmapped []string) {
	specs := make([]writeback.FieldValues, 0, len(fields))
	var direct []writeback.Mapped
	for _, f := range fields {
		if f.Field == "" {
			continue
		}
		cleaned := enrich.SanitizeValues(f.Values)
		if name, ok := strings.CutPrefix(f.Field, writeback.TagKeyFieldPrefix); ok {
			if !writeback.ValidTagKeyName(container, name) {
				unmapped = append(unmapped, f.Field)
				continue
			}
			direct = append(direct, writeback.Mapped{Field: f.Field, TagName: name, Source: f.Source,
				Values: cleaned, Delete: len(cleaned) == 0})
			continue
		}
		if len(cleaned) == 0 {
			if f.Field == genresField {
				if tag, ok := writeback.TagForField(genresField, container); ok {
					direct = append(direct, writeback.Mapped{Field: f.Field, TagName: tag, Source: f.Source, Delete: true})
				}
			}
			continue
		}
		specs = append(specs, writeback.FieldValues{Field: f.Field, Values: cleaned, Source: f.Source})
	}
	mapped, resolvedUnmapped := writeback.ResolveForContainer(container, specs)
	return append(mapped, direct...), append(unmapped, resolvedUnmapped...)
}

// detailLine renders the job_runs.detail summary. batchID (F48.9d), when
// non-empty, is appended so the batch id needed for Revert is visible in
// activity history without a schema change to job_runs.
func detailLine(path string, n int, unmapped []string, batchID string) string {
	base := filepath.Base(path)
	d := fmt.Sprintf("%s — %d field(s) written", base, n)
	if len(unmapped) > 0 {
		d += " · skipped (no mapping): " + strings.Join(unmapped, ", ")
	}
	if batchID != "" {
		d += " · batch " + batchID
	}
	return d
}

// sweepOrphans removes leftover copy→write temp files from an interrupted write so a
// crash never leaves clutter beside the media (F30.4e). Best-effort; bounded to
// mediaRoot. Skipped when mediaRoot is unset.
func (q *Queue) sweepOrphans() {
	if q.mediaRoot == "" {
		return
	}
	var removed int
	_ = filepath.WalkDir(q.mediaRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // never abort the sweep on a single error
		}
		if strings.HasSuffix(path, ".holodex-tmp") || strings.HasSuffix(path, ".holodex-new") {
			if rmErr := os.Remove(path); rmErr == nil {
				removed++
			}
		}
		return nil
	})
	if removed > 0 {
		q.log.Info("swept orphan writeback temp files", "count", removed)
	}
}
