package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"holodex/internal/model"
)

// jobRunRetentionDays bounds the activity history (F21.3, ADR-028). Older runs
// are pruned on every insert and once at startup; the window is fixed (no config
// in v1) and also caps the ?days= history query.
const jobRunRetentionDays = 30

// RecordJobRun durably appends one completed job pass and prunes anything past
// the retention window, both under the write lock so a burst of scans can't race
// the prune. Best-effort by contract: the scanner logs and continues on error,
// so recording never aborts a scan (ADR-019 NFR).
func (r *Repo) RecordJobRun(ctx context.Context, run model.JobRun) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO job_runs (kind, trigger, status, started_at, finished_at,
		                      duration_ms, seen, added, updated, removed, skipped,
		                      errors, error_message, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.Kind, run.Trigger, run.Status,
		run.StartedAt.UTC().Format(timeLayout), run.FinishedAt.UTC().Format(timeLayout),
		run.DurationMs, run.Seen, run.Added, run.Updated, run.Removed, run.Skipped,
		run.Errors, run.ErrorMessage, run.Detail,
	); err != nil {
		return fmt.Errorf("record job run: %w", err)
	}
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM job_runs WHERE started_at < ?`, jobRunCutoff()); err != nil {
		return fmt.Errorf("prune job runs: %w", err)
	}
	return nil
}

// HasSuccessfulJobRun reports whether a successful job run of the given kind exists
// in the history — a lightweight persistent marker for gating a one-time startup
// task (F38 studio backfill). NOTE: job history is pruned after the retention
// window, so this is a "ran recently" signal, not a permanent one; callers that
// also have a cheaper positive signal (e.g. rows already exist) should check that
// first, using this only for the edge case where the task legitimately produced no
// rows (a library where nothing resolves to a studio).
func (r *Repo) HasSuccessfulJobRun(ctx context.Context, kind string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM job_runs WHERE kind = ? AND status = ? LIMIT 1`,
		kind, model.JobStatusOK).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("has job run %q: %w", kind, err)
	}
	return true, nil
}

// PruneJobRuns removes runs older than the retention window, returning the count
// deleted. Called once at startup so a long-idle instance trims history even
// before its first new scan records (and prunes) one.
func (r *Repo) PruneJobRuns(ctx context.Context) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `DELETE FROM job_runs WHERE started_at < ?`, jobRunCutoff())
	if err != nil {
		return 0, fmt.Errorf("prune job runs: %w", err)
	}
	return res.RowsAffected()
}

// ListJobRuns returns runs within the last `days` (clamped to the retention
// window), newest first (F21.3). days <= 0 defaults to the full window.
func (r *Repo) ListJobRuns(ctx context.Context, days int) ([]model.JobRun, error) {
	if days <= 0 || days > jobRunRetentionDays {
		days = jobRunRetentionDays
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(timeLayout)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, kind, trigger, status, started_at, finished_at, duration_ms,
		       seen, added, updated, removed, skipped, errors, error_message, detail
		FROM job_runs WHERE started_at >= ?
		ORDER BY started_at DESC, id DESC`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("list job runs: %w", err)
	}
	defer rows.Close()

	var out []model.JobRun
	for rows.Next() {
		var (
			j          model.JobRun
			startedStr string
			finStr     string
		)
		if err := rows.Scan(&j.ID, &j.Kind, &j.Trigger, &j.Status, &startedStr, &finStr,
			&j.DurationMs, &j.Seen, &j.Added, &j.Updated, &j.Removed, &j.Skipped,
			&j.Errors, &j.ErrorMessage, &j.Detail); err != nil {
			return nil, err
		}
		j.StartedAt, _ = time.Parse(timeLayout, startedStr)
		j.FinishedAt, _ = time.Parse(timeLayout, finStr)
		out = append(out, j)
	}
	return out, rows.Err()
}

func jobRunCutoff() string {
	return time.Now().UTC().AddDate(0, 0, -jobRunRetentionDays).Format(timeLayout)
}

// LibraryCounts is the catalog-size snapshot for the activity surface (F21.1).
// People/Tags count only entities still linked to an active video, matching what
// the /people and /tags pages show (orphan rows linger after re-index).
type LibraryCounts struct {
	VideosActive   int `json:"videos_active"`
	VideosInactive int `json:"videos_inactive"`
	People         int `json:"people"`
	Tags           int `json:"tags"`
}

// LibraryCounts returns the catalog totals in a single round-trip. Cheap at
// personal scale; the API serves it behind the facet cache seam if it ever isn't.
func (r *Repo) LibraryCounts(ctx context.Context) (LibraryCounts, error) {
	var c LibraryCounts
	err := r.db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM videos WHERE active = 1 AND deleted_at IS NULL),
		  (SELECT COUNT(*) FROM videos WHERE active = 0 AND deleted_at IS NULL),
		  (SELECT COUNT(DISTINCT vp.person_id) FROM video_people vp
		     JOIN videos v ON v.id = vp.video_id AND v.active = 1 AND v.deleted_at IS NULL),
		  (SELECT COUNT(DISTINCT vt.tag_id) FROM video_tags vt
		     JOIN videos v ON v.id = vt.video_id AND v.active = 1 AND v.deleted_at IS NULL)`).
		Scan(&c.VideosActive, &c.VideosInactive, &c.People, &c.Tags)
	if err != nil {
		return LibraryCounts{}, fmt.Errorf("library counts: %w", err)
	}
	return c, nil
}
