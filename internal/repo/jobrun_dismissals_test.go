package repo_test

import (
	"context"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// recordRun is the one seeding helper the dismissal tests share: a run of the
// given kind/status `ago` before now, returning its id via the history read
// (RecordJobRun does not return the id — the audit table has never needed to).
func recordRun(t *testing.T, r *repo.Repo, kind, status string, ago time.Duration) int64 {
	t.Helper()
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Second).Add(-ago)
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: kind, Trigger: model.TriggerManual, Status: status,
		StartedAt: at, FinishedAt: at, ErrorMessage: status,
	}); err != nil {
		t.Fatalf("record %s/%s: %v", kind, status, err)
	}
	runs, err := r.ListJobRuns(ctx, 30)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, run := range runs {
		if run.Kind == kind && run.StartedAt.Equal(at) {
			return run.ID
		}
	}
	t.Fatalf("recorded run %s@%s not found", kind, at)
	return 0
}

func kindDigest(t *testing.T, d repo.JobRunDigest, kind string) repo.JobKindDigest {
	t.Helper()
	for _, k := range d.Kinds {
		if k.Kind == kind {
			return k
		}
	}
	t.Fatalf("kind %q missing from digest %+v", kind, d.Kinds)
	return repo.JobKindDigest{}
}

// TestDismissJobRun_DigestAndHistory is ADR-100 D1 + D3 end to end at the repo:
// a dismissed run leaves the digest's failure list and error count but not its
// run count or the history; last_status stays the newest run's truth and
// last_dismissed flags it (handoff D5); a later failure of the same kind is a
// new, undismissed run and surfaces on its own (the owner's "I only care about
// failures after dismissal").
func TestDismissJobRun_DigestAndHistory(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	oldErr := recordRun(t, r, model.JobKindWriteback, model.JobStatusErr, 3*time.Hour)
	newErr := recordRun(t, r, model.JobKindWriteback, model.JobStatusErr, 2*time.Hour)
	okRun := recordRun(t, r, model.JobKindScan, model.JobStatusOK, time.Hour)

	// Dismiss the kind's newest failure.
	if got, err := r.DismissJobRun(ctx, newErr); err != nil || !got {
		t.Fatalf("dismiss newest = %v/%v, want true", got, err)
	}
	// Double dismiss, a non-error run and an absent id are all the same no-op.
	for name, id := range map[string]int64{"again": newErr, "ok run": okRun, "absent": 999999} {
		if got, err := r.DismissJobRun(ctx, id); err != nil || got {
			t.Errorf("dismiss %s = %v/%v, want false, nil", name, got, err)
		}
	}

	d, err := r.JobRunDigest(ctx, 30)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	wb := kindDigest(t, d, model.JobKindWriteback)
	if wb.Runs != 2 || wb.Errors != 1 {
		t.Errorf("writeback runs/errors = %d/%d, want 2/1 (dismissing is triage, not erasure)", wb.Runs, wb.Errors)
	}
	if wb.LastStatus != model.JobStatusErr || !wb.LastDismissed {
		t.Errorf("writeback last_status/last_dismissed = %q/%v, want error/true (D5: newest run is the dismissed error)", wb.LastStatus, wb.LastDismissed)
	}
	if len(d.Failures) != 1 || d.Failures[0].ID != oldErr {
		t.Errorf("failures = %+v, want only the older, undismissed run %d", d.Failures, oldErr)
	}
	if sc := kindDigest(t, d, model.JobKindScan); sc.LastDismissed {
		t.Errorf("scan last_dismissed = true, want false — its newest run is ok")
	}

	// History keeps every run and stamps the dismissed one.
	runs, err := r.ListJobRuns(ctx, 30)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("history runs = %d, want 3 (a dismissal never removes a run)", len(runs))
	}
	for _, run := range runs {
		if (run.ID == newErr) != (run.DismissedAt != nil) {
			t.Errorf("run %d dismissed_at = %v, want set only on %d", run.ID, run.DismissedAt, newErr)
		}
	}

	// An hour later the kind fails again: a new undismissed run, so the badge
	// goes back to warn and the failure surfaces.
	fresh := recordRun(t, r, model.JobKindWriteback, model.JobStatusErr, time.Minute)
	d, err = r.JobRunDigest(ctx, 30)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	wb = kindDigest(t, d, model.JobKindWriteback)
	if wb.Errors != 2 || wb.LastStatus != model.JobStatusErr || wb.LastDismissed {
		t.Errorf("after fresh failure: errors/last_status/last_dismissed = %d/%q/%v, want 2/error/false", wb.Errors, wb.LastStatus, wb.LastDismissed)
	}
	if len(d.Failures) != 2 || d.Failures[0].ID != fresh {
		t.Errorf("failures after fresh failure = %+v, want [%d, %d]", d.Failures, fresh, oldErr)
	}
}

// TestDismissJobFailures_WindowAtRequestTime is ADR-100 D4: the bulk dismiss
// covers every undismissed error in the window from the rows that exist when it
// runs — an already-dismissed run is not counted twice, an ok run is never
// touched, and a failure recorded afterwards is not swallowed.
func TestDismissJobFailures_WindowAtRequestTime(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	a := recordRun(t, r, model.JobKindWriteback, model.JobStatusErr, 3*time.Hour)
	recordRun(t, r, model.JobKindEnrich, model.JobStatusErr, 2*time.Hour)
	recordRun(t, r, model.JobKindScan, model.JobStatusOK, time.Hour)
	if _, err := r.DismissJobRun(ctx, a); err != nil {
		t.Fatalf("pre-dismiss: %v", err)
	}

	n, err := r.DismissJobFailures(ctx, 30)
	if err != nil || n != 1 {
		t.Fatalf("dismiss all = %d/%v, want 1 (the enrich error; %d was already dismissed)", n, err, a)
	}
	if n, err := r.DismissJobFailures(ctx, 30); err != nil || n != 0 {
		t.Errorf("second dismiss all = %d/%v, want 0", n, err)
	}

	d, err := r.JobRunDigest(ctx, 30)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if len(d.Failures) != 0 {
		t.Errorf("failures after dismiss all = %+v, want none", d.Failures)
	}
	for _, k := range d.Kinds {
		if k.Errors != 0 {
			t.Errorf("%s errors = %d after dismiss all, want 0", k.Kind, k.Errors)
		}
	}

	// A failure that lands after the call was never in its SELECT.
	late := recordRun(t, r, model.JobKindEnrich, model.JobStatusErr, 0)
	d, err = r.JobRunDigest(ctx, 30)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if len(d.Failures) != 1 || d.Failures[0].ID != late || d.Failures[0].DismissedAt != nil {
		t.Errorf("late failure = %+v, want %d undismissed", d.Failures, late)
	}
}

// TestPruneJobRuns_CascadesDismissal is ADR-100 D2: the retention sweep is a
// one-line DELETE on job_runs and the dismissal goes with the run through the
// foreign key — which also pins that foreign_keys(ON) is on the test DSN, since
// the cascade does not exist without it.
func TestPruneJobRuns_CascadesDismissal(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()

	id := recordRun(t, r, model.JobKindScan, model.JobStatusErr, time.Hour)
	if got, err := r.DismissJobRun(ctx, id); err != nil || !got {
		t.Fatalf("dismiss = %v/%v", got, err)
	}
	// Age the run past the window — RecordJobRun would have pruned it on insert,
	// so back-date it in place to stage the startup-sweep case.
	old := time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339)
	if _, err := database.ExecContext(ctx, `UPDATE job_runs SET started_at = ? WHERE id = ?`, old, id); err != nil {
		t.Fatalf("age run: %v", err)
	}

	if n, err := r.PruneJobRuns(ctx); err != nil || n != 1 {
		t.Fatalf("prune = %d/%v, want 1", n, err)
	}
	var dismissals int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM job_run_dismissals`).Scan(&dismissals); err != nil {
		t.Fatalf("count dismissals: %v", err)
	}
	if dismissals != 0 {
		t.Errorf("dismissals after prune = %d, want 0 — a dismissal must not outlive its run", dismissals)
	}
}
