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

// PostWriteFunc is an optional best-effort hook run after a successful write — used
// to re-extract embedded cover art (a poster just written). Nil disables it.
type PostWriteFunc func(ctx context.Context, videoID int64, path string)

// JobField is one curated, write-enabled canonical field in a queued job's payload.
// Tag names are intentionally NOT stored — they are re-resolved from the file's
// container at processing time so a stale/forged tag can't be replayed (security C2).
type JobField struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
	Source string   `json:"source"`
}

// Queue is the durable writeback worker.
type Queue struct {
	repo        *repo.Repo
	write       WriteFunc
	postWrite   PostWriteFunc
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
// the curated, write-enabled canonical fields (sanitized by the caller).
func (q *Queue) Enqueue(ctx context.Context, videoID int64, fields []JobField) (int64, error) {
	payload, err := json.Marshal(fields)
	if err != nil {
		return 0, fmt.Errorf("marshal writeback payload: %w", err)
	}
	id, err := q.repo.EnqueueWriteback(ctx, videoID, string(payload))
	if err != nil {
		return 0, err
	}
	q.kick()
	return id, nil
}

// Depth returns the number of queued + in-flight jobs (for the SPA's position hint).
func (q *Queue) Depth(ctx context.Context) (int, error) { return q.repo.PendingWritebackCount(ctx) }

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
	finish := func(ok bool, errMsg string, written int, detail string) {
		run := model.JobRun{
			Kind: model.JobKindWriteback, Trigger: model.TriggerManual,
			Status: model.JobStatusOK, StartedAt: started, FinishedAt: time.Now(),
			DurationMs: time.Since(started).Milliseconds(), Updated: written, Detail: detail,
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
	if len(mapped) == 0 {
		// Nothing writable for this container — a success no-op, recorded so the
		// operator sees why (e.g. .avi with no tag mapping).
		finish(true, "", 0, detailLine(v.FilePath, 0, unmapped))
		return
	}

	batch := make([]writeback.FieldWrite, len(mapped))
	for i, m := range mapped {
		batch[i] = writeback.FieldWrite{TagName: m.TagName, Values: m.Values, IsImage: m.IsImage}
	}
	if err := q.write(ctx, v.FilePath, batch); err != nil {
		q.log.Warn("writeback batch failed", "id", job.ID, "video", job.VideoID, "err", err)
		finish(false, err.Error(), 0, detailLine(v.FilePath, 0, unmapped))
		return
	}

	// Audit rows — one per field, only on success (ADR-041 invariant).
	for _, m := range mapped {
		if auditErr := q.repo.InsertWriteback(ctx, job.VideoID, m.Field, m.TagName, strings.Join(m.Values, "\n"), m.Source); auditErr != nil {
			q.log.Warn("insert writeback audit", "id", job.ID, "field", m.Field, "err", auditErr)
		}
	}
	if q.postWrite != nil {
		q.postWrite(ctx, job.VideoID, v.FilePath)
	}
	finish(true, "", len(mapped), detailLine(v.FilePath, len(mapped), unmapped))
}

// buildBatch sanitizes values defensively and re-resolves canonical→tag for the
// container via the shared mapper the HTTP handler also uses
// (writeback.ResolveForContainer) — never trusting a stored tag (security C2).
// Unmappable fields are returned so the job_run detail can name them (per-field,
// not whole-batch — F30.4g).
func buildBatch(fields []JobField, container string) (mapped []writeback.Mapped, unmapped []string) {
	specs := make([]writeback.FieldValues, 0, len(fields))
	for _, f := range fields {
		if f.Field == "" {
			continue
		}
		if cleaned := enrich.SanitizeValues(f.Values); len(cleaned) > 0 {
			specs = append(specs, writeback.FieldValues{Field: f.Field, Values: cleaned, Source: f.Source})
		}
	}
	return writeback.ResolveForContainer(container, specs)
}

func detailLine(path string, n int, unmapped []string) string {
	base := filepath.Base(path)
	d := fmt.Sprintf("%s — %d field(s) written", base, n)
	if len(unmapped) > 0 {
		d += " · skipped (no mapping): " + strings.Join(unmapped, ", ")
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
