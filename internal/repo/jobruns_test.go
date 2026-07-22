package repo_test

import (
	"context"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

func TestJobRunsRecordAndList(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	rec := func(trig, status, msg string, started time.Time, added int) {
		t.Helper()
		if err := r.RecordJobRun(ctx, model.JobRun{
			Kind: model.JobKindScan, Trigger: trig, Status: status,
			StartedAt: started, FinishedAt: started.Add(time.Second), DurationMs: 1000,
			Seen: 10, Added: added, ErrorMessage: msg,
		}); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	rec(model.TriggerInitial, model.JobStatusOK, "", now.Add(-2*time.Minute), 5)
	rec(model.TriggerManual, model.JobStatusErr, "boom", now.Add(-time.Minute), 0)

	runs, err := r.ListJobRuns(ctx, 30)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	// Newest first.
	if runs[0].Trigger != model.TriggerManual {
		t.Errorf("first run trigger = %q, want manual", runs[0].Trigger)
	}
	if runs[0].Status != model.JobStatusErr || runs[0].ErrorMessage != "boom" {
		t.Errorf("error run not round-tripped: %+v", runs[0])
	}
	if runs[1].Added != 5 {
		t.Errorf("added = %d, want 5", runs[1].Added)
	}
}

// HasSuccessfulJobRun is the one-time-backfill marker (F38): false until a
// successful run of the kind exists; a failed run does not count (so the task can
// retry on the next boot).
func TestHasSuccessfulJobRun(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	if ok, err := r.HasSuccessfulJobRun(ctx, model.JobKindStudioBackfill); err != nil || ok {
		t.Fatalf("no runs yet: ok=%v err=%v, want false", ok, err)
	}
	// A failed run must not satisfy the marker.
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: model.JobKindStudioBackfill, Trigger: model.TriggerInitial, Status: model.JobStatusErr,
		StartedAt: now, FinishedAt: now,
	}); err != nil {
		t.Fatalf("record failed run: %v", err)
	}
	if ok, _ := r.HasSuccessfulJobRun(ctx, model.JobKindStudioBackfill); ok {
		t.Fatal("a failed run must not satisfy the marker")
	}
	// A successful run does.
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: model.JobKindStudioBackfill, Trigger: model.TriggerInitial, Status: model.JobStatusOK,
		StartedAt: now, FinishedAt: now,
	}); err != nil {
		t.Fatalf("record ok run: %v", err)
	}
	if ok, _ := r.HasSuccessfulJobRun(ctx, model.JobKindStudioBackfill); !ok {
		t.Fatal("a successful run should satisfy the marker")
	}
	// Scoped by kind — an unrelated successful scan doesn't leak.
	if ok, _ := r.HasSuccessfulJobRun(ctx, "some-other-kind"); ok {
		t.Fatal("marker must be scoped by kind")
	}
}

func TestJobRunsRetention(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	// A run older than the 30-day window is pruned by the insert's own sweep.
	old := time.Now().UTC().AddDate(0, 0, -40)
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: model.JobKindScan, Trigger: model.TriggerPeriodic, Status: model.JobStatusOK,
		StartedAt: old, FinishedAt: old,
	}); err != nil {
		t.Fatalf("record old: %v", err)
	}
	// A recent run survives.
	nowt := time.Now().UTC()
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: model.JobKindScan, Trigger: model.TriggerPeriodic, Status: model.JobStatusOK,
		StartedAt: nowt, FinishedAt: nowt,
	}); err != nil {
		t.Fatalf("record recent: %v", err)
	}

	runs, err := r.ListJobRuns(ctx, 30)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs after prune = %d, want 1 (old run pruned)", len(runs))
	}

	// Standalone prune (startup path) is callable and a no-op here.
	if _, err := r.PruneJobRuns(ctx); err != nil {
		t.Fatalf("prune: %v", err)
	}
}

// Attribution columns (ADR-071, migration 0028) round-trip, and a library-wide
// kind that names no entity stays zero-valued rather than needing a sentinel.
func TestJobRunsAttributionRoundTrip(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// A writeback run naming its video and its snapshot batch. The batch id is
	// deliberately non-numeric: a merge propagates under "merge-person-N-M"
	// (api.mergeBatchID), which is exactly what the retired detail-line regex
	// `/· batch (\d+)/` could not match, leaving Revert unavailable for the one
	// case shared batches exist for.
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: model.JobKindWriteback, Trigger: model.TriggerManual, Status: model.JobStatusOK,
		StartedAt: now, FinishedAt: now,
		EntityType: model.EnrichEntityVideo, EntityID: 412, BatchID: "merge-person-7-9",
	}); err != nil {
		t.Fatalf("record writeback: %v", err)
	}
	// A scan is library-wide — nothing to attribute.
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: model.JobKindScan, Trigger: model.TriggerInitial, Status: model.JobStatusOK,
		StartedAt: now.Add(-time.Minute), FinishedAt: now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("record scan: %v", err)
	}

	runs, err := r.ListJobRuns(ctx, 30)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	wb, scan := runs[0], runs[1]
	if wb.Kind != model.JobKindWriteback {
		t.Fatalf("newest run = %q, want writeback", wb.Kind)
	}
	if wb.EntityType != model.EnrichEntityVideo || wb.EntityID != 412 {
		t.Errorf("attribution = %q/#%d, want video/#412", wb.EntityType, wb.EntityID)
	}
	if wb.BatchID != "merge-person-7-9" {
		t.Errorf("batch id = %q, want the non-numeric merge batch preserved verbatim", wb.BatchID)
	}
	if scan.EntityType != "" || scan.EntityID != 0 || scan.BatchID != "" {
		t.Errorf("a library-wide run must stay unattributed, got %q/#%d/%q",
			scan.EntityType, scan.EntityID, scan.BatchID)
	}
}

// The digest (ADR-071 D3) rolls the window up per kind + lists failures, and its
// shape must not grow with the number of runs — that is the whole reason it
// exists. It also has to report each kind's *most recent* status, not an
// arbitrary one, and omit failures entirely when the window is clean.
func TestJobRunDigest(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Second)

	rec := func(kind, status string, ago time.Duration) {
		t.Helper()
		at := base.Add(-ago)
		if err := r.RecordJobRun(ctx, model.JobRun{
			Kind: kind, Trigger: model.TriggerManual, Status: status,
			StartedAt: at, FinishedAt: at,
		}); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	// scan: many successes, and its *newest* run succeeded.
	for i := 0; i < 20; i++ {
		rec(model.JobKindScan, model.JobStatusOK, time.Duration(i+2)*time.Minute)
	}
	// enrich: an older success, then a more recent failure — so last_status must
	// read "error" even though a success also exists in the window.
	rec(model.JobKindEnrich, model.JobStatusOK, 5*time.Minute)
	rec(model.JobKindEnrich, model.JobStatusErr, 1*time.Minute)

	d, err := r.JobRunDigest(ctx, 30)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	byKind := map[string]repo.JobKindDigest{}
	for _, k := range d.Kinds {
		byKind[k.Kind] = k
	}
	if got := byKind[model.JobKindScan]; got.Runs != 20 || got.Errors != 0 || got.LastStatus != model.JobStatusOK {
		t.Errorf("scan digest = %+v, want 20 runs / 0 errors / last success", got)
	}
	if got := byKind[model.JobKindEnrich]; got.Runs != 2 || got.Errors != 1 || got.LastStatus != model.JobStatusErr {
		t.Errorf("enrich digest = %+v, want 2 runs / 1 error / last error (the newest run failed)", got)
	}
	// enrich is the most-recently-active kind, so it sorts first.
	if len(d.Kinds) == 0 || d.Kinds[0].Kind != model.JobKindEnrich {
		t.Errorf("kinds[0] = %v, want enrich (most recent last_run)", d.Kinds)
	}
	if len(d.Failures) != 1 || d.Failures[0].Kind != model.JobKindEnrich {
		t.Errorf("failures = %+v, want the one enrich failure", d.Failures)
	}

	// The shape must be identical after piling on far more runs — the invariant
	// the whole digest exists for. Add 500 more clean scans; kinds stay 2 and
	// failures stay 1.
	for i := 0; i < 500; i++ {
		rec(model.JobKindScan, model.JobStatusOK, time.Duration(i+30)*time.Second)
	}
	d2, err := r.JobRunDigest(ctx, 30)
	if err != nil {
		t.Fatalf("digest 2: %v", err)
	}
	if len(d2.Kinds) != len(d.Kinds) || len(d2.Failures) != len(d.Failures) {
		t.Fatalf("digest grew with run count: kinds %d→%d, failures %d→%d (must be flat)",
			len(d.Kinds), len(d2.Kinds), len(d.Failures), len(d2.Failures))
	}
}

// A window with no failures carries an empty (non-nil) failures list, so the UI
// reads "clean" as clean rather than as a missing field.
func TestJobRunDigestCleanWindow(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind: model.JobKindScan, Trigger: model.TriggerInitial, Status: model.JobStatusOK,
		StartedAt: now, FinishedAt: now,
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	d, err := r.JobRunDigest(ctx, 30)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if d.Failures == nil || len(d.Failures) != 0 {
		t.Errorf("clean window failures = %v, want an empty non-nil slice", d.Failures)
	}
}

func TestLibraryCounts(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	_, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Alice", "Bob"}, []string{"x"}), nil)
	if err != nil {
		t.Fatalf("upsert a: %v", err)
	}
	b, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Alice"}, []string{"y"}), nil)
	if err != nil {
		t.Fatalf("upsert b: %v", err)
	}
	// Deactivate a (keep only b), so it counts as inactive and its Bob/x drop out.
	if _, err := r.DeactivateExcept(ctx, []int64{b}); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	c, err := r.LibraryCounts(ctx)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	want := repo.LibraryCounts{VideosActive: 1, VideosInactive: 1, People: 1, Tags: 1}
	if c != want {
		t.Errorf("counts = %+v, want %+v", c, want)
	}
}
