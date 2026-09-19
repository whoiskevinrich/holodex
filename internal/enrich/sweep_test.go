package enrich

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// seedPeople links names to one active video so ListPeople (the sweep's set) returns
// them, in insertion order.
func seedPeople(t *testing.T, r *repo.Repo, names ...string) {
	t.Helper()
	ctx := context.Background()
	vid, err := r.UpsertVideo(ctx, &model.Video{FilePath: "/m/sweep.mkv", Title: "Sweep"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	links := make([]repo.PersonRoleName, len(names))
	for i, n := range names {
		links[i] = repo.PersonRoleName{Name: n, Role: "actor"}
	}
	if err := r.ReconcileVideoPeople(ctx, vid, links, nil); err != nil {
		t.Fatal(err)
	}
}

// waitIdle polls Status until the sweep finishes and returns its summary.
func waitIdle(t *testing.T, sr *SweepRunner) SweepSummary {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st := sr.Status()
		if st.State == "idle" && st.LastRun != nil {
			return *st.LastRun
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("sweep did not finish: %+v", sr.Status())
	return SweepSummary{}
}

func newSweep(t *testing.T, fake ProviderClient, names ...string) (*SweepRunner, *repo.Repo) {
	t.Helper()
	svc, r := newSvc(t, NewFake("fake"))
	svc.newClient = func(Source) ProviderClient { return fake }
	seedPeople(t, r, names...)
	sr := NewSweepRunner(svc, r, slog.New(slog.NewTextHandler(io.Discard, nil)))
	sr.sleep = func(context.Context, time.Duration) error { return nil } // never wait in tests
	return sr, r
}

func TestSweepRunsEveryPersonUnderOneBatch(t *testing.T) {
	fake := NewFake("fake")
	fake.Searched = []string{"q"}
	sr, r := newSweep(t, fake, "Hayao Miyazaki", "Nobody Known")
	ctx := context.Background()

	if _, err := sr.Trigger("film", false); err == nil {
		t.Fatal("an unsupported kind must be refused")
	}
	started, err := sr.Trigger(model.EnrichEntityPerson, false)
	if err != nil || !started {
		t.Fatalf("Trigger = %v, %v; want started", started, err)
	}
	sum := waitIdle(t, sr)
	if sum.Kind != model.EnrichEntityPerson || sum.Total != 2 || sum.Linked != 1 || sum.NoCandidates != 1 || sum.Failed != 0 || sum.Error != "" {
		t.Fatalf("summary = %+v; want person/2/linked 1/no_candidates 1", sum)
	}
	if sum.BatchID == "" || sum.SkippedProviders == nil || len(sum.SkippedProviders) != 0 {
		t.Fatalf("summary batch/skipped = %+v; skipped_providers must be an empty list, not null", sum)
	}

	runs, err := r.ListJobRunsByBatch(ctx, sum.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, run := range runs {
		kinds[run.Kind]++
		if run.BatchID != sum.BatchID {
			t.Errorf("run %+v outside the batch", run)
		}
	}
	// One summary row + the auto-applied person's enrich run + two searched rows.
	if kinds[model.JobKindEnrichSweep] != 1 || kinds[model.JobKindEnrich] < 2 {
		t.Fatalf("batch runs by kind = %v; want 1 enrich-sweep + ≥2 enrich", kinds)
	}
	var summary model.JobRun
	for _, run := range runs {
		if run.Kind == model.JobKindEnrichSweep {
			summary = run
		}
	}
	if summary.Seen != 2 || summary.Updated != 1 || summary.Detail != "people · 2 · linked 1 · 1 no candidates" {
		t.Fatalf("summary run = %+v", summary)
	}

	// A second sweep without force skips the now-linked, just-fetched pair.
	if started, _ := sr.Trigger(model.EnrichEntityPerson, false); !started {
		t.Fatal("second sweep should start once the first is idle")
	}
	sum = waitIdle(t, sr)
	if sum.StaleSkipped != 1 || sum.NoCandidates != 1 {
		t.Fatalf("second sweep = %+v; want 1 stale_skipped + 1 no_candidates", sum)
	}
	if started, _ := sr.Trigger(model.EnrichEntityPerson, true); !started {
		t.Fatal("forced sweep should start")
	}
	if sum = waitIdle(t, sr); sum.Linked != 1 || sum.StaleSkipped != 0 {
		t.Fatalf("forced sweep = %+v; want the linked pair refreshed", sum)
	}
}

// gatedClient blocks every Resolve until released — to hold a sweep mid-flight.
type gatedClient struct {
	*Fake
	release chan struct{}
}

func (g *gatedClient) Resolve(ctx context.Context, entityType string, hint Hint) (ResolveResult, error) {
	select {
	case <-g.release:
	case <-ctx.Done():
		return ResolveResult{}, ctx.Err()
	}
	return g.Fake.Resolve(ctx, entityType, hint)
}

func TestSweepSingleFlightAcrossKinds(t *testing.T) {
	gate := &gatedClient{Fake: NewFake("fake"), release: make(chan struct{})}
	sr, _ := newSweep(t, gate, "Hayao Miyazaki")

	if started, _ := sr.Trigger(model.EnrichEntityPerson, false); !started {
		t.Fatal("first trigger should start")
	}
	deadline := time.Now().Add(5 * time.Second)
	for sr.Status().Total == 0 && time.Now().Before(deadline) { // listed, blocked on the gate
		time.Sleep(5 * time.Millisecond)
	}
	st := sr.Status()
	if st.State != "running" || st.Kind != model.EnrichEntityPerson || st.Total != 1 || st.Done != 0 || st.StartedAt == nil {
		t.Fatalf("running status = %+v", st)
	}
	if started, err := sr.Trigger(model.EnrichEntityStudio, false); started || err != nil {
		t.Fatalf("a studios sweep while people runs = %v, %v; want started:false, no error", started, err)
	}
	if started, _ := sr.Trigger(model.EnrichEntityPerson, false); started {
		t.Fatal("a second people sweep must not start")
	}
	close(gate.release)
	if sum := waitIdle(t, sr); sum.Total != 1 || sum.Linked != 1 {
		t.Fatalf("summary = %+v", sum)
	}
}

// failingClient makes every Resolve fail like a dead sidecar.
type failingClient struct{ *Fake }

func (f *failingClient) Resolve(context.Context, string, Hint) (ResolveResult, error) {
	return ResolveResult{}, errors.New("provider returned 502")
}

func TestSweepBreakerStopsCallingADeadProvider(t *testing.T) {
	names := []string{"A One", "B Two", "C Three", "D Four", "E Five", "F Six", "G Seven"}
	sr, _ := newSweep(t, &failingClient{Fake: NewFake("fake")}, names...)

	if started, _ := sr.Trigger(model.EnrichEntityPerson, false); !started {
		t.Fatal("trigger")
	}
	sum := waitIdle(t, sr)
	if sum.Failed != breakerFailures || sum.Skipped != len(names)-breakerFailures {
		t.Fatalf("summary = %+v; want %d failed then %d skipped", sum, breakerFailures, len(names)-breakerFailures)
	}
	if len(sum.SkippedProviders) != 1 || sum.SkippedProviders[0] != (SkippedProvider{Provider: "fake", Reason: BreakerStoppedResponding}) {
		t.Fatalf("skipped providers = %+v", sum.SkippedProviders)
	}
}

func TestSweepRetriesOnceThenBreaksOnPauses(t *testing.T) {
	fake := NewFake("fake")
	fake.RateLimited = 10 * time.Second
	names := []string{"A One", "B Two", "C Three", "D Four", "E Five"}
	sr, _ := newSweep(t, fake, names...)
	var slept []time.Duration
	sr.sleep = func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }

	if started, _ := sr.Trigger(model.EnrichEntityPerson, false); !started {
		t.Fatal("trigger")
	}
	sum := waitIdle(t, sr)
	// Each of the first three pairs: paused → wait → retry → still paused → failed,
	// one pause toward the breaker; the last two are skipped without a wait.
	if sum.Failed != breakerPauses || sum.Skipped != len(names)-breakerPauses {
		t.Fatalf("summary = %+v; want %d failed then %d skipped", sum, breakerPauses, len(names)-breakerPauses)
	}
	if len(slept) != breakerPauses {
		t.Fatalf("slept %v; want one wait per failed pair", slept)
	}
	if len(sum.SkippedProviders) != 1 || sum.SkippedProviders[0].Reason != BreakerRateLimited {
		t.Fatalf("skipped providers = %+v; want fake rate-limited", sum.SkippedProviders)
	}
}

// A provider whose entity_types excludes the kind is never called (spec P0-2): the
// sweep's provider set is the kind's supported sources, not every enabled one.
func TestSweepSkipsProvidersThatDoNotSupportTheKind(t *testing.T) {
	fake := NewFake("fake")
	svc, r := newSvc(t, fake)
	store, err := NewStore(writeSources(t, `
sources:
  - name: fake
    base_url: http://fake:9100
    entity_types: [studio]
    enabled: true
`), nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.store = store
	seedPeople(t, r, "Hayao Miyazaki")
	sr := NewSweepRunner(svc, r, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if started, _ := sr.Trigger(model.EnrichEntityPerson, true); !started {
		t.Fatal("trigger")
	}
	sum := waitIdle(t, sr)
	if sum.Total != 1 || sum.SweepCounts != (SweepCounts{}) || len(sum.SkippedProviders) != 0 {
		t.Fatalf("summary = %+v; want total 1 and every pair count 0", sum)
	}
	if fake.Calls != 0 {
		t.Fatalf("a studio-only provider must not be dialed for a people sweep (calls=%d)", fake.Calls)
	}
}
